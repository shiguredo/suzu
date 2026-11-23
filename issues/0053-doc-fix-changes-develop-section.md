# CHANGES.md の develop の種別の順序と、echo から net/http への移行の記載を正確にする

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-changes-develop-section
- Polished: {YYYY-MM-DD}

## 目的

`CHANGES.md` の `## develop` を shiguredo-changelog の規約に沿わせ、echo から net/http への移行による変化を、運用者が正確に把握できるようにする。

## 現状

### 規約の違反

- shiguredo-changelog は、エントリを CHANGE → ADD → UPDATE → FIX の順に書くと定めている。`## develop` は `[CHANGE]`、`[ADD]`、`[UPDATE]` の後に `[CHANGE]`、`[FIX]` の後に `[CHANGE]` があり、順序が崩れている。
- リリース済みのセクションのリリース日が、19 箇所で `**リリース日: YYYY-MM-DD` になっている。規約の書式は `**リリース日**: YYYY-MM-DD` で、閉じる `**` もない。

### 移行の `[CHANGE]` の記載が実際と異なるもの

移行の前後に同じリクエストを送って実測した結果と、記載が食い違う。

- 「メトリクスの suzu_http_response_size_bytes とアクセスログの bytes_out に、エラー応答のボディのサイズを含める」
  - panic の場合のアクセスログの bytes_out は、以前からエラー応答のボディ (JSON の 36 バイト) を含んでいた。
  - HEAD のリクエストでは、送信していないボディのサイズも含むため、「実際に送信した」という説明と食い違う。
- 「エラー応答を変更する」の対象の「/speech などがエラーになった場合」
  - 外部サービスへの接続が `SuzuError` で失敗し、そのステータスコードを返す経路は変わっていない (`Content-Type: application/json`、空のボディ、`X-Content-Type-Options` なし)。
- 「ボディを {"message":"..."} 形式の JSON から text/plain に変更する」
  - HEAD のリクエストへのエラー応答は、以前もボディと `Content-Type` がなかった。現在は `Content-Type: text/plain; charset=utf-8`、`X-Content-Type-Options: nosniff`、`Content-Length` が付く。
- 「外部サービスへの接続に成功した後、結果を 1 つも送信しないままエラーで終了した場合に … 追記しない」
  - 同じ状況で panic した場合も、以前は JSON を追記し、アクセスログの status は 500、メトリクスは記録されなかった。現在は追記せず、status は 200、error は `panic: ...`、メトリクスの code は 200 になる。この変化が書かれていない。

### 移行による変化で記載がないもの

運用に影響するもの:

- `RequestURI` が `*` で OPTIONS 以外のリクエスト (`GET *`、不完全な h2c の preface の `PRI *` など) が、404 の JSON ではなく、空のボディと `Connection: close` の 400 になる。メトリクスは `code="404",url="*"` から `code="400",url="unknown"` になり、アクセスログに error を出力しない。
- `suzu_http_requests_total` の HELP の文言が、`...partitioned by status code and HTTP method.` から `..., HTTP method and route.` に変わる。
- Exporter の内部エラーのログの出力先だけでなく書式も変わる。`echo:` の接頭辞がなくなり、標準の log の日時が付く。
- `PANIC-RECOVERED` のログに、`stack` のほかに `error`、`remote_ip`、`method`、`uri` が付く。
- Exporter の `HEAD /metrics` で `promhttp_metric_handler_requests_total` が増える。
- 絶対形式でパスが空のリクエスト (`GET http://example.com HTTP/1.1`) が、404 ではなく `Location: /` の 307 になる。
- 末尾スラッシュを取り除いたときのアクセスログの uri が、デコードしたパスではなく、エンコードしたままのパスになる (`/sp%65ech/` が `/speech` ではなく `/sp%65ech`)。`/speech/?` は `/speech` ではなく `/speech?` になる。
- エラー応答の書き込みに失敗したときに、echo のロガーが標準出力に出していたログが出なくなる。

影響の小さいもの (1 行にまとめて記載してよい):

- 巨大な `Content-Length` で `suzu_http_request_size_bytes` が負の値になっていたのが、正の値になる。
- アクセスログの latency とメトリクスの処理時間の計測の範囲に、ルーティングとエラー応答の書き込みが入る。
- Exporter の TCP keep-alive の間隔が 3 分から Go の既定 (15 秒) になる。
- Exporter を TLS で待ち受ける場合、HTTP/1.1 のクライアントと ALPN で `http/1.1` を合意するようになる。
- debug レベルの `FLUSH-FAILED` のログが出る。
- 起動時の `CONF` のログに `disable_trailing_slash_removal` と `disable_options_response` が出る。

### その他の記載の誤り

- `[ADD]` の「パスの末尾のスラッシュの除去と、登録したパスへの OPTIONS への応答を」は「への」が重なっている。「以前と同じく」としているが、GET を受け付けるパスの Allow ヘッダーの値とアクセスログの uri は以前と異なる。末尾スラッシュの除去がメインサーバーだけに適用され、Exporter には適用されないことが書かれていない。
- `[UPDATE] AWS SDK の HTTP クライアントの http.Transport を config.ini で設定可能にする` は、設定項目の追加のため `[ADD]` が妥当である。
- `[CHANGE] クライアントから取得した音声データの想定サイズが、最大サイズを超えていた場合にエラーにする` に、`audio_streaming_header = true` の場合だけという条件がない。
- `[CHANGE] ... クライアントに InternalServerError を返す` は、`500 Internal Server Error` と書く。

## 設計方針

- `## develop` のエントリを CHANGE → ADD → UPDATE → FIX の順に並べ直す。
- リリース済みのセクションのリリース日の書式を `**リリース日**: YYYY-MM-DD` に直す。
- 移行の `[CHANGE]` の記載を実測の結果に合わせて直し、記載がないものを追記する。
- 運用者向けの記載にする。

## 完了条件

- `## develop` のエントリが規約の順序で並んでいる。
- リリース日の書式が規約どおりになっている。
- 上記の移行の記載の誤りと記載漏れがない。
