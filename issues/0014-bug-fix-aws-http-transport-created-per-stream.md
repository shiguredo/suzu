# Amazon Transcribe のクライアントをストリームごとに作るため aws_http_* の接続数の設定が効かない

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-aws-http-transport-created-per-stream
- Polished: {YYYY-MM-DD}

## 目的

`aws_http_max_idle_conns` などの接続の再利用と上限に関する設定が、プロセス全体に対して効くようにする。

## 現状

- `amazon_transcribe_v2.go` の `AmazonTranscribeV2.Start` は、ストリームを始めるたびに `NewAmazonTranscribeClientV2` を呼ぶ。`NewAmazonTranscribeClientV2` は、そのたびに新しい `http.Transport` を作り、`config.LoadDefaultConfig` で認証情報の読み込みもやり直す。リトライで `Handle` が呼ばれ直した場合も同じ。
- 1 つの Transport が 1 つのストリームしか運ばないため、次の設定は上限や再利用の設定として働かない。
  - `aws_http_max_idle_conns`
  - `aws_http_max_idle_conns_per_host`
  - `aws_http_max_conns_per_host` (Transport ごとの上限で、プロセス全体の上限にならない)
- ストリームが終わった後のアイドル接続は、使われなくなった Transport に残る。`aws_http_idle_conn_timeout_sec` が既定の 0 (無制限) の場合、AWS 側が閉じるまで残る。`CloseIdleConnections` も呼んでいない。
- `NewAmazonTranscribeClientV2` の `config.LoadDefaultConfig` には、リクエストの ctx ではなく `context.TODO()` を渡している。
- net/http の `Transport` の説明は「Transports should be reused instead of created as needed.」としている。
- echo を使っていた頃から同じ構造で、HTTP サーバーの実装とは関係ない。

## 設計方針

- Amazon Transcribe のクライアント (または `http.Transport`) は、起動時に 1 回だけ作り、すべてのストリームで共有する。
- 認証情報の読み込みも起動時に 1 回にする。認証情報の更新は SDK の仕組みに任せる。
- `aws_http_*` の既定値の扱いは別の issue (aws_profile を設定していない場合の既定値) と整合させる。

## 完了条件

- 複数のストリームとリトライで、同じクライアントと Transport が使われる。
- `aws_http_max_conns_per_host` などの上限が、プロセス全体に対して効く。
