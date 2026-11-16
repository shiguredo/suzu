package suzu

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

	config := Config{
		Debug:                     true,
		ListenAddr:                "127.0.0.1",
		ListenPort:                48080,
		SkipBasicAuth:             true,
		LogStdout:                 true,
		DumpFile:                  "./test-dump.jsonl",
		TimeToWaitForOpusPacketMs: 500,
	}

	s, err := NewServer(&config, "aws")
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"version":"%s"}`, s.config.Version)

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
			req := httptest.NewRequest("GET", "/.ok", nil)
			req.Proto = tc.Request.Proto
			req.ProtoMajor = tc.Request.ProtoMajor
			req.ProtoMinor = tc.Request.ProtoMinor
			rec := httptest.NewRecorder()

			err := s.healthcheckHandler(rec, req)
			if assert.NoError(t, err) {
				assert.Equal(t, tc.Expect.StatusCode, rec.Code)
				// ヘルスチェックの応答は JSON のため、Content-Type は application/json になる
				assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
				resp := rec.Result()
				body, _ := io.ReadAll(resp.Body)
				assert.JSONEq(t, tc.Expect.Body, string(body))
			}

		})
	}
}
