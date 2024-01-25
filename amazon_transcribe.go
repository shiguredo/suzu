package suzu

import (
	"context"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/transcribestreamingservice"
)

const (
	AccessDeniedException       = "AccessDeniedException"
	IncompleteSignature         = "IncompleteSignature"
	InternalFailure             = "InternalFailure"
	InvalidAction               = "InvalidAction"
	InvalidClientTokenID        = "InvalidClientTokenId"
	InvalidParameterCombination = "InvalidParameterCombination"
	InvalidParameterValue       = "InvalidParameterValue"
	InvalidQueryParameter       = "InvalidQueryParameter"
	MalformedQueryString        = "MalformedQueryString"
	MissingAction               = "MissingAction"
	MissingAuthenticationToken  = "MissingAuthenticationToken"
	MissingParameter            = "MissingParameter"
	NotAuthorized               = "NotAuthorized"
	OptInRequired               = "OptInRequired"
	RequestExpired              = "RequestExpired"
	ServiceUnavailable          = "ServiceUnavailable"
	ThrottlingException         = "ThrottlingException"
	ValidationError             = "ValidationError"
)

var (
	amazonErrors = map[string]int{
		// https://docs.aws.amazon.com/transcribe/latest/APIReference/API_streaming_StartStreamTranscription.html#API_streaming_StartStreamTranscription_Errors
		transcribestreamingservice.ErrCodeLimitExceededException:      429,
		transcribestreamingservice.ErrCodeConflictException:           409,
		transcribestreamingservice.ErrCodeBadRequestException:         400,
		transcribestreamingservice.ErrCodeInternalFailureException:    500,
		transcribestreamingservice.ErrCodeServiceUnavailableException: 503,

		// https://docs.aws.amazon.com/transcribe/latest/APIReference/CommonErrors.html
		AccessDeniedException:       400,
		IncompleteSignature:         400,
		InternalFailure:             500,
		InvalidAction:               400,
		InvalidClientTokenID:        403,
		InvalidParameterCombination: 400,
		InvalidParameterValue:       400,
		InvalidQueryParameter:       400,
		MalformedQueryString:        404,
		MissingAction:               400,
		MissingAuthenticationToken:  403,
		MissingParameter:            400,
		NotAuthorized:               400,
		OptInRequired:               403,
		RequestExpired:              400,
		ServiceUnavailable:          503,
		ThrottlingException:         400,
		ValidationError:             400,
	}
)

type AmazonTranscribe struct {
	LanguageCode                      string
	MediaEncoding                     string
	MediaSampleRateHertz              int64
	EnablePartialResultsStabilization bool
	NumberOfChannels                  int64
	EnableChannelIdentification       bool
	PartialResultsStability           string
	Region                            string
	Debug                             bool
	Config                            Config
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

func (at *AmazonTranscribe) Start(ctx context.Context, r io.Reader) (*transcribestreamingservice.StartStreamTranscriptionEventStream, error) {
	config := at.Config
	client := NewAmazonTranscribeClient(config)
	input := NewStartStreamTranscriptionInput(at)

	resp, err := client.StartStreamTranscriptionWithContext(ctx, &input)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			code, ok := amazonErrors[awsErr.Code()]
			if !ok {
				return nil, err
			}
			return nil, &SuzuError{
				Code:    code,
				Message: awsErr.Message(),
			}
		}
		return nil, err
	}

	stream := resp.GetStream()

	go func() {
		defer stream.Close()

		if err := transcribestreamingservice.StreamAudioFromReader(ctx, stream, FrameSize, r); err != nil {
			return
		}
	}()

	return stream, nil
}
