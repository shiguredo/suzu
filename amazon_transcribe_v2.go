package suzu

import (
	"context"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/transcribestreaming"
	"github.com/aws/aws-sdk-go-v2/service/transcribestreaming/types"

	zlog "github.com/rs/zerolog/log"
)

type AmazonTranscribeV2 struct {
	LanguageCode                      string
	MediaEncoding                     types.MediaEncoding
	MediaSampleRateHertz              int64
	EnablePartialResultsStabilization bool
	NumberOfChannels                  int64
	EnableChannelIdentification       bool
	PartialResultsStability           string
	Region                            string
	Debug                             bool
	Config                            Config
}

func NewAmazonTranscribeV2(c Config, languageCode string, sampleRateHertz, audioChannelCount int64) *AmazonTranscribeV2 {
	return &AmazonTranscribeV2{
		Region:                            c.AwsRegion,
		LanguageCode:                      languageCode,
		MediaEncoding:                     types.MediaEncodingOggOpus,
		MediaSampleRateHertz:              sampleRateHertz,
		EnablePartialResultsStabilization: c.AwsEnablePartialResultsStabilization,
		PartialResultsStability:           c.AwsPartialResultsStability,
		NumberOfChannels:                  audioChannelCount,
		EnableChannelIdentification:       c.AwsEnableChannelIdentification,
		Config:                            c,
	}
}

func NewStartStreamTranscriptionInputV2(at *AmazonTranscribeV2) transcribestreaming.StartStreamTranscriptionInput {
	var numberOfChannels *int32
	if at.EnableChannelIdentification {
		c := int32(at.NumberOfChannels)
		numberOfChannels = &c
	}

	sampleRateHertz := int32(at.MediaSampleRateHertz)

	if !at.EnablePartialResultsStabilization {
		return transcribestreaming.StartStreamTranscriptionInput{
			LanguageCode:                      types.LanguageCode(at.LanguageCode),
			MediaEncoding:                     at.MediaEncoding,
			MediaSampleRateHertz:              &sampleRateHertz,
			NumberOfChannels:                  numberOfChannels,
			EnablePartialResultsStabilization: at.EnablePartialResultsStabilization,
			EnableChannelIdentification:       at.EnableChannelIdentification,
		}
	} else {
		return transcribestreaming.StartStreamTranscriptionInput{
			LanguageCode:                      types.LanguageCode(at.LanguageCode),
			MediaEncoding:                     at.MediaEncoding,
			MediaSampleRateHertz:              &sampleRateHertz,
			NumberOfChannels:                  numberOfChannels,
			EnablePartialResultsStabilization: at.EnablePartialResultsStabilization,
			PartialResultsStability:           types.PartialResultsStability(at.PartialResultsStability),
			EnableChannelIdentification:       at.EnableChannelIdentification,
		}
	}
}

func NewAmazonTranscribeClientV2(c Config) (*transcribestreaming.Client, error) {
	// TODO: 後で変更する
	tr := &http.Transport{}
	httpClient := &http.Client{Transport: tr}

	ctx := context.TODO()

	var cfg aws.Config
	if c.AwsProfile != "" {
		// TODO: logLevel の指定
		var err error
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(c.AwsRegion),
			config.WithSharedConfigProfile(c.AwsProfile),
			config.WithSharedCredentialsFiles([]string{c.AwsCredentialFile}),
			config.WithHTTPClient(httpClient),
		)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		cfg, err = config.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, err
		}
	}

	client := transcribestreaming.NewFromConfig(cfg)
	return client, nil
}

func (at *AmazonTranscribeV2) Start(ctx context.Context, r io.ReadCloser) (*transcribestreaming.StartStreamTranscriptionEventStream, error) {
	config := at.Config
	client, err := NewAmazonTranscribeClientV2(config)
	if err != nil {
		return nil, err
	}
	input := NewStartStreamTranscriptionInputV2(at)

	resp, err := client.StartStreamTranscription(ctx, &input)
	if err != nil {
		// TODO: v2 には存在しないため、変更されたエラーに置き換える
		// if reqErr, ok := err.(awserr.RequestFailure); ok {
		// 	code := reqErr.StatusCode()
		// 	message := reqErr.Message()

		// 	var retry bool
		// 	if code == http.StatusTooManyRequests {
		// 		retry = true
		// 	}

		// 	return nil, &SuzuError{
		// 		Code:    code,
		// 		Message: message,
		// 		Retry:   retry,
		// 	}
		// }
		return nil, err
	}

	stream := resp.GetStream()

	go func() {
		defer r.Close()
		defer func() {
			if err := stream.Close(); err != nil {
				zlog.Error().Err(err).Send()
			}
		}()

		frame := make([]byte, FrameSize)
		for {
			n, err := r.Read(frame)
			if err != nil {
				if err != io.EOF {
					zlog.Error().Err(err).Send()
				}
				break
			}
			if n > 0 {
				err := stream.Send(ctx, &types.AudioStreamMemberAudioEvent{
					Value: types.AudioEvent{
						AudioChunk: frame[:n],
					},
				})
				if err != nil {
					zlog.Error().Err(err).Send()
					break
				}
			}
		}
	}()

	return stream, nil
}
