# config_example.ini に記載のない設定を追加し、説明と表記を直す

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-config-example
- Polished: {YYYY-MM-DD}

## 目的

`config_example.ini` を見れば、使える設定項目とその効果が分かるようにする。

## 現状

- `config.go` の `Config` にあるが、`config_example.ini` に記載がない設定がある。
  - `http2_max_concurrent_streams`、`http2_max_read_frame_size`、`http2_idle_timeout`
  - `gcp_enable_spoken_emojis`
- `http2_idle_timeout` は `http.Server` の `IdleTimeout` に設定しており、HTTP/1.1 の keep-alive にも効く。名前からは HTTP/2 だけに効くように読める。
- `disable_trailing_slash_removal` の説明に、末尾スラッシュの除去がメインサーバーだけに適用され、Prometheus の待ち受け (Exporter) には適用されないことが書かれていない。直後の `disable_options_response` は「Prometheus の待ち受けにも適用します」としているため、両方が Exporter に適用されると誤解しうる。
- `disable_options_response` の説明に、405 の `Allow` ヘッダーに `OPTIONS` を含めなくなることが書かれていない。
- `audio_streaming_header` の説明は「推奨値は true です」としているが、例の値は `false` である。
- 表記が shiguredo-doc の規約と異なる。
  - 全角の括弧「（」「）」が多数ある (「（MB）」「（秒）」「（ミリ秒）」など)。
  - 「サーバ」「ヘッダ」と、末尾の長音がない表記がある。
  - 日本語の文に半角のカンマを使っている (`final_result_only` の説明)。
- `aws_http_*` のコメントの値の説明 (既定値の扱い) の誤りは、aws_profile を設定していない場合の既定値の issue で扱う。

## 設計方針

- 記載のない設定を、既定値と効果の説明とともに追加する。`http2_idle_timeout` は HTTP/1.1 にも効くことを書く。
- 末尾スラッシュの除去と OPTIONS への応答の説明を、適用範囲と挙動が分かるように直す。
- `audio_streaming_header` の例の値と説明を一致させる。どちらにそろえるかは、既定値の扱いを確認して決める。
- 表記を shiguredo-doc の規約に合わせる (半角の括弧、「サーバー」「ヘッダー」、日本語の文では読点)。

## 完了条件

- `Config` のすべての設定項目が `config_example.ini` に記載されている。
- 上記の説明と表記の問題がない。
