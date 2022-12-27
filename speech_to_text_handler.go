package suzu

import (
	"context"
	"encoding/json"
	"io"
	"time"

	zlog "github.com/rs/zerolog/log"
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
					zlog.Error().
						Str("CHANNEL-ID", args.SoraChannelID).
						Str("CONNECTION-ID", args.SoraConnectionID).
						Str("MESSAGE", err.GetMessage()).
						Int32("CODE", err.GetCode()).
						Send()
				}
				zlog.Error().
					Str("CHANNEL-ID", args.SoraChannelID).
					Str("CONNECTION-ID", args.SoraConnectionID).
					Str("MESSAGE", err.GetMessage()).
					Int32("CODE", err.GetCode()).
					Send()
				w.Close()
				return
			}

			for _, result := range resp.Results {
				for _, alternative := range result.Alternatives {
					if args.Config.GcpEnableWordConfidence {
						for _, word := range alternative.Words {
							zlog.Debug().
								Str("CHANNEL-ID", args.SoraChannelID).
								Str("CONNECTION-ID", args.SoraConnectionID).
								Str("Wrod", word.Word).
								Float32("Confidence", word.Confidence).
								Str("StartTime", word.StartTime.String()).
								Str("EndTime", word.EndTime.String()).
								Send()
						}
					}
					transcript := alternative.Transcript
					if args.Config.GcpInterimResults {
						if result.IsFinal {
							resp := Response{
								Message: transcript,
							}
							if err := encoder.Encode(resp); err != nil {
								w.CloseWithError(err)
								return
							}
						} else {
							zlog.Debug().
								Str("CHANNEL-ID", args.SoraChannelID).
								Str("CONNECTION-ID", args.SoraConnectionID).
								Str("Transcript", transcript).
								Send()
						}
					} else {
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
	}()

	return r, nil
}
