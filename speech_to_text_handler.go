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

type GcpResult struct {
	IsFinal   *bool    `json:"is_final,omitempty"`
	Stability *float32 `json:"stability,omitempty"`
	TranscriptionResult
}

func GcpErrorResult(err error) GcpResult {
	return GcpResult{
		TranscriptionResult: TranscriptionResult{
			Type:  "gcp",
			Error: err,
		},
	}
}

func (gr *GcpResult) WithIsFinal(isFinal bool) *GcpResult {
	gr.IsFinal = &isFinal
	return gr
}

func (gr *GcpResult) WithStability(stability float32) *GcpResult {
	gr.Stability = &stability
	return gr
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

	stt := NewSpeechToText(args.Config)
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
				// TODO: 音声の長さの上限値に達した場合の処理の追加
				// if err.Code == 3 || err.Code == 11 {
				// }
				zlog.Error().
					Str("CHANNEL-ID", args.SoraChannelID).
					Str("CONNECTION-ID", args.SoraConnectionID).
					Str("MESSAGE", err.GetMessage()).
					Int32("CODE", err.GetCode()).
					Send()
				w.Close()
				return
			}

			for _, res := range resp.Results {
				var result GcpResult
				result.Type = "gcp"
				if stt.Config.GcpResultIsFinal {
					result.WithIsFinal(res.IsFinal)
				}
				if stt.Config.GcpResultStability {
					result.WithStability(res.Stability)
				}

				for _, alternative := range res.Alternatives {
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
					result.Message = transcript
					if err := encoder.Encode(result); err != nil {
						w.CloseWithError(err)
						return
					}
				}
			}
		}
	}()

	return r, nil
}
