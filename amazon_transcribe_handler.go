package suzu

import (
	"context"
	"encoding/json"
	"io"

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

	OnResultFunc func(context.Context, json.Encoder, string, string, string, any) error
}

func NewAmazonTranscribeHandler(config Config, channelID, connectionID string, sampleRate uint32, channelCount uint16, languageCode string, onResultFunc any) serviceHandlerInterface {
	return &AmazonTranscribeHandler{
		Config:       config,
		ChannelID:    channelID,
		ConnectionID: connectionID,
		SampleRate:   sampleRate,
		ChannelCount: channelCount,
		LanguageCode: languageCode,
		OnResultFunc: onResultFunc.(func(context.Context, json.Encoder, string, string, string, any) error),
	}
}

func (h *AmazonTranscribeHandler) Handle(ctx context.Context, reader io.Reader) (*io.PipeReader, error) {
	at := NewAmazonTranscribe(h.Config, h.LanguageCode, int64(h.SampleRate), int64(h.ChannelCount))
	stream, err := at.Start(ctx, reader)
	if err != nil {
		return nil, err
	}

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
						if err := h.OnResultFunc(ctx, *encoder, h.ChannelID, h.ConnectionID, h.LanguageCode, e.Transcript.Results); err != nil {
							w.CloseWithError(err)
							return
						}
					} else {
						for _, res := range e.Transcript.Results {
							if at.Config.SendFinalResultOnly {
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
							for _, alt := range res.Alternatives {
								var message string
								if alt.Transcript != nil {
									message = *alt.Transcript
								}
								result.Message = message
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
			// 復帰が不可能なエラー以外は再接続を試みる
			switch err.(type) {
			case *transcribestreamingservice.LimitExceededException,
				*transcribestreamingservice.InternalFailureException:
				zlog.Error().
					Err(err).
					Str("ChannelID", h.ChannelID).
					Str("ConnectionID", h.ConnectionID).
					Send()

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
