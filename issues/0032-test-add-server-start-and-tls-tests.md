# サーバーの起動と停止、TLS、クライアント認証、Exporter の TLS のテストを追加する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/add-server-start-and-tls-tests
- Polished: {YYYY-MM-DD}

## 目的

echo から net/http への移行で書き換えた、サーバーの起動と停止の処理と TLS の経路を、テストで確認できるようにする。

## 現状

- `server.go` の `Server.Start` / `Server.StartExporter` を呼ぶテストがない。`server_test.go` の `serveForTest` は `http.Server.Serve` を直接呼ぶため、これらの経路を通らない。
- 移行で、Exporter の待ち受けを echo の `StartTLS` から `ListenAndServeTLS` に変え、待ち受けのチャネルにバッファを持たせ、`http.ErrServerClosed` の判定を `errors.Is` に変えた。いずれもテストがない。
- `https`、`tls_verify_cacert_path` (クライアント認証)、`exporter_https` を設定するテストがない。`appendCerts` / `appendCertsFromPEM` もテストを通らない。
- 移行の前後で、TLS、クライアント認証、Exporter の TLS の挙動が同じであることは手動で実測して確認した。

## 設計方針

- 証明書と鍵は、テストの中で `crypto/x509` で生成して `t.TempDir()` に書き出す。リポジトリの `cert/` は使わない。
- 次を確認する。
  - ctx を終了すると `Start` / `StartExporter` が `ctx.Err()` を返し、待ち受けが止まる。
  - 使用中のポートを指定すると、`Start` がすぐにエラーを返す。
  - `https = true` で、HTTP/2 over TLS の `GET /.ok` が 200 を返し、`POST /test` の結果を逐次受信できる。
  - `tls_verify_cacert_path` を設定すると、クライアント証明書なしと、別の CA の証明書ではハンドシェイクに失敗し、正しい証明書では成功する。ディレクトリを指定した場合も確認する。
  - `exporter_https = true` で、`/metrics` が 200 を返す。
- `Start` が待ち受けるポートは設定から決まるため、空いているポートを選んでから渡す。ポートの競合を避けるために `Start` が listener を受け取れるようにするなど、テストのためにコードを変える場合は、変更の前にユーザーに確認する。
- モックやスタブは使わない。

## 完了条件

- 上記の各項目を確認するテストがあり、`make test` で通る。
