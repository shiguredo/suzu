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

	"go.uber.org/goleak"
)

var (
	config = Config{
		Debug:                     true,
		ListenAddr:                "127.0.0.1",
		ListenPort:                48080,
		SkipBasicAuth:             true,
		LogDebug:                  true,
		LogStdout:                 true,
		DumpFile:                  "./test-dump.jsonl",
		TimeToWaitForOpusPacketMs: 500,
	}
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
				return
			}

			if _, err := w.Write(s.Payload); err != nil {
				// 停止条件を r.Close() にしているため、io: read/write on closed pipe エラーは出力される
				t.Log(err.Error())
				return
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
	handler := AmazonTranscribeHandler
	path := "/speech"
	serviceType := "aws"
	if testing.Short() {
		handler = TestHandler
		path = "/test"
	}
	s, err := NewServer(&config, "aws")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("success", func(t *testing.T) {
		//opt := goleak.IgnoreCurrent()
		opt := goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start")
		defer goleak.VerifyNone(t, opt)

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

		h := s.createSpeechHandler(serviceType, handler)
		err := h(c)
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

	t.Run("unexpected http proto version", func(t *testing.T) {
		opt := goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start")
		defer goleak.VerifyNone(t, opt)

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

		h := s.createSpeechHandler(serviceType, handler)
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
		opt := goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start")
		defer goleak.VerifyNone(t, opt)

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

		h := s.createSpeechHandler(serviceType, handler)
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
		opt := goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start")
		defer goleak.VerifyNone(t, opt)

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
		req.Header.Set("sora-audio-streaming-language-code", "en-JP")
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		h := s.createSpeechHandler(serviceType, handler)
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
		assert.Contains(t, buf.String()[:n], "UNSUPPORTED-LANGUAGE-CODE: aws, en-JP")
	})

	t.Run("packet read error", func(t *testing.T) {
		opt := goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start")
		defer goleak.VerifyNone(t, opt)

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

		h := s.createSpeechHandler(serviceType, handler)
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
		assert.Contains(t, buf.String()[:n], "packet read error")
	})

	t.Run("silent packet", func(t *testing.T) {
		opt := goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start")
		defer goleak.VerifyNone(t, opt)

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

		h := s.createSpeechHandler(serviceType, handler)
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

}

func TestHealthcheckHandler(t *testing.T) {
	type request struct {
		Proto      string
		ProtoMajor int
		ProtoMinor int
	}
	type expect struct {
		StatusCode int
		Body       string
	}

	s, err := NewServer(&config, "aws")
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"revision":"%s"}`, s.config.Revision)

	testCaces := []struct {
		Name    string
		Request request
		Expect  expect
	}{
		{"HTTP/2", request{"HTTP/2", 2, 0}, expect{http.StatusOK, body}},
		{"HTTP/1.1", request{"HTTP/1.1", 1, 1}, expect{http.StatusOK, body}},
	}

	for _, tc := range testCaces {
		t.Run(tc.Name, func(t *testing.T) {
			opt := goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start")
			defer goleak.VerifyNone(t, opt)

			e := echo.New()
			req := httptest.NewRequest("GET", "/.ok", nil)
			req.Proto = tc.Request.Proto
			req.ProtoMajor = tc.Request.ProtoMajor
			req.ProtoMinor = tc.Request.ProtoMinor
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := s.healthcheckHandler(c)
			if assert.NoError(t, err) {
				assert.Equal(t, tc.Expect.StatusCode, rec.Code)
				resp := rec.Result()
				body, _ := io.ReadAll(resp.Body)
				assert.JSONEq(t, tc.Expect.Body, string(body))
			}

		})
	}
}
