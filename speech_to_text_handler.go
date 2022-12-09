package suzu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

func init() {
	ServiceHandlers.registerHandler("gcp", SpeechToTextHandler)
}

func SpeechToTextHandler(ctx context.Context, conn io.Reader, args HandlerArgs) (*io.PipeReader, error) {

	d := time.Duration(args.Config.TimeToWaitForOpusPacketMs) * time.Millisecond

	reader, err := readerWithSilentPacketFromOpusReader(d, conn)
	if err != nil {
		return nil, err
	}

	oggReader, oggWriter := io.Pipe()

	go func() {
		defer oggWriter.Close()
		if err := opus2ogg(ctx, reader, oggWriter, args.SampleRate, args.ChannelCount, args.Config); err != nil {
			oggWriter.CloseWithError(err)
			return
		}
	}()

	stt := NewSpeechToText()
	stream, err := stt.Start(ctx, args.Config, args, oggReader)
	if err != nil {
		oggWriter.CloseWithError(err)
		return nil, err
	}

	r, w := io.Pipe()

	interimResults := false
	go func() {
		encoder := json.NewEncoder(w)

		for {
			resp, err := stream.Recv()
			if err != nil {
				w.CloseWithError(err)
				return
			}
			if err := resp.Error; err != nil {
				if err.Code == 3 || err.Code == 11 {
					fmt.Println(err)
				}
				fmt.Println(err)
				w.Close()
				return
			}

			for _, result := range resp.Results {
				for _, alternative := range result.Alternatives {
					if args.Config.EnableWordConfidence {
						for _, word := range alternative.Words {
							fmt.Printf("%s, Confidence: %v\n", word.Word, word)
						}
					}
					transcript := alternative.Transcript
					if interimResults {
						resp := Response{
							Message: transcript,
						}
						if err := encoder.Encode(resp); err != nil {
							w.CloseWithError(err)
							return
						}
					} else {
						if result.IsFinal {
							resp := Response{
								Message: transcript,
							}
							if err := encoder.Encode(resp); err != nil {
								w.CloseWithError(err)
								return
							}
						}
					}
				}
			}
		}
	}()

	return r, nil
}
