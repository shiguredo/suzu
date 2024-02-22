package suzu

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"testing/iotest"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

func readDumpFile(t *testing.T, filename string, d time.Duration) *io.PipeReader {
	t.Helper()

	f, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}

	scanner := bufio.NewScanner(f)

	r, w := io.Pipe()

	go func() {
		defer w.Close()
		defer f.Close()

		for scanner.Scan() {
			b := scanner.Bytes()
			s := struct {
				Payload []byte `json:"payload"`
			}{}
			if err := json.Unmarshal(b, &s); err != nil {
				t.Error(err.Error())
				break
			}

			if _, err := w.Write(s.Payload); err != nil {
				// 停止条件を r.Close() にしているため、io: read/write on closed pipe エラーは出力される
				//t.Log(err.Error())
				fmt.Println(err.Error())
				break
			}

			if d > 0 {
				time.Sleep(d)
			}
		}

		if err := scanner.Err(); err != nil {
			t.Error(err.Error())
			return
		}
	}()

	return r
}

func TestSpeechHandler(t *testing.T) {
	config := Config{
		Debug:                     true,
		ListenAddr:                "127.0.0.1",
		ListenPort:                48080,
		SkipBasicAuth:             true,
		LogStdout:                 true,
		DumpFile:                  "./test-dump.jsonl",
		TimeToWaitForOpusPacketMs: 500,
	}

	path := "/test"
	serviceType := "test"

	s, err := NewServer(&config, serviceType)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("success", func(t *testing.T) {
		r := readDumpFile(t, "testdata/dump.jsonl", 0)
		defer r.Close()

		e := echo.New()
		req := httptest.NewRequest("POST", path, r)
		req.Header.Set("sora-audio-streaming-language-code", "ja-JP")
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		h := s.createSpeechHandler(serviceType, nil)
		err := h(c)
		if assert.NoError(t, err) {
			assert.Equal(t, http.StatusOK, rec.Code)

			delim := []byte("\n")[0]
			var lastMessage string
			for {
				line, err := rec.Body.ReadBytes(delim)
				if err != nil {
					if !assert.ErrorIs(t, err, io.EOF) {
						t.Logf("READ-ERROR: %v\n", err)
					}
					break
				}
				var result TranscriptionResult
				if err := json.Unmarshal(line, &result); err != nil {
					t.Error(err)
				}

				assert.Equal(t, "test", result.Type)
				assert.NotEmpty(t, result.Message)
				lastMessage = result.Message
			}
			// TODO: テストデータは固定のため、すべてのメッセージを確認する
			assert.Equal(t, "n: 3", lastMessage)
		}

	})

	t.Run("unexpected http proto version", func(t *testing.T) {
		logger := log.Logger
		defer func() {
			log.Logger = logger
		}()

		pr, pw, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		log.Logger = zerolog.New(pw).With().Caller().Timestamp().Logger()

		r := readDumpFile(t, "testdata/dump.jsonl", 0)
		defer r.Close()

		e := echo.New()
		req := httptest.NewRequest("POST", path, r)
		req.Header.Set("sora-audio-streaming-language-code", "ja-JP")
		req.Proto = "HTTP/1.1"
		req.ProtoMajor = 1
		req.ProtoMinor = 1
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		h := s.createSpeechHandler(serviceType, nil)
		err = h(c)
		if assert.Error(t, err) {
			assert.Equal(t, http.StatusBadRequest, err.(*echo.HTTPError).Code)
		}

		pw.Close()

		var buf bytes.Buffer
		n, err := buf.ReadFrom(pr)
		if err != nil {
			t.Fatal(err)
		}
		assert.Contains(t, buf.String()[:n], "INVALID-HTTP-PROTOCOL")
	})

	t.Run("missing sora-audio-streaming-language-code header", func(t *testing.T) {
		logger := log.Logger
		defer func() {
			log.Logger = logger
		}()

		pr, pw, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		log.Logger = zerolog.New(pw).With().Caller().Timestamp().Logger()

		r := readDumpFile(t, "testdata/dump.jsonl", 0)
		defer r.Close()

		e := echo.New()
		req := httptest.NewRequest("POST", path, r)
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		h := s.createSpeechHandler(serviceType, nil)
		err = h(c)
		if assert.Error(t, err) {
			assert.Equal(t, http.StatusInternalServerError, err.(*echo.HTTPError).Code)
		}

		pw.Close()

		var buf bytes.Buffer
		n, err := buf.ReadFrom(pr)
		if err != nil {
			t.Fatal(err)
		}
		assert.Contains(t, buf.String()[:n], "MISSING-SORA-AUDIO-STREAMING-LANGUAGE-CODE")
	})

	t.Run("unsupported language code", func(t *testing.T) {
		logger := log.Logger
		defer func() {
			log.Logger = logger
		}()

		_, pw, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		log.Logger = zerolog.New(pw).With().Caller().Timestamp().Logger()

		r := readDumpFile(t, "testdata/dump.jsonl", 0)
		defer r.Close()

		e := echo.New()
		req := httptest.NewRequest("POST", path, r)
		req.Header.Set("sora-audio-streaming-language-code", "en-JP")
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		h := s.createSpeechHandler(serviceType, nil)
		err = h(c)
		if assert.NoError(t, err) {
			assert.Equal(t, http.StatusOK, rec.Code)
		}

		pw.Close()
	})

	t.Run("packet read error", func(t *testing.T) {
		logger := log.Logger
		defer func() {
			log.Logger = logger
		}()

		pr, pw, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		log.Logger = zerolog.New(pw).With().Caller().Timestamp().Logger()

		r := iotest.ErrReader(errors.New("packet read error"))

		e := echo.New()
		req := httptest.NewRequest("POST", path, r)
		req.Header.Set("sora-audio-streaming-language-code", "ja-JP")
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		h := s.createSpeechHandler(serviceType, nil)
		err = h(c)
		if assert.Error(t, err) {
			assert.Equal(t, "packet read error", err.Error())
		}

		pw.Close()

		var buf bytes.Buffer
		n, err := buf.ReadFrom(pr)
		if err != nil {
			t.Fatal(err)
		}
		assert.Contains(t, buf.String()[:n], "packet read error")
	})

	t.Run("silent packet", func(t *testing.T) {
		timeout := config.TimeToWaitForOpusPacketMs
		defer func() {
			config.TimeToWaitForOpusPacketMs = timeout
		}()

		config.TimeToWaitForOpusPacketMs = 100

		s, err := NewServer(&config, "aws")
		if err != nil {
			t.Fatal(err)
		}

		r := readDumpFile(t, "testdata/dump.jsonl", 150*time.Millisecond)
		defer r.Close()

		e := echo.New()
		req := httptest.NewRequest("POST", path, r)
		req.Header.Set("sora-audio-streaming-language-code", "ja-JP")
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		req2 := req.WithContext(ctx)
		rec := httptest.NewRecorder()
		c := e.NewContext(req2, rec)

		h := s.createSpeechHandler(serviceType, nil)
		err = h(c)
		if assert.NoError(t, err) {
			assert.Equal(t, http.StatusOK, rec.Code)

			delim := []byte("\n")[0]
			for {
				line, err := rec.Body.ReadBytes(delim)
				if err != nil {
					if !assert.ErrorIs(t, err, io.EOF) {
						t.Logf("READ-ERROR: %v\n", err)
					}
					break
				}
				var result TranscriptionResult
				if err := json.Unmarshal(line, &result); err != nil {
					t.Error(err)
				}

				assert.Equal(t, "test", result.Type)
				assert.NotEmpty(t, result.Message)
			}
		}

	})

	t.Run("onresultfunc error", func(t *testing.T) {
		r := readDumpFile(t, "testdata/dump.jsonl", 0)
		defer r.Close()

		e := echo.New()
		req := httptest.NewRequest("POST", path, r)
		req.Header.Set("sora-audio-streaming-language-code", "ja-JP")
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		h := s.createSpeechHandler(serviceType, func(ctx context.Context, w io.WriteCloser, chnanelID, connectionID, languageCode string, results any) error {
			return fmt.Errorf("ON-RESULT-ERROR")
		})
		err := h(c)
		if assert.Error(t, err) {
			assert.Equal(t, "ON-RESULT-ERROR", err.Error())
		}

	})

	t.Run("stream error", func(t *testing.T) {
		r := readDumpFile(t, "testdata/dump.jsonl", 0)
		defer r.Close()

		e := echo.New()
		req := httptest.NewRequest("POST", path, r)
		req.Header.Set("sora-audio-streaming-language-code", "ja-JP")
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		h := s.createSpeechHandler(serviceType, func(ctx context.Context, w io.WriteCloser, chnanelID, connectionID, languageCode string, results any) error {
			go func() {
				defer w.Close()

				encoder := json.NewEncoder(w)
				if err := encoder.Encode(NewSuzuErrorResponse(fmt.Errorf("STREAM-ERROR"))); err != nil {
					return
				}
			}()

			return nil
		})
		err := h(c)
		if assert.NoError(t, err) {
			assert.Equal(t, http.StatusOK, rec.Code)

			delim := []byte("\n")[0]
			for {
				line, err := rec.Body.ReadBytes(delim)
				if err != nil {
					assert.ErrorIs(t, err, io.EOF)
					break
				}

				var result TranscriptionResult
				if err := json.Unmarshal(line, &result); err != nil {
					assert.ErrorIs(t, err, io.EOF)
				}

				assert.Equal(t, "error", result.Type)
				if assert.NotEmpty(t, result.Reason) {
					assert.Equal(t, "STREAM-ERROR", result.Reason)
					assert.Empty(t, result.Message)
				}
			}
		}

	})
}
