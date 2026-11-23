# Speech-to-Text への接続に失敗した経路でクライアントを閉じていない

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-gcp-client-leak-on-start-error
- Polished: {YYYY-MM-DD}

## 目的

Google Cloud Speech-to-Text への接続が失敗するたびに、gRPC の接続と goroutine が残り続けないようにする。

## 現状

- `speech_to_text.go` の `SpeechToText.Start` は、`speech.NewClient` でクライアントを作った後、次の 3 つの失敗の経路で `client.Close()` を呼ばずに return する。
  - `client.StreamingRecognize` の失敗
  - 設定 (`StreamingRequest`) の `stream.Send` の失敗
  - 最初の音声の `stream.Send` の失敗 (この経路は `stream.CloseSend()` だけを呼ぶ)
- クライアントを閉じるのは、成功時に起動する送信用の goroutine の defer だけである。
- gRPC の ClientConn は、リクエストの ctx が終了しても閉じられない。そのため GCP の障害時には、失敗したリクエストの数だけ接続と goroutine が残る。`max_retry` を設定している場合は、リトライのたびに増える。
- echo を使っていた頃から同じコードで、HTTP サーバーの実装とは関係ない。

## 設計方針

- `speech.NewClient` が成功した直後から送信用の goroutine にクライアントを渡すまでの間に、エラーで return する経路ではクライアントを閉じる。
- goroutine に渡した後は、これまでどおり goroutine 側で閉じる。

## 完了条件

- `SpeechToText.Start` がエラーを返すすべての経路で、作成したクライアントが閉じられる。
