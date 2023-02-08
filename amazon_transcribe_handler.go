package suzu

import (
	"context"
	"encoding/json"
	"io"
)

func init() {
	ServiceHandlers.registerHandler("aws", AmazonTranscribeHandler)
}

func AmazonTranscribeHandler(ctx context.Context, reader io.Reader, args HandlerArgs) (*io.PipeReader, error) {
	at := NewAmazonTranscribe(args.Config, args.LanguageCode, int64(args.SampleRate), int64(args.ChannelCount))

	oggReader, oggWriter := io.Pipe()
	go func() {
		if err := opus2ogg(ctx, reader, oggWriter, args.SampleRate, args.ChannelCount, args.Config); err != nil {
			oggWriter.CloseWithError(err)
			return
		}
		oggWriter.Close()
	}()

	go func() {
		defer at.Close()

		if err := at.Start(ctx, args.Config, oggReader); err != nil {
			at.ResultCh <- AwsErrorResult(err)
			return
		}
	}()

	r, w := io.Pipe()
	go func() {
		encoder := json.NewEncoder(w)

		for result := range at.ResultCh {
			if err := result.Error; err != nil {
				w.CloseWithError(err)
				return
			}

			if err := encoder.Encode(result); err != nil {
				w.CloseWithError(err)
				return
			}
		}
	}()

	return r, nil
}
