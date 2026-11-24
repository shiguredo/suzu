# メインサーバーと Exporter に HTTP/1.1 のリクエストヘッダーの読み込みのタイムアウトを設定する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/add-http-server-timeouts
- Polished: 2026-11-25

## 目的

HTTP/1.1 のリクエストヘッダーを送り終えない接続が、待ち受けのリソースを無期限に占有しないようにする。

## 現状

- `server.go` の `NewServer` が作る `http.Server` (メインサーバーと Exporter) は、`ReadHeaderTimeout`、`ReadTimeout`、`WriteTimeout` を設定していない。`ReadHeaderTimeout` が 0 の場合は `ReadTimeout` が使われ、`ReadTimeout` も 0 の場合は無制限になる。
- メインサーバーは、`/.ok` のヘルスチェックのために HTTP/1.1 も受け付ける。
- `IdleTimeout` は `http2_idle_timeout` で設定しているが、`http2_idle_timeout` の既定値は 0 で、`ReadTimeout` も 0 のため、アイドル接続も無制限になる。
- echo を使っていた頃から同じ設定である。

## 設計方針

- `config.go` の `Config` に `HTTPReadHeaderTimeoutSec int` (`http_read_header_timeout_sec`) を追加する。単位は秒とする。
  - `setDefaultsConfig` で、未設定または 0 の場合は既定値の 5 秒を設定する。負の値は `validateConfig` でエラーにする。
  - メインサーバーと Exporter の両方で、この同じ設定値を使う。
  - `config_example.ini` の HTTP サーバーの設定の近くに、設定項目、既定値、意味を記載する。
- `server.go` の `NewServer` で、メインサーバーと Exporter の `http.Server` の `ReadHeaderTimeout` に、設定値を `time.Duration` に変換して設定する。
- `ReadHeaderTimeout` は HTTP/1.1 のリクエストヘッダーの読み込みに使われる。HTTP/2 のリクエストヘッダーは Go の HTTP/2 サーバーが処理するため、この設定の対象外とする。このことを `server.go` の `NewServer` のコメントに書く。
- メインサーバーと Exporter のどちらにも、`ReadTimeout` と `WriteTimeout` は設定しない。メインサーバーでは、長時間続く音声ストリーミングのリクエストを切ってしまうためである。その理由を `server.go` の `NewServer` のコメントに書く。
- `CHANGES.md` の `## develop` に `[ADD]` として、設定項目、既定値、設定した場合の挙動を記載する。担当者の行を含める。

## 完了条件

- HTTP/1.1 のリクエストヘッダーを送り終えない接続が、設定した時間で閉じられる。
- 音声ストリーミングのセッションは、これまでどおり時間で切られない。
- `server.go` の `NewServer` に、`ReadTimeout` と `WriteTimeout` を設定しない理由と、`ReadHeaderTimeout` が HTTP/1.1 のリクエストヘッダーの読み込みに使われ、HTTP/2 のリクエストヘッダーは対象外であることがコメントで書かれている。
- 上記を確認するテストが `server_test.go` にあり、`make test` で通る。テストでは次のことを確認する。
  - `NewServer` が返す `mainServer` と `exporterServer` の `ReadHeaderTimeout` が、設定した値になること。
  - `http_read_header_timeout_sec` に 1 秒などの短い値を設定し、TCP 接続でリクエストヘッダーの一部だけを送り、期限を設けて接続が閉じられることを確認すること。
  - 同じ設定で、音声ストリーミングのリクエストをヘッダーのタイムアウトより長く続けても、レスポンスが途中で切られないこと。
- `config_test.go` に、`http_read_header_timeout_sec` が未設定または 0 の場合は `HTTPReadHeaderTimeoutSec` が 5 になり、負の値の場合は `NewConfig` がエラーになることを確認するテストがある。
- `config_example.ini` に `http_read_header_timeout_sec` が記載され、既定値と意味が分かる。
- `CHANGES.md` の `## develop` に `[ADD]` のエントリがある。
