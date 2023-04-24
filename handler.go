package suzu

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
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

func getServiceHandler(serviceType string, config Config, channelID, connectionID string, sampleRate uint32, channelCount uint16, languageCode string, onResultFunc any) (serviceHandlerInterface, error) {
	newHandlerFunc, err := NewServiceHandlerFuncs.get(serviceType)
	if err != nil {
		return nil, err
	}

	return (*newHandlerFunc)(config, channelID, connectionID, sampleRate, channelCount, languageCode, onResultFunc), nil
}

// https://echo.labstack.com/cookbook/streaming-response/
// TODO(v): http/2 の streaming を使ってレスポンスを戻す方法を調べる

// https://github.com/herrberk/go-http2-streaming/blob/master/http2/server.go
// 受信時はくるくるループを回す
func (s *Server) createSpeechHandler(serviceType string, onResultFunc func(context.Context, io.WriteCloser, string, string, string, any) error) echo.HandlerFunc {
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
			Str("CHANNEL-ID", h.SoraChannelID).
			Str("CONNECTION-ID", h.SoraConnectionID).
			Str("LANGUAGE-CODE", h.SoraAudioStreamingLanguageCode).
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

		d := time.Duration(s.config.TimeToWaitForOpusPacketMs) * time.Millisecond
		r := NewOpusReader(d, c.Request().Body)
		defer r.Close()

		serviceHandler, err := getServiceHandler(serviceType, *s.config, h.SoraChannelID, h.SoraConnectionID, sampleRate, channelCount, languageCode, onResultFunc)
		if err != nil {
			zlog.Error().
				Err(err).
				Str("CHANNEL-ID", h.SoraChannelID).
				Str("CONNECTION-ID", h.SoraConnectionID).
				Send()
			return echo.NewHTTPError(http.StatusInternalServerError)
		}

		retryCount := 0

		// サーバへの接続・結果の送信処理
		// サーバへの再接続が期待できる限りは、再接続を試みる
		for {
			zlog.Info().
				Str("CHANNEL-ID", h.SoraChannelID).
				Str("CONNECTION-ID", h.SoraConnectionID).
				Msg("NEW-REQUEST")

			reader, err := serviceHandler.Handle(ctx, r)
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
					} else if strings.Contains(err.Error(), "client disconnected") {
						// http.http2errClientDisconnected を使用したエラーの場合は、クライアントから切断されたため終了
						// TODO: エラーレベルを見直す
						zlog.Error().
							Err(err).
							Str("CHANNEL-ID", h.SoraChannelID).
							Str("CONNECTION-ID", h.SoraConnectionID).
							Send()
						return echo.NewHTTPError(499)
					} else if errors.Is(err, ErrServerDisconnected) {
						if *s.config.Retry {
							// サーバから切断されたが再度接続できる可能性があるため、接続を試みる
							retryCount += 1

							zlog.Debug().
								Err(err).
								Str("CHANNEL-ID", h.SoraChannelID).
								Str("CONNECTION-ID", h.SoraConnectionID).
								Int("RETRY-COUNT", retryCount).
								Send()
							break
						} else {
							// サーバから切断されたが再接続させない設定の場合
							zlog.Error().
								Err(err).
								Str("CHANNEL-ID", h.SoraChannelID).
								Str("CONNECTION-ID", h.SoraConnectionID).
								Send()
							return echo.NewHTTPError(http.StatusInternalServerError)
						}
					}

					zlog.Error().
						Err(err).
						Str("CHANNEL-ID", h.SoraChannelID).
						Str("CONNECTION-ID", h.SoraConnectionID).
						Send()
					// サーバから切断されたが再度の接続が期待できない場合、または、想定外のエラーの場合は InternalServerError
					return echo.NewHTTPError(http.StatusInternalServerError)
				}

				// メッセージが空でない場合はクライアントに結果を送信する
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

type opusRequest struct {
	Payload []byte
	Error   error
}

func readPacket(opusReader io.Reader) chan opusRequest {
	ch := make(chan opusRequest)

	go func() {
		defer close(ch)

		for {
			buf := make([]byte, FrameSize)
			n, err := opusReader.Read(buf)
			if err != nil {
				ch <- opusRequest{
					Error: err,
				}
				return
			}

			if n > 0 {
				ch <- opusRequest{
					Payload: buf[:n],
				}
			}

		}
	}()

	return ch
}

func NewOpusReader(d time.Duration, opusReader io.ReadCloser) io.ReadCloser {
	r, w := io.Pipe()

	ch := readPacket(opusReader)

	go func() {
		timer := time.NewTimer(d)
		defer func() {
			if !timer.Stop() {
				<-timer.C
			}
		}()

		for {
			var payload []byte
			select {
			case <-timer.C:
				payload = silentPacket()
			case req, ok := <-ch:
				if !ok {
					w.Close()
					return
				}
				if err := req.Error; err != nil {
					w.CloseWithError(err)
					return
				}

				payload = req.Payload
			}

			if _, err := w.Write(payload); err != nil {
				w.CloseWithError(err)
				opusReader.Close()
			}

			timer.Reset(d)
		}
	}()

	return r
}

func silentPacket() []byte {
	return []byte{252, 255, 254}
}
