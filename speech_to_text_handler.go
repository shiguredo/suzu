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

	OnResultFunc func(context.Context, io.WriteCloser, string, string, string, any) error
}

func NewSpeechToTextHandler(config Config, channelID, connectionID string, sampleRate uint32, channelCount uint16, languageCode string, onResultFunc any) serviceHandlerInterface {
	return &SpeechToTextHandler{
		Config:       config,
		ChannelID:    channelID,
		ConnectionID: connectionID,
		SampleRate:   sampleRate,
		ChannelCount: channelCount,
		LanguageCode: languageCode,
		OnResultFunc: onResultFunc.(func(context.Context, io.WriteCloser, string, string, string, any) error),
	}
}

type GcpResult struct {
	IsFinal   *bool    `json:"is_final,omitempty"`
	Stability *float32 `json:"stability,omitempty"`
	TranscriptionResult
}

func NewGcpResult() GcpResult {
	return GcpResult{
		TranscriptionResult: TranscriptionResult{
			Type: "gcp",
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

func (gr *GcpResult) SetMessage(message string) *GcpResult {
	gr.Message = message
	return gr
}

func (h *SpeechToTextHandler) Handle(ctx context.Context, reader io.Reader) (*io.PipeReader, error) {
	stt := NewSpeechToText(h.Config, h.LanguageCode, int32(h.SampleRate), int32(h.ChannelCount))

	oggReader, oggWriter := io.Pipe()
	go func() {
		defer oggWriter.Close()
		if err := opus2ogg(ctx, reader, oggWriter, h.SampleRate, h.ChannelCount, h.Config); err != nil {
			zlog.Error().
				Err(err).
				Str("channel_id", h.ChannelID).
				Str("connection_id", h.ConnectionID).
				Send()
			oggWriter.CloseWithError(err)
			return
		}
	}()

	stream, err := stt.Start(ctx, oggReader)
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
					Str("channel_id", h.ChannelID).
					Str("connection_id", h.ConnectionID).
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
						Str("channel_id", h.ChannelID).
						Str("connection_id", h.ConnectionID).
						Int32("code", status.GetCode()).
						Msg(status.GetMessage())
					err := ErrServerDisconnected

					w.CloseWithError(err)
					return
				}
				zlog.Error().
					Str("channel_id", h.ChannelID).
					Str("connection_id", h.ConnectionID).
					Int32("code", status.GetCode()).
					Msg(status.GetMessage())

				w.Close()
				return
			}

			if h.OnResultFunc != nil {
				if err := h.OnResultFunc(ctx, w, h.ChannelID, h.ConnectionID, h.LanguageCode, resp.Results); err != nil {
					if err := encoder.Encode(NewSuzuErrorResponse(err.Error())); err != nil {
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
				for _, res := range resp.Results {
					if stt.Config.FinalResultOnly {
						if !res.IsFinal {
							continue
						}
					}

					result := NewGcpResult()
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
									Str("channel_id", h.ChannelID).
									Str("connection_id", h.ConnectionID).
									Str("wrod", word.Word).
									Float32("confidence", word.Confidence).
									Str("start_time", word.StartTime.String()).
									Str("end_time", word.EndTime.String()).
									Send()
							}
						}
						transcript := alternative.Transcript
						result.SetMessage(transcript)
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
