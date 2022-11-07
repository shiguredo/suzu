package suzu

import (
	"context"
	"fmt"
	"io"
)

func SpeechToTextHandler(ctx context.Context, body io.Reader, args HandlerArgs) (<-chan Response, error) {
	ch := make(chan Response)

	r, w := io.Pipe()

	go func() {
		defer w.Close()
		if err := opus2ogg(ctx, body, w, args.SampleRate, args.ChannelCount, args.Config); err != nil {
			fmt.Println(err)
			return
		}
	}()

	stt := NewSpeechToText()
	stream, err := stt.Start(ctx, args.Config, args, r)
	if err != nil {
		return nil, err
	}

	interimResults := false
	go func() {
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				fmt.Println(err)
				return
			}
			if err != nil {
				fmt.Println(err)
				return
			}
			if err := resp.Error; err != nil {
				if err.Code == 3 || err.Code == 11 {
					fmt.Println(err)
				}
				fmt.Println(err)
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
						ch <- Response{
							Message: transcript,
						}
					} else {
						if result.IsFinal {
							ch <- Response{
								Message: transcript,
							}
						}
					}
				}
			}
		}
	}()

	return ch, nil
}
