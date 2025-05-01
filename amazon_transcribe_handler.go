package suzu

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/service/transcribestreamingservice"
	zlog "github.com/rs/zerolog/log"
)

func init() {
	NewServiceHandlerFuncs.register("awsv1", NewAmazonTranscribeHandler)
}

type AmazonTranscribeHandler struct {
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

func NewAmazonTranscribeHandler(config Config, channelID, connectionID string, sampleRate uint32, channelCount uint16, languageCode string, onResultFunc any) serviceHandlerInterface {
	return &AmazonTranscribeHandler{
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

type AwsResult struct {
	ChannelID *string `json:"channel_id,omitempty"`
	IsPartial *bool   `json:"is_partial,omitempty"`
	ResultID  *string `json:"result_id,omitempty"`
	TranscriptionResult
}

func NewAwsResult() AwsResult {
	return AwsResult{
		TranscriptionResult: TranscriptionResult{
			Type: "aws",
		},
	}
}

func (ar *AwsResult) WithChannelID(channelID string) *AwsResult {
	ar.ChannelID = &channelID
	return ar
}

func (ar *AwsResult) WithIsPartial(isPartial bool) *AwsResult {
	ar.IsPartial = &isPartial
	return ar
}

func (ar *AwsResult) WithResultID(resultID string) *AwsResult {
	ar.ResultID = &resultID
	return ar
}

func (ar *AwsResult) SetMessage(message string) *AwsResult {
	ar.Message = message
	return ar
}

func (h *AmazonTranscribeHandler) UpdateRetryCount() int {
	defer h.mu.Unlock()
	h.mu.Lock()
	h.RetryCount++
	return h.RetryCount
}

func (h *AmazonTranscribeHandler) GetRetryCount() int {
	return h.RetryCount
}

func (h *AmazonTranscribeHandler) ResetRetryCount() int {
	defer h.mu.Unlock()
	h.mu.Lock()
	h.RetryCount = 0
	return h.RetryCount
}

func (h *AmazonTranscribeHandler) IsErrorForRetryTarget(err error) bool {
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

	// 復帰が不可能なエラー以外は再接続を試みる
	switch err.(type) {
	case *transcribestreamingservice.LimitExceededException,
		*transcribestreamingservice.InternalFailureException:
		return true
	default:
		// サーバから切断された場合は再接続を試みる
		if strings.Contains(err.Error(), "http2: server sent GOAWAY and closed the connection;") {
			return true
		}
	}

	return false
}

func (h *AmazonTranscribeHandler) Handle(ctx context.Context, opusCh chan opusChannel, header soraHeader) (*io.PipeReader, error) {
	at := NewAmazonTranscribe(h.Config, h.LanguageCode, int64(h.SampleRate), int64(h.ChannelCount))

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
				case *transcribestreamingservice.TranscriptEvent:
					if h.OnResultFunc != nil {
						if err := h.OnResultFunc(ctx, w, h.ChannelID, h.ConnectionID, h.LanguageCode, e.Transcript.Results); err != nil {
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
						for _, res := range e.Transcript.Results {
							if at.Config.FinalResultOnly {
								// IsPartial: true の場合は結果を返さない
								if *res.IsPartial {
									continue
								}
							}

							result := NewAwsResult()
							if at.Config.AwsResultIsPartial {
								result.WithIsPartial(*res.IsPartial)
							}
							if at.Config.AwsResultChannelID {
								result.WithChannelID(*res.ChannelId)
							}
							if at.Config.AwsResultID {
								result.WithResultID(*res.ResultId)
							}

							for _, alt := range res.Alternatives {
								message, ok := buildMessage(at.Config, *alt, *res.IsPartial)
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

			if ok := h.IsErrorForRetryTarget(err); ok {
				err = errors.Join(err, ErrServerDisconnected)
			}

			w.CloseWithError(err)
			return
		}
		w.Close()
	}()

	return r, nil
}

func contentFilterByTranscribedTime(config Config, item transcribestreamingservice.Item) bool {
	minimumTranscribedTime := config.MinimumTranscribedTime

	// minimumTranscribedTime が設定されていない場合はフィルタリングしない
	if minimumTranscribedTime <= 0 {
		return true
	}

	// 句読点の場合はフィルタリングしない
	if *item.Type == transcribestreamingservice.ItemTypePunctuation {
		return true
	}

	// StartTime または EndTime が nil の場合はフィルタリングしない
	if (item.StartTime == nil) || (item.EndTime == nil) {
		return true
	}

	// 発話時間が minimumTranscribedTime 未満の場合はフィルタリングする
	return (*item.EndTime - *item.StartTime) >= minimumTranscribedTime
}

func contentFilterByConfidenceScore(config Config, item transcribestreamingservice.Item, isPartial bool) bool {
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
	if *item.Type == transcribestreamingservice.ItemTypePunctuation {
		return true
	}

	// Confidence が nil の場合はフィルタリングしない
	if item.Confidence == nil {
		return true
	}

	// 信頼スコアが minimumConfidenceScore 未満の場合はフィルタリングする
	return *item.Confidence >= minimumConfidenceScore
}

func buildMessage(config Config, alt transcribestreamingservice.Alternative, isPartial bool) (string, bool) {
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
		if !contentFilterByTranscribedTime(config, *item) {
			continue
		}

		if !contentFilterByConfidenceScore(config, *item, isPartial) {
			continue
		}

		if *item.Type == transcribestreamingservice.ItemTypePronunciation {
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
