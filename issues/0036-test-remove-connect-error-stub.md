# テスト用のスタブ ConnectErrorTestHandler をやめる

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/remove-connect-error-stub
- Polished: 2026-11-25

## 目的

AGENTS.md の「モックやスタブは絶対に利用しないこと」に従い、外部サービスへの接続の失敗を、本物のハンドラーと実際の失敗で確認する。

## 現状

- `test_handler_test.go` の `ConnectErrorTestHandler` は、`Handle` で固定の `SuzuError` を返すだけのテスト専用の実装で、スタブにあたる。`NewConnectErrorTestHandlerFactory` で `NewServiceHandlerFuncs` の `test` を差し替えて使っている。
- `TestSpeechHandler` の次の 2 つのテストが使っている。
  - 「connect error returns suzu error status code」 (499 を返すことを確認する)
  - 「connect error with invalid suzu error status code returns 500」
- 同じファイルの SuzuConfError のテストは、本物の `gcp` のハンドラーと存在しない認証情報ファイルで、実際の失敗の経路を作っている。
- `0038-test-cleanup-duplicate-and-misplaced-tests.md` が先に完了すると、`TestSpeechHandler` は `handler_test.go` に移動している。`ConnectErrorTestHandler` と `NewConnectErrorTestHandlerFactory` は、ファイルパスではなくシンボル名で探すこと。
- 既存の issue (リトライ対象のエラーでリトライすると panic する) は、テスト専用のサービスハンドラーを追加しない方針に更新済みである。再現手順は実際の Amazon Transcribe が 429 を返す状況を使い、テスト専用のハンドラーを前提にしていない。ただし、その本文は `ConnectErrorTestHandler` を参照しているため、削除に合わせて記述を更新する必要がある。

## 設計方針

- `ConnectErrorTestHandler` と `NewConnectErrorTestHandlerFactory` を削除する。
- 「3 桁以外のステータスコードは 500」の経路は、本物の `aws` のハンドラーで、Amazon Transcribe の接続先 (`AWS_ENDPOINT_URL_TRANSCRIBE_STREAMING`) を閉じたポートに向けて再現する。この場合 SDK は `awshttp.ResponseError` (ステータスコード 0) を返し、`INVALID-STATUS-CODE` を経て 500 になることを確認済み。
  - テストでは `Config` の `AwsRegion` を設定し、`AWS_ACCESS_KEY_ID` と `AWS_SECRET_ACCESS_KEY` にダミー値を設定する。これらがないと、SDK は HTTP リクエストを送る前にリージョンまたは認証情報の解決で失敗し、`SuzuConfError` の経路になる。
  - `0037-test-fix-nondeterministic-tests.md` と同じく、`HOME`、`AWS_CONFIG_FILE`、`AWS_SHARED_CREDENTIALS_FILE`、`AWS_PROFILE` を `t.TempDir()` などに向けて、開発者の AWS の設定ファイルを読まないようにする。
- 外部サービスが返す 4xx をそのまま返す経路は、実際の外部サービスに対する E2E テスト (GitHub Actions のみで実行する) で確認する。ローカルで 4xx を返す HTTP サーバーを立てる方法はスタブにあたるかの判断が必要なため、採用する前にユーザーに確認する。
  - E2E テストでは、`Config` の `AwsRegion` と `SampleRate`、`ChannelCount` を有効な値にし、ダミーの認証情報で実際の Amazon Transcribe に接続する。ダミーの認証情報では AWS が 403 を返すため、403 がそのままクライアントに返り、ボディが空であることを確認する。GitHub Actions 以外ではスキップする。
  - E2E テストでは、`AWS_ENDPOINT_URL` と `AWS_ENDPOINT_URL_TRANSCRIBE_STREAMING` を設定しない (または実エンドポイントを設定する) ようにし、開発者の環境の接続先設定に依存しないようにする。
- `issues/0004-bug-fix-retry-nil-reader-panic.md` のテスト方針は、既にテスト専用のサービスハンドラーを追加しない方針になっている。`ConnectErrorTestHandler` を削除した後に、0004 の本文にある `ConnectErrorTestHandler` への参照を更新する。

## 完了条件

- `ConnectErrorTestHandler` と `NewConnectErrorTestHandlerFactory` が削除され、テストのコードに決めた結果を返すテスト専用のサービスハンドラーがない。
- 削除したテストで確認していた経路 (`SuzuError` のステータスコードを返す、3 桁以外は 500) を、本物のハンドラーを使ったテストか E2E テストで確認できる。
- `make test` が通り、E2E テストは GitHub Actions でのみ実行され、ローカルではスキップされる。
- `issues/0004-bug-fix-retry-nil-reader-panic.md` の本文から `ConnectErrorTestHandler` への参照がなくなっている。
