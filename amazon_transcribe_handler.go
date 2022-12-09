package suzu

import (
	"context"
	"encoding/json"
	"io"
	"time"
)

func init() {
	ServiceHandlers.registerHandler("aws", AmazonTranscribeHandler)
}

func AmazonTranscribeHandler(ctx context.Context, conn io.Reader, args HandlerArgs) (*io.PipeReader, error) {
	at := NewAmazonTranscribe(args.Config.AwsRegion, args.LanguageCode, int64(args.SampleRate), int64(args.ChannelCount), args.Config.AwsEnablePartialResultsStabilization, args.Config.AwsEnableChannelIdentification)

	d := time.Duration(args.Config.TimeToWaitForOpusPacketMs) * time.Millisecond

	reader, err := readerWithSilentPacketFromOpusReader(d, conn)
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

	go func() {
		defer at.Close()

		if err := at.Start(ctx, args.Config, oggReader); err != nil {
			at.ResultCh <- TranscriptionResult{
				Error: err,
			}
			return
		}
	}()

	r, w := io.Pipe()
	go func() {
		encoder := json.NewEncoder(w)

		for tr := range at.ResultCh {
			if err := tr.Error; err != nil {
				w.CloseWithError(err)
				return
			}

			res := Response{
				ChannelID: tr.ChannelID,
				Message:   string(tr.Message),
			}
			if err := encoder.Encode(res); err != nil {
				w.CloseWithError(err)
				return
			}
		}
	}()

	return r, nil
}
