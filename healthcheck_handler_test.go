package suzu

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
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
