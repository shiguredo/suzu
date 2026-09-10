# Prometheus で取得できる統計情報を追加する

- Created: 2026-09-10
- Completed: {YYYY-MM-DD}
- Branch: feature/add-prometheus-metrics
- Polished: {YYYY-MM-DD}

## 目的

Suzu の Prometheus Exporter は HTTP レベルのメトリクスしか公開しておらず、音声ストリーミングサービスとしての状態を監視できない。セッション数、音声データ量、外部の音声認識サービスへの接続状況をメトリクスとして取得できるようにし、運用時に何セッション動いているか、どこで失敗しているかをダッシュボードとアラートで把握できるようにする。

## 現状

- `server.go` の `NewServer` で `echo-contrib/prometheus` の `NewPrometheus("echo", nil)` を利用している。公開されるのは以下だけ。
  - `echo_requests_total` / `echo_request_duration_seconds` / `echo_request_size_bytes` / `echo_response_size_bytes`
  - client_golang のデフォルト Registry による `go_*` / `process_*`
- `echo-contrib/prometheus` パッケージは Deprecated で、後継は `echo-contrib/echoprometheus`。
- `/speech` はセッションの間レスポンスを返し続けるストリーミングのため、`echo_request_duration_seconds` のデフォルトバケット (最大 10 秒) ではセッションの継続時間を把握できない。
- セッションの開始と終了、リトライ (`handler.go` の `createSpeechHandler`)、受信した音声データ (`handler.go` の `readPacket`)、外部サービスからの切断 (`ErrServerDisconnected`) はログにしか出ておらず、メトリクスとして集計されていない。

## 設計方針

- アプリケーション固有のメトリクスは prometheus/client_golang の `NewCounterVec` / `NewGaugeVec` / `NewHistogramVec` で定義し、`prometheus.DefaultRegisterer` に登録する。名前の衝突を避けるため `suzu_` を付ける。
- 追加候補は以下とする。実装時に取捨選択してよい。
  - `suzu_sessions_total{service}`: セッション開始数の累計
  - `suzu_sessions_active{service}`: 現在のセッション数
  - `suzu_session_retries_total{service}`: 外部サービスへの再接続回数
  - `suzu_audio_bytes_received_total{service}`: Sora から受信した音声データ量
  - `suzu_results_sent_total{service}`: Sora へ送信した認識結果の件数
  - `suzu_upstream_disconnects_total{service}`: 外部サービスから切断された回数
  - `suzu_session_duration_seconds{service}`: セッションの継続時間 (バケットは 10 秒から 1 時間程度)
- `service` ラベルには `getServiceHandler` に渡される service type (`aws` / `awsv2` / `gcp` / `test` / `dump`) を使う。
- `echo-contrib/prometheus` が Deprecated であることは認識しておく。`echoprometheus` への移行はメトリクス名 (`echo_*`) が変わり監視側への影響があるため本 issue には含めず、必要になった時点で別 issue とする。
- 追加したメトリクスの `doc/USE.md` への追記は別 issue とする。

## 完了条件

- 追加したメトリクスが `/metrics` から取得できる。
- セッションの開始と終了、リトライ、音声データの受信、外部サービスからの切断がメトリクスに反映されることを確認できる。
