package suzu

import (
	"context"
	"encoding/json"
	"fmt"
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

func PacketDumpHandler(ctx context.Context, body io.Reader, args HandlerArgs) (<-chan Response, error) {
	ch := handlePacketDump(ctx, args.Config.DumpFile, body, args.SoraChannelID, args.SoraConnectionID, args.LanguageCode, args.SampleRate, args.ChannelCount)
	return ch, nil
}

func handlePacketDump(ctx context.Context, filename string, r io.Reader, channelID, connectionID, languageCode string, sampleRate uint32, channelCount uint16) <-chan Response {
	ch := make(chan Response)

	go func() {
		defer close(ch)

		f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case ch <- Response{
				Error: err,
			}:
				return
			}

		}
		defer f.Close()

		enc := json.NewEncoder(f)

		buf := make([]byte, 4*1024)

		for {
			n, err := r.Read(buf)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				case ch <- Response{
					Error: err,
				}:
					return
				}

			}
			if n > 0 {
				p := make([]byte, n)
				copy(p, buf[:n])
				dump := dump{
					Timestamp:    time.Now().UnixMilli(),
					ChannelID:    channelID,
					ConnectionID: connectionID,
					LanguageCode: languageCode,
					SampleRate:   sampleRate,
					ChannelCount: channelCount,
					Payload:      p,
				}
				if err := enc.Encode(dump); err != nil {
					select {
					case <-ctx.Done():
						return
					case ch <- Response{
						Error: err,
					}:
						return
					}
				}
				select {
				case <-ctx.Done():
					return
				case ch <- Response{
					ChannelID: &[]string{"ch_0"}[0],
					Message:   fmt.Sprintf("n: %d", n),
				}:
				}
			}
		}
	}()

	return ch
}
