# 起動時に設定値と -service の値を検証する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/change-validate-config-at-startup
- Polished: {YYYY-MM-DD}

## 目的

意図と異なる設定のまま起動して、運用中に気づく事態を防ぐ。起動時に検証できるものは起動時にエラーにする。

## 現状

- `http2_max_concurrent_streams` / `http2_max_read_frame_size` は、0 または範囲外の値を指定すると、Go の net/http が黙って既定値を使う (`http.HTTP2Config` の説明は「If zero or invalid, a default value is used」)。
  - `MaxConcurrentStreams` は 0 か範囲外で 250、`MaxReadFrameSize` は 0、16 KiB 未満、16 MiB - 1 を超える値で 1 MiB になる。
- `http2_idle_timeout` は `http.Server` の `IdleTimeout` に設定しており、HTTP/1.1 の keep-alive にも効く。名前からは HTTP/2 だけに効くように読める。
- `https = false` のまま `tls_verify_cacert_path` を設定すると、`server.go` の `NewServer` は `TLSConfig` を設定するが、`Start` は `ListenAndServe` で待ち受けるため使われない。クライアント認証なしで待ち受け、`config.go` の `validateConfig` もこの組み合わせを拒否しない。
- `cmd/suzu/main.go` は `-service` の値を検証しない。登録されていないサービス名でも起動し、すべての `/speech` が `languages.go` の `ErrUnsupportedService` で 500 になる。
- これらの設定と挙動は、`config_example.ini` に書かれていない (`http2_*` は記載自体がない)。

## 設計方針

- `validateConfig` で次をエラーにする。
  - `http2_max_concurrent_streams` / `http2_max_read_frame_size` が範囲外の値 (0 は既定値として許可する)
  - `https = false` かつ `tls_verify_cacert_path` が設定されている
- `-service` の値は、登録されているサービス名 (`NewServiceHandlerFuncs`) に含まれない場合に起動時にエラーにする。
- `http2_idle_timeout` が HTTP/1.1 にも効くことは、設定のドキュメントの issue で扱う。
- `CHANGES.md` の `## develop` に、起動できなくなる設定の組み合わせを後方互換のない変更として記載する。

## 完了条件

- 上記の不正な設定と `-service` の値で、原因の分かるエラーで起動を中止する。
- 上記を確認するテストがある。
