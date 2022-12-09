package suzu

import (
	"context"
	"fmt"
	"io"

	speech "cloud.google.com/go/speech/apiv1"
	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1"
)

type SpeechToText struct{}

func NewSpeechToText() SpeechToText {
	return SpeechToText{}
}

func (stt SpeechToText) Start(ctx context.Context, config Config, args HandlerArgs, r io.Reader) (speechpb.Speech_StreamingRecognizeClient, error) {
	recognitionConfig := NewRecognitionConfig(config, args)
	speechpbRecognitionConfig := NewSpeechpbRecognitionConfig(recognitionConfig)
	singleUtterance := config.GcpSingleUtterance
	interimResults := config.GcpInterimResults
	streamingRecognitionConfig := NewStreamingRecognitionConfig(speechpbRecognitionConfig, singleUtterance, interimResults)

	client, err := speech.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
		return nil, err
	}

	if err := stream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: streamingRecognitionConfig,
	}); err != nil {
		return nil, err
	}

	go func() {
		defer stream.CloseSend()
		for {
			buf := make([]byte, FrameSize)
			n, err := r.Read(buf)
			if err != nil {
				// TODO: エラー処理
				fmt.Println(err)
				return
			}
			if n > 0 {
				audioContent := buf[:n]
				if err := stream.Send(&speechpb.StreamingRecognizeRequest{
					StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
						AudioContent: audioContent,
					},
				}); err != nil {
					fmt.Println(err)
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
	// Adaptation:
	SpeechContexts             []*speechpb.SpeechContext
	EnableWordTimeOffsets      bool
	EnableWordConfidence       bool
	EnableAutomaticPunctuation bool
	// EnableSpokenPunctuation:
	// EnableSpokenEmojis:
	// DiarizationConfig:
	// Metadata:
	Model       string
	UseEnhanced bool
}

func NewRecognitionConfig(c Config, args HandlerArgs) RecognitionConfig {
	return RecognitionConfig{
		Encoding:                            speechpb.RecognitionConfig_OGG_OPUS,
		SampleRateHertz:                     int32(args.SampleRate),
		AudioChannelCount:                   int32(args.ChannelCount),
		EnableSeparateRecognitionPerChannel: c.GcpEnableSeparateRecognitionPerChannel,
		LanguageCode:                        args.LanguageCode,
		AlternativeLanguageCodes:            c.GcpAlternativeLanguageCodes,
		MaxAlternatives:                     c.GcpMaxAlternatives,
		ProfanityFilter:                     c.GcpProfanityFilter,
		// Adaptation:
		SpeechContexts: []*speechpb.SpeechContext{
			// &speechpb.SpeechContext{
			// Phrases: []string{},
			// },
		},
		EnableWordTimeOffsets:      c.GcpEnableWordTimeOffsets,
		EnableWordConfidence:       c.GcpEnableWordConfidence,
		EnableAutomaticPunctuation: c.GcpEnableAutomaticPunctuation,
		// EnableSpokenPunctuation:
		// EnableSpokenEmojis:
		// DiarizationConfig:
		// Metadata:
		Model:       c.GcpModel,
		UseEnhanced: c.GcpUseEnhanced,
	}
}

func NewSpeechpbRecognitionConfig(rc RecognitionConfig) *speechpb.RecognitionConfig {
	return &speechpb.RecognitionConfig{
		Encoding:                            rc.Encoding,
		SampleRateHertz:                     rc.SampleRateHertz,
		AudioChannelCount:                   rc.AudioChannelCount,
		EnableSeparateRecognitionPerChannel: rc.EnableSeparateRecognitionPerChannel,
		LanguageCode:                        rc.LanguageCode,
		//AlternativeLanguageCodes:            rc.AlternativeLanguageCodes,
		MaxAlternatives: rc.MaxAlternatives,
		ProfanityFilter: rc.ProfanityFilter,
		// Adaptation:
		SpeechContexts:        rc.SpeechContexts,
		EnableWordTimeOffsets: rc.EnableWordTimeOffsets,
		//EnableWordConfidence:       rc.EnableWordConfidence,
		EnableAutomaticPunctuation: rc.EnableAutomaticPunctuation,
		// EnableSpokenPunctuation:
		// EnableSpokenEmojis:
		// DiarizationConfig:
		// Metadata:
		Model:       rc.Model,
		UseEnhanced: rc.UseEnhanced,
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
