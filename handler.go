package suzu

import (
	"context"
	"encoding/binary"
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
	Reason  string `json:"reason,omitempty"`
	Type    string `json:"type"`
}

func NewSuzuErrorResponse(err error) TranscriptionResult {
	return TranscriptionResult{
		Type:   "error",
		Reason: err.Error(),
	}
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
				Str("channel_id", h.SoraChannelID).
				Str("connection_id", h.SoraConnectionID).
				Send()
			return echo.NewHTTPError(http.StatusInternalServerError)
		}

		zlog.Debug().
			Str("channel_id", h.SoraChannelID).
			Str("connection_id", h.SoraConnectionID).
			Str("language_code", h.SoraAudioStreamingLanguageCode).
			Msg("CONNECTED")

		c.Response().Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		// すぐにヘッダを送信したい場合はここで c.Response().Flush() を実行する

		ctx := c.Request().Context()
		// TODO: context.WithCancelCause(ctx) に変更する
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// TODO: ヘッダから取得する
		sampleRate := uint32(s.config.SampleRate)
		channelCount := uint16(s.config.ChannelCount)

		d := time.Duration(s.config.TimeToWaitForOpusPacketMs) * time.Millisecond
		r := NewOpusReader(*s.config, d, c.Request().Body)
		defer r.Close()

		serviceHandler, err := getServiceHandler(serviceType, *s.config, h.SoraChannelID, h.SoraConnectionID, sampleRate, channelCount, languageCode, onResultFunc)
		if err != nil {
			zlog.Error().
				Err(err).
				Str("channel_id", h.SoraChannelID).
				Str("connection_id", h.SoraConnectionID).
				Send()
			return echo.NewHTTPError(http.StatusInternalServerError)
		}

		// サーバへの接続・結果の送信処理
		// サーバへの再接続が期待できる限りは、再接続を試みる
		for {
			zlog.Info().
				Str("channel_id", h.SoraChannelID).
				Str("connection_id", h.SoraConnectionID).
				Int("retry_count", serviceHandler.GetRetryCount()).
				Msg("NEW-REQUEST")

			reader, err := serviceHandler.Handle(ctx, r)
			if err != nil {
				zlog.Error().
					Err(err).
					Str("channel_id", h.SoraChannelID).
					Str("connection_id", h.SoraConnectionID).
					Send()
				if err, ok := err.(*SuzuError); ok {
					if err.IsRetry() {
						if s.config.MaxRetry > serviceHandler.GetRetryCount() {
							serviceHandler.UpdateRetryCount()

							// 連続のリトライを避けるために少し待つ
							time.Sleep(time.Duration(s.config.RetryIntervalMs) * time.Millisecond)

							// リトライ対象のエラーのため、クライアントとの接続は切らずにリトライする
							continue
						}
					}
					// SuzuError の場合はその Status Code を返す
					return c.NoContent(err.Code)
				}
				// SuzuError 以外の場合は 500 を返す
				return echo.NewHTTPError(http.StatusInternalServerError, err)
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
							Str("channel_id", h.SoraChannelID).
							Str("connection_id", h.SoraConnectionID).
							Send()
						return err
					} else if errors.Is(err, ErrServerDisconnected) {
						if s.config.MaxRetry < 1 {
							// サーバから切断されたが再接続させない設定の場合
							zlog.Error().
								Err(err).
								Str("channel_id", h.SoraChannelID).
								Str("connection_id", h.SoraConnectionID).
								Send()
							return err
						}

						if s.config.MaxRetry > serviceHandler.GetRetryCount() {
							// サーバから切断されたが再度接続できる可能性があるため、接続を試みる

							serviceHandler.UpdateRetryCount()

							// TODO: 必要な場合は連続のリトライを避けるために少し待つ処理を追加する

							break
						} else {
							zlog.Error().
								Err(err).
								Str("channel_id", h.SoraChannelID).
								Str("connection_id", h.SoraConnectionID).
								Send()
							// max_retry を超えた場合は終了
							return c.NoContent(http.StatusOK)
						}
					}

					// サーバから切断されたが再度の接続が期待できない場合
					return err
				}

				// 1 度でも接続結果を受け取れた場合はリトライ回数をリセットする
				serviceHandler.ResetRetryCount()

				// メッセージが空でない場合はクライアントに結果を送信する
				if n > 0 {
					if _, err := c.Response().Write(buf[:n]); err != nil {
						zlog.Error().
							Err(err).
							Str("channel_id", h.SoraChannelID).
							Str("connection_id", h.SoraConnectionID).
							Send()
						return err
					}
					c.Response().Flush()
				}
			}
		}
	}
}

func readPacketWithHeader(reader io.Reader) (io.Reader, error) {
	r, w := io.Pipe()

	go func() {
		length := 0
		payloadLength := 0
		var payload []byte

		for {
			buf := make([]byte, 20+0xffff)
			n, err := reader.Read(buf)
			if err != nil {
				w.CloseWithError(err)
				return
			}

			payload = append(payload, buf[:n]...)
			length += n

			if length > 20 {
				// timestamp(64), sequence number(64), length(32)
				h := payload[0:20]
				p := payload[20:length]

				payloadLength = int(binary.BigEndian.Uint32(h[16:20]))

				if length == (20 + payloadLength) {
					if _, err := w.Write(p); err != nil {
						w.CloseWithError(err)
						return
					}
					payload = []byte{}
					length = 0
					continue
				}

				// payload が足りないのでさらに読み込む
				if length < (20 + payloadLength) {
					// 前の payload へ追加して次へ
					payload = append(payload, p...)
					continue
				}

				// 次の frame が含まれている場合
				if length > (20 + payloadLength) {
					if _, err := w.Write(p[:payloadLength]); err != nil {
						w.CloseWithError(err)
						return
					}
					// 次の payload 処理へ
					payload = p[payloadLength:]
					length = len(payload)

					// 次の payload がすでにある場合の処理
					for {
						if length > 20 {
							h = payload[0:20]
							p = payload[20:length]

							payloadLength = int(binary.BigEndian.Uint32(h[16:20]))

							// すでに次の payload が全てある場合
							if length == (20 + payloadLength) {
								if _, err := w.Write(p); err != nil {
									w.CloseWithError(err)
									return
								}
								payload = []byte{}
								length = 0
								continue
							}

							if length > (20 + payloadLength) {
								if _, err := w.Write(p[:payloadLength]); err != nil {
									w.CloseWithError(err)
									return
								}

								// 次の payload 処理へ
								payload = p[payloadLength:]
								length = len(payload)
								continue
							}
						} else {
							// payload が足りないので、次の読み込みへ
							break
						}
					}

					continue
				}
			} else {
				// ヘッダー分に足りなければ次の読み込みへ
				continue
			}
		}
	}()

	return r, nil
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

	var r io.Reader
	if c.AudioStreamingHeader {
		r, err = readPacketWithHeader(opusReader)
		if err != nil {
			return err
		}
	} else {
		r = opusReader
	}

	for {
		buf := make([]byte, FrameSize)
		n, err := r.Read(buf)
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

func NewOpusReader(c Config, d time.Duration, opusReader io.ReadCloser) io.ReadCloser {
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
				payload = silentPacket(c.AudioStreamingHeader)
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

func silentPacket(audioStreamingHeader bool) []byte {
	var packet []byte
	silentPacket := []byte{252, 255, 254}
	if audioStreamingHeader {
		t := time.Now().UTC()
		unixTime := make([]byte, 8)
		binary.BigEndian.PutUint64(unixTime, uint64(t.UnixMicro()))

		// 0 で固定
		seqNum := make([]byte, 8)

		length := make([]byte, 4)
		binary.BigEndian.PutUint32(length, uint32(len(silentPacket)))

		packet = append(unixTime, seqNum...)
		packet = append(packet, length...)
		packet = append(packet, silentPacket...)
	} else {
		packet = silentPacket
	}

	return packet
}
