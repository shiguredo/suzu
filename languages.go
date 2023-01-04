package suzu

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/transcribestreamingservice"
)

var (
	ErrMissinuAudioStreamingLanguageCode = fmt.Errorf("MISSING-SORA-AUDIO-STREAMING-LANGUAGE-CODE")
	ErrUnsupportedLanguageCode           = fmt.Errorf("UNSUPPORTED-LANGUAGE-CODE")
	ErrUnsupportedService                = fmt.Errorf("UNSUPPORTED-SERVICE")
)

func GetLanguageCode(serviceType, lang string, f func(string) (string, error)) (string, error) {
	if lang == "" {
		return "", ErrMissinuAudioStreamingLanguageCode
	}

	if f != nil {
		return f(lang)
	}

	if serviceType == "aws" {
		for _, languageCode := range transcribestreamingservice.LanguageCode_Values() {
			if languageCode == lang {
				return languageCode, nil
			}
		}
	} else if serviceType == "gcp" {
		return lang, nil
	} else {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedService, serviceType)
	}

	return "", fmt.Errorf("%w: %s, %s", ErrUnsupportedLanguageCode, serviceType, lang)
}
