package suzu

import (
	"net/http"

	"github.com/labstack/echo/v4"
	zlog "github.com/rs/zerolog/log"
)

type suzuContext struct {
	Config *Config

	SampleRate   uint32
	ChannelCount uint16

	ChannelID    string
	ConnectionID string
	LanguageCode string

	echo.Context
}

func NewSuzuContext(c echo.Context, config Config, channelID, connectionID, languageCode string, sampleRate uint32, channelCount uint16) *suzuContext {
	return &suzuContext{
		Config: &config,

		SampleRate:   sampleRate,
		ChannelCount: channelCount,

		ChannelID:    channelID,
		ConnectionID: connectionID,
		LanguageCode: languageCode,

		Context: c,
	}
}

func HTTPVersionValidation(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// http/2 じゃなかったらエラー
		if c.Request().ProtoMajor != 2 {
			zlog.Error().
				Msg("INVALID-HTTP-PROTOCOL")
			return echo.NewHTTPError(http.StatusBadRequest)
		}
		return next(c)
	}
}

func (s *Server) SuzuContext(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		h := struct {
			SoraChannelID string `header:"Sora-Channel-Id"`
			// SoraSessionID       string `header:"sora-session-id"`
			// SoraClientID        string `header:"sora-client-id"`
			SoraConnectionID string `header:"sora-connection-id"`
			// SoraAudioCodecType  string `header:"sora-audio-codec-type"`
			// SoraAudioSampleRate int64  `header:"sora-audio-sample-rate"`
			SoraAudioStreamingLanguageCode string `header:"sora-audio-streaming-language-code"`
		}{}
		if err := (&echo.DefaultBinder{}).BindHeaders(c, &h); err != nil {
			zlog.Error().
				Err(err).
				Msg("INVALID-HEADER")
			return echo.NewHTTPError(http.StatusBadRequest)
		}

		sampleRate := uint32(s.config.SampleRate)
		channelCount := uint16(s.config.ChannelCount)

		sc := NewSuzuContext(c, *s.config, h.SoraChannelID, h.SoraConnectionID, h.SoraAudioStreamingLanguageCode, sampleRate, channelCount)

		return next(sc)
	}
}
