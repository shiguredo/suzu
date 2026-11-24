# 起動時に設定値と -service の値を検証する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/change-validate-config-at-startup
- Polished: 2026-11-24

## 目的

意図と異なる設定のまま起動して、運用中に気づく事態を防ぐ。起動時に検証できるものは起動時にエラーにする。

## 現状

- `http2_max_concurrent_streams` / `http2_max_read_frame_size` は、0 または範囲外の値を指定すると、Go 1.27.4 の net/http が黙って既定値を使う。
  - `http.HTTP2Config` の `MaxReadFrameSize` の説明は「If zero or invalid, a default value is used」、`MaxConcurrentStreams` の説明は「If zero, MaxConcurrentStreams defaults to at least 100」である。
  - `net/http/internal/http2` の `setConfigDefaults` では、`MaxConcurrentStreams` は 1 以上 `math.MaxInt32` 以下、`MaxReadFrameSize` は 16 KiB 以上 16 MiB - 1 以下が有効で、それ以外は既定値になる。既定値はそれぞれ 250 と 1 MiB である。
- `http2_idle_timeout` は `http.Server` の `IdleTimeout` に設定しており、HTTP/1.1 の keep-alive にも効く。名前からは HTTP/2 だけに効くように読める。
- `https = false` のまま `tls_verify_cacert_path` を設定すると、`server.go` の `NewServer` は `TLSConfig` を設定するが、`Start` は `ListenAndServe` で待ち受けるため使われない。クライアント認証なしで待ち受け、`config.go` の `validateConfig` もこの組み合わせを拒否しない。
- `cmd/suzu/main.go` は `-service` の値を検証しない。登録されていないサービス名でも起動する。HTTP/2 の `/speech` は、言語コードのヘッダーがある場合は `languages.go` の `ErrUnsupportedService` で 500 になり、ヘッダーがない場合も `ErrMissingAudioStreamingLanguageCode` で 500 になる。
- `http2_*` は `config_example.ini` に記載自体がなく、`tls_verify_cacert_path` には `https = true` が必要であることが書かれていない。`-service` は設定ファイルの項目ではない。

## 設計方針

- `config.go` の `validateConfig` で次をエラーにする。エラーには設定名と、該当する場合は有効な範囲を含める。
  - `http2_max_concurrent_streams` が 0 でも 1 以上 2147483647 (`math.MaxInt32`) 以下でもない
  - `http2_max_read_frame_size` が 0 でも 16 KiB 以上 16 MiB - 1 以下でもない
  - `https = false` かつ `tls_verify_cacert_path` が設定されている
- `-service` の値は、`server.go` の `NewServer` で検証し、登録されているサービス名に含まれない場合にエラーにする。エラーには指定された値と有効なサービス名を含める。`cmd/suzu/main.go` は `NewServer` のエラーで起動を中止する。テストから呼べるよう、`main` ではなく `NewServer` で検証する。
- `NewServiceHandlerFuncs` に登録されているサービス名は `aws`、`awsv2`、`gcp`、`test`、`dump` である。`test` と `dump` も `-service` に指定できるため、検証には `test` と `dump` を除外しない一覧を使う。`issues/0044-refactor-reduce-exported-symbols.md` で `NewServiceHandlerFuncs` を非公開にする場合は、その issue で追加するサービス名の取得関数を使う。
- `http2_idle_timeout` が HTTP/1.1 にも効くことと、`config_example.ini` の `http2_*` の記載は、設定のドキュメントの issue (`issues/0054-doc-fix-config-example.md`) で扱う。
- `config_example.ini` の `tls_verify_cacert_path` の説明に、`https = true` が必要であることを書く。
- `CHANGES.md` の `## develop` に、起動できなくなる設定の値と組み合わせ、`-service` の値を、後方互換のない変更として記載する。

## 完了条件

- 上記の不正な設定と `-service` の値で、原因の分かるエラーで起動を中止する。
- `NewConfig` と `NewServer` を直接呼ぶテストで、上記のエラーと、有効な値 (0 を含む) ではエラーにならないことを確認する。
- `config_example.ini` の `tls_verify_cacert_path` の説明に、`https = true` が必要であることが書かれている。
- `CHANGES.md` の `## develop` に、起動できなくなる設定の値と組み合わせ、`-service` の値が、後方互換のない変更として記載されている。
