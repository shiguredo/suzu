# Prometheus 用 Exporter の設定とメトリクスをドキュメント化する

- Created: 2026-09-10
- Completed: {YYYY-MM-DD}
- Branch: feature/update-prometheus-exporter-doc
- Polished: {YYYY-MM-DD}

## 目的

Suzu の Prometheus Exporter の設定方法と、公開されるメトリクスを `doc/USE.md` に記載する。現状は `config_example.ini` のコメントと `CHANGES.md` の 1 行以外に情報がなく、監視側で scrape 設定を用意するときに何が取得できるのか分からない。

## 現状

- Exporter は `server.go` の `NewServer` で `echo-contrib/prometheus` の `NewPrometheus` を使って構築し、`StartExporter` で待ち受ける。
- 設定は `config.ini` の `exporter_https` / `exporter_listen_addr` / `exporter_listen_port` の 3 項目。未設定時のデフォルトは `config.go` の `defaultExporterListenAddr` / `defaultExporterListenPort` で `0.0.0.0` / `5891`。
- メトリクスのパスは echo-contrib のデフォルトで `/metrics`。
- 公開されるメトリクスは echo-contrib の標準メトリクス (`echo_requests_total` / `echo_request_duration_seconds` / `echo_request_size_bytes` / `echo_response_size_bytes`) と、client_golang のデフォルト Registry による `go_*` / `process_*`。
- `exporter_https` は `CHANGES.md` の 2023.5.0 に追加の記録があるだけで、有効化する条件 (`tls_fullchain_file` / `tls_privkey_file` が必要) や使い方はどこにも書かれていない。
- `doc/USE.md` には Exporter に関する記載が一切ない。

## 設計方針

- `doc/USE.md` に Prometheus で監視するためのセクションを追加する。
- 記載内容は以下を最低限とする。
  - 設定キー (`exporter_https` / `exporter_listen_addr` / `exporter_listen_port`) とデフォルト値
  - メトリクスのエンドポイント (`http(s)://{exporter_listen_addr}:{exporter_listen_port}/metrics`)
  - Prometheus の scrape 設定例
  - 公開されるメトリクスの一覧 (`echo_*` / `go_*` / `process_*`)
- 公開メトリクスの一覧は手で書くため、統計情報の追加 (別 issue) が入ると乖離し得る。実装が正であることと、変更時はこの一覧も更新することを添える。
- 既存の doc は Markdown のため Markdown で書く。

## 完了条件

- `doc/USE.md` に Exporter の設定キー、メトリクスエンドポイント、scrape 設定例、公開メトリクスの一覧が記載されている。
- 記載内容が `server.go` / `config.go` の実装と一致している。
