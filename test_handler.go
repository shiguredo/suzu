package suzu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type TestResult struct {
	ChannelID *string `json:"channel_id,omitempty"`
	TranscriptionResult
}

func TestErrorResult(err error) TestResult {
	return TestResult{
		TranscriptionResult: TranscriptionResult{
			Type:         "test",
			Error:        err,
			ErrorMessage: err.Error(),
		},
	}
}

func TestHandler(ctx context.Context, opusReader io.Reader, args HandlerArgs) (*io.PipeReader, error) {
	c := args.Config

	d := time.Duration(c.TimeToWaitForOpusPacketMs) * time.Millisecond

	reader, err := readerWithSilentPacketFromOpusReader(d, opusReader)
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
				var result TestResult
				result.Type = "test"
				result.Message = fmt.Sprintf("n: %d", n)
				result.ChannelID = &[]string{"ch_0"}[0]
				if err := encoder.Encode(result); err != nil {
					w.CloseWithError(err)
					return
				}
			}
		}
	}()

	return r, nil
}
