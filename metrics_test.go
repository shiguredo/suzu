package suzu

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// request_size_bytes に記録するリクエストのおおよそのサイズを、approximateRequestSize が正しく計算することを確認する
func TestApproximateRequestSize(t *testing.T) {
	// ボディを含まないリクエストのサイズ (パス、メソッド、プロトコル、ヘッダー、Host の合計)
	newRequest := func() (*http.Request, int) {
		req := httptest.NewRequest(http.MethodPost, "/speech", nil)
		req.Header.Set("Sora-Channel-Id", "sora")
		size := len("/speech") + len(http.MethodPost) + len(req.Proto) + len("Sora-Channel-Id") + len("sora") + len(req.Host)
		return req, size
	}

	t.Run("ボディのサイズが分かる場合は ContentLength を足す", func(t *testing.T) {
		req, size := newRequest()
		req.ContentLength = 3

		assert.Equal(t, float64(size+3), approximateRequestSize(req))
	})

	t.Run("ボディのサイズが不明 (-1) の場合はボディを含めない", func(t *testing.T) {
		req, size := newRequest()
		req.ContentLength = -1

		assert.Equal(t, float64(size), approximateRequestSize(req))
	})

	// ContentLength はクライアントが 2^63-1 まで指定でき、int で足すとあふれて負値になる
	// float64 の精度ではボディ以外のサイズが丸められるため、あふれていないこと (正の値であること) だけを確認する
	t.Run("ContentLength が int64 の最大値でもあふれずに正の値になる", func(t *testing.T) {
		req, _ := newRequest()
		req.ContentLength = math.MaxInt64

		assert.Positive(t, approximateRequestSize(req))
	})
}
