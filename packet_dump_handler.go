package suzu

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"time"
)

func init() {
	ServiceHandlerMakers.Register("dump", new(PacketDumpHandlerMaker))
}

type PacketDumpHandler struct {
	Config Config

	ChannelID    string
	ConnectionID string
	SampleRate   uint32
	ChannelCount uint16
	LanguageCode string
	RetryCounter RetryCounter

	OnResultFunc func(context.Context, io.WriteCloser, string, string, string, any) error
}

type PacketDumpHandlerMaker struct{}

func (h *PacketDumpHandlerMaker) New(config Config, channelID, connectionID string, sampleRate uint32, channelCount uint16, languageCode string, onResultFunc any) serviceHandlerInterface {
	return &PacketDumpHandler{
		Config:       config,
		ChannelID:    channelID,
		ConnectionID: connectionID,
		SampleRate:   sampleRate,
		ChannelCount: channelCount,
		LanguageCode: languageCode,
		RetryCounter: NewRetryCounter(),
		OnResultFunc: onResultFunc.(func(context.Context, io.WriteCloser, string, string, string, any) error),
	}
}

type PacketDumpResult struct {
	Timestamp    int64  `json:"timestamp"`
	ChannelID    string `json:"channel_id"`
	ConnectionID string `json:"connection_id"`
	LanguageCode string `json:"language_code"`
	SampleRate   uint32 `json:"sample_rate"`
	ChannelCount uint16 `json:"channel_count"`
	Payload      []byte `json:"payload"`
}

func (h *PacketDumpHandler) UpdateRetryCount() int {
	return h.RetryCounter.Update()
}

func (h *PacketDumpHandler) GetRetryCount() int {
	return h.RetryCounter.Get()
}

func (h *PacketDumpHandler) ResetRetryCount() int {
	return h.RetryCounter.Reset()
}

func (h *PacketDumpHandler) Handle(ctx context.Context, reader io.Reader) (*io.PipeReader, error) {
	c := h.Config
	filename := c.DumpFile
	channelID := h.ChannelID
	connectionID := h.ConnectionID

	r, w := io.Pipe()

	go func() {
		f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			w.CloseWithError(err)
			return
		}
		defer f.Close()
		defer w.Close()

		mw := io.MultiWriter(f, w)
		encoder := json.NewEncoder(mw)

		for {
			buf := make([]byte, FrameSize)
			n, err := reader.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				dump := &PacketDumpResult{
					Timestamp:    time.Now().UnixMilli(),
					ChannelID:    channelID,
					ConnectionID: connectionID,
					LanguageCode: h.LanguageCode,
					SampleRate:   h.SampleRate,
					ChannelCount: h.ChannelCount,
					Payload:      buf[:n],
				}

				if h.OnResultFunc != nil {
					if err := h.OnResultFunc(ctx, w, h.ChannelID, h.ConnectionID, h.LanguageCode, dump); err != nil {
						w.CloseWithError(err)
						return
					}
				} else {
					if err := encoder.Encode(dump); err != nil {
						w.CloseWithError(err)
						return
					}
				}
			}
		}
	}()

	return r, nil
}
