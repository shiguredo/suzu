package suzu

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/transcribestreaming/types"
	"github.com/aws/aws-sdk-go/service/transcribestreamingservice"
)

var (
	ErrMissingAudioStreamingLanguageCode = fmt.Errorf("MISSING-SORA-AUDIO-STREAMING-LANGUAGE-CODE")
	ErrUnsupportedLanguageCode           = fmt.Errorf("UNSUPPORTED-LANGUAGE-CODE")
	ErrUnsupportedService                = fmt.Errorf("UNSUPPORTED-SERVICE")
)

func GetLanguageCode(serviceType, lang string, f func(string) (string, error)) (string, error) {
	if lang == "" {
		return "", ErrMissingAudioStreamingLanguageCode
	}

	if f != nil {
		return f(lang)
	}

	switch serviceType {
	case "aws":
		for _, languageCode := range transcribestreamingservice.LanguageCode_Values() {
			if languageCode == lang {
				return languageCode, nil
			}
		}
		return "", fmt.Errorf("%w: %s", ErrUnsupportedLanguageCode, lang)
	case "awsv2":
		lc := new(types.LanguageCode)
		for _, languageCode := range lc.Values() {
			if languageCode == types.LanguageCode(lang) {
				return lang, nil
			}
		}
		return "", fmt.Errorf("%w: %s", ErrUnsupportedLanguageCode, lang)
	case "gcp", "test", "dump":
		return lang, nil
	}

	return "", fmt.Errorf("%w: %s", ErrUnsupportedService, serviceType)
}
