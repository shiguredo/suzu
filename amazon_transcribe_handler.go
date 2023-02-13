package suzu

import (
	"context"
	"encoding/json"
	"io"

	"github.com/aws/aws-sdk-go/service/transcribestreamingservice"
)

func init() {
	ServiceHandlers.registerHandler("aws", AmazonTranscribeHandler)
}

func AmazonTranscribeHandler(ctx context.Context, reader io.Reader, args HandlerArgs) (*io.PipeReader, error) {
	oggReader, oggWriter := io.Pipe()
	go func() {
		defer oggWriter.Close()
		if err := opus2ogg(ctx, reader, oggWriter, args.SampleRate, args.ChannelCount, args.Config); err != nil {
			oggWriter.CloseWithError(err)
			return
		}
	}()

	at := NewAmazonTranscribe(args.Config, args.LanguageCode, int64(args.SampleRate), int64(args.ChannelCount))
	stream, err := at.Start(ctx, args.Config, oggReader)
	if err != nil {
		oggWriter.CloseWithError(err)
		return nil, err
	}

	r, w := io.Pipe()

	go func() {
		encoder := json.NewEncoder(w)

	L:
		for {
			select {
			case <-ctx.Done():
				break L
			case event := <-stream.Events():
				switch e := event.(type) {
				case *transcribestreamingservice.TranscriptEvent:
					for _, res := range e.Transcript.Results {
						var result AwsResult
						result.Type = "aws"
						if at.Config.AwsResultIsPartial {
							result.WithIsPartial(*res.IsPartial)
						}
						if at.Config.AwsResultChannelID {
							result.WithChannelID(*res.ChannelId)
						}
						for _, alt := range res.Alternatives {
							var message string
							if alt.Transcript != nil {
								message = *alt.Transcript
							}
							result.Message = message
							if err := encoder.Encode(result); err != nil {
								w.CloseWithError(err)
								return
							}
						}
					}
				default:
					break L
				}
			}
		}

		if err := stream.Err(); err != nil {
			w.CloseWithError(err)
			return
		}

		w.Close()
	}()

	return r, nil
}
