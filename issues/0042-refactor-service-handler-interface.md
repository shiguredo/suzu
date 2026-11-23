# サービスハンドラーのインターフェースと重複した実装を整理する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/refactor-service-handler-interface
- Polished: {YYYY-MM-DD}

## 目的

`aws`、`gcp`、`test`、`dump` の 4 つのサービスハンドラーの重複を減らし、責務と型を明確にする。外部から見える挙動は変えない。

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
  - `speech_to_text_handler.go` の `SpeechToTextHandler.Handle` は、`h.Config` と `stt.Config` を同じ関数の中で混在させている。`resp.Error` の分岐は、if の両方で同じ `CloseWithError` と return をしている。コメントの「音声の長さの上限値に達した場合」は、`resp.Error` 全般の説明として正しくない。
  - `test_handler.go` の `TestHandler.Handle` は、`&[]string{"ch_0"}[0]` で文字列のポインターを作り、`err != io.EOF` で比較している。

## 設計方針

- リトライ回数は `createSpeechHandler` のローカルの変数にし、インターフェースから外す。インターフェースは `Handle` と `IsRetryTarget` にする。
- 4 つのハンドラーで共通のフィールドは 1 つの構造体にまとめる。
- `onResultFunc` は型付きの引数にする。テストでの使い方 (既存のテストが結果の処理を差し替えている) は変えない。
- 上記の細部を直す。
- 既存のテストが、検証内容を変えずに通ることを確認する。

## 完了条件

- 上記の整理が終わり、既存のテストが通る。
- クライアントが受け取る内容、アクセスログ、メトリクスが変わらない。
