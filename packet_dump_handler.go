package suzu

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"time"
)

type dump struct {
	Timestamp    int64  `json:"timestamp"`
	ChannelID    string `json:"channel_id"`
	ConnectionID string `json:"connection_id"`
	LanguageCode string `json:"language_code"`
	SampleRate   uint32 `json:"sample_rate"`
	ChannelCount uint16 `json:"channel_count"`
	Payload      []byte `json:"payload"`
}

func PacketDumpHandler(ctx context.Context, body io.Reader, args HandlerArgs) (*io.PipeReader, error) {
	c := args.Config
	filename := c.DumpFile
	channelID := args.SoraChannelID
	connectionID := args.SoraConnectionID

	r, w := io.Pipe()

	go func() {
		f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			w.CloseWithError(err)
			return
		}
		defer f.Close()
		defer w.Close()

		mv := io.MultiWriter(f, w)
		encoder := json.NewEncoder(mv)

		for {
			buf := make([]byte, FrameSize)
			n, err := r.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				dump := dump{
					Timestamp:    time.Now().UnixMilli(),
					ChannelID:    channelID,
					ConnectionID: connectionID,
					LanguageCode: args.LanguageCode,
					SampleRate:   args.SampleRate,
					ChannelCount: args.ChannelCount,
					Payload:      buf[:n],
				}
				if err := encoder.Encode(dump); err != nil {
					w.CloseWithError(err)
					return
				}
			}
		}
	}()

	return r, nil
}
