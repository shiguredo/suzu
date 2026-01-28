package suzu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

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
	RetryCount   int
	mu           sync.Mutex

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

func (h *SpeechToTextHandler) UpdateRetryCount() int {
	defer h.mu.Unlock()
	h.mu.Lock()
	h.RetryCount++
	return h.RetryCount
}

func (h *SpeechToTextHandler) GetRetryCount() int {
	return h.RetryCount
}

func (h *SpeechToTextHandler) ResetRetryCount() int {
	defer h.mu.Unlock()
	h.mu.Lock()
	h.RetryCount = 0
	return h.RetryCount
}

func (h *SpeechToTextHandler) IsRetryTarget(args any) bool {
	switch err := args.(type) {
	case error:
		if (strings.Contains(err.Error(), "code = OutOfRange")) ||
			(strings.Contains(err.Error(), "code = InvalidArgument")) ||
			(strings.Contains(err.Error(), "code = ResourceExhausted")) {
			return true
		}

		if isRetryTargetByConfig(h.Config, err.Error()) {
			return true
		}
	case codes.Code:
		code := err

		if code == codes.OutOfRange ||
			code == codes.InvalidArgument ||
			code == codes.ResourceExhausted {
			return true
		}

		if isRetryTargetByConfig(h.Config, code.String()) {
			return true
		}
	default:
		// error, codes.Code ではない場合はリトライしない
	}

	return false
}

func (h *SpeechToTextHandler) Handle(ctx context.Context, opusCh chan opus, header soraHeader) (*io.PipeReader, error) {
	stt := NewSpeechToText(h.Config, h.LanguageCode, int32(h.SampleRate), int32(h.ChannelCount))

	packetReader, err := opus2ogg(ctx, opusCh, h.SampleRate, h.ChannelCount, h.Config, header)
	if err != nil {
		return nil, err
	}

	stream, err := stt.Start(ctx, packetReader, header)
	if err != nil {
		return nil, err
	}

	h.ResetRetryCount()

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

				if h.IsRetryTarget(err) {
					err = errors.Join(err, ErrServerDisconnected)
				}

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

			if status := resp.Error; status != nil {
				// 音声の長さの上限値に達した場合
				err := fmt.Errorf("%s", status.GetMessage())
				code := codes.Code(status.GetCode())

				zlog.Error().
					Err(err).
					Str("channel_id", h.ChannelID).
					Str("connection_id", h.ConnectionID).
					Int32("code", status.GetCode()).
					Send()

				if h.IsRetryTarget(code) {
					err = errors.Join(err, ErrServerDisconnected)
					w.CloseWithError(err)
					return
				}

				w.CloseWithError(err)
				return
			}

			if h.OnResultFunc != nil {
				if err := h.OnResultFunc(ctx, w, h.ChannelID, h.ConnectionID, h.LanguageCode, resp.Results); err != nil {
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
