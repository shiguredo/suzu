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

// https://echo.labstack.com/cookbook/streaming-response/
// TODO(v): http/2 の streaming を使ってレスポンスを戻す方法を調べる

// https://github.com/herrberk/go-http2-streaming/blob/master/http2/server.go
// 受信時はくるくるループを回す
func (s *Server) createSpeechHandler(f func(context.Context, io.Reader, HandlerArgs) (*io.PipeReader, error)) echo.HandlerFunc {
	return func(c echo.Context) error {
		zlog.Debug().Msg("CONNECTING")
		// http/2 じゃなかったらエラー
		if c.Request().ProtoMajor != 2 {
			zlog.Error().Msg("INVALID-HTTP-PROTOCOL")
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
			zlog.Error().Err(err).Msg("INVALID-HEADER")
			return echo.NewHTTPError(http.StatusBadRequest)
		}

		languageCode, err := GetLanguageCode(h.SoraAudioStreamingLanguageCode, nil)
		if err != nil {
			zlog.Error().Err(err).Str("CHANNEL-ID", h.SoraChannelID).Str("CONNECTION-ID", h.SoraConnectionID).Send()
			return echo.NewHTTPError(http.StatusInternalServerError)
		}

		zlog.Debug().Msg("CONNECTED")

		c.Response().Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		c.Response().WriteHeader(http.StatusOK)

		ctx := c.Request().Context()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// TODO: ヘッダから取得する
		sampleRate := uint32(s.config.SampleRate)
		channelCount := uint16(s.config.ChannelCount)

		args := NewHandlerArgs(*s.config, sampleRate, channelCount, h.SoraChannelID, h.SoraConnectionID, languageCode)

		reader, err := f(ctx, c.Request().Body, args)
		if err != nil {
			zlog.Error().Err(err).Str("CHANNEL-ID", h.SoraChannelID).Str("CONNECTION-ID", h.SoraConnectionID).Send()
			// TODO: status code
			return echo.NewHTTPError(http.StatusInternalServerError)
		}
		defer reader.Close()

		// if _, err := io.Copy(c.Response(), reader); err != nil {
		// if err.Error() == "client disconnected" {
		// return echo.NewHTTPError(499)
		// }
		// zlog.Error().Err(err).Str("CHANNEL-ID", h.SoraChannelID).Str("CONNECTION-ID", h.SoraConnectionID).Send()
		// return echo.NewHTTPError(http.StatusInternalServerError)
		// }

		for {
			buf := make([]byte, FrameSize)
			n, err := reader.Read(buf)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				} else if err.Error() == "client disconnected" {
					zlog.Error().Err(err).Str("CHANNEL-ID", h.SoraChannelID).Str("CONNECTION-ID", h.SoraConnectionID).Send()
					return echo.NewHTTPError(499)
				}
				zlog.Error().Err(err).Str("CHANNEL-ID", h.SoraChannelID).Str("CONNECTION-ID", h.SoraConnectionID).Send()
				return echo.NewHTTPError(http.StatusInternalServerError)
			}

			if n > 0 {
				if _, err := c.Response().Write(buf[:n]); err != nil {
					zlog.Error().Err(err).Str("CHANNEL-ID", h.SoraChannelID).Str("CONNECTION-ID", h.SoraConnectionID).Send()
					return echo.NewHTTPError(http.StatusInternalServerError)
				}
				c.Response().Flush()
			}
		}

		return c.NoContent(http.StatusOK)
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

type Response struct {
	ChannelID *string `json:"channel_id"`
	Message   string  `json:"message"`
	Error     error   `json:"error,omitempty"`
}

func readerWithSilentPacketFromOpusReader(d time.Duration, opusReader io.Reader) (io.Reader, error) {
	type reqeust struct {
		Payload []byte
		Error   error
	}

	r, w := io.Pipe()
	ch := make(chan reqeust)

	go func() {
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
		for {
			select {
			case <-timer.C:
				if _, err := w.Write(silentPacket()); err != nil {
					w.CloseWithError(err)
					return
				}
			case req := <-ch:
				if err := req.Error; err != nil {
					w.CloseWithError(err)
					if !timer.Stop() {
						<-timer.C
					}
					return
				}
				if _, err := w.Write(req.Payload); err != nil {
					w.CloseWithError(err)
					if !timer.Stop() {
						<-timer.C
					}
					return
				}
			}
			timer.Reset(d)
		}
	}()

	return r, nil
}

func silentPacket() []byte {
	return []byte{252, 255, 254}
}

type serviceHandler func(ctx context.Context, conn io.Reader, args HandlerArgs) (*io.PipeReader, error)

type serviceHandlers struct {
	Handlers map[string]serviceHandler
}

func NewServiceHandlers() serviceHandlers {
	return serviceHandlers{
		Handlers: make(map[string]serviceHandler),
	}
}

var ServiceHandlers = NewServiceHandlers()

func (sh *serviceHandlers) registerHandler(name string, handler serviceHandler) {
	sh.Handlers[name] = handler
}

func (sh *serviceHandlers) getServiceHandler(name string) (serviceHandler, error) {
	h, ok := sh.Handlers[name]
	if !ok {
		return nil, fmt.Errorf("UNREGISTERED-SERVICE: %s", name)
	}

	return h, nil
}

func (sh *serviceHandlers) GetNames() []string {
	var names []string
	for name := range sh.Handlers {
		names = append(names, name)
	}

	return names
}
