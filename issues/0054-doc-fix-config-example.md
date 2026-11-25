# config_example.ini に記載のない設定を追加し、説明と表記を直す

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-config-example
- Polished: 2026-11-25

## 目的

`config_example.ini` を見れば、使える設定項目とその効果が分かるようにする。

## 現状

- `config.go` の `Config` にあるが、`config_example.ini` に記載がない設定がある。
  - `http2_max_concurrent_streams`、`http2_max_read_frame_size`、`http2_idle_timeout`
  - `gcp_enable_spoken_emojis`
  - `basic_auth_username` と `basic_auth_password` も記載がないが、これらは `issues/0048-remove-unused-basic-auth-config.md` で削除するため、本 issue では追加も変更もしない。`skip_basic_auth` は `config_example.ini` にあるが、同じく 0048 で削除する。
  - `Config.Version` は `ini` タグがなく、`VERSION` から設定するため、`config_example.ini` には記載しない。`issues/0049-remove-dead-code.md` で削除する予定である。
- `http2_max_concurrent_streams`、`http2_max_read_frame_size`、`http2_idle_timeout` は、`server.go` の `NewServer` でメインサーバーにだけ設定している。Exporter には設定していない。
- `http2_idle_timeout` は `http.Server` の `IdleTimeout` に設定しており、HTTP/1.1 の keep-alive にも効く。名前からは HTTP/2 だけに効くように読める。単位は秒である。
- `http2_max_concurrent_streams` と `http2_max_read_frame_size` は、`0` の場合は Go の既定値になる。`http2_max_read_frame_size` は 16 KiB から 16 MiB の範囲だけが有効で、範囲外は Go の既定値になる。`http2_idle_timeout` は `0` の場合はタイムアウトしない。
- `disable_trailing_slash_removal` の説明に、末尾スラッシュの除去がメインサーバーだけに適用され、Prometheus の待ち受け (Exporter) には適用されないことが書かれていない。直後の `disable_options_response` は「Prometheus の待ち受けにも適用します」としているため、両方が Exporter に適用されると誤解しうる。
- `disable_options_response` の説明は「204 と Allow ヘッダーを返さない」としているが、`true` にした場合の OPTIONS は 405 で、405 の応答には ServeMux が `Allow` ヘッダーを付ける。`Allow` に `OPTIONS` を含めないことが書かれていない。
- `audio_streaming_header` の説明は「推奨値は `true` です」としているが、例の値は `false` である。`config.go` の `setDefaultsConfig` は `AudioStreamingHeader` を設定していないため、既定値は `false` である。`doc/USE.md` は `true` を推奨している。
- 表記が shiguredo-doc の規約と異なる。
  - 全角の括弧「（」「）」が多数ある (「（MB）」「（秒）」「（ミリ秒）」など)。
  - 「サーバ」「ヘッダ」と、末尾の長音がない表記がある。
  - 日本語の文に半角のカンマを使っている (`final_result_only` と `aws_profile` の説明など)。
- `aws_http_*` のコメントの値の説明 (既定値の扱い) の誤りは、`issues/0013-bug-fix-aws-http-client-defaults-lost.md` で扱う。

## 設計方針

- `issues/0048-remove-unused-basic-auth-config.md` と `issues/0013-bug-fix-aws-http-client-defaults-lost.md` を先に完了させる。0048 で削除する `skip_basic_auth`、`basic_auth_username`、`basic_auth_password` は本 issue では追加も変更もしない。
- 記載のない設定を、既定値と効果の説明とともに追加する。
  - `http2_max_concurrent_streams`、`http2_max_read_frame_size`、`http2_idle_timeout` はメインサーバーにだけ適用されることを書く。
  - `http2_max_concurrent_streams` と `http2_max_read_frame_size` は `0` の場合は Go の既定値になること、`http2_max_read_frame_size` はバイト単位で 16 KiB から 16 MiB の範囲だけが有効であることを書く。
  - `http2_idle_timeout` は秒単位で、`0` の場合はタイムアウトしないこと、HTTP/1.1 の keep-alive にも効くことを書く。
  - `gcp_enable_spoken_emojis` は、`true` の場合は最終結果の話し言葉の絵文字を Unicode の絵文字に変換し、既定値は `false` であることを書く。
- 末尾スラッシュの除去と OPTIONS への応答の説明を、適用範囲と挙動が分かるように直す。
  - `disable_trailing_slash_removal` はメインサーバーだけに適用され、Prometheus の待ち受けには適用されないことを書く。
  - `disable_options_response` は、`true` にした場合に OPTIONS が 405 になり、405 の `Allow` ヘッダーに `OPTIONS` を含めないこと、Prometheus の待ち受けにも適用することを書く。
- `audio_streaming_header` の例の値を `true` にし、説明に既定値が `false` であることと、クライアントがヘッダーを付与する場合は `true` を指定することを書く。`doc/USE.md` の推奨と合わせる。
- 表記を shiguredo-doc の規約に合わせる (半角の括弧と前後の半角スペース、「サーバー」「ヘッダー」、日本語の文では読点)。
- `aws_http_*` のコメントの値と説明は 0013 で直す。本 issue では 0013 の変更後のコメントの表記だけを直す。

## 完了条件

- `issues/0048-remove-unused-basic-auth-config.md` と `issues/0013-bug-fix-aws-http-client-defaults-lost.md` が完了している。
- `Config` のすべての設定項目 (0048 で削除する項目と `Config.Version` を除く) が `config_example.ini` に記載されている。
- 追加した `http2_max_concurrent_streams`、`http2_max_read_frame_size`、`http2_idle_timeout`、`gcp_enable_spoken_emojis` の説明に、既定値と効果 (適用範囲を含む) が書かれている。
- `disable_trailing_slash_removal` の説明に、メインサーバーだけに適用され、Prometheus の待ち受けには適用されないことが書かれている。
- `disable_options_response` の説明に、`true` にした場合の OPTIONS が 405 になり、405 の `Allow` ヘッダーに `OPTIONS` を含めないこと、Prometheus の待ち受けにも適用することが書かれている。
- `audio_streaming_header` の例の値が `true` で、説明に既定値が `false` であることが書かれている。
- `config_example.ini` の表記が shiguredo-doc の規約に合っている。
- `aws_http_*` のコメントの値と説明は 0013 の完了条件に従い、本 issue では表記だけを直している。
- `config_example.ini` を `NewConfig` で読み込める。
