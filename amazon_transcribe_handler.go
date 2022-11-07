package suzu

import (
	"context"
	"io"
)

func AmazonTranscribeHandler(ctx context.Context, body io.Reader, args HandlerArgs) (<-chan Response, error) {
	ch := make(chan Response)
	r, w := io.Pipe()

	at := NewAmazonTranscribe(args.Config.AwsRegion, args.LanguageCode, int64(args.SampleRate), int64(args.ChannelCount), args.Config.AwsEnablePartialResultsStabilization, args.Config.AwsEnableChannelIdentification)

	go func() {
		defer w.Close()
		if err := opus2ogg(ctx, body, w, args.SampleRate, args.ChannelCount, args.Config); err != nil {
			at.ResultCh <- TranscriptionResult{
				Error: err,
			}
			return
		}
	}()

	go func() {
		defer at.Close()
		if err := at.Start(ctx, args.Config, r); err != nil {
			at.ResultCh <- TranscriptionResult{
				Error: err,
			}
			return
		}
	}()

	go func() {
		defer close(ch)

		for tr := range at.ResultCh {
			if err := tr.Error; err != nil {
				ch <- Response{
					Error: err,
				}
				return
			}
			ch <- Response{
				ChannelID: tr.ChannelID,
				Message:   string(tr.Message),
			}
		}
	}()

	return ch, nil
}
