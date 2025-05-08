package suzu

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

func init() {
	NewServiceHandlerFuncs.register("dump", NewPacketDumpHandler)
}

type PacketDumpHandler struct {
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

func NewPacketDumpHandler(config Config, channelID, connectionID string, sampleRate uint32, channelCount uint16, languageCode string, onResultFunc any) serviceHandlerInterface {
	return &PacketDumpHandler{
		Config:       config,
		ChannelID:    channelID,
		ConnectionID: connectionID,
		SampleRate:   sampleRate,
		ChannelCount: channelCount,
		LanguageCode: languageCode,
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
	defer h.mu.Unlock()
	h.mu.Lock()
	h.RetryCount++
	return h.RetryCount
}

func (h *PacketDumpHandler) GetRetryCount() int {
	return h.RetryCount
}

func (h *PacketDumpHandler) ResetRetryCount() int {
	defer h.mu.Unlock()
	h.mu.Lock()
	h.RetryCount = 0
	return h.RetryCount
}

// IsRetryTarget は本ハンドラではリトライしないため、常に false を返す
func (h *PacketDumpHandler) IsRetryTarget(any) bool {
	return false
}

func (h *PacketDumpHandler) Handle(ctx context.Context, opusCh chan opusChannel, header soraHeader) (*io.PipeReader, error) {
	c := h.Config
	filename := c.DumpFile
	channelID := h.ChannelID
	connectionID := h.ConnectionID

	r, w := io.Pipe()

	reader := opusChannelToIOReadCloser(ctx, opusCh)

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
