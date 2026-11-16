package suzu

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	zlog "github.com/rs/zerolog/log"
)

// responseWriter は、メトリクスとアクセスログに必要な送信の状態を記録する http.ResponseWriter のラッパー
// ラッパーが記録した送信済みの状態をメトリクスとアクセスログに、recover やアダプター (errorHandlerFunc) が記録したエラーを
// アクセスログに届けるため、newMainHandler がリクエストごとに 1 回だけ生成し、すべての層で同じものを使う
// 1xx の情報レスポンスは扱わない (Suzu では送信しない)
type responseWriter struct {
	http.ResponseWriter

	// 送信したステータスコード (headerSent が true のときだけ有効)
	status int
	// ResponseWriter に書き込んだボディのバイト数
	// HEAD リクエストやクライアントの切断で、実際には送信されなかった分も含む
	size int64
	// レスポンスヘッダーを送信済みかどうか
	// true になった後はステータスコードを変更できない
	headerSent bool
	// アダプターまたは recover が記録したエラー (アクセスログの error に出力する)
	err error
}

// newResponseWriter は w をラップした responseWriter を生成する
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w}
}

// WriteHeader はステータスコードを送信する
// 送信済みの場合はステータスコードを変更できないため、委譲も記録もせずに無視する
func (w *responseWriter) WriteHeader(code int) {
	if w.headerSent {
		return
	}
	// 委譲先は不正なステータスコードで panic するため、委譲が終わってから記録する
	// 先に記録すると、何も送信していないのに recover が送信済みと判断して 500 を返さなくなる
	w.ResponseWriter.WriteHeader(code)
	w.status = code
	w.headerSent = true
}

// Write はレスポンスボディを書き込む
// 書き込みに失敗しても net/http はヘッダーを送信済みとして扱うため、委譲の結果にかかわらず記録する
func (w *responseWriter) Write(b []byte) (int, error) {
	w.markHeaderSent()
	n, err := w.ResponseWriter.Write(b)
	w.size += int64(n)
	return n, err
}

// FlushError はバッファしているレスポンスをクライアントに送信する
// http.ResponseController の Flush は FlushError、http.Flusher、Unwrap の順に探すため、
// Unwrap だけを実装すると Flush がこのラッパーを素通りし、ヘッダーの送信を記録できない
// Write と同じく、送信に失敗しても net/http はヘッダーを送信済みとして扱うため、委譲の結果にかかわらず記録する
func (w *responseWriter) FlushError() error {
	w.markHeaderSent()
	return http.NewResponseController(w.ResponseWriter).Flush()
}

// Unwrap は Flush 以外の http.ResponseController の操作を元の ResponseWriter に届けるために使う
func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// markHeaderSent は WriteHeader を呼ばずに Write や Flush をした場合に、200 を送信済みとして記録する
// この場合 net/http は 200 でヘッダーを送信する
func (w *responseWriter) markHeaderSent() {
	if w.headerSent {
		return
	}
	w.status = http.StatusOK
	w.headerSent = true
}

// statusCode はクライアントに送信したステータスコードを返す
// 何も書き込まずにハンドラーが終了した場合も net/http は 200 を送信するため、200 を返す
func (w *responseWriter) statusCode() int {
	if !w.headerSent {
		return http.StatusOK
	}
	return w.status
}

// newMainHandler はメインサーバーのハンドラーを組み立てる
// recover の後でメトリクスとアクセスログを記録するため、panic で 500 を返した場合も両方に反映される
func newMainHandler(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := newResponseWriter(w)
		start := time.Now()

		// メトリクスが参照する r.Pattern は mux が r に設定するため、r を差し替えずに渡す
		serveWithRecover(rw, r, mux)

		latency := time.Since(start)
		observeHTTPMetrics(rw, r, latency)
		writeAccessLog(rw, r, latency)
	})
}

// serveWithRecover は next で発生した panic を捕まえ、スタックトレース付きで zerolog に出力する
// panic の内容はエラーとしてラッパーに記録し、ヘッダー未送信なら 500 を返す
// recover しない場合、panic は zerolog を経由せずに net/http のログに出力される
// またヘッダー未送信の場合、クライアントはステータスコードを受け取れない (HTTP/2 ではストリームがリセットされ、HTTP/1.1 では接続が閉じられる)
// ヘッダー送信済みの場合はステータスコードを変更できないため、記録だけを行い、レスポンスはそのまま終了する
func serveWithRecover(rw *responseWriter, r *http.Request, next http.Handler) {
	defer func() {
		v := recover()
		if v == nil {
			return
		}
		// http.ErrAbortHandler はレスポンスを中断するための panic のため、net/http に処理を任せる
		// この場合、メトリクスとアクセスログは記録されない
		if v == http.ErrAbortHandler {
			panic(v)
		}

		// アクセスログで panic によるエラーだと分かるように接頭辞を付ける
		err := fmt.Errorf("panic: %v", v)

		zlog.Error().
			Err(err).
			Str("remote_ip", remoteIP(r.RemoteAddr)).
			Str("method", r.Method).
			Str("uri", r.RequestURI).
			Str("stack", string(debug.Stack())).
			Msg("PANIC-RECOVERED")

		rw.err = err

		if rw.headerSent {
			return
		}
		// アダプターと同じく、panic の内容はクライアントに返さず、ステータスコードの文言だけを返す
		http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}()

	next.ServeHTTP(rw, r)
}

// writeAccessLog はアクセスログを zerolog の info で出力する
func writeAccessLog(rw *responseWriter, r *http.Request, latency time.Duration) {
	// LB からのヘルスチェックはログを出力しない
	if strings.HasPrefix(r.URL.Path, "/.ok") {
		return
	}

	zlog.Info().
		Str("remote_ip", remoteIP(r.RemoteAddr)).
		Str("host", r.Host).
		Str("user_agent", r.UserAgent()).
		Str("uri", r.RequestURI).
		Int("status", rw.statusCode()).
		// rw.err が nil の場合、zerolog は error キーを出力しない
		Err(rw.err).
		Str("latency", latency.String()).
		Str("bytes_in", r.Header.Get("Content-Length")).
		Int64("bytes_out", rw.size).
		Msg(r.Method)
}

// remoteIP は RemoteAddr のホスト部を返す
// X-Forwarded-For などのヘッダーはクライアントが自由に指定できるため参照しない
func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// RemoteAddr はハンドラーやテストがポートを含まない形で設定する場合もあるため、
		// その場合は IP アドレスとして解釈できるときだけそのまま返す
		if net.ParseIP(remoteAddr) != nil {
			return remoteAddr
		}
		return ""
	}
	return host
}

// errorHandlerFunc はエラーを返す HTTP ハンドラー
// ServeHTTP で http.Handler として扱い、エラー応答の書き出しをハンドラーの各経路ではなく 1 箇所で行う
type errorHandlerFunc func(http.ResponseWriter, *http.Request) error

// ServeHTTP は errorHandlerFunc を http.Handler として扱うためのアダプター
// ハンドラーが返したエラーは、レスポンスヘッダーが未送信の場合に限りエラー応答に変換する
// ヘッダーを送信済みの場合はステータスコードを変更できないため、エラーの記録だけを行う
func (h errorHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// newMainHandler と mux の間で ResponseWriter を別のものに差し替えると、ここで記録したエラーがアクセスログに届かなくなる
	// newMainHandler を通らずに *responseWriter 以外の ResponseWriter で呼ばれた場合も、
	// 送信済みを判定できるようにここでラッパーを生成する
	rw, ok := w.(*responseWriter)
	if !ok {
		rw = newResponseWriter(w)
	}

	err := h(rw, r)
	if err == nil {
		return
	}

	rw.err = err

	if rw.headerSent {
		return
	}

	code := http.StatusInternalServerError
	var httpErr *httpError
	if errors.As(err, &httpErr) {
		code = httpErr.code
	}
	// 内部エラーの文言はクライアントに返さず、ステータスコードの文言だけを返す
	http.Error(rw, http.StatusText(code), code)
}

// httpError はクライアントに返すステータスコードを持つエラー
// ハンドラーが返すと、ヘッダー未送信の場合に限りアダプターがこのステータスコードで応答する
type httpError struct {
	code int
}

// newHTTPError は code で応答する httpError を生成する
func newHTTPError(code int) *httpError {
	return &httpError{code: code}
}

// Error は「400 Bad Request」の形式の文言を返す
// この文言はアクセスログの error に出力される
func (e *httpError) Error() string {
	return fmt.Sprintf("%d %s", e.code, http.StatusText(e.code))
}
