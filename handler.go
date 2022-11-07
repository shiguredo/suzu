package suzu

import (
	"context"
	"encoding/json"
	"errors"
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
func (s *Server) createSpeechHandler(f func(context.Context, io.Reader, HandlerArgs) (<-chan Response, error)) echo.HandlerFunc {
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

		// resultCh は関数内で閉じる想定
		resultCh, err := f(ctx, c.Request().Body, args)
		if err != nil {
			zlog.Error().Err(err).Str("CHANNEL-ID", h.SoraChannelID).Str("CONNECTION-ID", h.SoraConnectionID).Send()
			// TODO: status code
			return echo.NewHTTPError(http.StatusInternalServerError)
		}

		enc := json.NewEncoder(c.Response())

		for result := range resultCh {
			if err := result.Error; err != nil {
				if errors.Is(err, io.EOF) {
					return c.NoContent(http.StatusOK)
				} else if err.Error() == "client disconnected" {
					return echo.NewHTTPError(499)
				}
				zlog.Error().Err(err).Str("CHANNEL-ID", h.SoraChannelID).Str("CONNECTION-ID", h.SoraConnectionID).Send()
				return echo.NewHTTPError(http.StatusInternalServerError)
			}

			if err := enc.Encode(result); err != nil {
				zlog.Error().Err(err).Str("CHANNEL-ID", h.SoraChannelID).Str("CONNECTION-ID", h.SoraConnectionID).Send()
				return echo.NewHTTPError(http.StatusInternalServerError)
			}

			c.Response().Flush()
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
		return err
	}
	defer o.Close()

	if err := o.writeHeaders(); err != nil {
		return err
	}

	ch := make(chan Response)

	go func() {
		defer close(ch)

		// TODO: エラー処理
		t, err := time.ParseDuration(c.TimeToWaitForOpusPacket)
		if err != nil {
			ch <- Response{
				Error: err,
			}
			return
		}

		reader := NewReaderWithTimer(opusReader)
		resultCh := reader.Read(ctx, t)

		for res := range resultCh {
			if err := res.Error; err != nil {
				ch <- Response{
					Error: res.Error,
				}
				return
			}
			ch <- Response{
				ChannelID: res.ChannelID,
				Message:   res.Message,
			}
		}
	}()

	for response := range ch {
		if err := response.Error; err != nil {
			return err
		}

		opus := codecs.OpusPacket{}
		_, err := opus.Unmarshal([]byte(response.Message))
		if err != nil {
			// TODO: 停止または継続処理
			return err
		}

		if err := o.Write(&opus); err != nil {
			// TODO: 停止または継続処理
			return err
		}
	}

	return nil
}

type Response struct {
	ChannelID *string `json:"channel_id"`
	Message   string  `json:"message"`
	Error     error   `json:"error,omitempty"`
}

type ReaderWithTimer struct {
	R io.Reader
}

func NewReaderWithTimer(r io.Reader) ReaderWithTimer {
	return ReaderWithTimer{r}
}

func (r *ReaderWithTimer) Read(ctx context.Context, d time.Duration) <-chan Response {
	type res struct {
		Message []byte
		Error   error
	}

	responseCh := make(chan Response)
	resCh := make(chan res)

	go func() {
		defer close(resCh)

		buf := make([]byte, 4*1024)

		for {
			n, err := r.R.Read(buf)
			if err != nil {
				resCh <- res{
					Error: err,
				}
				return
			}
			if n > 0 {
				m := make([]byte, n)
				copy(m, buf[:n])
				resCh <- res{
					Message: m,
				}
			}
		}
	}()

	timer := time.NewTimer(d)

	go func() {
		defer close(responseCh)
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				responseCh <- Response{
					Message: string(silentPacket()),
				}
			case ret := <-resCh:
				if err := ret.Error; err != nil {
					responseCh <- Response{
						Error: err,
					}
					return
				}
				responseCh <- Response{
					Message: string(ret.Message),
				}
			}

			timer.Reset(d)
		}
	}()

	return responseCh
}

func silentPacket() []byte {
	return []byte{252, 255, 254}
}
