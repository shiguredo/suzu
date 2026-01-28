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

		// 読み込み時の追加処理のオプション関数指定
		packetReaderOptions := []packetReaderOption{}
		if !s.config.DisableSilentPacket {
			packetReaderOptions = append(packetReaderOptions, optionSilentPacket)
		}
		if s.config.AudioStreamingHeader {
			packetReaderOptions = append(packetReaderOptions, optionReadPacketWithHeader)
		}

		opusCh := newOpusChannel(ctx, *s.config, c.Request().Body, packetReaderOptions)

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
				// EOF の場合は、クライアントとの接続が切れたため終了
				if errors.Is(err, io.EOF) {
					return c.NoContent(http.StatusOK)
				}

				// StopAudioStreaming API を実行せずに接続が切れた場合など
				if errors.Is(err, context.Canceled) {
					return c.NoContent(http.StatusOK)
				}

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

func opus2ogg(ctx context.Context, opusCh chan opus, sampleRate uint32, channelCount uint16, c Config, header soraHeader) (io.ReadCloser, error) {
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

		// 最初の音声データの受信時に、Ogg ヘッダを書き込み、その後に音声データを書き込む
		select {
		case <-ctx.Done():
			oggWriter.CloseWithError(ctx.Err())
			return
		case opus, ok := <-opusCh:
			if !ok {
				oggWriter.CloseWithError(io.EOF)
				return
			}

			if opus.Err != nil {
				oggWriter.CloseWithError(opus.Err)
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

		// 以降は受信した音声データを書き込む
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

				if opus.Err != nil {
					oggWriter.CloseWithError(opus.Err)
					return
				}

				opusPacket := codecs.OpusPacket{}
				_, err := opusPacket.Unmarshal(opus.Payload)
				if err != nil {
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

// opus データを格納する構造体
type opus struct {
	Payload []byte
	Err     error
}

// 受信した Payload を読み込み、オプション関数に従った opus データを受け取る channel を返す
func newOpusChannel(ctx context.Context, c Config, r io.ReadCloser, fs []packetReaderOption) chan opus {
	// 受信した Payload を読み込み、読み込んだデータを受け取る channel を返す
	packetCh := readPacket(ctx, r)
	opusCh := packetCh

	// 読み込み時の追加処理を順番に適用する
	for _, f := range fs {
		opusCh = f(ctx, c, opusCh)
	}

	return opusCh
}

// 受信した Payload を読み込み、読み込んだデータを受け取る channel を返す
func readPacket(ctx context.Context, opusReader io.Reader) chan opus {
	ch := make(chan opus)

	go func() {
		defer close(ch)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			buf := make([]byte, FrameSize)
			n, err := opusReader.Read(buf)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				case ch <- opus{Err: err}:
					return
				}
			}

			if n > 0 {
				payload := make([]byte, n)
				copy(payload, buf[:n])
				select {
				case <-ctx.Done():
					return
				case ch <- opus{Payload: payload}:
				}
			}
		}
	}()

	return ch
}

// パケット読み込み時のオプション関数の型定義
type packetReaderOption func(ctx context.Context, c Config, ch chan opus) chan opus

func optionSilentPacket(ctx context.Context, c Config, packetCh chan opus) chan opus {
	ch := make(chan opus)

	go func() {
		defer close(ch)

		if c.DisableSilentPacket {
			select {
			case <-ctx.Done():
				return
			default:
			}

			for {
				select {
				case <-ctx.Done():
					return
				case req, ok := <-packetCh:
					if !ok {
						return
					}
					// 受信したデータをそのまま送信する
					select {
					case <-ctx.Done():
						return
					case ch <- req:
					}
				}
			}
		} else {
			// 無音パケットを送信する間隔
			d := time.Duration(c.TimeToWaitForOpusPacketMs) * time.Millisecond

			timer := time.NewTimer(d)
			defer func() {
				if !timer.Stop() {
					<-timer.C
				}
			}()

			for {
				var opusPacket opus
				select {
				case <-timer.C:
					payload := silentPacket(c.AudioStreamingHeader)
					opusPacket = opus{Payload: payload}
				case req, ok := <-packetCh:
					if !ok {
						return
					}

					opusPacket = req
				}

				// 受信したデータ、または、無音パケットを送信する
				select {
				case <-ctx.Done():
					return
				case ch <- opusPacket:
				}

				// 受信したらタイマーをリセットする
				timer.Reset(d)
			}
		}
	}()

	return ch
}

// パケット読み込み時のヘッダー処理オプション関数
func optionReadPacketWithHeader(ctx context.Context, c Config, packetCh chan opus) chan opus {
	ch := make(chan opus)

	go func() {
		defer close(ch)

		length := 0
		payloadLength := 0
		var payload []byte

		for {
			select {
			case <-ctx.Done():
				return
			case req, ok := <-packetCh:
				if !ok {
					return
				}
				if req.Err != nil {
					select {
					case <-ctx.Done():
						return
					case ch <- opus{Err: req.Err}:
						return
					}
				}

				packet := req.Payload
				payload = append(payload, packet...)
				length += len(packet)

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

				select {
				case <-ctx.Done():
					return
				case ch <- opus{Payload: p[:payloadLength]}:
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
					select {
					case <-ctx.Done():
						return
					case ch <- opus{Payload: p[:payloadLength]}:
					}

					// 残りの処理へ
					payload = p[payloadLength:]
					length = len(payload)
				}
			}
		}
	}()

	return ch
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

func opusChannelToIOReadCloser(ctx context.Context, ch <-chan opus) io.ReadCloser {
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
				if opus.Err != nil {
					w.CloseWithError(opus.Err)
					return
				}

				_, err := w.Write(opus.Payload)
				if err != nil {
					w.CloseWithError(err)
					return
				}
			}
		}
	}()

	return r
}

// receiveFirstAudioData は、音声データを 1 つだけ受信するための関数です
func receiveFirstAudioData(r io.ReadCloser) ([]byte, error) {
	for {
		buf := make([]byte, FrameSize)
		n, err := r.Read(buf)
		if err != nil {
			return nil, err
		}

		if n > 0 {
			// データを取得できた場合は終了
			return buf[:n], nil
		}
	}
}
