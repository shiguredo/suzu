package suzu

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pion/rtp/codecs"
	zlog "github.com/rs/zerolog/log"
)

const (
	FrameSize        = 1024 * 10
	HeaderLength     = 20
	MaxPayloadLength = 0xffff
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

type soraHeader struct {
	SoraChannelID string `header:"sora-channel-id"`
	SoraSessionID string `header:"sora-session-id"`
	// SoraClientID        string `header:"sora-client-id"`
	SoraConnectionID string `header:"sora-connection-id"`
	// SoraAudioCodecType  string `header:"sora-audio-codec-type"`
	// SoraAudioSampleRate int64  `header:"sora-audio-sample-rate"`
	SoraAudioStreamingLanguageCode string `header:"sora-audio-streaming-language-code"`
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

		h := soraHeader{}
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
		// すぐにヘッダを送信したいので c.Response().Flush() を実行する
		c.Response().Flush()

		ctx := c.Request().Context()
		// TODO: context.WithCancelCause(ctx) に変更する
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// TODO: ヘッダから取得する
		sampleRate := uint32(s.config.SampleRate)
		channelCount := uint16(s.config.ChannelCount)

		d := time.Duration(s.config.TimeToWaitForOpusPacketMs) * time.Millisecond
		opusReader := NewOpusReader(*s.config, d, c.Request().Body)
		defer opusReader.Close()

		var r io.Reader
		if s.config.AudioStreamingHeader {
			r = readPacketWithHeader(opusReader)
		} else {
			// ヘッダー処理なし
			r = opusReader
		}

		opusCh := readOpus(ctx, r)

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

			// リトライ時にこれ以降の処理のみを cancel する
			serviceHandlerCtx, cancelServiceHandler := context.WithCancel(ctx)
			defer cancelServiceHandler()

			reader, err := serviceHandler.Handle(serviceHandlerCtx, opusCh, h)
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

							// リトライ対象のエラーのため、クライアントとの接続は切らずにリトライする
							retryTimer := time.NewTimer(time.Duration(s.config.RetryIntervalMs) * time.Millisecond)

						retry:
							select {
							case <-retryTimer.C:
								zlog.Debug().
									Err(err).
									Str("channel_id", h.SoraChannelID).
									Str("connection_id", h.SoraConnectionID).
									Msg("retry")
								cancelServiceHandler()
								continue
							case _, ok := <-opusCh:
								if ok {
									// channel が閉じるか、または、リトライのタイマーが発火するまで繰り返す
									goto retry
								}
								retryTimer.Stop()
								zlog.Debug().
									Err(err).
									Str("channel_id", h.SoraChannelID).
									Str("connection_id", h.SoraConnectionID).
									Msg("retry interrupted")
								// リトライする前にクライアントとの接続でエラーが発生した場合は終了する
								return fmt.Errorf("%s", "retry interrupted")
							}
						}
					}
					// SuzuError の場合はその Status Code を返す
					return c.NoContent(err.Code)
				}

				// SuzuConfError の場合は、設定不備等で復帰が困難な場合を想定しているため、
				// type: error のエラーメッセージをクライアントに返して、リトライ対象から外す
				var suzuConfErr *SuzuConfError
				if errors.As(err, &suzuConfErr) {
					errMessage, err := json.Marshal(NewSuzuErrorResponse(suzuConfErr))
					if err != nil {
						zlog.Error().
							Err(err).
							Str("channel_id", h.SoraChannelID).
							Str("connection_id", h.SoraConnectionID).
							Send()
						return err
					}

					// 切断前にクライアントに type: error のエラーメッセージを返す
					if _, err := c.Response().Write(errMessage); err != nil {
						zlog.Error().
							Err(err).
							Str("channel_id", h.SoraChannelID).
							Str("connection_id", h.SoraConnectionID).
							Send()
						return err
					}
					c.Response().Flush()
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
						errs := err.(interface{ Unwrap() []error }).Unwrap()
						// 元の err を取得する
						err := errs[0]

						if s.config.MaxRetry < 1 {
							// サーバから切断されたが再接続させない設定の場合
							zlog.Error().
								Err(ErrServerDisconnected).
								Err(err).
								Str("channel_id", h.SoraChannelID).
								Str("connection_id", h.SoraConnectionID).
								Send()

							errMessage, err := json.Marshal(NewSuzuErrorResponse(err))
							if err != nil {
								zlog.Error().
									Err(err).
									Str("channel_id", h.SoraChannelID).
									Str("connection_id", h.SoraConnectionID).
									Send()
								return err
							}

							if _, err := c.Response().Write(errMessage); err != nil {
								zlog.Error().
									Err(err).
									Str("channel_id", h.SoraChannelID).
									Str("connection_id", h.SoraConnectionID).
									Send()
								return err
							}
							c.Response().Flush()
							return ErrServerDisconnected
						}

						if s.config.MaxRetry > serviceHandler.GetRetryCount() {
							// サーバから切断されたが再度接続できる可能性があるため、接続を試みる

							serviceHandler.UpdateRetryCount()

							// TODO: 必要な場合は連続のリトライを避けるために少し待つ処理を追加する
							cancelServiceHandler()
							break
						} else {
							zlog.Error().
								Err(err).
								Str("channel_id", h.SoraChannelID).
								Str("connection_id", h.SoraConnectionID).
								Send()

							errMessage, err := json.Marshal(NewSuzuErrorResponse(err))
							if err != nil {
								zlog.Error().
									Err(err).
									Str("channel_id", h.SoraChannelID).
									Str("connection_id", h.SoraConnectionID).
									Send()
								return err
							}

							if _, err := c.Response().Write(errMessage); err != nil {
								zlog.Error().
									Err(err).
									Str("channel_id", h.SoraChannelID).
									Str("connection_id", h.SoraConnectionID).
									Send()
								return err
							}
							c.Response().Flush()

							// max_retry を超えた場合は終了
							return c.NoContent(http.StatusOK)
						}
					} else {
						zlog.Debug().
							Err(err).
							Str("channel_id", h.SoraChannelID).
							Str("connection_id", h.SoraConnectionID).
							Send()

						orgErr := err

						// サーバから切断されたが再度の接続が期待できないため type: error のエラーメッセージをクライアントに送信する
						errMessage, err := json.Marshal(NewSuzuErrorResponse(err))
						if err != nil {
							zlog.Error().
								Err(err).
								Str("channel_id", h.SoraChannelID).
								Str("connection_id", h.SoraConnectionID).
								Send()
							return err
						}

						if _, err := c.Response().Write(errMessage); err != nil {
							zlog.Error().
								Err(err).
								Str("channel_id", h.SoraChannelID).
								Str("connection_id", h.SoraConnectionID).
								Send()
							return err
						}
						c.Response().Flush()

						return orgErr
					}
				}

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

func readPacketWithHeader(reader io.Reader) io.Reader {
	r, w := io.Pipe()

	go func() {
		length := 0
		payloadLength := 0
		var payload []byte

		for {
			buf := make([]byte, HeaderLength+MaxPayloadLength)
			n, err := reader.Read(buf)
			if err != nil {
				w.CloseWithError(err)
				return
			}

			payload = append(payload, buf[:n]...)
			length += n

			// ヘッダー分のデータが揃っていないので、次の読み込みへ
			if length < HeaderLength {
				continue
			}

			// timestamp(64), sequence number(64), length(32)
			h := payload[:HeaderLength]
			p := payload[HeaderLength:]

			payloadLength = int(binary.BigEndian.Uint32(h[16:HeaderLength]))

			// payload が足りないので、次の読み込みへ
			if length < (HeaderLength + payloadLength) {
				continue
			}

			if _, err := w.Write(p[:payloadLength]); err != nil {
				w.CloseWithError(err)
				return
			}

			payload = p[payloadLength:]
			length = len(payload)

			// 全てのデータを書き込んだ場合は次の読み込みへ
			if length == 0 {
				continue
			}

			// 次の frame が含まれている場合
			for {
				// ヘッダー分のデータが揃っていないので、次の読み込みへ
				if length < HeaderLength {
					break
				}

				h = payload[:HeaderLength]
				p = payload[HeaderLength:]

				payloadLength = int(binary.BigEndian.Uint32(h[16:HeaderLength]))

				// payload が足りないので、次の読み込みへ
				if length < (HeaderLength + payloadLength) {
					break
				}

				// データが足りているので payloadLength まで書き込む
				if _, err := w.Write(p[:payloadLength]); err != nil {
					w.CloseWithError(err)
					return
				}

				// 残りの処理へ
				payload = p[payloadLength:]
				length = len(payload)
			}
		}
	}()

	return r
}

func readOpus(ctx context.Context, reader io.Reader) chan opusChannel {
	opusCh := make(chan opusChannel)

	go func() {
		defer close(opusCh)

		for {
			select {
			case <-ctx.Done():
				opusCh <- opusChannel{
					Error: ctx.Err(),
				}
				return
			default:
				buf := make([]byte, FrameSize)
				n, err := reader.Read(buf)
				if err != nil {
					opusCh <- opusChannel{
						Error: err,
					}
					return
				}

				if n > 0 {
					opusCh <- opusChannel{
						Payload: buf[:n],
					}

				}
			}
		}
	}()

	return opusCh
}

func opus2ogg(ctx context.Context, opusCh chan opusChannel, sampleRate uint32, channelCount uint16, c Config, header soraHeader) (io.ReadCloser, error) {
	oggReader, oggWriter := io.Pipe()

	writers := []io.Writer{}

	var f *os.File
	if c.EnableOggFileOutput {
		fileName := fmt.Sprintf("%s-%s.ogg", header.SoraSessionID, header.SoraConnectionID)
		filePath := path.Join(c.OggDir, fileName)

		var err error
		f, err = os.Create(filePath)
		if err != nil {
			return nil, err
		}
		writers = append(writers, f)
	}
	writers = append(writers, oggWriter)

	multiWriter := io.MultiWriter(writers...)

	go func() {
		o, err := NewWithoutHeader(multiWriter, sampleRate, channelCount)
		if err != nil {
			oggWriter.CloseWithError(err)
			return
		}
		defer o.Close()

		if c.EnableOggFileOutput {
			o.fd = f
		}

		for {
			select {
			case <-ctx.Done():
				oggWriter.CloseWithError(ctx.Err())
				return
			case opus, ok := <-opusCh:
				if !ok {
					oggWriter.CloseWithError(io.EOF)
					return
				}

				if err := opus.Error; err != nil {
					oggWriter.CloseWithError(err)
					return
				}

				opusPacket := codecs.OpusPacket{}
				_, err := opusPacket.Unmarshal(opus.Payload)
				if err != nil {
					oggWriter.CloseWithError(err)
					return
				}

				// Ogg ヘッダを書き込む
				if err := o.WriteHeaders(); err != nil {
					oggWriter.CloseWithError(err)
					return
				}

				if err := o.Write(&opusPacket); err != nil {
					oggWriter.CloseWithError(err)
					return
				}
			}
		}
	}()

	return oggReader, nil
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
		if c.DisableSilentPacket {
			for {
				req, ok := <-ch
				if !ok {
					w.Close()
					return
				}
				if err := req.Error; err != nil {
					w.CloseWithError(err)
					return
				}

				if _, err := w.Write(req.Payload); err != nil {
					w.CloseWithError(err)
					opusReader.Close()
				}
			}
		} else {
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

type opusChannel struct {
	Payload []byte
	Error   error
}

func opusChannelToIOReadCloser(ctx context.Context, ch chan opusChannel) io.ReadCloser {
	r, w := io.Pipe()

	go func() {
		defer w.Close()

		for {
			select {
			case <-ctx.Done():
				w.CloseWithError(ctx.Err())
				return
			case opus, ok := <-ch:
				if !ok {
					w.CloseWithError(io.EOF)
					return
				}

				if err := opus.Error; err != nil {
					w.CloseWithError(err)
					return
				}

				if _, err := w.Write(opus.Payload); err != nil {
					w.CloseWithError(err)
					return
				}
			}
		}
	}()

	return r
}
