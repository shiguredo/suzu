package suzu

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/transcribestreamingservice"
)

type TranscriptionResult struct {
	Message string `json:"message,omitempty"`
	Error   error  `json:"error,omitempty"`
	Type    string `json:"type"`
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
	ResultCh                            chan AwsResult
	Config                              Config
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
		ResultCh:                          make(chan AwsResult),
		Config:                            config,
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
		//cfg = cfg.WithLogLevel(aws.LogDebugWithRequestErrors)
	}

	// TODO: 後で変更する
	tr := &http.Transport{}
	cfg = cfg.WithHTTPClient(&http.Client{Transport: tr})

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
		// デフォルトの HTTPClient の場合は、同時に複数接続する場合に HTTP リクエストがエラーになるため、aws.Config に独自の HTTPClient を指定する
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
	if at.StartStreamTranscriptionEventStream != nil {
		return at.StartStreamTranscriptionEventStream.Close()
	}
	return nil
}

type AwsResult struct {
	ChannelID *string `json:"channel_id,omitempty"`
	IsPartial *bool   `json:"is_partial,omitempty"`
	TranscriptionResult
}

func AwsErrorResult(err error) AwsResult {
	return AwsResult{
		TranscriptionResult: TranscriptionResult{
			Type:  "aws",
			Error: err,
		},
	}
}

func (ar *AwsResult) WithChannelID(channelID string) *AwsResult {
	ar.ChannelID = &channelID
	return ar
}

func (ar *AwsResult) WithIsPartial(isPartial bool) *AwsResult {
	ar.IsPartial = &isPartial
	return ar
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

						at.ResultCh <- result
					}
				}
			default:
				break L
			}
		}
	}

	if err := at.StartStreamTranscriptionEventStream.Err(); err != nil {
		err := fmt.Errorf("UNEXPECTED-STREAM-EVENT: %w", err)
		at.ResultCh <- AwsErrorResult(err)
		return
	}

	// io.EOF の場合は err は nil になるため明示的に io.EOF を送る
	at.ResultCh <- AwsErrorResult(io.EOF)

}

func (at *AmazonTranscribe) streamAudioFromReader(ctx context.Context, r io.Reader, frameSize int) error {
	if err := transcribestreamingservice.StreamAudioFromReader(ctx, at.StartStreamTranscriptionEventStream, frameSize, r); err != nil {
		return err
	}
	return nil
}
