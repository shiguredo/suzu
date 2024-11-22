package suzu

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"

	"github.com/aws/aws-sdk-go/service/transcribestreamingservice"
	zlog "github.com/rs/zerolog/log"
)

func init() {
	NewServiceHandlerFuncs.register("aws", NewAmazonTranscribeHandler)
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

func (h *AmazonTranscribeHandler) Handle(ctx context.Context, reader io.Reader) (*io.PipeReader, error) {
	at := NewAmazonTranscribe(h.Config, h.LanguageCode, int64(h.SampleRate), int64(h.ChannelCount))

	oggReader, oggWriter := io.Pipe()
	go func() {
		defer oggWriter.Close()
		if err := opus2ogg(ctx, reader, oggWriter, h.SampleRate, h.ChannelCount, h.Config); err != nil {
			if !errors.Is(err, io.EOF) {
				zlog.Error().
					Err(err).
					Str("channel_id", h.ChannelID).
					Str("connection_id", h.ConnectionID).
					Send()
			}

			oggWriter.CloseWithError(err)
			return
		}
	}()

	stream, err := at.Start(ctx, oggReader)
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
								var message string
								if alt.Transcript != nil {
									message = *alt.Transcript
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

			// 復帰が不可能なエラー以外は再接続を試みる
			switch err.(type) {
			case *transcribestreamingservice.LimitExceededException,
				*transcribestreamingservice.InternalFailureException:
				// TODO: 元の err を送信する
				err = ErrServerDisconnected
			default:
			}

			w.CloseWithError(err)
			return
		}

		w.Close()
	}()

	return r, nil
}
