package suzu

import (
	"context"
	"encoding/json"
	"errors"
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
		OnResultFunc: onResultFunc.(func(context.Context, io.WriteCloser, string, string, string, any) error),
	}
}

type AwsResult struct {
	ChannelID *string `json:"channel_id,omitempty"`
	IsPartial *bool   `json:"is_partial,omitempty"`
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

func (ar *AwsResult) SetMessage(message string) *AwsResult {
	ar.Message = message
	return ar
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
			// 復帰が不可能なエラー以外は再接続を試みる
			switch err.(type) {
			case *transcribestreamingservice.LimitExceededException:
				zlog.Error().
					Err(err).
					Str("channel_id", h.ChannelID).
					Str("connection_id", h.ConnectionID).
					Send()

				// リトライしない設定の場合はクライアントにエラーを返し、再度接続するかはクライアント側で判断する
				if !*at.Config.Retry {
					if err := encoder.Encode(NewSuzuErrorResponse(err)); err != nil {
						zlog.Error().
							Err(err).
							Str("channel_id", h.ChannelID).
							Str("connection_id", h.ConnectionID).
							Send()
					}
				}

				err = ErrServerDisconnected
			default:
				// 再接続を想定している以外のエラーの場合はクライアントにエラーを返し、再度接続するかはクライアント側で判断する
				if err := encoder.Encode(NewSuzuErrorResponse(err)); err != nil {
					zlog.Error().
						Err(err).
						Str("channel_id", h.ChannelID).
						Str("connection_id", h.ConnectionID).
						Send()
				}
			}

			w.CloseWithError(err)
			return
		}

		w.Close()
	}()

	return r, nil
}
