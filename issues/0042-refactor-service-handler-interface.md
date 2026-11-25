# サービスハンドラーのインターフェースと重複した実装を整理する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/refactor-service-handler-interface
- Polished: 2026-11-25

## 目的

`aws`、`gcp`、`test`、`dump` の 4 つのサービスハンドラーの重複を減らし、責務と型を明確にする。クライアントが受け取る内容、アクセスログ、メトリクスは変えない。

## 現状

- `service_handler.go` の `serviceHandlerInterface` は、`Handle` と `IsRetryTarget` のほかに、リトライ回数を扱う `UpdateRetryCount` / `GetRetryCount` / `ResetRetryCount` を持つ。
  - 4 つのハンドラー (`AmazonTranscribeV2Handler`、`SpeechToTextHandler`、`TestHandler`、`PacketDumpHandler`) が、同じフィールド、同じコンストラクター、同じリトライ回数のメソッドを持っている。
  - リトライ回数を増やすのは `handler.go` の `createSpeechHandler` で、0 に戻すのは `AmazonTranscribeV2Handler.Handle` / `SpeechToTextHandler.Handle` である。責務が分かれている。
  - `GetRetryCount` だけがロックを取らずに読む。
  - `UpdateRetryCount` / `ResetRetryCount` の戻り値はどこでも使われていない。
- `newServiceHandlerFunc` の最後の引数 `onResultFunc` は `any` で、各コンストラクターが `onResultFunc.(func(...) error)` で型を戻している。本番では `server.go` の `NewServer` が常に nil を渡しており、テストのための差し込み口になっている。
- `service_handler.go` の細部
  - `newServiceHandlerFuncs.get` が関数へのポインターを返す。map 型にポインターのレシーバーを使っている。
  - `isRetryTargetByConfig` のコメント「message が retry_targets に含まれているかどうか」は向きが逆で、実装は `strings.Contains(message, target)` である。`len(retryTargets) > 0` の判定は不要である。
- 各ハンドラーの細部
  - `amazon_transcribe_v2.go` の `AmazonTranscribeV2.Start` の `config := at.Config` が、import している aws の `config` パッケージを隠している。
  - `AmazonTranscribeV2` は `Config` を丸ごと持ちながら、その一部のフィールドを重複して持っている。チャネル数とサンプルレートは uint16 / uint32 から int64 を経由して int32 に変換している。
  - `AmazonTranscribeV2Handler.Handle` のエラーログは `h.GetRetryCount()` を `retry_count` として出力している。`Handle` の成功時に 0 に戻るため、この値は常に 0 である。
  - `speech_to_text_handler.go` の `SpeechToTextHandler.Handle` は、`h.Config` と `stt.Config` を同じ関数の中で混在させている。`resp.Error` の分岐は、if の両方で同じ `CloseWithError` と return をしている。コメントの「音声の長さの上限値に達した場合」は、`resp.Error` 全般の説明として正しくない。
  - `test_handler.go` の `TestHandler.Handle` は、`&[]string{"ch_0"}[0]` で文字列のポインターを作り、`err != io.EOF` で比較している。

## 設計方針

- リトライ回数は、`createSpeechHandler` が返すハンドラー関数の中でリクエストごとに持つローカル変数にし、`serviceHandlerInterface` から外す。
  - `Handle` が成功したら 0 に戻す (現在の `ResetRetryCount` と同じく、外部サービスへの接続に成功したら 0 に戻す)。
  - 4 つのハンドラーから `RetryCount` / `mu` と `UpdateRetryCount` / `GetRetryCount` / `ResetRetryCount` を削除する。
  - `handler.go` の `retry_count` を出力するログは、このローカル変数の値にする。
- インターフェースは `Handle` と `IsRetryTarget` にする。
- `AmazonTranscribeV2Handler.Handle` のエラーログの `retry_count` は常に 0 のため、フィールドを削除する。このログはアクセスログではない。
- 4 つのハンドラーで共通のフィールドは 1 つの構造体にまとめ、その構造体を初期化するヘルパー関数を 1 つ用意する。各コンストラクターはヘルパーで共通の構造体を作り、各ハンドラーの型で返す。
- `onResultFunc` は `func(context.Context, io.WriteCloser, string, string, string, any) error` の型付きの引数にする。`newServiceHandlerFunc` と `getServiceHandler` の最後の引数も同じ型にし、テストで `newServiceHandlerFunc` を組み立てている箇所 (`NewConnectErrorTestHandlerFactory` など) は引数の型を合わせる。テストの検証内容は変えない。
- 上記の細部を次のように直す。
  - `AmazonTranscribeV2` から `Region`、`EnablePartialResultsStabilization`、`PartialResultsStability`、`EnableChannelIdentification`、未使用の `Debug` を削除し、`at.Config` を参照する。`MediaSampleRateHertz` と `NumberOfChannels` は `int32` で受け取り、`int64` を経由しない。
  - `AmazonTranscribeV2.Start` の `config := at.Config` を `c := at.Config` などに変え、`config` パッケージを隠さない。
  - `SpeechToTextHandler.Handle` は `h.Config` に統一する。`resp.Error` の分岐は、if の外で `w.CloseWithError(err)` と return を 1 回だけ行う。コメントを `resp.Error` 全般の説明に直す。
  - `TestHandler.Handle` は `NewTestResult("ch_0", message)` を使い、`errors.Is(err, io.EOF)` で判定する。
  - `newServiceHandlerFuncs` のメソッドは値レシーバーにし、`get` は関数の値と error を返す。`isRetryTargetByConfig` のコメントを「message が retry_targets のいずれかを含むかどうか」に直し、`len(retryTargets) > 0` の判定を削除する。
- 既存のテストが、検証内容を変えずに通ることを確認する。

## 完了条件

- `serviceHandlerInterface` が `Handle` と `IsRetryTarget` だけになり、4 つのハンドラーからリトライ回数のフィールドとメソッドがなくなる。
- リトライ回数はリクエストごとに独立しており、`Handle` が成功したら 0 に戻る。
- `AmazonTranscribeV2` から `Region`、`EnablePartialResultsStabilization`、`PartialResultsStability`、`EnableChannelIdentification`、`Debug` がなくなり、`at.Config` を参照している。`MediaSampleRateHertz` と `NumberOfChannels` は `int32` で扱われ、`int64` を経由しない。
- 上記の整理が終わり、既存のテストが通る。
- クライアントが受け取る内容、アクセスログ、メトリクスが変わらない。
