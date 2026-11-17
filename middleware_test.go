package suzu

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
)

// panicHandlerForTest は panic するハンドラー
// スタックトレースにこの関数名が含まれることを確認するため、無名関数ではなく名前付きの関数にする
func panicHandlerForTest(w http.ResponseWriter, r *http.Request) {
	panic("test panic")
}

// scrapeMetricsForTest は既定のレジストリのメトリクスをテキスト形式で取得する
func scrapeMetricsForTest() string {
	rec := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return rec.Body.String()
}

// responseWriter が送信済みのステータスコードを net/http と同じ規則で記録することを確認する
func TestResponseWriter(t *testing.T) {
	// createSpeechHandler は外部サービス接続成功後に Flush でヘッダーを送信し、
	// その後 WriteHeader で 200 以外を指定することがある
	// 送信済みのステータスコードは変更できないため、記録も 200 のままでなければならない
	t.Run("Flush の後に WriteHeader を呼んでも記録したステータスコードは変わらない", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := newResponseWriter(rec)

		// ハンドラーと同じく http.ResponseController 経由で Flush する
		if err := http.NewResponseController(rw).Flush(); err != nil {
			t.Fatalf("Flush に失敗した: %v", err)
		}

		// Flush でヘッダーを送信済みと判定される
		assert.True(t, rw.headerSent)
		assert.Equal(t, http.StatusOK, rw.statusCode())
		// Flush が元の ResponseWriter まで届いている
		assert.True(t, rec.Flushed)

		// 送信済みの後に別のステータスコードを指定しても無視される
		rw.WriteHeader(http.StatusInternalServerError)

		assert.Equal(t, http.StatusOK, rw.statusCode())
	})

	// SuzuConfError の経路などでは、WriteHeader を呼ばずに結果を Write する
	// net/http はこのとき 200 でヘッダーを送信するため、その後のエラー応答で 500 を記録してはいけない
	t.Run("WriteHeader を呼ばずに Write すると 200 を送信済みとして記録する", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := newResponseWriter(rec)

		n, err := rw.Write([]byte("result\n"))
		if err != nil {
			t.Fatalf("書き込みに失敗した: %v", err)
		}

		assert.True(t, rw.headerSent)
		assert.Equal(t, http.StatusOK, rw.statusCode())
		assert.Equal(t, int64(n), rw.size)

		// 送信済みの後に別のステータスコードを指定しても無視される
		rw.WriteHeader(http.StatusInternalServerError)

		assert.Equal(t, http.StatusOK, rw.statusCode())
	})

	// 委譲先が panic した場合は何も送信されていないため、送信済みとして記録してはいけない
	// 記録してしまうと、recover がヘッダー送信済みと判断して 500 を返さなくなる
	t.Run("委譲先が panic した WriteHeader は送信済みとして記録しない", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := newResponseWriter(rec)

		// httptest.ResponseRecorder も net/http の ResponseWriter と同じく、3 桁でないステータスコードで panic する
		assert.Panics(t, func() {
			rw.WriteHeader(42)
		})

		assert.False(t, rw.headerSent)
	})
}

// Sora の audio_streaming_url や LB のヘルスチェックのパスに付いた末尾のスラッシュを取り除くことを確認する
func TestRemoveTrailingSlash(t *testing.T) {
	testCases := []struct {
		name       string
		target     string
		path       string
		requestURI string
	}{
		{"ルートのパスはそのまま", "/", "/", "/"},
		{"末尾がスラッシュでないパスはそのまま", "/speech?q=1", "/speech", "/speech?q=1"},
		{"末尾のスラッシュを取り除き、クエリは残す", "/speech/?q=1", "/speech", "/speech?q=1"},
		// 取り除くのは 1 つだけ
		{"末尾のスラッシュが 2 つある場合は 1 つだけ取り除く", "/speech//", "/speech/", "/speech/"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.target, nil)
			originalPath := req.URL.Path
			originalRequestURI := req.RequestURI

			got := removeTrailingSlash(req)

			assert.Equal(t, tc.path, got.URL.Path)
			assert.Equal(t, tc.requestURI, got.RequestURI)
			// 受け取ったリクエストは書き換えない
			assert.Equal(t, originalPath, req.URL.Path)
			assert.Equal(t, originalRequestURI, req.RequestURI)
		})
	}
}

// アダプター (errorHandlerFunc) がハンドラーの返したエラーをヘッダーの送信状況に応じて扱うことを確認する
func TestErrorHandlerFunc(t *testing.T) {
	// ヘッダーが未送信の場合、アダプターは httpError 以外のエラーを 500 に変換する
	// 内部エラーの文言はクライアントに返さず、ステータスコードの文言だけを返す
	// ラッパーを通さずに httptest.ResponseRecorder を直接渡し、アダプターがラッパーを生成する経路も確認する
	t.Run("ヘッダー未送信で httpError 以外のエラーを返すと内部エラーの文言を含めずに 500 を返す", func(t *testing.T) {
		rec := httptest.NewRecorder()

		h := errorHandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return errors.New("internal error detail")
		})
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/test", nil))

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Equal(t, "Internal Server Error\n", rec.Body.String())
		assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	})

	// createSpeechHandler は外部サービス接続成功後に Flush でヘッダーを送信し、その後エラーを返すことがある
	// ヘッダーを送信済みの場合、アダプターはエラー応答を書かずにエラーの記録だけを行う
	t.Run("Flush の後にハンドラーがエラーを返してもエラー応答を書かない", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := newResponseWriter(rec)
		handlerErr := errors.New("handler error")

		h := errorHandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			if err := http.NewResponseController(w).Flush(); err != nil {
				return err
			}
			return handlerErr
		})
		h.ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/test", nil))

		// Flush で送信した 200 のまま、エラー応答のボディも書かれない
		assert.Empty(t, rec.Body.String())
		// アクセスログに出力するためのエラーは記録されている
		assert.ErrorIs(t, rw.err, handlerErr)
	})
}

// newMainHandler に mux を渡し、recover とアクセスログとメトリクスを確認する
// サーバーを起動せずに呼ぶため、ハンドラーはテストの goroutine で実行される
// そのため再度の panic を assert.PanicsWithValue で捕まえられ、ログもハンドラーの終了時点で出力済みになる
func TestMainHandler(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /panic", http.HandlerFunc(panicHandlerForTest))
	mux.Handle("GET /panic-after-flush", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 書き込みや Flush に失敗した場合は、別の内容で panic させて検証で検出できるようにする
		if _, err := w.Write([]byte("result\n")); err != nil {
			panic(err)
		}
		if err := http.NewResponseController(w).Flush(); err != nil {
			panic(err)
		}
		panic("test panic after flush")
	}))
	mux.Handle("GET /abort", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	mux.Handle("POST /bad-request", errorHandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		return newHTTPError(http.StatusBadRequest)
	}))
	mux.Handle("GET /.ok", errorHandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		return nil
	}))
	h := newMainHandler(mux, true)

	t.Run("ヘッダー未送信で panic すると 500 を返し、PANIC-RECOVERED とアクセスログの error とメトリクスに記録する", func(t *testing.T) {
		logs := captureLog(t)

		// メトリクスはパッケージで共有するコレクターに記録されるため、前後の差分で確認する
		series := `suzu_http_requests_total{code="500",method="GET",url="/panic"}`
		before, _ := metricValueForTest(t, scrapeMetricsForTest(), series)

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Equal(t, "Internal Server Error\n", rec.Body.String())

		entries := logEntries(t, logs)

		// panic の内容とスタックトレースを zerolog に出力する
		panicLogs := findLogs(entries, "PANIC-RECOVERED")
		if assert.Len(t, panicLogs, 1) {
			assert.Equal(t, "error", panicLogs[0]["level"])
			assert.Equal(t, "panic: test panic", panicLogs[0]["error"])
			assert.Equal(t, http.MethodGet, panicLogs[0]["method"])
			assert.Equal(t, "/panic", panicLogs[0]["uri"])
			assert.Equal(t, "192.0.2.1", panicLogs[0]["remote_ip"])

			stack, _ := panicLogs[0]["stack"].(string)
			// スタックトレースには panic したハンドラーのフレームが含まれる
			assert.Contains(t, stack, "panicHandlerForTest")
			// すべての goroutine ではなく、panic した goroutine の分だけを出力する
			// 行末の「created by ... in goroutine N」と区別するため、行頭の見出しだけを数える
			assert.Len(t, regexp.MustCompile(`(?m)^goroutine \d+ \[`).FindAllString(stack, -1), 1)
		}

		// recover の後に記録するアクセスログに、500 と panic の内容が出力される
		accessLogs := findLogs(entries, http.MethodGet)
		if assert.Len(t, accessLogs, 1) {
			assert.Equal(t, float64(http.StatusInternalServerError), accessLogs[0]["status"])
			assert.Equal(t, "panic: test panic", accessLogs[0]["error"])
		}

		// recover の後に記録するメトリクスにも、500 として記録される
		after, ok := metricValueForTest(t, scrapeMetricsForTest(), series)
		if assert.True(t, ok, "系列が出力されていない: %s", series) {
			assert.Equal(t, before+1, after)
		}
	})

	t.Run("ヘッダー送信後に panic してもボディを追記せず、送信済みの 200 を記録する", func(t *testing.T) {
		logs := captureLog(t)

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic-after-flush", nil))

		assert.Equal(t, "result\n", rec.Body.String())

		accessLogs := findLogs(logEntries(t, logs), http.MethodGet)
		if assert.Len(t, accessLogs, 1) {
			assert.Equal(t, float64(http.StatusOK), accessLogs[0]["status"])
			assert.Equal(t, "panic: test panic after flush", accessLogs[0]["error"])
		}
	})

	// http.ErrAbortHandler はレスポンスを中断するための panic で、net/http がストリームのリセットや切断を行う
	t.Run("http.ErrAbortHandler は recover した後に再度 panic し、応答もログも書かない", func(t *testing.T) {
		logs := captureLog(t)

		rec := httptest.NewRecorder()
		assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/abort", nil))
		})

		assert.Empty(t, rec.Body.String())

		entries := logEntries(t, logs)
		assert.Empty(t, findLogs(entries, "PANIC-RECOVERED"))
		assert.Empty(t, findLogs(entries, http.MethodGet))
	})

	t.Run("アクセスログを remote_ip などのキーで出力し、error と bytes_out にエラー応答を反映する", func(t *testing.T) {
		logs := captureLog(t)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/bad-request?q=1", strings.NewReader("abc"))
		req.Header.Set("User-Agent", "suzu-test")
		// bytes_in は Content-Length ヘッダーの値で、httptest.NewRequest はヘッダーを設定しないため明示する
		req.Header.Set("Content-Length", "3")
		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		// アクセスログの message は HTTP メソッド
		accessLogs := findLogs(logEntries(t, logs), http.MethodPost)
		if assert.Len(t, accessLogs, 1) {
			accessLog := accessLogs[0]

			keys := make([]string, 0, len(accessLog))
			for key := range accessLog {
				keys = append(keys, key)
			}
			// time はアクセスログではなく、テスト用のロガーの設定 (Timestamp) で付くキー
			assert.ElementsMatch(t, []string{
				"level", "time", "remote_ip", "host", "user_agent", "uri", "status", "error", "latency", "bytes_in", "bytes_out", "message",
			}, keys)

			assert.Equal(t, "info", accessLog["level"])
			// httptest.NewRequest の RemoteAddr は 192.0.2.1:1234 で、ポートを除いたものになる
			assert.Equal(t, "192.0.2.1", accessLog["remote_ip"])
			assert.Equal(t, "example.com", accessLog["host"])
			assert.Equal(t, "suzu-test", accessLog["user_agent"])
			// uri はクエリを含むリクエスト URI
			assert.Equal(t, "/bad-request?q=1", accessLog["uri"])
			assert.Equal(t, float64(http.StatusBadRequest), accessLog["status"])
			assert.Equal(t, "3", accessLog["bytes_in"])
			// latency は time.Duration の文字列表現で、0 より大きい
			latencyText, _ := accessLog["latency"].(string)
			latency, err := time.ParseDuration(latencyText)
			if assert.NoError(t, err) {
				assert.Positive(t, latency)
			}
			// ステータスコードだけのエラーは「400 Bad Request」形式になる
			assert.Equal(t, "400 Bad Request", accessLog["error"])
			// bytes_out は書き込んだエラー応答のボディ ("Bad Request\n") のバイト数
			assert.Equal(t, float64(len("Bad Request\n")), accessLog["bytes_out"])
		}
	})

	// 末尾のスラッシュはルーティングとアクセスログより前に取り除く
	t.Run("末尾スラッシュ付きのパスは取り除いた後のパスでルーティングし、アクセスログの uri にも反映する", func(t *testing.T) {
		logs := captureLog(t)

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/bad-request/?q=1", nil))

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		accessLogs := findLogs(logEntries(t, logs), http.MethodPost)
		if assert.Len(t, accessLogs, 1) {
			assert.Equal(t, "/bad-request?q=1", accessLogs[0]["uri"])
		}
	})

	// ServeMux が直接応答する 404 はアダプターを通らないため、エラーは記録されない
	t.Run("ServeMux が直接応答する 404 ではアクセスログに error を出力しない", func(t *testing.T) {
		logs := captureLog(t)

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/not-found", nil))

		assert.Equal(t, http.StatusNotFound, rec.Code)

		accessLogs := findLogs(logEntries(t, logs), http.MethodGet)
		if assert.Len(t, accessLogs, 1) {
			assert.Equal(t, float64(http.StatusNotFound), accessLogs[0]["status"])
			assert.NotContains(t, accessLogs[0], "error")
		}
	})

	t.Run("/.ok へのリクエストはアクセスログを出力しない", func(t *testing.T) {
		logs := captureLog(t)

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/.ok", nil))

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, findLogs(logEntries(t, logs), http.MethodGet))
	})
}
