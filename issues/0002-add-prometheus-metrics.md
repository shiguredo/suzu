# Prometheus で取得できる統計情報を追加する

- Created: 2026-09-10
- Completed: {YYYY-MM-DD}
- Branch: feature/add-prometheus-metrics
- Polished: 2026-11-23
- Refreshed: 2026-11-20

## 目的

Suzu の Prometheus Exporter は HTTP リクエストのメトリクスと、Go/process などのメトリクスしか公開しておらず、音声ストリーミングサービスとしての状態を監視できない。セッション数、音声データ量、外部の音声認識サービスへの接続状況をメトリクスとして取得できるようにし、運用時に何セッション動いているか、どこで失敗しているかをダッシュボードとアラートで把握できるようにする。

## 現状

- Exporter は `server.go` の `NewServer` で、専用の `http.ServeMux` に `promhttp.Handler()` を `GET /metrics` として登録している。公開されるのは以下だけ。
  - `metrics.go` の `suzu_http_requests_total` / `suzu_http_request_duration_seconds` / `suzu_http_request_size_bytes` / `suzu_http_response_size_bytes` (ラベルは `code` / `method` / `url`)。メインサーバーのリクエストの終了時に、`middleware.go` の `newMainHandler` が記録する。
  - client_golang の既定のレジストリと `promhttp.Handler()` による `go_*` / `process_*` / `promhttp_metric_handler_*`
- `/speech` はセッションの間レスポンスを返し続けるストリーミングのため、`suzu_http_request_duration_seconds` のバケット (`prometheus.DefBuckets`、最大 10 秒) では 10 秒を超えるセッションがすべて +Inf バケットに入り、セッションの継続時間の分布を把握できない。
- セッションの開始と終了、リトライ (`handler.go` の `createSpeechHandler`)、外部サービスとのストリームのエラー (`ErrServerDisconnected`) はログにしか出ておらず、メトリクスとして集計されていない。
- 受信した音声データ (`handler.go` の `readPacket`) の量は、ログにもメトリクスにも出ていない。

## 設計方針

- アプリケーション固有のメトリクスは prometheus/client_golang の `NewCounterVec` / `NewGaugeVec` / `NewHistogramVec` で定義し、`prometheus.DefaultRegisterer` に登録する。名前の衝突を避けるため `suzu_` を付ける。
- 登録は既存の HTTP メトリクスと同じく、パッケージの初期化時に 1 回だけ行う (`promauto` を使う)。`NewServer` はテストで複数回呼ばれるため、`NewServer` の中で登録すると 2 回目以降が重複登録になり panic する。
- 追加候補は以下とする。実装時に取捨選択してよい。
  - `suzu_sessions_total{service}`: セッション開始数の累計
  - `suzu_sessions_active{service}`: 現在のセッション数
  - `suzu_session_retries_total{service}`: 外部サービスへの再接続回数
  - `suzu_audio_bytes_received_total{service}`: Sora から受信したバイト数 (`readPacket` が `r.Body` から読み取ったバイト数。`audio_streaming_header` が有効な場合は音声ストリーミングのヘッダーを含む)
  - `suzu_results_sent_total{service}`: Sora へ送信した認識結果の件数
  - `suzu_upstream_disconnects_total{service}`: 外部サービスとのストリームがエラーで終了した回数 (`ErrServerDisconnected` を含むエラーが検出された回数。切断だけでなく、再接続対象のエラーを含む)
  - `suzu_session_duration_seconds{service}`: セッションの継続時間 (バケットは 10 秒から 1 時間程度)
- セッションの開始は `createSpeechHandler` の `CONNECTED` のログと同じ箇所で記録する。HTTP/2 でないリクエストや言語コードのエラーはセッションに含めない。セッションを開始した場合だけ、`DISCONNECTED` のログと同じ箇所 (ハンドラーが終了するすべての経路の defer) で `suzu_sessions_active` の減算と `suzu_session_duration_seconds` の観測を行う。
- `service` ラベルには `getServiceHandler` に渡される service type (`aws` / `awsv2` / `gcp` / `test` / `dump`) を使う。
- 追加したメトリクスの `doc/USE.md` への追記は別 issue とする。

## 完了条件

- 追加したメトリクスが `/metrics` から取得できる。
- セッションの開始と終了、リトライ、音声データの受信、外部サービスとのストリームのエラーがメトリクスに反映されることを確認できる。
