package suzu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	zlog "github.com/rs/zerolog/log"
)

func init() {
	NewServiceHandlerFuncs.register("test", NewTestHandler)
}

type TestHandler struct {
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

func NewTestHandler(config Config, channelID, connectionID string, sampleRate uint32, channelCount uint16, languageCode string, onResultFunc any) serviceHandlerInterface {
	return &TestHandler{
		Config:       config,
		ChannelID:    channelID,
		ConnectionID: connectionID,
		SampleRate:   sampleRate,
		ChannelCount: channelCount,
		LanguageCode: languageCode,
		OnResultFunc: onResultFunc.(func(context.Context, io.WriteCloser, string, string, string, any) error),
	}
}

type TestResult struct {
	ChannelID *string `json:"channel_id,omitempty"`
	TranscriptionResult
}

func NewTestResult(channelID, message string) TestResult {
	return TestResult{
		TranscriptionResult: TranscriptionResult{
			Type:    "test",
			Message: message,
		},
		ChannelID: &channelID,
	}
}

func (h *TestHandler) UpdateRetryCount() int {
	defer h.mu.Unlock()
	h.mu.Lock()
	h.RetryCount++
	return h.RetryCount
}

func (h *TestHandler) GetRetryCount() int {
	return h.RetryCount
}

func (h *TestHandler) ResetRetryCount() int {
	defer h.mu.Unlock()
	h.mu.Lock()
	h.RetryCount = 0
	return h.RetryCount
}

// 本ハンドラでは使用しないため、常に false を返す
func (h *TestHandler) IsRetry(any) bool {
	return false
}

func (h *TestHandler) Handle(ctx context.Context, opusCh chan opusChannel, header soraHeader) (*io.PipeReader, error) {
	r, w := io.Pipe()

	reader := opusChannelToIOReadCloser(ctx, opusCh)

	go func() {
		encoder := json.NewEncoder(w)

		for {
			buf := make([]byte, FrameSize)
			n, err := reader.Read(buf)
			if err != nil {
				if err != io.EOF {
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

			if n > 0 {
				message := fmt.Sprintf("n: %d", n)
				channelID := &[]string{"ch_0"}[0]
				result := NewTestResult(*channelID, message)

				if h.OnResultFunc != nil {
					if err := h.OnResultFunc(ctx, w, h.ChannelID, h.ConnectionID, h.LanguageCode, result); err != nil {
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
					if err := encoder.Encode(result); err != nil {
						w.CloseWithError(err)
						return
					}
				}
			}
		}
	}()

	return r, nil
}
