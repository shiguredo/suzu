# Speech-to-Text の開始に失敗した経路でクライアントを閉じていない

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-gcp-client-leak-on-start-error
- Polished: 2026-11-23

## 目的

Google Cloud Speech-to-Text の開始処理が失敗するたびに、gRPC の接続と goroutine が残り続けないようにする。

## 現状

- `speech_to_text.go` の `SpeechToText.Start` は、`speech.NewClient` でクライアントを作った後、次の 3 つの失敗の経路で `client.Close()` を呼ばずに return する。
  - `client.StreamingRecognize` の失敗
  - 設定 (`StreamingRequest`) の `stream.Send` の失敗
  - 最初の音声の `stream.Send` の失敗 (この経路は `stream.CloseSend()` だけを呼ぶ)
- クライアントを閉じるのは、成功時に起動する goroutine の defer だけである。
- gRPC の ClientConn は、リクエストの ctx が終了しても閉じられない。そのため GCP の障害時には、失敗したリクエストの数だけ接続と goroutine が残る。
- `max_retry` を設定していても、`handler.go` の `createSpeechHandler` は `SpeechToTextHandler.Handle` が返した `*SuzuError` の `IsRetry()` が真の場合だけリトライし、`Start` のエラーはこれに該当しない。そのため、リトライのたびに増えるのではなく、クライアントを作成した後にエラーを返す呼び出しごとに 1 つ残る。
- echo を使っていた頃から同じコードで、HTTP サーバーの実装とは関係ない。

## 設計方針

- `speech.NewClient` が成功した直後から、成功時にクライアントの所有権を引き渡すまでの間に、エラーで return する経路ではクライアントを閉じる。
- クライアントの所有権を引き渡した後は、これまでどおり引き渡し先の goroutine 側で閉じる。現在の実装では送信用の goroutine、0008 の変更後は `SpeechToTextHandler.Handle` の受信用の goroutine が引き渡し先になる。

## 完了条件

- `SpeechToText.Start` がエラーを返すすべての経路で、作成したクライアントが閉じられる。
