# createSpeechHandler のエラーの経路のテストを追加する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/add-speech-handler-error-path-tests
- Polished: 2026-11-25

## 目的

`handler.go` の `createSpeechHandler` の主要なエラーの経路で、クライアントが受け取るステータスコードとボディ、アクセスログ、メトリクスをテストで固定する。

## 現状

- 次の経路を通るテストがない (カバレッジで確認した)。
  - `reader.Read` が `ErrServerDisconnected` を返す経路のすべて (`max_retry` が 1 未満、再接続する、リトライの上限に達する)
  - `reader.Read` が `client disconnected` を含むエラーを返す経路
  - `serviceHandler.Handle` が `context.Canceled` を返す経路
- `CHANGES.md` の `## develop` に書いた、再接続に失敗した場合のアクセスログの status と error の挙動は、`max_retry` が 1 未満とリトライの上限に達する経路の挙動である。
- `test_handler_test.go` の `TestSpeechHandler` の「unsupported language code」は、`test` サービスで実行している。`languages.go` の `GetLanguageCode` は `test` サービスでどの言語コードも受け付けるため、実際には正常に終了することを確認している。`aws` で対応していない言語コードを渡すと 500 を返す経路を通るテストはない。
- リトライの対象となる `SuzuError` の経路のテストがないことは、`issues/0004-bug-fix-retry-nil-reader-panic.md` で扱っている。
- `TestSpeechHandler` は 0038 が先に完了すると `handler_test.go` に移動している。テスト名で探すこと。

## 設計方針

- `ErrServerDisconnected` の経路は、既存の「stream error」のテストと同じく、本物の `TestHandler` の `onResultFunc` から `errors.Join(err, ErrServerDisconnected)` を返して再現する。`err` は nil でないエラーにする。
- 再接続する経路は、`onResultFunc` が最初の呼び出しだけ `errors.Join(err, ErrServerDisconnected)` を返し、次の呼び出しでは nil を返すようにして、2 回目の `Handle` が結果をクライアントに送信することを確認する。最初のエラー応答をクライアントが受信してから次の音声を送信し、1 回目の `Handle` の読み込みが終わってから 2 回目に音声が届くようにする。
- `reader.Read` が `client disconnected` を含むエラーを返す経路は、同じく本物の `TestHandler` の `onResultFunc` から `client disconnected` を含むエラーを返して再現する。
- `serviceHandler.Handle` が `context.Canceled` を返す経路は、リクエストの context を終了した状態で本物の `aws` ハンドラーを `createSpeechHandler` に渡して再現する。
  `aws` ハンドラーの `Handle` は、最初の音声データを待つ `receiveFirstAudioData` で `context.Canceled` を返すため、外部サービスへの接続は行わない。
  クライアントの切断を待たずに決定的に再現するため、`createSpeechHandler` を `/speech` に登録した `http.NewServeMux` を `newMainHandler` に渡し、アクセスログとメトリクスを確認する。
  クライアントが受け取る内容は `httptest.ResponseRecorder` で確認する。
  テスト専用のサービスハンドラーは追加しない。
- `ErrServerDisconnected` と `client disconnected` の経路は、`server_test.go` と同じく、`NewServer` が組み立てた `http.Server` を HTTP/2 over TCP で待ち受けて、クライアントが受け取る内容、アクセスログ、メトリクスの code を確認する。
  `onResultFunc` を差し替えるテストでは、`NewServer` で作った `Server` の `mainServer.Handler` を、`s.createSpeechHandler("test", onResultFunc)` を `/speech` に登録した `http.NewServeMux` を `newMainHandler` に渡したハンドラーに置き換え、`mainServer` の `Protocols` などの設定はそのまま使う。
  `Config` は `server_test.go` の `TestServer` と同じく、`DisableSilentPacket` を有効にし、`DumpFile` を `t.TempDir()` にする。
  メトリクスはパッケージで共有するコレクターに記録されるため、前後の差分で確認する。
  アクセスログはハンドラーの終了後に出力されるため、`assert.Eventually` などで出力を待ってから確認する。
- 「unsupported language code」は、`aws` サービスで対応していない言語コードを渡し、`createSpeechHandler` が `httpError` を返すことを確認するテストに直す。
  `TestSpeechHandler` の他のサブテストは `test` サービスのままにし、このサブテストだけ `s.createSpeechHandler("aws", nil)` を呼ぶ。
  `createSpeechHandler` を直接呼んで `httpError` を確認し、`/speech` に登録した `http.NewServeMux` を `newMainHandler` に渡してアクセスログとメトリクスを確認する。
  `issues/0026-change-invalid-language-code-status.md` が先に完了している場合は、`httpError` のステータスコードは 400 になるため、その値に合わせる。
  0026 でテストが変更済みの場合も、アクセスログとメトリクスの確認は本 issue で追加する。
- モックやスタブは使わない。

## 完了条件

- 上記の各経路を通るテストがあり、`make test` で通る。
- 各経路のテストで、クライアントが受け取るステータスコードとボディ、アクセスログの status と error、メトリクスの code を確認している。
- 「unsupported language code」のテストが、対応していない言語コードのエラーを確認している。
