# aws_profile を設定していない場合に AWS SDK の既定の HTTP クライアントの設定が失われる

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-aws-http-client-defaults-lost
- Polished: 2026-11-24

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
- `config_example.ini` の `aws_http_*` の説明は「コメントアウトしてある設定値は、net/http パッケージの DefaultTransport の設定を参考に指定」としているが、実際の `Config` の値は 0 (bool は false) である。`aws_http_*` のコメントは値を示す形式で、他の設定項目にも同じ形式があるため、読み手はコメントの値を既定値だと誤解する。

## 設計方針

- `amazon_transcribe_v2.go` の `NewAmazonTranscribeClientV2` で、`aws_http_*` を設定していない項目は AWS SDK の既定値を使う。`awshttp.NewBuildableClient().WithTransportOptions` で既定の Transport を基に、設定された項目だけを上書きする方式を第一候補とする。
- `Config` の `aws_http_*` は int と bool のため、未設定と 0 / false を区別できない。0 と false は未設定として扱い、AWS SDK の既定値を残す。このため、0 を指定した場合の挙動が変わる (たとえばタイムアウトを無効にできなくなる)。この扱いを `config_example.ini` に書く。
- `Proxy` は常に `http.ProxyFromEnvironment` を使う。
- `AWS_DEFAULTS_MODE` を `standard` などに設定した場合、`transcribestreaming` の `resolveHTTPClient` が `TLSHandshakeTimeout` と接続のタイムアウトを既定モードの値で上書きする (standard は 3100 ミリ秒)。この場合 `aws_http_tls_handshake_timeout_ms` は効かない。2026.2.4 の `aws_profile` 未設定時も同じ経路だったためこの挙動は許容し、テストは既定 (legacy) で行う。この挙動を `config_example.ini` に書く。
- `config_example.ini` の `aws_http_*` のコメントの値を、AWS SDK の既定値と一致させる。「net/http パッケージの DefaultTransport の設定を参考に指定」という説明も、コメントアウトした値が AWS SDK の既定の Transport の値である旨に直す。
- `amazon_transcribe_v2_test.go` の `TestNewAmazonTranscribeClientV2_WithoutProfile_AppliesHTTPClientAndLogMode` は、`client.Options().HTTPClient` を `*http.Client` と断言している。`*awshttp.BuildableClient` と `GetTransport()` に合わせて書き換え、`Proxy` と未設定項目の既定値を確認する。
- `CHANGES.md` の `## develop` の `[FIX] config.ini で aws_profile が設定されていない場合でも、config.ini に設定した HTTPClient 関連の設定を反映する` を `[CHANGE]` に変更し、`## develop` の CHANGE の位置に移す。`aws_profile` の有無にかかわらず未設定の `aws_http_*` が AWS SDK の既定値になること、プロキシの環境変数が反映されること、0 を指定しても AWS SDK の既定値になることを書く。

## 完了条件

- `aws_profile` の有無にかかわらず、`aws_http_*` を設定していない項目が AWS SDK の既定値になり、プロキシの環境変数が反映される。
- `config_example.ini` の `aws_http_*` のコメントの値と説明が、AWS SDK の既定値と一致し、0 / false を指定した場合の扱いと `AWS_DEFAULTS_MODE` による上書きが書かれている。
- `amazon_transcribe_v2_test.go` の `TestNewAmazonTranscribeClientV2_WithoutProfile_AppliesHTTPClientAndLogMode` が新しい HTTP クライアントの型に合わせて書き換えられ、生成した HTTP クライアントの Transport の値 (プロキシと未設定項目の既定値を含む) を確認している。
- `CHANGES.md` の `## develop` の該当するエントリが `[CHANGE]` になり、実際の挙動と一致している。
