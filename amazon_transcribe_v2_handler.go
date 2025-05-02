package suzu

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/transcribestreaming/types"
	zlog "github.com/rs/zerolog/log"
)

func init() {
	NewServiceHandlerFuncs.register("aws", NewAmazonTranscribeV2Handler)
	NewServiceHandlerFuncs.register("awsv2", NewAmazonTranscribeV2Handler)
}

type AmazonTranscribeV2Handler struct {
	Config Config

	ChannelID    string
	ConnectionID string
	SampleRate   uint32
	ChannelCount uint16
	LanguageCode string
	RetryCount   int
	mu           sync.Mutex

	OnResultFunc func(context.Context, io.WriteCloser, string, string, string, any) error
}

func NewAmazonTranscribeV2Handler(config Config, channelID, connectionID string, sampleRate uint32, channelCount uint16, languageCode string, onResultFunc any) serviceHandlerInterface {
	return &AmazonTranscribeV2Handler{
		Config:       config,
		ChannelID:    channelID,
		ConnectionID: connectionID,
		SampleRate:   sampleRate,
		ChannelCount: channelCount,
		LanguageCode: languageCode,
		RetryCount:   0,
		OnResultFunc: onResultFunc.(func(context.Context, io.WriteCloser, string, string, string, any) error),
	}
}

type AwsResultV2 struct {
	ChannelID *string `json:"channel_id,omitempty"`
	IsPartial *bool   `json:"is_partial,omitempty"`
	ResultID  *string `json:"result_id,omitempty"`
	TranscriptionResult
}

func NewAwsResultV2() AwsResultV2 {
	return AwsResultV2{
		TranscriptionResult: TranscriptionResult{
			Type: "aws",
		},
	}
}

func (ar *AwsResultV2) WithChannelID(channelID string) *AwsResultV2 {
	ar.ChannelID = &channelID
	return ar
}

func (ar *AwsResultV2) WithIsPartial(isPartial bool) *AwsResultV2 {
	ar.IsPartial = &isPartial
	return ar
}

func (ar *AwsResultV2) WithResultID(resultID string) *AwsResultV2 {
	ar.ResultID = &resultID
	return ar
}

func (ar *AwsResultV2) SetMessage(message string) *AwsResultV2 {
	ar.Message = message
	return ar
}

func (h *AmazonTranscribeV2Handler) UpdateRetryCount() int {
	defer h.mu.Unlock()
	h.mu.Lock()
	h.RetryCount++
	return h.RetryCount
}

func (h *AmazonTranscribeV2Handler) GetRetryCount() int {
	return h.RetryCount
}

func (h *AmazonTranscribeV2Handler) ResetRetryCount() int {
	defer h.mu.Unlock()
	h.mu.Lock()
	h.RetryCount = 0
	return h.RetryCount
}

func (h *AmazonTranscribeV2Handler) IsRetry(args any) bool {
	switch err := args.(type) {
	case error:
		// retry_targets が設定されていない場合は固定のエラー判定処理へ
		retryTargets := h.Config.RetryTargets

		// retry_targets が設定されている場合は、リトライ対象のエラーかどうかを判定する
		if retryTargets != "" {
			// retry_targets = BadRequestException,ConflictException のように指定されている想定
			retryTargetList := strings.Split(retryTargets, ",")
			// retry_targets が設定されている場合は、リトライ対象のエラーかどうかを判定する
			for _, target := range retryTargetList {
				if strings.Contains(err.Error(), target) {
					return true
				}
			}
		}

		switch err.(type) {
		case *types.LimitExceededException,
			*types.InternalFailureException:
			return true
		default:
			// サーバから切断された場合は再接続を試みる
			if strings.Contains(err.Error(), "http2: server sent GOAWAY and closed the connection;") {
				return true
			}
		}

		return false
	default:
		// 引数が error 型ではない場合はリトライしない
		return false
	}
}

func (h *AmazonTranscribeV2Handler) Handle(ctx context.Context, opusCh chan opusChannel, header soraHeader) (*io.PipeReader, error) {
	at := NewAmazonTranscribeV2(h.Config, h.LanguageCode, int64(h.SampleRate), int64(h.ChannelCount))

	packetReader, err := opus2ogg(ctx, opusCh, h.SampleRate, h.ChannelCount, h.Config, header)
	if err != nil {
		return nil, err
	}

	stream, err := at.Start(ctx, packetReader)
	if err != nil {
		return nil, err
	}

	// リクエストが成功した時点でリトライカウントをリセットする
	h.ResetRetryCount()

	r, w := io.Pipe()

	go func() {
		encoder := json.NewEncoder(w)

	L:
		for {
			select {
			case <-ctx.Done():
				break L
			case event := <-stream.Events():
				switch e := event.(type) {
				case *types.TranscriptResultStreamMemberTranscriptEvent:
					if h.OnResultFunc != nil {
						if err := h.OnResultFunc(ctx, w, h.ChannelID, h.ConnectionID, h.LanguageCode, e.Value.Transcript.Results); err != nil {
							if err := encoder.Encode(NewSuzuErrorResponse(err)); err != nil {
								zlog.Error().
									Err(err).
									Str("channel_id", h.ChannelID).
									Str("connection_id", h.ConnectionID).
									Send()
							}
							w.CloseWithError(err)
							return
						}
					} else {
						for _, res := range e.Value.Transcript.Results {
							if at.Config.FinalResultOnly {
								// IsPartial: true の場合は結果を返さない
								if res.IsPartial {
									continue
								}
							}

							result := NewAwsResult()
							if at.Config.AwsResultIsPartial {
								result.WithIsPartial(res.IsPartial)
							}
							if at.Config.AwsResultChannelID {
								result.WithChannelID(*res.ChannelId)
							}
							if at.Config.AwsResultID {
								result.WithResultID(*res.ResultId)
							}

							for _, alt := range res.Alternatives {
								message, ok := buildMessageV2(at.Config, alt, res.IsPartial)
								if !ok {
									continue
								}

								result.SetMessage(message)
								if err := encoder.Encode(result); err != nil {
									w.CloseWithError(err)
									return
								}
							}
						}
					}
				default:
					break L
				}
			}
		}

		if err := stream.Err(); err != nil {
			zlog.Error().
				Err(err).
				Str("channel_id", h.ChannelID).
				Str("connection_id", h.ConnectionID).
				Int("retry_count", h.GetRetryCount()).
				Send()

			if ok := h.IsRetry(err); ok {
				err = errors.Join(err, ErrServerDisconnected)
			}

			w.CloseWithError(err)
			return
		}
		w.Close()
	}()

	return r, nil
}

func contentFilterByTranscribedTimeV2(config Config, item types.Item) bool {
	minimumTranscribedTime := config.MinimumTranscribedTime

	// minimumTranscribedTime が設定されていない場合はフィルタリングしない
	if minimumTranscribedTime <= 0 {
		return true
	}

	// 句読点の場合はフィルタリングしない
	if item.Type == types.ItemTypePunctuation {
		return true
	}

	// 発話時間が minimumTranscribedTime 未満の場合はフィルタリングする
	return (item.EndTime - item.StartTime) >= minimumTranscribedTime
}

func contentFilterByConfidenceScoreV2(config Config, item types.Item, isPartial bool) bool {
	minimumConfidenceScore := config.MinimumConfidenceScore

	// minimumConfidenceScore が設定されていない場合はフィルタリングしない
	if minimumConfidenceScore <= 0 {
		return true
	}

	// isPartial が true の場合はフィルタリングしない
	if isPartial {
		return true
	}

	// 句読点の場合はフィルタリングしない
	if item.Type == types.ItemTypePunctuation {
		return true
	}

	// Confidence が nil の場合はフィルタリングしない
	if item.Confidence == nil {
		return true
	}

	// 信頼スコアが minimumConfidenceScore 未満の場合はフィルタリングする
	return *item.Confidence >= minimumConfidenceScore
}

func buildMessageV2(config Config, alt types.Alternative, isPartial bool) (string, bool) {
	var message string

	minimumTranscribedTime := config.MinimumTranscribedTime
	minimumConfidenceScore := config.MinimumConfidenceScore

	// 両方無効の場合には全てのメッセージを返す
	if (minimumTranscribedTime <= 0) && (minimumConfidenceScore <= 0) {
		return *alt.Transcript, true
	}

	items := alt.Items

	includePronunciation := false

	for _, item := range items {
		if !contentFilterByTranscribedTimeV2(config, item) {
			continue
		}

		if !contentFilterByConfidenceScoreV2(config, item, isPartial) {
			continue
		}

		if item.Type == types.ItemTypePronunciation {
			includePronunciation = true
		}

		message += *item.Content
	}

	// 各評価の結果、句読点のみかメッセージが空の場合は次へ
	if !includePronunciation || (message == "") {
		return "", false
	}

	return message, true
}
