package suzu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	OnResultFunc func(context.Context, json.Encoder, any) error
}

func NewTestHandler(config Config, channelID, connectionID string, sampleRate uint32, channelCount uint16, languageCode string, onResultFunc any) serviceHandlerInterface {
	return &TestHandler{
		Config:       config,
		ChannelID:    channelID,
		ConnectionID: connectionID,
		SampleRate:   sampleRate,
		ChannelCount: channelCount,
		LanguageCode: languageCode,
		OnResultFunc: onResultFunc.(func(context.Context, json.Encoder, any) error),
	}
}

type TestResult struct {
	ChannelID *string `json:"channel_id,omitempty"`
	TranscriptionResult
}

func TestErrorResult(err error) TestResult {
	return TestResult{
		TranscriptionResult: TranscriptionResult{
			Type:  "test",
			Error: err,
		},
	}
}

func (h *TestHandler) Handle(ctx context.Context, reader io.Reader) (*io.PipeReader, error) {
	r, w := io.Pipe()

	go func() {
		encoder := json.NewEncoder(w)

		for {
			buf := make([]byte, FrameSize)
			n, err := reader.Read(buf)
			if err != nil {
				w.CloseWithError(err)
				return
			}

			if n > 0 {
				var result TestResult
				result.Type = "test"
				result.Message = fmt.Sprintf("n: %d", n)
				result.ChannelID = &[]string{"ch_0"}[0]

				if h.OnResultFunc != nil {
					if err := h.OnResultFunc(ctx, *encoder, result); err != nil {
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
