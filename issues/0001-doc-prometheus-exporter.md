# Prometheus 用 Exporter の設定とメトリクスをドキュメント化する

- Created: 2026-09-10
- Completed: {YYYY-MM-DD}
- Branch: feature/update-prometheus-exporter-doc
- Polished: {YYYY-MM-DD}
- Refreshed: 2026-11-20

## 目的

Suzu の Prometheus Exporter の設定方法と、公開されるメトリクスを `doc/USE.md` に記載する。現状は `config_example.ini` のコメントと `CHANGES.md` の変更の記録 (2023.5.0 の `exporter_https` の追加と、`## develop` の net/http への移行の記載) 以外に情報がなく、監視側で scrape 設定を用意するときに何が取得できるのか分からない。

## 現状

- Exporter は `server.go` の `NewServer` で、専用の `http.ServeMux` に `promhttp.Handler()` を `GET /metrics` として登録して組み立て (`Server.exporterServer`)、`StartExporter` で待ち受ける。
- HTTP メトリクスは `metrics.go` で定義し、パッケージの初期化時に既定のレジストリに登録する。計測するのはメインサーバーのリクエストだけで (`middleware.go` の `newMainHandler` から `observeHTTPMetrics` を呼ぶ)、Exporter へのリクエストは計測しない。
- Exporter 固有の設定は `config.ini` の `exporter_https` / `exporter_listen_addr` / `exporter_listen_port` の 3 項目。未設定時のデフォルトは `config.go` の `defaultExporterListenAddr` / `defaultExporterListenPort` で `0.0.0.0` / `5891`。
  - ほかに `disable_options_response` が Exporter にも適用される (true にすると `OPTIONS /metrics` が 405 になる)。`disable_trailing_slash_removal` は Exporter には適用されず、`/metrics/` は常に 404 になる。
- メトリクスのパスは `NewServer` で登録した `/metrics` (固定)。GET と HEAD に応答する。
- 公開されるメトリクスは `metrics.go` の HTTP メトリクス (`suzu_http_requests_total` / `suzu_http_request_duration_seconds` / `suzu_http_request_size_bytes` / `suzu_http_response_size_bytes`、ラベルは `code` / `method` / `url`) と、client_golang の既定のレジストリと `promhttp.Handler()` による `go_*` / `process_*` / `promhttp_metric_handler_*`。
- `exporter_https` は `CHANGES.md` の 2023.5.0 に追加の記録があるだけで、有効化する条件 (`tls_fullchain_file` / `tls_privkey_file` が必要) や使い方はどこにも書かれていない。
- `doc/USE.md` には Exporter に関する記載が一切ない。

## 設計方針

- `doc/USE.md` に Prometheus で監視するためのセクションを追加する。
- 記載内容は以下を最低限とする。
  - 設定キー (`exporter_https` / `exporter_listen_addr` / `exporter_listen_port`) とデフォルト値
  - メトリクスのエンドポイント (`http(s)://{exporter_listen_addr}:{exporter_listen_port}/metrics`)
  - Prometheus の scrape 設定例
  - 公開されるメトリクスの一覧 (`suzu_http_*` とそのラベル / `go_*` / `process_*` / `promhttp_metric_handler_*`)
- 公開メトリクスの一覧は手で書くため、統計情報の追加 (別 issue) が入ると乖離し得る。実装が正であることと、変更時はこの一覧も更新することを添える。
- 既存の doc は Markdown のため Markdown で書く。

## 完了条件

- `doc/USE.md` に Exporter の設定キー、メトリクスエンドポイント、scrape 設定例、公開メトリクスの一覧が記載されている。
- 記載内容が `server.go` / `metrics.go` / `config.go` の実装と一致している。
