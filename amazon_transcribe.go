package suzu

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/transcribestreamingservice"
	zlog "github.com/rs/zerolog/log"
)

type TranscriptionResult struct {
	ChannelID *string `json:"channel_id"`
	Message   []byte  `json:"message"`
	Error     error   `json:"error,omitempty"`
}

const (
	FrameSize = 1024 * 10
)

type AmazonTranscribe struct {
	LanguageCode                        string
	MediaEncoding                       string
	MediaSampleRateHertz                int64
	EnablePartialResultsStabilization   bool
	NumberOfChannels                    int64
	EnableChannelIdentification         bool
	PartialResultsStability             string
	Region                              string
	Debug                               bool
	StartStreamTranscriptionEventStream *transcribestreamingservice.StartStreamTranscriptionEventStream
	ResultCh                            chan TranscriptionResult
}

func NewAmazonTranscribe(config Config, languageCode string, sampleRateHertz, audioChannelCount int64) *AmazonTranscribe {
	return &AmazonTranscribe{
		Region:                            config.AwsRegion,
		LanguageCode:                      languageCode,
		MediaEncoding:                     transcribestreamingservice.MediaEncodingOggOpus,
		MediaSampleRateHertz:              sampleRateHertz,
		EnablePartialResultsStabilization: config.AwsEnablePartialResultsStabilization,
		PartialResultsStability:           config.AwsPartialResultsStability,
		NumberOfChannels:                  audioChannelCount,
		EnableChannelIdentification:       config.AwsEnableChannelIdentification,
		ResultCh:                          make(chan TranscriptionResult),
	}
}

func NewStartStreamTranscriptionInput(at *AmazonTranscribe) transcribestreamingservice.StartStreamTranscriptionInput {
	var numberOfChannels *int64
	if at.EnableChannelIdentification {
		numberOfChannels = aws.Int64(at.NumberOfChannels)
	}
	var partialResultsStability *string
	if !at.EnablePartialResultsStabilization {
		partialResultsStability = nil
	} else {
		partialResultsStability = &at.PartialResultsStability
	}

	return transcribestreamingservice.StartStreamTranscriptionInput{
		LanguageCode:                      aws.String(at.LanguageCode),
		MediaEncoding:                     aws.String(transcribestreamingservice.MediaEncodingOggOpus),
		MediaSampleRateHertz:              aws.Int64(at.MediaSampleRateHertz),
		NumberOfChannels:                  numberOfChannels,
		EnablePartialResultsStabilization: aws.Bool(at.EnablePartialResultsStabilization),
		PartialResultsStability:           partialResultsStability,
		EnableChannelIdentification:       aws.Bool(at.EnableChannelIdentification),
	}
}

func NewAmazonTranscribeClient(config Config) *transcribestreamingservice.TranscribeStreamingService {
	cfg := aws.NewConfig().WithRegion(config.AwsRegion)

	if config.Debug {
		cfg = cfg.WithLogLevel(aws.LogDebug)
	}

	var sess *session.Session
	if config.AwsProfile != "" {
		sessOpts := session.Options{
			Config:            *cfg,
			Profile:           config.AwsProfile,
			SharedConfigFiles: []string{config.AwsCredentialFile},
			SharedConfigState: session.SharedConfigEnable,
		}
		sess = session.Must(session.NewSessionWithOptions(sessOpts))
	} else {
		sess = session.Must(session.NewSession(cfg))
	}
	return transcribestreamingservice.New(sess, cfg)
}

func (at *AmazonTranscribe) Start(ctx context.Context, config Config, r io.Reader) error {
	if err := at.startTranscribeService(ctx, config); err != nil {
		return err
	}

	if err := at.streamAudioFromReader(ctx, r, FrameSize); err != nil {
		return err
	}

	return nil
}

func (at *AmazonTranscribe) startTranscribeService(ctx context.Context, config Config) error {

	client := NewAmazonTranscribeClient(config)
	input := NewStartStreamTranscriptionInput(at)

	resp, err := client.StartStreamTranscriptionWithContext(ctx, &input)
	if err != nil {
		return err
	}

	stream := resp.GetStream()
	at.StartStreamTranscriptionEventStream = stream

	go at.ReceiveResults(ctx)

	return nil
}

func (at *AmazonTranscribe) Close() error {
	return at.StartStreamTranscriptionEventStream.Close()
}

func (at *AmazonTranscribe) ReceiveResults(ctx context.Context) {
L:
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-at.StartStreamTranscriptionEventStream.Events():
			switch e := event.(type) {
			case *transcribestreamingservice.TranscriptEvent:
				for _, res := range e.Transcript.Results {
					if !aws.BoolValue(res.IsPartial) {
						for _, alt := range res.Alternatives {
							var message []byte
							if alt.Transcript != nil {
								message = []byte(*alt.Transcript)
							}
							// TODO: 他に必要なフィールドも送信する
							at.ResultCh <- TranscriptionResult{
								ChannelID: res.ChannelId,
								Message:   message,
							}
						}
					}
				}
			default:
				break L
			}
		}
	}

	if err := at.StartStreamTranscriptionEventStream.Err(); err != nil {
		err := fmt.Errorf("UNEXPECTED-STREAM-EVENT: %w", err)
		at.ResultCh <- TranscriptionResult{
			Error: err,
		}
		return
	}

}

func (at *AmazonTranscribe) streamAudioFromReader(ctx context.Context, r io.Reader, frameSize int) error {
	if err := transcribestreamingservice.StreamAudioFromReader(ctx, at.StartStreamTranscriptionEventStream, frameSize, r); err != nil {
		zlog.Debug().Err(err).Send()
		return err
	}
	return nil
}
