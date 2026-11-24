# ルーティングの細部とメトリクスのラベル、ResponseWriter のラッパーのテストを追加する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/add-routing-and-metrics-label-tests
- Polished: 2026-11-25

## 目的

`CHANGES.md` の `## develop` に書いた、echo から net/http への移行によるルーティングとメトリクスの変化を、テストで固定する。あわせて、移行で追加した `responseWriter` のラッパーの `Unwrap` と、`CHANGES.md` に明記していない `GET *` の 400 をテストで固定する。

ルーティングの挙動とメトリクスの `url` ラベルは `http.ServeMux` の実装の細部に依存し、`Unwrap` は `http.ResponseController` に依存するため、Go の更新で変わった場合に検出できるようにする。

## 現状

- 次の挙動を確認するテストがない。
  - `HEAD /.ok` と Exporter の `HEAD /metrics` が 200 を返す。
  - `//speech` のような正規化されていないパスが 307 になり、そのときの `url` ラベルが、正規化した後のパスとメソッドに一致するルートがあればそのルートのパス、なければ `unknown` になる。
  - `/%73peech` のようなパーセントエンコードされたパスが、デコードしたパスとしてルートに一致する。
  - 不正な UTF-8 のパス (`/%ff`) が 404 になり、panic しない。
  - `GET *` が 400 になり、`url` ラベルが `unknown` になる。
- `metrics.go` の `httpMetricsURLLabelValue` と `httpMetricsMethodLabelValue` に単体テストがない。
- `middleware.go` の `responseWriter.Unwrap` を通るテストがない。`http.ResponseController` の `SetWriteDeadline` などが元の ResponseWriter に届くことは、手動で確認しただけである。

## 設計方針

- ルーティングの挙動は、`server_test.go` の `TestServer` と同じく、`NewServer` が組み立てた `http.Server` を待ち受けて確認する。
- `httpMetricsURLLabelValue` と `httpMetricsMethodLabelValue` は、テーブル駆動の単体テストで確認する。
- `Unwrap` は、ラッパー越しの `http.NewResponseController(w).SetWriteDeadline` がエラーを返さないことを、実際のサーバーで確認する。
- モックやスタブは使わない。

## 完了条件

- 上記の各挙動を確認するテストがあり、`make test` で通る。
