package suzu

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/iotest"
	"time"

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
			if d > 0 {
				time.Sleep(d)
			}

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

		req := httptest.NewRequest("POST", path, r)
		req.Header.Set("sora-audio-streaming-language-code", "ja-JP")
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0
		rec := httptest.NewRecorder()

		h := s.createSpeechHandler(serviceType, nil)
		err := h(rec, req)
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
		logs := captureLog(t)

		r := readDumpFile(t, "testdata/dump.jsonl", 0)
		defer r.Close()

		req := httptest.NewRequest("POST", path, r)
		req.Header.Set("sora-audio-streaming-language-code", "ja-JP")
		req.Proto = "HTTP/1.1"
		req.ProtoMajor = 1
		req.ProtoMinor = 1
		rec := httptest.NewRecorder()

		h := s.createSpeechHandler(serviceType, nil)
		err := h(rec, req)
		var httpErr *httpError
		if assert.ErrorAs(t, err, &httpErr) {
			assert.Equal(t, http.StatusBadRequest, httpErr.code)
		}

		assert.Contains(t, logs.String(), "INVALID-HTTP-PROTOCOL")
	})

	t.Run("missing sora-audio-streaming-language-code header", func(t *testing.T) {
		logs := captureLog(t)

		r := readDumpFile(t, "testdata/dump.jsonl", 0)
		defer r.Close()

		req := httptest.NewRequest("POST", path, r)
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0
		rec := httptest.NewRecorder()

		h := s.createSpeechHandler(serviceType, nil)
		err := h(rec, req)
		var httpErr *httpError
		if assert.ErrorAs(t, err, &httpErr) {
			assert.Equal(t, http.StatusInternalServerError, httpErr.code)
		}

		assert.Contains(t, logs.String(), "MISSING-SORA-AUDIO-STREAMING-LANGUAGE-CODE")
	})

	t.Run("unsupported language code", func(t *testing.T) {
		r := readDumpFile(t, "testdata/dump.jsonl", 0)
		defer r.Close()

		req := httptest.NewRequest("POST", path, r)
		req.Header.Set("sora-audio-streaming-language-code", "en-JP")
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0
		rec := httptest.NewRecorder()

		h := s.createSpeechHandler(serviceType, nil)
		err := h(rec, req)
		if assert.NoError(t, err) {
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})

	t.Run("packet read error", func(t *testing.T) {
		r := iotest.ErrReader(errors.New("packet read error"))

		req := httptest.NewRequest("POST", path, r)
		req.Header.Set("sora-audio-streaming-language-code", "ja-JP")
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0
		rec := httptest.NewRecorder()

		h := s.createSpeechHandler(serviceType, nil)
		err = h(rec, req)
		if assert.Error(t, err) {
			assert.Equal(t, "packet read error", err.Error())
		}

		line, err := rec.Body.ReadBytes([]byte("\n")[0])
		if err != nil {
			t.Fatal(err)
		}
		assert.Contains(t, string(line), "packet read error")
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

		h := s.createSpeechHandler(serviceType, nil)
		err = h(rec, req2)
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

		req := httptest.NewRequest("POST", path, r)
		req.Header.Set("sora-audio-streaming-language-code", "ja-JP")
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0
		rec := httptest.NewRecorder()

		h := s.createSpeechHandler(serviceType, func(ctx context.Context, w io.WriteCloser, chnanelID, connectionID, languageCode string, results any) error {
			return fmt.Errorf("ON-RESULT-ERROR")
		})
		err := h(rec, req)
		if assert.Error(t, err) {
			assert.Equal(t, "ON-RESULT-ERROR", err.Error())
		}

	})

	t.Run("stream error", func(t *testing.T) {
		r := readDumpFile(t, "testdata/dump.jsonl", 0)
		defer r.Close()

		req := httptest.NewRequest("POST", path, r)
		req.Header.Set("sora-audio-streaming-language-code", "ja-JP")
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0
		rec := httptest.NewRecorder()

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
		err := h(rec, req)
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

	t.Run("connect error returns suzu error status code", func(t *testing.T) {
		// テストのみで使用するステータスコード
		expectedCode := 499
		NewServiceHandlerFuncs.register("test", NewConnectErrorTestHandlerFactory(expectedCode, "CONNECT-ERROR"))
		defer NewServiceHandlerFuncs.register("test", NewTestHandler)

		r := readDumpFile(t, "testdata/dump.jsonl", 0)
		defer r.Close()

		req := httptest.NewRequest("POST", path, r)
		req.Header.Set("sora-audio-streaming-language-code", "ja-JP")
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0
		rec := httptest.NewRecorder()

		h := s.createSpeechHandler(serviceType, nil)
		err := h(rec, req)
		if assert.NoError(t, err) {
			assert.Equal(t, expectedCode, rec.Code)
			assert.Empty(t, rec.Body.String())
		}
	})

	t.Run("connect error with invalid suzu error status code returns 500", func(t *testing.T) {
		NewServiceHandlerFuncs.register("test", NewConnectErrorTestHandlerFactory(0, "CONNECT-ERROR-WITH-INVALID-STATUS"))
		defer NewServiceHandlerFuncs.register("test", NewTestHandler)

		r := readDumpFile(t, "testdata/dump.jsonl", 0)
		defer r.Close()

		req := httptest.NewRequest("POST", path, r)
		req.Header.Set("sora-audio-streaming-language-code", "ja-JP")
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0
		rec := httptest.NewRecorder()

		h := s.createSpeechHandler(serviceType, nil)
		err := h(rec, req)
		if assert.NoError(t, err) {
			assert.Equal(t, http.StatusInternalServerError, rec.Code)
			assert.Empty(t, rec.Body.String())
		}
	})

	// SuzuConfError は設定の不備などで復帰が難しいエラーで、type: error の結果をクライアントに送信してから終了する
	// 原因をアクセスログの error に残すため、ハンドラーは SuzuConfError をそのまま返す
	t.Run("SuzuConfError は type: error の結果を送信してからそのまま返す", func(t *testing.T) {
		gcpConfig := Config{
			ListenAddr: "127.0.0.1",
			// 存在しない認証ファイルを指定して、外部サービスに接続する前に SuzuConfError にする
			GcpCredentialFile: filepath.Join(t.TempDir(), "not-found.json"),
		}
		s, err := NewServer(&gcpConfig, "gcp")
		if err != nil {
			t.Fatalf("サーバーの生成に失敗した: %v", err)
		}

		r := readDumpFile(t, "testdata/dump.jsonl", 0)
		defer r.Close()

		req := httptest.NewRequest("POST", "/speech", r)
		req.Header.Set("sora-audio-streaming-language-code", "ja-JP")
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0
		rec := httptest.NewRecorder()

		h := s.createSpeechHandler("gcp", nil)
		err = h(rec, req)

		// type: error の結果を書き込んで送信済みのため、ステータスコードは 200 のまま
		assert.Equal(t, http.StatusOK, rec.Code)
		var result TranscriptionResult
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("結果を JSON として解釈できなかった: %v", err)
		}
		assert.Equal(t, "error", result.Type)

		// 送信した結果と同じ SuzuConfError が返る
		var confErr *SuzuConfError
		if assert.ErrorAs(t, err, &confErr) {
			assert.Equal(t, confErr.Error(), result.Reason)
		}
	})

	t.Run("IsRetryTarget", func(t *testing.T) {
		channelID := "test-channel-id"
		connectionID := "test-connection-id"
		sampleRate := uint32(48000)
		channelCount := uint16(2)
		languageCode := "ja-JP"
		onResultFunc := func(context.Context, io.WriteCloser, string, string, string, any) error { return nil }

		// このハンドラではリトライしないため、常に false を返す
		testCases := []struct {
			Name         string
			RetryTargets []string
			Error        error
			Expect       bool
		}{
			{
				Name:         "retry target is empty",
				RetryTargets: []string{},
				Error:        errors.New(""),
				Expect:       false,
			},
			{
				Name:         "match",
				RetryTargets: []string{"UNEXPECTED-ERROR", "BAD-REQUEST"},
				Error:        errors.New("UNEXPECTED-ERROR"),
				Expect:       false,
			},
			{
				Name:         "mismatched error",
				RetryTargets: []string{"UNEXPECTED-ERROR"},
				Error:        errors.New("ERROR"),
				Expect:       false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				config := Config{
					RetryTargets: tc.RetryTargets,
				}

				serviceHandler, err := getServiceHandler(serviceType, config, channelID, connectionID, sampleRate, channelCount, languageCode, onResultFunc)
				assert.NoError(t, err)

				assert.Equal(t, tc.Expect, serviceHandler.IsRetryTarget(tc.Error))
			})
		}
	})
}

type ConnectErrorTestHandler struct {
	code    int
	message string
}

func NewConnectErrorTestHandlerFactory(code int, message string) newServiceHandlerFunc {
	return func(Config, string, string, uint32, uint16, string, any) serviceHandlerInterface {
		return &ConnectErrorTestHandler{
			code:    code,
			message: message,
		}
	}
}

func (h *ConnectErrorTestHandler) Handle(context.Context, chan opus, soraHeader) (*io.PipeReader, error) {
	return nil, &SuzuError{
		Code:    h.code,
		Message: h.message,
	}
}

func (h *ConnectErrorTestHandler) UpdateRetryCount() int { return 0 }

func (h *ConnectErrorTestHandler) GetRetryCount() int { return 0 }

func (h *ConnectErrorTestHandler) ResetRetryCount() int { return 0 }

func (h *ConnectErrorTestHandler) IsRetryTarget(any) bool { return false }
