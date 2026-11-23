# テスト用のスタブ ConnectErrorTestHandler をやめる

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/remove-connect-error-stub
- Polished: {YYYY-MM-DD}

## 目的

AGENTS.md の「モックやスタブは絶対に利用しないこと」に従い、外部サービスへの接続の失敗を、本物のハンドラーと実際の失敗で確認する。

## 現状

- `test_handler_test.go` の `ConnectErrorTestHandler` は、`Handle` で固定の `SuzuError` を返すだけのテスト専用の実装で、スタブにあたる。`NewConnectErrorTestHandlerFactory` で `NewServiceHandlerFuncs` の `test` を差し替えて使っている。
- `TestSpeechHandler` の次の 2 つのテストが使っている。
  - 「connect error returns suzu error status code」 (499 を返すことを確認する)
  - 「connect error with invalid suzu error status code returns 500」
- 同じファイルの SuzuConfError のテストは、本物の `gcp` のハンドラーと存在しない認証情報ファイルで、実際の失敗の経路を作っている。
- 既存の issue (リトライ対象のエラーでリトライすると panic する) の再現手順も、決めたエラーを返すサービスハンドラーを前提にしている。

## 設計方針

- `ConnectErrorTestHandler` と `NewConnectErrorTestHandlerFactory` を削除する。
- 「3 桁以外のステータスコードは 500」の経路は、本物の `aws` のハンドラーで、Amazon Transcribe の接続先 (`AWS_ENDPOINT_URL_TRANSCRIBE_STREAMING`) を閉じたポートに向けて再現する。この場合 SDK は `awshttp.ResponseError` (ステータスコード 0) を返し、`INVALID-STATUS-CODE` を経て 500 になることを確認済み。
- 外部サービスが返す 4xx をそのまま返す経路は、実際の外部サービスに対する E2E テスト (GitHub Actions のみで実行する) で確認する。ローカルで 4xx を返す HTTP サーバーを立てる方法はスタブにあたるかの判断が必要なため、採用する前にユーザーに確認する。
- リトライの issue の再現手順とテストの方針も、この方針に合わせる。

## 完了条件

- テストのコードに、決めた結果を返すテスト専用のサービスハンドラーがない。
- 削除したテストで確認していた経路 (`SuzuError` のステータスコードを返す、3 桁以外は 500) を、本物のハンドラーを使ったテストか E2E テストで確認できる。
