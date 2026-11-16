package suzu

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	httpMetricsNamespace = "suzu"
	httpMetricsSubsystem = "http"

	// 1 KB は 1024 バイトとする
	httpMetricsKB float64 = 1024
	httpMetricsMB float64 = 1024 * httpMetricsKB

	// ラベルの値を決められない場合に使う値
	httpMetricsUnknownLabelValue = "unknown"
)

var (
	// リクエストサイズとレスポンスサイズのバケット (1 KB から 10 MB)
	// 境界を変えると、蓄積済みの値とバケット単位で比較できなくなるため、変えないこと
	httpMetricsSizeBuckets = []float64{
		1 * httpMetricsKB,
		2 * httpMetricsKB,
		5 * httpMetricsKB,
		10 * httpMetricsKB,
		100 * httpMetricsKB,
		500 * httpMetricsKB,
		1 * httpMetricsMB,
		2.5 * httpMetricsMB,
		5 * httpMetricsMB,
		10 * httpMetricsMB,
	}

	// すべての HTTP メトリクスに付けるラベルの名前
	// ラベルの値の種類の数だけ時系列が増えるため、クライアントが自由に指定できる値はラベルにそのまま入れない
	// Host ヘッダーもクライアントが自由に指定できるため、host ラベルは付けない
	// インスタンスの識別には Prometheus が付ける instance ラベルを使う
	httpMetricsLabelNames = []string{"code", "method", "url"}
)

// HTTP メトリクスのコレクター
// パッケージの初期化時に 1 回だけ prometheus.DefaultRegisterer に登録する
// NewServer はテストで複数回呼ばれるため、NewServer の中で登録すると 2 回目以降が重複登録になる
var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: httpMetricsNamespace,
		Subsystem: httpMetricsSubsystem,
		Name:      "requests_total",
		Help:      "How many HTTP requests processed, partitioned by status code, HTTP method and route.",
	}, httpMetricsLabelNames)

	httpRequestDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: httpMetricsNamespace,
		Subsystem: httpMetricsSubsystem,
		Name:      "request_duration_seconds",
		Help:      "The HTTP request latencies in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, httpMetricsLabelNames)

	httpRequestSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: httpMetricsNamespace,
		Subsystem: httpMetricsSubsystem,
		Name:      "request_size_bytes",
		Help:      "The HTTP request sizes in bytes.",
		Buckets:   httpMetricsSizeBuckets,
	}, httpMetricsLabelNames)

	httpResponseSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: httpMetricsNamespace,
		Subsystem: httpMetricsSubsystem,
		Name:      "response_size_bytes",
		Help:      "The HTTP response sizes in bytes.",
		Buckets:   httpMetricsSizeBuckets,
	}, httpMetricsLabelNames)
)

// observeHTTPMetrics は 1 リクエスト分の HTTP メトリクスを記録する
// r.Pattern を参照するため、r は mux の ServeHTTP に渡したリクエストでなければならない
func observeHTTPMetrics(rw *responseWriter, r *http.Request, latency time.Duration) {
	labels := prometheus.Labels{
		"code":   strconv.Itoa(rw.statusCode()),
		"method": httpMetricsMethodLabelValue(r.Method),
		"url":    httpMetricsURLLabelValue(r.Pattern),
	}

	httpRequestsTotal.With(labels).Inc()
	httpRequestDurationSeconds.With(labels).Observe(latency.Seconds())
	httpRequestSizeBytes.With(labels).Observe(approximateRequestSize(r))
	httpResponseSizeBytes.With(labels).Observe(float64(rw.size))
}

// httpMetricsMethodLabelValue は method ラベルの値を返す
// net/http が定数として定義しているメソッド以外は unknown にまとめる
func httpMetricsMethodLabelValue(method string) string {
	switch method {
	case http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
		http.MethodTrace:
		return method
	default:
		return httpMetricsUnknownLabelValue
	}
}

// httpMetricsURLLabelValue は url ラベルの値として、マッチしたルートのパスを返す
// pattern は http.ServeMux が r.Pattern に設定するパターン (POST /speech など) で、メソッドを除いたものを返す
// ルートにマッチしなかった場合 (404 や 405 で pattern が空の場合) は、リクエストパスではなく unknown を返す
func httpMetricsURLLabelValue(pattern string) string {
	if pattern == "" {
		return httpMetricsUnknownLabelValue
	}
	// パターンは [METHOD ][HOST]/[PATH] の形式で、メソッドとそれ以降は 1 つ以上の空白またはタブで区切られている
	if i := strings.IndexAny(pattern, " \t"); i >= 0 {
		return strings.TrimLeft(pattern[i+1:], " \t")
	}
	return pattern
}

// approximateRequestSize はリクエストのおおよそのサイズを返す
// パス、メソッド、プロトコル、ヘッダー、Host、ボディのサイズ (ContentLength) の合計とする
// r はサーバーが受け取ったリクエストで、URL は nil にならない
func approximateRequestSize(r *http.Request) float64 {
	size := len(r.URL.Path)
	size += len(r.Method)
	size += len(r.Proto)
	for name, values := range r.Header {
		size += len(name)
		for _, value := range values {
			size += len(value)
		}
	}
	size += len(r.Host)

	// ContentLength はクライアントが 2^63-1 まで指定できるため、int で足すとあふれて負値になる
	// そのため float64 で足す
	// 不明 (-1) の場合はボディのサイズを含めない
	if r.ContentLength > 0 {
		return float64(size) + float64(r.ContentLength)
	}
	return float64(size)
}
