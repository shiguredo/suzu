package suzu

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pion/rtp/codecs"
	zlog "github.com/rs/zerolog/log"
)

const (
	FrameSize = 1024 * 10
)

var (
	// TODO: 分かりにくい場合はエラー名を変更する
	// このエラーの場合は再接続を試みる
	ErrServerDisconnected = fmt.Errorf("SERVER-DISCONNECTED")
)

type TranscriptionResult struct {
	Message string `json:"message,omitempty"`
	Error   error  `json:"error,omitempty"`
	Type    string `json:"type"`
}

// https://echo.labstack.com/cookbook/streaming-response/
// TODO(v): http/2 の streaming を使ってレスポンスを戻す方法を調べる

// https://github.com/herrberk/go-http2-streaming/blob/master/http2/server.go
// 受信時はくるくるループを回す
func (s *Server) createSpeechHandler(serviceType string, f serviceHandler) echo.HandlerFunc {
	return func(c echo.Context) error {
		zlog.Debug().Msg("CONNECTING")
		// http/2 じゃなかったらエラー
		if c.Request().ProtoMajor != 2 {
			zlog.Error().
				Msg("INVALID-HTTP-PROTOCOL")
			return echo.NewHTTPError(http.StatusBadRequest)
		}

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
		defer func() {
			zlog.Debug().
				Str("channel_id", h.SoraChannelID).
				Str("connection_id", h.SoraConnectionID).
				Msg("DISCONNECTED")
		}()

		languageCode, err := GetLanguageCode(serviceType, h.SoraAudioStreamingLanguageCode, nil)
		if err != nil {
			zlog.Error().
				Err(err).
				Str("CHANNEL-ID", h.SoraChannelID).
				Str("CONNECTION-ID", h.SoraConnectionID).
				Send()
			return echo.NewHTTPError(http.StatusInternalServerError)
		}

		zlog.Debug().
			Str("channel_id", h.SoraChannelID).
			Str("connection_id", h.SoraConnectionID).
			Str("language_code", h.SoraAudioStreamingLanguageCode).
			Msg("CONNECTED")

		c.Response().Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		c.Response().WriteHeader(http.StatusOK)

		ctx := c.Request().Context()
		// TODO: context.WithCancelCause(ctx) に変更する
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// TODO: ヘッダから取得する
		sampleRate := uint32(s.config.SampleRate)
		channelCount := uint16(s.config.ChannelCount)

		args := NewHandlerArgs(*s.config, sampleRate, channelCount, h.SoraChannelID, h.SoraConnectionID, languageCode)

		d := time.Duration(args.Config.TimeToWaitForOpusPacketMs) * time.Millisecond
		r, err := readerWithSilentPacketFromOpusReader(d, c.Request().Body)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError)
		}

		retryCount := 0

		for {
			zlog.Info().
				Str("CHANNEL-ID", h.SoraChannelID).
				Str("CONNECTION-ID", h.SoraConnectionID).
				Msg("NEW-REQUEST")

			oggReader, oggWriter := io.Pipe()

			go func() {
				defer oggWriter.Close()
				if err := opus2ogg(ctx, r, oggWriter, sampleRate, channelCount, *s.config); err != nil {
					oggWriter.CloseWithError(err)
					return
				}
			}()

			reader, err := f(ctx, oggReader, args)
			if err != nil {
				zlog.Error().
					Err(err).
					Str("CHANNEL-ID", h.SoraChannelID).
					Str("CONNECTION-ID", h.SoraConnectionID).
					Send()
				// TODO: エラー内容で status code を変更する
				return echo.NewHTTPError(http.StatusInternalServerError)
			}
			defer reader.Close()

			for {
				buf := make([]byte, FrameSize)
				n, err := reader.Read(buf)
				if err != nil {
					if errors.Is(err, io.EOF) {
						return c.NoContent(http.StatusOK)
					} else if err.Error() == "failed to read audio, client disconnected" {
						// TODO: エラーレベルを見直す
						zlog.Error().
							Err(err).
							Str("CHANNEL-ID", h.SoraChannelID).
							Str("CONNECTION-ID", h.SoraConnectionID).
							Send()
						return echo.NewHTTPError(499)
					} else if errors.Is(err, ErrServerDisconnected) {
						retryCount += 1

						zlog.Debug().
							Err(err).
							Str("CHANNEL-ID", h.SoraChannelID).
							Str("CONNECTION-ID", h.SoraConnectionID).
							Int("RETRY-COUNT", retryCount).
							Send()
						break
					}
					zlog.Error().
						Err(err).
						Str("CHANNEL-ID", h.SoraChannelID).
						Str("CONNECTION-ID", h.SoraConnectionID).
						Send()
					return echo.NewHTTPError(http.StatusInternalServerError)
				}

				if n > 0 {
					if _, err := c.Response().Write(buf[:n]); err != nil {
						zlog.Error().
							Err(err).
							Str("CHANNEL-ID", h.SoraChannelID).
							Str("CONNECTION-ID", h.SoraConnectionID).
							Send()
						return echo.NewHTTPError(http.StatusInternalServerError)
					}
					c.Response().Flush()
				}
			}
		}
	}
}

type HandlerArgs struct {
	Config           Config
	SoraChannelID    string
	SoraConnectionID string
	SampleRate       uint32
	ChannelCount     uint16
	LanguageCode     string
}

func NewHandlerArgs(config Config, sampleRate uint32, channelCount uint16, soraChannelID, soraConnectionID, languageCode string) HandlerArgs {
	return HandlerArgs{
		Config:           config,
		SampleRate:       sampleRate,
		ChannelCount:     channelCount,
		SoraChannelID:    soraChannelID,
		SoraConnectionID: soraConnectionID,
		LanguageCode:     languageCode,
	}
}

func opus2ogg(ctx context.Context, opusReader io.Reader, oggWriter io.Writer, sampleRate uint32, channelCount uint16, c Config) error {
	o, err := NewWith(oggWriter, sampleRate, channelCount)
	if err != nil {
		if w, ok := oggWriter.(*io.PipeWriter); ok {
			w.CloseWithError(err)
		}
		return err
	}
	defer o.Close()

	if err := o.writeHeaders(); err != nil {
		if w, ok := oggWriter.(*io.PipeWriter); ok {
			w.CloseWithError(err)
		}
		return err
	}

	for {
		buf := make([]byte, FrameSize)
		n, err := opusReader.Read(buf)
		if err != nil {
			if w, ok := oggWriter.(*io.PipeWriter); ok {
				w.CloseWithError(err)
			}
			return err
		}
		if n > 0 {
			opus := codecs.OpusPacket{}
			_, err := opus.Unmarshal(buf[:n])
			if err != nil {
				if w, ok := oggWriter.(*io.PipeWriter); ok {
					w.CloseWithError(err)
				}
				return err
			}

			if err := o.Write(&opus); err != nil {
				if w, ok := oggWriter.(*io.PipeWriter); ok {
					w.CloseWithError(err)
				}
				return err
			}
		}
	}
}

func readerWithSilentPacketFromOpusReader(d time.Duration, opusReader io.Reader) (io.Reader, error) {
	type reqeust struct {
		Payload []byte
		Error   error
	}

	r, w := io.Pipe()
	ch := make(chan reqeust)

	go func() {
		defer close(ch)

		for {
			buf := make([]byte, FrameSize)
			n, err := opusReader.Read(buf)
			if err != nil {
				ch <- reqeust{
					Error: err,
				}
				return
			}

			if n > 0 {
				ch <- reqeust{
					Payload: buf[:n],
				}
			}
		}
	}()

	timer := time.NewTimer(d)
	go func() {
		defer func() {
			if !timer.Stop() {
				<-timer.C
			}
		}()

		var payload []byte
		for {
			select {
			case <-timer.C:
				payload = silentPacket()
			case req := <-ch:
				if err := req.Error; err != nil {
					w.CloseWithError(err)
					return
				}
				payload = req.Payload
			}

			if _, err := w.Write(payload); err != nil {
				w.CloseWithError(err)
				return
			}
			timer.Reset(d)
		}
	}()

	return r, nil
}

func silentPacket() []byte {
	return []byte{252, 255, 254}
}
