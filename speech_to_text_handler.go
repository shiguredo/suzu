package suzu

import (
	"context"
	"encoding/json"
	"io"
	"strings"

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

func SpeechToTextHandler(ctx context.Context, reader io.Reader, args HandlerArgs) (*io.PipeReader, error) {
	stt := NewSpeechToText(args.Config, args.LanguageCode, int32(args.SampleRate), int32(args.ChannelCount))
	stream, err := stt.Start(ctx, args.Config, reader)
	if err != nil {
		return nil, err
	}

	r, w := io.Pipe()

	go func() {
		encoder := json.NewEncoder(w)

		for {
			resp, err := stream.Recv()
			if err != nil {
				zlog.Error().
					Err(err).
					Str("CHANNEL-ID", args.SoraChannelID).
					Str("CONNECTION-ID", args.SoraConnectionID).
					Send()

				if (strings.Contains(err.Error(), "code = OutOfRange")) ||
					(strings.Contains(err.Error(), "code = InvalidArgument")) ||
					(strings.Contains(err.Error(), "code = ResourceExhausted")) {
					w.CloseWithError(ErrServerDisconnected)
					return
				}

				w.CloseWithError(err)
				return
			}
			if status := resp.Error; err != nil {
				// 音声の長さの上限値に達した場合
				if status.Code == 3 || status.Code == 11 || status.Code == 8 {
					zlog.Error().
						Err(err).
						Str("CHANNEL-ID", args.SoraChannelID).
						Str("CONNECTION-ID", args.SoraConnectionID).
						Str("MESSAGE", status.GetMessage()).
						Int32("CODE", status.GetCode()).
						Send()
					err := ErrServerDisconnected
					w.CloseWithError(err)
					return
				}
				zlog.Error().
					Str("CHANNEL-ID", args.SoraChannelID).
					Str("CONNECTION-ID", args.SoraConnectionID).
					Str("MESSAGE", status.GetMessage()).
					Int32("CODE", status.GetCode()).
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
