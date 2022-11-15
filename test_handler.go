package suzu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

func TestHandler(ctx context.Context, opusReader io.Reader, args HandlerArgs) (*io.PipeReader, error) {
	c := args.Config

	d, err := time.ParseDuration(c.TimeToWaitForOpusPacket)
	if err != nil {
		return nil, err
	}

	reader, err := readerWithSilentPacketFromOpusReader(ctx, d, opusReader)
	if err != nil {
		return nil, err
	}

	oggReader, oggWriter := io.Pipe()

	go func() {
		if err := opus2ogg(ctx, reader, oggWriter, args.SampleRate, args.ChannelCount, args.Config); err != nil {
			oggWriter.CloseWithError(err)
			return
		}

		oggWriter.Close()
	}()

	r, w := io.Pipe()

	go func() {
		encoder := json.NewEncoder(w)

		for {
			buf := make([]byte, FrameSize)
			n, err := oggReader.Read(buf)
			if err != nil {
				w.CloseWithError(err)
				return
			}

			if n > 0 {
				res := Response{
					ChannelID: &[]string{"ch_0"}[0],
					Message:   fmt.Sprintf("n: %d", n),
				}
				if err := encoder.Encode(res); err != nil {
					w.CloseWithError(err)
					return
				}
			}
		}
	}()

	return r, nil
}
