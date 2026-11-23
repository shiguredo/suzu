# createSpeechHandler のエラーの経路のテストを追加する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/add-speech-handler-error-path-tests
- Polished: {YYYY-MM-DD}

## 目的

`handler.go` の `createSpeechHandler` の主要なエラーの経路で、クライアントが受け取るステータスコードとボディ、アクセスログ、メトリクスをテストで固定する。

## 現状

- 次の経路を通るテストがない (カバレッジで確認した)。
  - `reader.Read` が `ErrServerDisconnected` を返す経路のすべて (`max_retry` が 1 未満、再接続する、リトライの上限を超える)
  - `reader.Read` が `client disconnected` を含むエラーを返す経路
  - `serviceHandler.Handle` が `context.Canceled` を返す経路
- `CHANGES.md` の `## develop` に書いた、再接続に失敗した場合のアクセスログの status と error の挙動は、これらの経路の挙動である。
- `test_handler_test.go` の `TestSpeechHandler` の「unsupported language code」は、`test` サービスで実行している。`languages.go` の `GetLanguageCode` は `test` サービスでどの言語コードも受け付けるため、実際には正常に終了することを確認している。`aws` で対応していない言語コードを渡すと 500 を返す経路を通るテストはない。
- リトライの対象となる `SuzuError` の経路のテストがないことは、既存の issue (リトライ対象のエラーでリトライすると panic する) で扱っている。

## 設計方針

- `ErrServerDisconnected` の経路は、既存の「stream error」のテストと同じく、本物の `TestHandler` の `onResultFunc` から `errors.Join(err, ErrServerDisconnected)` を返して再現する。
- `server_test.go` と同じく、`NewServer` が組み立てた `http.Server` を HTTP/2 over TCP で待ち受けて、クライアントが受け取る内容、アクセスログ、メトリクスの code を確認する。
- 「unsupported language code」は、`aws` サービスで対応していない言語コードを渡し、500 (`httpError`) が返ることを確認するテストに直す。言語コードのエラーのステータスコードを変える issue が先に完了した場合は、その値に合わせる。
- モックやスタブは使わない。

## 完了条件

- 上記の各経路を通るテストがあり、`make test` で通る。
- 「unsupported language code」のテストが、対応していない言語コードのエラーを確認している。
