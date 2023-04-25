package suzu

import (
	"fmt"

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
	case "gcp", "test", "dump":
		return lang, nil
	}

	return "", fmt.Errorf("%w: %s", ErrUnsupportedService, serviceType)
}
