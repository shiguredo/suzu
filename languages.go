package suzu

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/transcribestreamingservice"
)

var (
	ErrMissinuAudioStreamingLanguageCode = fmt.Errorf("MISSING-SORA-AUDIO-STREAMING-LANGUAGE-CODE")
	ErrUnsupportedLanguageCode           = fmt.Errorf("UNSUPPORTED-LANGUAGE-CODE")
)

func GetLanguageCode(lang string, f func(string) (string, error)) (string, error) {
	if lang == "" {
		return "", ErrMissinuAudioStreamingLanguageCode
	}

	if f != nil {
		return f(lang)
	}

	for _, languageCode := range transcribestreamingservice.LanguageCode_Values() {
		if languageCode == lang {
			return languageCode, nil
		}
	}

	return "", fmt.Errorf("%w: %s", ErrUnsupportedLanguageCode, lang)
}
