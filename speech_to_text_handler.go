package suzu

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	zlog "github.com/rs/zerolog/log"

	"google.golang.org/grpc/codes"
)

func init() {
	NewServiceHandlerFuncs.register("gcp", NewSpeechToTextHandler)
}

type SpeechToTextHandler struct {
	Config Config

	ChannelID    string
	ConnectionID string
	SampleRate   uint32
	ChannelCount uint16
	LanguageCode string

	OnResultFunc func(context.Context, json.Encoder, string, string, string, any) error
}

func NewSpeechToTextHandler(config Config, channelID, connectionID string, sampleRate uint32, channelCount uint16, languageCode string, onResultFunc any) serviceHandlerInterface {
	return &SpeechToTextHandler{
		Config:       config,
		ChannelID:    channelID,
		ConnectionID: connectionID,
		SampleRate:   sampleRate,
		ChannelCount: channelCount,
		LanguageCode: languageCode,
		OnResultFunc: onResultFunc.(func(context.Context, json.Encoder, string, string, string, any) error),
	}
}

type GcpResult struct {
	IsFinal   *bool    `json:"is_final,omitempty"`
	Stability *float32 `json:"stability,omitempty"`
	TranscriptionResult
}

func GcpErrorResult(err error) GcpResult {
	return GcpResult{
		TranscriptionResult: TranscriptionResult{
			Type:  "gcp",
			Error: err,
		},
	}
}

func (gr *GcpResult) WithIsFinal(isFinal bool) *GcpResult {
	gr.IsFinal = &isFinal
	return gr
}

func (gr *GcpResult) WithStability(stability float32) *GcpResult {
	gr.Stability = &stability
	return gr
}

func (h *SpeechToTextHandler) Handle(ctx context.Context, reader io.Reader) (*io.PipeReader, error) {
	stt := NewSpeechToText(h.Config, h.LanguageCode, int32(h.SampleRate), int32(h.ChannelCount))
	stream, err := stt.Start(ctx, reader)
	if err != nil {
		return nil, err
	}

	r, w := io.Pipe()

	go func() {
		encoder := json.NewEncoder(w)

		for {
			resp, err := stream.Recv()
			if err != nil {
				zlog.Error().
					Err(err).
					Str("CHANNEL-ID", h.ChannelID).
					Str("CONNECTION-ID", h.ConnectionID).
					Send()

				if (strings.Contains(err.Error(), "code = OutOfRange")) ||
					(strings.Contains(err.Error(), "code = InvalidArgument")) ||
					(strings.Contains(err.Error(), "code = ResourceExhausted")) {
					w.CloseWithError(ErrServerDisconnected)
					return
				}

				w.CloseWithError(err)
				return
			}
			if status := resp.Error; err != nil {
				// 音声の長さの上限値に達した場合
				code := codes.Code(status.GetCode())
				if code == codes.OutOfRange ||
					code == codes.InvalidArgument ||
					code == codes.ResourceExhausted {

					zlog.Error().
						Err(err).
						Str("CHANNEL-ID", h.ChannelID).
						Str("CONNECTION-ID", h.ConnectionID).
						Str("MESSAGE", status.GetMessage()).
						Int32("CODE", status.GetCode()).
						Send()
					err := ErrServerDisconnected
					w.CloseWithError(err)
					return
				}
				zlog.Error().
					Str("CHANNEL-ID", h.ChannelID).
					Str("CONNECTION-ID", h.ConnectionID).
					Str("MESSAGE", status.GetMessage()).
					Int32("CODE", status.GetCode()).
					Send()
				w.Close()
				return
			}
			if h.OnResultFunc != nil {
				if err := h.OnResultFunc(ctx, *encoder, h.ChannelID, h.ConnectionID, h.LanguageCode, resp.Results); err != nil {
					w.CloseWithError(err)
					return
				}
			} else {
				for _, res := range resp.Results {
					var result GcpResult
					result.Type = "gcp"
					if stt.Config.GcpResultIsFinal {
						result.WithIsFinal(res.IsFinal)
					}
					if stt.Config.GcpResultStability {
						result.WithStability(res.Stability)
					}

					for _, alternative := range res.Alternatives {
						if h.Config.GcpEnableWordConfidence {
							for _, word := range alternative.Words {
								zlog.Debug().
									Str("CHANNEL-ID", h.ChannelID).
									Str("CONNECTION-ID", h.ConnectionID).
									Str("Wrod", word.Word).
									Float32("Confidence", word.Confidence).
									Str("StartTime", word.StartTime.String()).
									Str("EndTime", word.EndTime.String()).
									Send()
							}
						}
						transcript := alternative.Transcript
						result.Message = transcript
						if err := encoder.Encode(result); err != nil {
							w.CloseWithError(err)
							return
						}
					}
				}
			}
		}
	}()

	return r, nil
}
