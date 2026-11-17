package suzu

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// serveForTest は server を 127.0.0.1 の空いているポートで待ち受け、そのベース URL を返す
// テストの終了時にサーバーを閉じ、Serve の終了まで待つ
func serveForTest(t *testing.T, server *http.Server) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("待ち受けに失敗した: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := server.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("サーバーが予期せず終了した: %v", err)
		}
	}()

	t.Cleanup(func() {
		server.Close()
		<-done
	})

	return "http://" + ln.Addr().String()
}

// newH2CClientForTest は HTTP/2 over TCP (h2c) で接続するクライアントを返す
// テストの終了時にアイドル接続を閉じる
func newH2CClientForTest(t *testing.T) *http.Client {
	t.Helper()

	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	transport := &http.Transport{Protocols: protocols}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport}
}

// fetchMetricsForTest は Exporter の /metrics をテキスト形式で取得する
func fetchMetricsForTest(t *testing.T, client *http.Client, exporterURL string) string {
	t.Helper()

	resp, err := client.Get(exporterURL + "/metrics")
	if err != nil {
		t.Fatalf("メトリクスの取得に失敗した: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("メトリクスの取得で 200 以外が返った: %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("メトリクスの読み込みに失敗した: %v", err)
	}
	return string(b)
}

// metricValueForTest はテキスト形式のメトリクスから series の値を返す
// series は名前とラベルを含む系列 (suzu_http_requests_total{code="200",method="GET",url="/.ok"} など)
// ラベルは Prometheus の出力と同じ順に書くこと (通常のラベルは名前の昇順 (code、method、url) で、ヒストグラムの le は最後)
// 系列が出力されていない場合は 2 つ目の戻り値に false を返す
func metricValueForTest(t *testing.T, metrics, series string) (float64, bool) {
	t.Helper()

	for line := range strings.SplitSeq(metrics, "\n") {
		v, found := strings.CutPrefix(line, series+" ")
		if !found {
			continue
		}
		// 値は 2.7541e-05 のような指数表記になることがあるため、浮動小数点数として解釈する
		value, err := strconv.ParseFloat(v, 64)
		if err != nil {
			t.Fatalf("メトリクスの値を解釈できなかった: %s: %v", line, err)
		}
		return value, true
	}
	return 0, false
}

// TestSpeechHandler と TestHealthcheckHandler はハンドラーを直接呼ぶため、ルーティングとミドルウェアを通らない
// ここでは NewServer が組み立てた http.Server を実際に待ち受け、標準ライブラリの HTTP クライアントで確認する
func TestServer(t *testing.T) {
	config := Config{
		ListenAddr: "127.0.0.1",
		// 無音パケットを挿入しないようにして、送信したパケットの分だけ結果が返るようにする
		// Config をリテラルで作ると無音パケットの間隔が 0 になり、無音パケットが絶えず挿入される
		DisableSilentPacket: true,
		// /dump のダンプファイルは追記されるため、テストごとに新しいファイルにする
		DumpFile: filepath.Join(t.TempDir(), "dump.jsonl"),
	}

	s, err := NewServer(&config, "test")
	if err != nil {
		t.Fatalf("サーバーの生成に失敗した: %v", err)
	}

	mainURL := serveForTest(t, s.mainServer)
	exporterURL := serveForTest(t, s.exporterServer)

	// HTTP/2 over TCP (h2c) で接続するクライアント
	h2cClient := newH2CClientForTest(t)

	// HTTP/1.1 で接続するクライアント
	// Exporter は Protocols を設定していないため HTTP/2 over TCP (h2c) を受け付けず、Exporter への接続にもこれを使う
	http1Protocols := new(http.Protocols)
	http1Protocols.SetHTTP1(true)
	http1Transport := &http.Transport{Protocols: http1Protocols}
	t.Cleanup(http1Transport.CloseIdleConnections)
	http1Client := &http.Client{Transport: http1Transport}

	t.Run("POST /test の結果を HTTP/2 over TCP で逐次受信できる", func(t *testing.T) {
		// リクエストボディを閉じない限りハンドラーは終了しないため、
		// ボディを送信している途中で結果を受信できれば、レスポンスは逐次送信されている
		pr, pw := io.Pipe()
		defer pw.Close()

		// レスポンスがまとめて送信される場合、ヘッダーや結果を受信できずに待ち続けるため、期限を設けて失敗させる
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		// HTTP/2 の Transport は、レスポンスヘッダーを受信した後はリクエストボディを送り終えるまで ctx を見ない
		// 期限を過ぎたらリクエストボディを閉じて、待ち続けているボディの読み込みをエラーで終わらせる
		stop := context.AfterFunc(ctx, func() {
			pw.CloseWithError(ctx.Err())
		})
		defer stop()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, mainURL+"/test", pr)
		if err != nil {
			t.Fatalf("リクエストの生成に失敗した: %v", err)
		}
		req.Header.Set("sora-audio-streaming-language-code", "ja-JP")

		// ヘッダーは外部サービスへの接続成功後に送信される
		// test サービスは外部サービスに接続せず、音声を待たずに接続に成功するため、ボディを送信する前にレスポンスを受け取れる
		resp, err := h2cClient.Do(req)
		if err != nil {
			t.Fatalf("リクエストに失敗した: %v", err)
		}
		defer resp.Body.Close()

		assert.Equal(t, 2, resp.ProtoMajor)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		body := bufio.NewReader(resp.Body)

		// パケットを 1 つ送信するたびに、その結果を 1 行受信できることを確認する
		for range 3 {
			if _, err := pw.Write(silentPacket()); err != nil {
				t.Fatalf("パケットの送信に失敗した: %v", err)
			}

			line, err := body.ReadBytes('\n')
			if err != nil {
				t.Fatalf("リクエストボディの送信中に結果を受信できなかった: %v", err)
			}

			var result TranscriptionResult
			if err := json.Unmarshal(line, &result); err != nil {
				t.Fatalf("結果を JSON として解釈できなかった: %v", err)
			}
			assert.Equal(t, "test", result.Type)
			// 無音パケットは 3 バイト
			assert.Equal(t, "n: 3", result.Message)
		}

		// リクエストボディを閉じるとハンドラーが終了し、レスポンスも終了する
		if err := pw.Close(); err != nil {
			t.Fatalf("リクエストボディを閉じられなかった: %v", err)
		}
		rest, err := io.ReadAll(body)
		if err != nil {
			t.Fatalf("レスポンスボディの読み込みに失敗した: %v", err)
		}
		assert.Empty(t, rest)
	})

	// /dump は受信した音声をダンプファイルに書き込み、同じ内容を結果として返す
	// 結果の channel_id と connection_id は、Sora が付与するリクエストヘッダーから取り出した値になる
	t.Run("POST /dump は Sora のヘッダーの値を含むダンプを返し、ダンプファイルに書き込む", func(t *testing.T) {
		// 1 回の読み込みで受信できるように、パケットは 1 つだけ送る
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, mainURL+"/dump", bytes.NewReader(silentPacket()))
		if err != nil {
			t.Fatalf("リクエストの生成に失敗した: %v", err)
		}
		req.Header.Set("sora-channel-id", "test-channel-id")
		req.Header.Set("sora-connection-id", "test-connection-id")
		req.Header.Set("sora-audio-streaming-language-code", "ja-JP")

		resp, err := h2cClient.Do(req)
		if err != nil {
			t.Fatalf("リクエストに失敗した: %v", err)
		}
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("レスポンスボディの読み込みに失敗した: %v", err)
		}

		var result PacketDumpResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("結果を JSON として解釈できなかった: %v", err)
		}
		assert.Equal(t, "test-channel-id", result.ChannelID)
		assert.Equal(t, "test-connection-id", result.ConnectionID)
		assert.Equal(t, "ja-JP", result.LanguageCode)
		assert.Equal(t, silentPacket(), result.Payload)

		// ダンプファイルには結果と同じ内容が書き込まれる
		// ダンプファイルに書き込んでから結果を送信するため、結果を受信した時点で書き込みは終わっている
		dump, err := os.ReadFile(config.DumpFile)
		if err != nil {
			t.Fatalf("ダンプファイルの読み込みに失敗した: %v", err)
		}
		assert.Equal(t, string(body), string(dump))
	})

	// /speech は HTTP/2 必須のため、HTTP/1.1 のリクエストはアダプターが 400 に変換する
	t.Run("HTTP/1.1 での POST /speech は 400 になる", func(t *testing.T) {
		resp, err := http1Client.Post(mainURL+"/speech", "application/octet-stream", nil)
		if err != nil {
			t.Fatalf("リクエストに失敗した: %v", err)
		}
		defer resp.Body.Close()

		assert.Equal(t, 1, resp.ProtoMajor)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		// ボディは http.StatusText の文言だけになる
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("レスポンスボディの読み込みに失敗した: %v", err)
		}
		assert.Equal(t, "Bad Request\n", string(body))
	})

	// Sora の audio_streaming_url や LB のヘルスチェックのパスを末尾スラッシュ付きで設定していても受け付ける
	t.Run("末尾スラッシュ付きのパスを末尾スラッシュなしのルートとして受け付ける", func(t *testing.T) {
		// /speech/ は /speech のハンドラーに届き、HTTP/1.1 のため 400 になる
		resp, err := http1Client.Post(mainURL+"/speech/", "application/octet-stream", nil)
		if err != nil {
			t.Fatalf("リクエストに失敗した: %v", err)
		}
		resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		// /.ok/ は /.ok のハンドラーに届く
		resp, err = http1Client.Get(mainURL + "/.ok/")
		if err != nil {
			t.Fatalf("リクエストに失敗した: %v", err)
		}
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	})

	// 登録したパスへの OPTIONS には、受け付けるメソッドを Allow ヘッダーに入れて 204 を返す
	// メソッドが合わない 405 の Allow ヘッダーにも OPTIONS が入る
	t.Run("OPTIONS に 204 と Allow ヘッダーを返し、405 の Allow ヘッダーにも OPTIONS を含める", func(t *testing.T) {
		testCases := []struct {
			baseURL    string
			method     string
			path       string
			statusCode int
			allow      string
		}{
			{mainURL, http.MethodOptions, "/speech", http.StatusNoContent, "OPTIONS, POST"},
			// ServeMux は GET のルートを HEAD にも一致させるため、HEAD も含める
			{mainURL, http.MethodOptions, "/.ok", http.StatusNoContent, "GET, HEAD, OPTIONS"},
			{mainURL, http.MethodGet, "/speech", http.StatusMethodNotAllowed, "OPTIONS, POST"},
			// 登録していないパスは 404 のまま
			{mainURL, http.MethodOptions, "/not-found", http.StatusNotFound, ""},
			{exporterURL, http.MethodOptions, "/metrics", http.StatusNoContent, "GET, HEAD, OPTIONS"},
		}
		for _, tc := range testCases {
			req, err := http.NewRequestWithContext(t.Context(), tc.method, tc.baseURL+tc.path, nil)
			if err != nil {
				t.Fatalf("リクエストの生成に失敗した: %v", err)
			}
			resp, err := http1Client.Do(req)
			if err != nil {
				t.Fatalf("リクエストに失敗した: %v", err)
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				t.Fatalf("レスポンスボディの読み込みに失敗した: %v", err)
			}

			assert.Equal(t, tc.statusCode, resp.StatusCode, "%s %s のステータスコード", tc.method, tc.path)
			assert.Equal(t, tc.allow, resp.Header.Get("Allow"), "%s %s の Allow ヘッダー", tc.method, tc.path)
			if tc.statusCode == http.StatusNoContent {
				assert.Empty(t, body, "%s %s のボディ", tc.method, tc.path)
			}
		}
	})

	// メトリクスはパッケージで共有するコレクターに記録され、他のテストのリクエストも含まれる
	// そのため、このサブテストのリクエストの前後で値を取得し、差分で確認する
	t.Run("Exporter の /metrics から suzu_http_* を取得でき、ラベルの値が制限されている", func(t *testing.T) {
		before := fetchMetricsForTest(t, http1Client, exporterURL)

		// url ラベルと method ラベルを確認するためのリクエストを送り、ステータスコードもあわせて確認する
		// 404 と 405 は ServeMux が直接応答し、アダプターを通らない
		testCases := []struct {
			client     *http.Client
			method     string
			path       string
			body       []byte
			statusCode int
		}{
			// ルートにマッチする
			{h2cClient, http.MethodGet, "/.ok", nil, http.StatusOK},
			// どのルートにもマッチしない
			{h2cClient, http.MethodGet, "/not-found", nil, http.StatusNotFound},
			// パスはルートに一致するが、メソッドが合わずに 405 になる
			{h2cClient, http.MethodGet, "/speech", nil, http.StatusMethodNotAllowed},
			// net/http が定数として定義していないメソッド
			{h2cClient, "FOO", "/speech", nil, http.StatusMethodNotAllowed},
			// ルートにマッチし、アダプターが 400 を返す
			// リクエストサイズを確認するため、ヘッダーを含めて 2 KB を超え 5 KB 以下になるボディを付ける
			{http1Client, http.MethodPost, "/speech", bytes.Repeat([]byte("a"), 3000), http.StatusBadRequest},
		}
		// GET /.ok のレスポンスボディのバイト数 (response_size_bytes の確認に使う)
		var okBodySize int
		// GET /.ok の、クライアントがリクエストを送信してからレスポンスボディを読み終えるまでの往復時間
		// (request_duration_seconds の確認に使う)
		var okRoundTrip time.Duration
		for _, tc := range testCases {
			req, err := http.NewRequestWithContext(t.Context(), tc.method, mainURL+tc.path, bytes.NewReader(tc.body))
			if err != nil {
				t.Fatalf("リクエストの生成に失敗した: %v", err)
			}
			start := time.Now()
			resp, err := tc.client.Do(req)
			if err != nil {
				t.Fatalf("リクエストに失敗した: %v", err)
			}
			body, err := io.ReadAll(resp.Body)
			roundTrip := time.Since(start)
			resp.Body.Close()
			if err != nil {
				t.Fatalf("レスポンスボディの読み込みに失敗した: %v", err)
			}

			assert.Equal(t, tc.statusCode, resp.StatusCode, "%s %s のステータスコード", tc.method, tc.path)
			if tc.path == "/.ok" {
				okBodySize = len(body)
				okRoundTrip = roundTrip
			}
		}

		after := fetchMetricsForTest(t, http1Client, exporterURL)

		// series の値が前後で delta だけ増えていることを確認する
		assertIncreased := func(series string, delta float64) {
			t.Helper()
			beforeValue, _ := metricValueForTest(t, before, series)
			afterValue, ok := metricValueForTest(t, after, series)
			if assert.True(t, ok, "系列が出力されていない: %s", series) {
				assert.Equal(t, beforeValue+delta, afterValue, "系列の値の増分: %s", series)
			}
		}

		// ルートにマッチした場合はルートのパスが入る
		assertIncreased(`suzu_http_requests_total{code="200",method="GET",url="/.ok"}`, 1)
		// ルートにマッチしなかった場合は、リクエストパスではなく unknown が入る
		assertIncreased(`suzu_http_requests_total{code="404",method="GET",url="unknown"}`, 1)
		// メソッドだけが合わない 405 も、ルートにはマッチしていないため unknown が入る
		assertIncreased(`suzu_http_requests_total{code="405",method="GET",url="unknown"}`, 1)
		// net/http が定数として定義していないメソッドは unknown にまとめられる
		assertIncreased(`suzu_http_requests_total{code="405",method="unknown",url="unknown"}`, 1)

		// POST のパターンからメソッドを除いたパスが url に入り、アダプターが返した 400 が code に入る
		assertIncreased(`suzu_http_requests_total{code="400",method="POST",url="/speech"}`, 1)

		// レスポンスサイズはボディのバイト数
		assertIncreased(`suzu_http_response_size_bytes_sum{code="200",method="GET",url="/.ok"}`, float64(okBodySize))
		// リクエストサイズにはボディのサイズが含まれ、2 KB を超え 5 KB 以下のバケットに入る
		// le="2048" は増えていないこと (増分が 0) を確認する
		assertIncreased(`suzu_http_request_size_bytes_bucket{code="400",method="POST",url="/speech",le="2048"}`, 0)
		assertIncreased(`suzu_http_request_size_bytes_bucket{code="400",method="POST",url="/speech",le="5120"}`, 1)

		// リクエストの処理時間は秒で記録される
		// 計測の開始はリクエストの受信後で、計測はハンドラーが終了してレスポンスの終わりを送る前に確定するため、
		// サーバーが計測する処理時間はクライアントの往復時間に収まる
		// ミリ秒で記録すると値が 1000 倍になり、往復時間を大きく超える
		beforeDuration, _ := metricValueForTest(t, before, `suzu_http_request_duration_seconds_sum{code="200",method="GET",url="/.ok"}`)
		afterDuration, ok := metricValueForTest(t, after, `suzu_http_request_duration_seconds_sum{code="200",method="GET",url="/.ok"}`)
		if assert.True(t, ok, "処理時間の系列が出力されていない") {
			assert.Greater(t, afterDuration-beforeDuration, 0.0)
			assert.LessOrEqual(t, afterDuration-beforeDuration, okRoundTrip.Seconds(), "処理時間がクライアントの往復時間を超えている")
		}

		// 処理時間のバケットは prometheus.DefBuckets (5 ms から 10 秒)
		assertIncreased(`suzu_http_request_duration_seconds_count{code="200",method="GET",url="/.ok"}`, 1)
		for _, le := range []string{"0.005", "0.01", "0.025", "0.05", "0.1", "0.25", "0.5", "1", "2.5", "5", "10", "+Inf"} {
			_, ok := metricValueForTest(t, after, `suzu_http_request_duration_seconds_bucket{code="200",method="GET",url="/.ok",le="`+le+`"}`)
			assert.True(t, ok, "処理時間のバケットの境界が出力されていない: %s", le)
		}

		// サイズのバケットの境界は 1 KB (1024 バイト) から 10 MB
		for _, le := range []string{"1024", "2048", "5120", "10240", "102400", "512000", "1.048576e+06", "2.62144e+06", "5.24288e+06", "1.048576e+07", "+Inf"} {
			_, ok := metricValueForTest(t, after, `suzu_http_response_size_bytes_bucket{code="200",method="GET",url="/.ok",le="`+le+`"}`)
			assert.True(t, ok, "バケットの境界が出力されていない: %s", le)
		}

		// リクエストパスやメソッドがそのままラベルに入ることはない
		assert.NotContains(t, after, `url="/not-found"`)
		assert.NotContains(t, after, `method="FOO"`)
		// Exporter へのリクエストはメトリクスの計測対象にしない
		assert.NotContains(t, after, `url="/metrics"`)
	})
}

// sora-session-id は Ogg ファイルの出力時のファイル名にだけ使われるため、Ogg ファイルの出力を有効にして確認する
// あわせて /speech に NewServer で指定したサービスが割り当てられることも確認する
func TestServerOggFileOutput(t *testing.T) {
	config := Config{
		ListenAddr: "127.0.0.1",
		// Config をリテラルで作ると無音パケットの間隔が 0 になり、空のボディでも無音パケットを受信して外部サービスへの接続に進む場合がある
		// そのため無音パケットを挿入しないようにする
		DisableSilentPacket: true,
		EnableOggFileOutput: true,
		OggDir:              t.TempDir(),
	}

	// aws サービスは Ogg ファイルを作ってから最初の音声を待ち、音声を受信してから外部サービスに接続する
	// ボディを空にすると、Ogg ファイルを作った後、外部サービスに接続する前に EOF で終了する
	s, err := NewServer(&config, "aws")
	if err != nil {
		t.Fatalf("サーバーの生成に失敗した: %v", err)
	}
	mainURL := serveForTest(t, s.mainServer)

	h2cClient := newH2CClientForTest(t)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, mainURL+"/speech", http.NoBody)
	if err != nil {
		t.Fatalf("リクエストの生成に失敗した: %v", err)
	}
	req.Header.Set("sora-channel-id", "test-channel-id")
	req.Header.Set("sora-session-id", "test-session-id")
	req.Header.Set("sora-connection-id", "test-connection-id")
	// aws サービスが対応している言語コードを指定する
	req.Header.Set("sora-audio-streaming-language-code", "ja-JP")

	resp, err := h2cClient.Do(req)
	if err != nil {
		t.Fatalf("リクエストに失敗した: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("レスポンスボディの読み込みに失敗した: %v", err)
	}

	// 音声を受信する前に EOF になったため、結果を送らずに 200 で終了する
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, body)

	// Ogg ファイルの名前は <sora-session-id>-<sora-connection-id>.ogg になる
	entries, err := os.ReadDir(config.OggDir)
	if err != nil {
		t.Fatalf("Ogg ファイルの出力先の読み込みに失敗した: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	assert.Equal(t, []string{"test-session-id-test-connection-id.ogg"}, names)
}

// disable_trailing_slash_removal と disable_options_response を有効にした場合は、末尾のスラッシュを取り除かず、OPTIONS にも応答しない
// 既定 (どちらも無効) の挙動は TestServer で確認する
func TestServerWithTrailingSlashRemovalAndOptionsResponseDisabled(t *testing.T) {
	config := Config{
		ListenAddr:                  "127.0.0.1",
		DisableTrailingSlashRemoval: true,
		DisableOptionsResponse:      true,
	}

	s, err := NewServer(&config, "test")
	if err != nil {
		t.Fatalf("サーバーの生成に失敗した: %v", err)
	}

	mainURL := serveForTest(t, s.mainServer)
	exporterURL := serveForTest(t, s.exporterServer)

	http1Protocols := new(http.Protocols)
	http1Protocols.SetHTTP1(true)
	http1Transport := &http.Transport{Protocols: http1Protocols}
	t.Cleanup(http1Transport.CloseIdleConnections)
	http1Client := &http.Client{Transport: http1Transport}

	testCases := []struct {
		baseURL    string
		method     string
		path       string
		statusCode int
		allow      string
	}{
		// 末尾スラッシュ付きのパスはどのルートにも一致しない
		{mainURL, http.MethodPost, "/speech/", http.StatusNotFound, ""},
		{mainURL, http.MethodGet, "/.ok/", http.StatusNotFound, ""},
		// OPTIONS のルートがないため、メソッドが合わない 405 になり、Allow ヘッダーに OPTIONS を含めない
		{mainURL, http.MethodOptions, "/speech", http.StatusMethodNotAllowed, "POST"},
		{mainURL, http.MethodGet, "/speech", http.StatusMethodNotAllowed, "POST"},
		{exporterURL, http.MethodOptions, "/metrics", http.StatusMethodNotAllowed, "GET, HEAD"},
	}
	for _, tc := range testCases {
		req, err := http.NewRequestWithContext(t.Context(), tc.method, tc.baseURL+tc.path, nil)
		if err != nil {
			t.Fatalf("リクエストの生成に失敗した: %v", err)
		}
		resp, err := http1Client.Do(req)
		if err != nil {
			t.Fatalf("リクエストに失敗した: %v", err)
		}
		resp.Body.Close()

		assert.Equal(t, tc.statusCode, resp.StatusCode, "%s %s のステータスコード", tc.method, tc.path)
		assert.Equal(t, tc.allow, resp.Header.Get("Allow"), "%s %s の Allow ヘッダー", tc.method, tc.path)
	}
}
