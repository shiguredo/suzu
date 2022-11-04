package suzu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
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

func AmazonTranscribeHandler(ctx context.Context, body io.Reader, args HandlerArgs) (<-chan Response, error) {
	ch := make(chan Response)
	r, w := io.Pipe()

	at := NewAmazonTranscribe(args.Config.AwsRegion, args.LanguageCode, int64(args.SampleRate), int64(args.ChannelCount), args.Config.AwsEnablePartialResultsStabilization, args.Config.AwsEnableChannelIdentification)

	go func() {
		defer w.Close()
		if err := opus2ogg(ctx, body, w, args.SampleRate, args.ChannelCount, args.Config); err != nil {
			at.ResultCh <- TranscriptionResult{
				Error: err,
			}
			return
		}
	}()

	go func() {
		defer at.Close()
		if err := at.Start(ctx, args.Config, r); err != nil {
			at.ResultCh <- TranscriptionResult{
				Error: err,
			}
			return
		}
	}()

	go func() {
		defer close(ch)

		for tr := range at.ResultCh {
			if err := tr.Error; err != nil {
				ch <- Response{
					Error: err,
				}
				return
			}
			ch <- Response{
				ChannelID: tr.ChannelID,
				Message:   string(tr.Message),
			}
		}
	}()

	return ch, nil
}

func TestHandler(ctx context.Context, body io.Reader, args HandlerArgs) (<-chan Response, error) {
	ch := handleTest(ctx, body, args.Config)
	return ch, nil
}

func PacketDumpHandler(ctx context.Context, body io.Reader, args HandlerArgs) (<-chan Response, error) {
	ch := handlePacketDump(ctx, args.Config.DumpFile, body, args.SoraChannelID, args.SoraConnectionID, args.LanguageCode, args.SampleRate, args.ChannelCount)
	return ch, nil
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

func handleTest(ctx context.Context, r io.Reader, c Config) <-chan Response {
	ch := make(chan Response)

	go func() {
		defer close(ch)

		// TODO: エラー処理
		t, _ := time.ParseDuration(c.TimeToWaitForOpusPacket)

		reader := NewReaderWithTimer(r)
		resultCh := reader.Read(ctx, t)

		for {
			select {
			case <-ctx.Done():
				return
			case res := <-resultCh:
				if err := res.Error; err != nil {
					ch <- Response{
						Error: res.Error,
					}
					return
				}
				ch <- Response{
					ChannelID: &[]string{"ch_0"}[0],
					Message:   fmt.Sprintf("n: %d", len(res.Message)),
				}
			}

		}
	}()

	return ch
}

type dump struct {
	Timestamp    int64  `json:"timestamp"`
	ChannelID    string `json:"channel_id"`
	ConnectionID string `json:"connection_id"`
	LanguageCode string `json:"language_code"`
	SampleRate   uint32 `json:"sample_rate"`
	ChannelCount uint16 `json:"channel_count"`
	Payload      []byte `json:"payload"`
}

func handlePacketDump(ctx context.Context, filename string, r io.Reader, channelID, connectionID, languageCode string, sampleRate uint32, channelCount uint16) <-chan Response {
	ch := make(chan Response)

	go func() {
		defer close(ch)

		f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case ch <- Response{
				Error: err,
			}:
				return
			}

		}
		defer f.Close()

		enc := json.NewEncoder(f)

		buf := make([]byte, 4*1024)

		for {
			n, err := r.Read(buf)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				case ch <- Response{
					Error: err,
				}:
					return
				}

			}
			if n > 0 {
				p := make([]byte, n)
				copy(p, buf[:n])
				dump := dump{
					Timestamp:    time.Now().UnixMilli(),
					ChannelID:    channelID,
					ConnectionID: connectionID,
					LanguageCode: languageCode,
					SampleRate:   sampleRate,
					ChannelCount: channelCount,
					Payload:      p,
				}
				if err := enc.Encode(dump); err != nil {
					select {
					case <-ctx.Done():
						return
					case ch <- Response{
						Error: err,
					}:
						return
					}
				}
				select {
				case <-ctx.Done():
					return
				case ch <- Response{
					ChannelID: &[]string{"ch_0"}[0],
					Message:   fmt.Sprintf("n: %d", n),
				}:
				}
			}
		}
	}()

	return ch
}

func (s *Server) healthcheckHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"revision": s.config.Revision,
	})
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

func SpeechToTextHandler(ctx context.Context, body io.Reader, args HandlerArgs) (<-chan Response, error) {
	ch := make(chan Response)

	r, w := io.Pipe()

	go func() {
		defer w.Close()
		if err := opus2ogg(ctx, body, w, args.SampleRate, args.ChannelCount, args.Config); err != nil {
			fmt.Println(err)
			return
		}
	}()

	stt := NewSpeechToText()
	stream, err := stt.Start(ctx, args.Config, args, r)
	if err != nil {
		return nil, err
	}

	interimResults := false
	go func() {
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				fmt.Println(err)
				return
			}
			if err != nil {
				fmt.Println(err)
				return
			}
			if err := resp.Error; err != nil {
				if err.Code == 3 || err.Code == 11 {
					fmt.Println(err)
				}
				fmt.Println(err)
				return
			}

			for _, result := range resp.Results {
				for _, alternative := range result.Alternatives {
					if args.Config.EnableWordConfidence {
						for _, word := range alternative.Words {
							fmt.Printf("%s, Confidence: %v\n", word.Word, word)
						}
					}
					transcript := alternative.Transcript
					if interimResults {
						ch <- Response{
							Message: transcript,
						}
					} else {
						if result.IsFinal {
							ch <- Response{
								Message: transcript,
							}
						}
					}
				}
			}
		}
	}()

	return ch, nil
}
