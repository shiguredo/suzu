# aws_profile を設定していない場合に AWS SDK の既定の HTTP クライアントの設定が失われる

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-aws-http-client-defaults-lost
- Polished: {YYYY-MM-DD}

## 目的

`aws_profile` を設定していない利用者が、リリース済みの 2026.2.4 と同じく、プロキシの環境変数と AWS SDK の既定のタイムアウトを使えるようにする。あわせて、`config_example.ini` の `aws_http_*` の既定値の説明を実装と一致させる。

## 現状

- 2026.2.4 の `amazon_transcribe_v2.go` の `NewAmazonTranscribeClientV2` は、`aws_profile` を設定していない場合に `config.WithHTTPClient` を渡していなかった。そのため AWS SDK の既定の HTTP クライアントが使われていた。既定の Transport (aws-sdk-go-v2 の `aws/transport/http` の `BuildableClient`) は次の値を持つ。
  - `Proxy: http.ProxyFromEnvironment`
  - 接続のタイムアウト 30 秒、TLS ハンドシェイクのタイムアウト 10 秒、アイドル接続のタイムアウト 90 秒
  - `MaxIdleConns` 100、`MaxIdleConnsPerHost` 10、`MaxConnsPerHost` 2048、`ExpectContinueTimeout` 1 秒
  - `ForceAttemptHTTP2: true`、TLS 1.2 以上
- develop の `NewAmazonTranscribeClientV2` は、`aws_profile` の有無にかかわらず、`aws_http_*` の設定値だけで組み立てた `http.Transport` を `config.WithHTTPClient` で常に渡す。
  - `Proxy` を設定していないため、`HTTPS_PROXY` / `NO_PROXY` の環境変数を無視する。
  - `aws_http_*` を設定しない項目は 0 になる。たとえば TLS ハンドシェイクのタイムアウトとアイドル接続のタイムアウトは無制限になる。
- この変化は `CHANGES.md` の `## develop` の `[FIX] config.ini で aws_profile が設定されていない場合でも、config.ini に設定した HTTPClient 関連の設定を反映する` に書かれておらず、後方互換のない変更として扱われていない。
- `config_example.ini` の `aws_http_*` の説明は「コメントアウトしてある設定値は、net/http パッケージの DefaultTransport の設定を参考に指定」としているが、実際の既定値は 0 である。このファイルの他の項目はコメントの値が既定値のため、読み手は既定値だと誤解する。

## 設計方針

- `aws_http_*` を設定していない項目は、AWS SDK の既定値を使う。`awshttp.NewBuildableClient().WithTransportOptions` で既定の Transport を基に、設定された項目だけを上書きする方式を第一候補とする。
- `Proxy` は常に `http.ProxyFromEnvironment` を使う。
- `config_example.ini` の `aws_http_*` のコメントの値を、実際の既定値と一致させる。
- `CHANGES.md` の `## develop` の該当する記載を、実際の挙動に合わせる。

## 完了条件

- `aws_profile` の有無にかかわらず、`aws_http_*` を設定していない項目が AWS SDK の既定値になり、プロキシの環境変数が反映される。
- `config_example.ini` の `aws_http_*` のコメントの値が、実際の既定値と一致する。
- 生成した HTTP クライアントの Transport の値を確認するテストがある。
