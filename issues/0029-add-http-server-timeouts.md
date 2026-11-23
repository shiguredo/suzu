# メインサーバーと Exporter にリクエストヘッダーの読み込みのタイムアウトを設定する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/add-http-server-timeouts
- Polished: {YYYY-MM-DD}

## 目的

リクエストヘッダーを送り終えない接続が、待ち受けのリソースを無期限に占有しないようにする。

## 現状

- `server.go` の `NewServer` が作る `http.Server` (メインサーバーと Exporter) は、`ReadHeaderTimeout` と `ReadTimeout` を設定していない。net/http では 0 は無制限である。
- メインサーバーは、`/.ok` のヘルスチェックのために HTTP/1.1 も受け付ける。
- `IdleTimeout` は `http2_idle_timeout` で設定しているが、既定は 0 のため、`ReadTimeout` も 0 の場合は無制限になる。
- echo を使っていた頃から同じ設定である。

## 設計方針

- メインサーバーと Exporter に `ReadHeaderTimeout` を設定する。値は設定で指定できるようにし、既定値は 0 以外にする。
- `ReadTimeout` と `WriteTimeout` は、長時間続く音声ストリーミングのリクエストを切ってしまうため設定しない。その理由をコメントに書く。
- `config_example.ini` に記載し、`CHANGES.md` の `## develop` に記載する。

## 完了条件

- リクエストヘッダーを送り終えない接続が、設定した時間で閉じられる。
- 音声ストリーミングのセッションは、これまでどおり時間で切られない。
- 上記を確認するテストがある。
