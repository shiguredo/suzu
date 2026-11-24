# Amazon Transcribe のクライアントをストリームごとに作るため aws_http_* の接続数の設定が効かない

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-aws-http-transport-created-per-stream
- Polished: 2026-11-24

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

- Amazon Transcribe のクライアントは、Suzu の起動時に 1 回だけ作り、すべてのストリームとリトライで共有する。`AmazonTranscribeV2.Start` は `NewAmazonTranscribeClientV2` を呼ばない。
- クライアントの生成は `server.go` の `NewServer` で行い、`Server` が参照する `Config` に実行時にだけ設定する非公開のフィールド (例: `awsTranscribeClient *transcribestreaming.Client`) で保持する。`newServiceHandlerFunc` の引数を増やすと 4 つのハンドラーとテストに影響するため、この方法を第一候補とする。
- `-service` が `aws` または `awsv2` のときだけクライアントを生成し、`gcp` / `test` / `dump` では生成しない。生成に失敗した場合は `NewServer` がエラーを返し、`cmd/suzu/main.go` が Fatal で終了する。
- `handler.go` の `createSpeechHandler` は `*s.config` を AWS のハンドラーに渡すため、`Config` のフィールドに設定したクライアントが `AmazonTranscribeV2Handler` と `AmazonTranscribeV2` まで渡る。`AmazonTranscribeV2.Start` はそのフィールドを使う。
- `AmazonTranscribeV2.Start` でクライアントが nil の場合は、`NewServer` を経由しないテストなどのためにエラーを返す。`Config` を直接組み立てて `Handle` を呼ぶテストは、クライアントを設定するか `NewServer` 経由にする。
- `NewAmazonTranscribeClientV2` はクライアントの組み立てを 1 箇所に保つため残す。`aws_http_*` の既定値の扱いは `issues/0013-bug-fix-aws-http-client-defaults-lost.md` で扱い、この issue では変更しない。0013 の変更とは独立して実装できる。
- `config.LoadDefaultConfig` に渡す ctx は、起動時の処理のためリクエストの ctx ではなく `context.Background()` にする (`context.TODO()` でも動作は同じ)。`LoadDefaultConfig` は認証情報の取得を遅延するため、起動時に認証情報の取得は行わない。
- keep-alive が有効な場合、共有した Transport のアイドル接続は次のストリームで再利用する。ストリーム終了時に `CloseIdleConnections` を呼ばない。`aws_http_idle_conn_timeout_sec` (0013 適用後は未設定時の AWS SDK の既定値) で期限切れになる。Suzu のプロセス終了時は OS が閉じるため、明示的に `CloseIdleConnections` を呼ぶ必要はない。
- `transcribestreaming.Client` と `http.Transport` は複数の goroutine から同時に使えるため、複数のストリームが同時に走っても追加の排他制御は不要である。
- `NewServer` を `aws` で呼ぶテストは、`t.Setenv` で `HOME` と `AWS_*` を固定し、開発者の `~/.aws` と環境変数に依存しないようにする。

## 完了条件

- `-service` が `aws` または `awsv2` のとき、`NewServer` が `NewAmazonTranscribeClientV2` を 1 回だけ呼び、`Server` がそのクライアントを保持する。`AmazonTranscribeV2.Start` と `AmazonTranscribeV2Handler.Handle` のリトライで、同じクライアントと Transport が使われる。
- `-service` が `gcp` / `test` / `dump` のとき、AWS のクライアントを作らない。
- `aws_http_max_conns_per_host`、`aws_http_max_idle_conns`、`aws_http_max_idle_conns_per_host` が、共有した 1 つの Transport の設定としてプロセス全体に効く。複数のストリームが同時に走っても、上限が Transport ごとに分かれない。
- keep-alive が有効な場合、ストリーム終了後に `CloseIdleConnections` を呼ばず、アイドル接続が次のストリームで再利用される。
- クライアントの生成に失敗した場合、`NewServer` がエラーを返し、起動しない。
- `NewServer` が保持するクライアントと、`/speech` の AWS ハンドラーに渡る `Config` のクライアントが同じであることを確認するテストがある。
