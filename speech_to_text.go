package suzu

import (
	"context"
	"errors"
	"io"

	speech "cloud.google.com/go/speech/apiv1"
	zlog "github.com/rs/zerolog/log"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/types/known/wrapperspb"

	speechpb "cloud.google.com/go/speech/apiv1/speechpb"
)

type SpeechToText struct {
	SampleReate  int32
	ChannelCount int32
	LanguageCode string
	Config       Config
}

func NewSpeechToText(config Config, languageCode string, sampleRate, channelCount int32) SpeechToText {
	return SpeechToText{
		LanguageCode: languageCode,
		SampleReate:  sampleRate,
		ChannelCount: channelCount,
		Config:       config,
	}
}

func (stt SpeechToText) Start(ctx context.Context, r io.Reader) (speechpb.Speech_StreamingRecognizeClient, error) {
	config := stt.Config
	recognitionConfig := NewRecognitionConfig(config, stt.LanguageCode, int32(config.SampleRate), int32(config.ChannelCount))
	speechpbRecognitionConfig := NewSpeechpbRecognitionConfig(recognitionConfig)
	streamingRecognitionConfig := NewStreamingRecognitionConfig(speechpbRecognitionConfig, config.GcpSingleUtterance, config.GcpInterimResults)

	var opts []option.ClientOption
	credentialFile := config.GcpCredentialFile
	if credentialFile != "" {
		opts = append(opts, option.WithCredentialsFile(credentialFile))
	}

	client, err := speech.NewClient(ctx, opts...)
	if err != nil {
		return nil, &SuzuError{
			Code:    500,
			Message: err.Error(),
		}
	}
	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
		return nil, &SuzuError{
			Code:    500,
			Message: err.Error(),
		}
	}

	if err := stream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: streamingRecognitionConfig,
	}); err != nil {
		return nil, &SuzuError{
			Code:    500,
			Message: err.Error(),
		}
	}

	go func() {
		defer stream.CloseSend()
		for {
			buf := make([]byte, FrameSize)
			n, err := r.Read(buf)
			if err != nil {
				if errors.Is(err, io.EOF) {
					// TODO: エラー処理
					zlog.Info().Err(err).Send()
					return
				}
				zlog.Error().Err(err).Send()
				return
			}
			if n > 0 {
				audioContent := buf[:n]
				if err := stream.Send(&speechpb.StreamingRecognizeRequest{
					StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
						AudioContent: audioContent,
					},
				}); err != nil {
					if errors.Is(err, io.EOF) {
						// TODO: エラー処理
						zlog.Info().Err(err).Send()
						return
					}
					zlog.Error().Err(err).Send()
					return
				}
			}
		}
	}()

	return stream, nil
}

type RecognitionConfig struct {
	Encoding                            speechpb.RecognitionConfig_AudioEncoding
	SampleRateHertz                     int32
	AudioChannelCount                   int32
	EnableSeparateRecognitionPerChannel bool
	LanguageCode                        string
	AlternativeLanguageCodes            []string
	MaxAlternatives                     int32
	ProfanityFilter                     bool
	SpeechContexts                      []*speechpb.SpeechContext
	EnableWordTimeOffsets               bool
	EnableWordConfidence                bool
	EnableAutomaticPunctuation          bool
	EnableSpokenPunctuation             bool
	EnableSpokenEmojis                  bool
	Model                               string
	UseEnhanced                         bool
}

func NewRecognitionConfig(c Config, languageCode string, sampleRate, channelCount int32) RecognitionConfig {
	return RecognitionConfig{
		Encoding:                            speechpb.RecognitionConfig_OGG_OPUS,
		SampleRateHertz:                     sampleRate,
		AudioChannelCount:                   channelCount,
		EnableSeparateRecognitionPerChannel: c.GcpEnableSeparateRecognitionPerChannel,
		LanguageCode:                        languageCode,
		AlternativeLanguageCodes:            c.GcpAlternativeLanguageCodes,
		MaxAlternatives:                     c.GcpMaxAlternatives,
		ProfanityFilter:                     c.GcpProfanityFilter,
		SpeechContexts:                      []*speechpb.SpeechContext{
			// &speechpb.SpeechContext{
			// Phrases: []string{},
			// },
		},
		EnableWordTimeOffsets:      c.GcpEnableWordTimeOffsets,
		EnableWordConfidence:       c.GcpEnableWordConfidence,
		EnableAutomaticPunctuation: c.GcpEnableAutomaticPunctuation,
		EnableSpokenPunctuation:    c.GcpEnableSpokenPunctuation,
		EnableSpokenEmojis:         c.GcpEnableSpokenEmojis,
		Model:                      c.GcpModel,
		UseEnhanced:                c.GcpUseEnhanced,
	}
}

func NewSpeechpbRecognitionConfig(rc RecognitionConfig) *speechpb.RecognitionConfig {
	return &speechpb.RecognitionConfig{
		Encoding:                            rc.Encoding,
		SampleRateHertz:                     rc.SampleRateHertz,
		AudioChannelCount:                   rc.AudioChannelCount,
		EnableSeparateRecognitionPerChannel: rc.EnableSeparateRecognitionPerChannel,
		LanguageCode:                        rc.LanguageCode,
		AlternativeLanguageCodes:            rc.AlternativeLanguageCodes,
		MaxAlternatives:                     rc.MaxAlternatives,
		ProfanityFilter:                     rc.ProfanityFilter,
		SpeechContexts:                      rc.SpeechContexts,
		EnableWordTimeOffsets:               rc.EnableWordTimeOffsets,
		EnableWordConfidence:                rc.EnableWordConfidence,
		EnableAutomaticPunctuation:          rc.EnableAutomaticPunctuation,
		EnableSpokenPunctuation:             wrapperspb.Bool(rc.EnableSpokenPunctuation),
		EnableSpokenEmojis:                  wrapperspb.Bool(rc.EnableSpokenEmojis),
		Model:                               rc.Model,
		UseEnhanced:                         rc.UseEnhanced,
	}
}

func NewStreamingRecognitionConfig(recognitionConfig *speechpb.RecognitionConfig, singleUtterance, interimResults bool) *speechpb.StreamingRecognizeRequest_StreamingConfig {
	return &speechpb.StreamingRecognizeRequest_StreamingConfig{
		StreamingConfig: &speechpb.StreamingRecognitionConfig{
			Config:          recognitionConfig,
			SingleUtterance: singleUtterance,
			InterimResults:  interimResults,
		},
	}
}
