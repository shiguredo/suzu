# リトライ対象のエラーで外部サービスへの接続をリトライすると panic する

- Created: 2026-11-17
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-retry-nil-reader-panic
- Polished: 2026-11-23

## 目的

外部サービスへの接続がリトライ対象のエラー (Amazon Transcribe の 429) で失敗したとき、`max_retry` に従ってリトライできるようにする。現状はリトライのタイマーが発火した時点で panic し、リトライが一度も実行されない。

## 現状

- `handler.go` の `createSpeechHandler` は、`serviceHandler.Handle` がエラーを返したとき、そのエラーが `*SuzuError` で `IsRetry()` が真、かつ `s.config.MaxRetry > serviceHandler.GetRetryCount()` なら、`retry_interval_ms` だけ待ってからリトライする。
- リトライのタイマーが発火した `case <-retryTimer.C:` の分岐で `reader.Close()` を呼んでいる。この `reader` は `Handle` がエラーと一緒に返した値で、エラーのときは nil の `*io.PipeReader` になる。
  - たとえば `AmazonTranscribeV2Handler.Handle` は、`at.Start` が失敗すると `return nil, err` を返す。
- `(*io.PipeReader).Close` は内部で `r.pipe` を参照するため、nil に対して呼ぶと nil ポインタ参照で panic する。
- リトライ対象の `SuzuError` (`Retry: true`) を返すのは、`AmazonTranscribeV2.Start` が `StartStreamTranscription` で HTTP ステータスコード 429 を受け取った場合だけ。
- そのため、`max_retry` が `GetRetryCount()` より大きいときに Amazon Transcribe が 429 を返すと、リトライは行われず panic する。レスポンスヘッダーをまだ送っていなければ、panic を recover した処理が 500 を返す。前の試行でレスポンスヘッダーを送った後であれば、ステータスコードは 200 のまま終了する。
  - アクセスログの error と `PANIC-RECOVERED` のログに `runtime error: invalid memory address or nil pointer dereference` が出る。
- `max_retry` の既定値は 0 のため、既定の設定では発生しない。
- echo を使っていた頃から同じコードで、HTTP サーバーの実装とは関係なく発生する。
- テストで使っている `ConnectErrorTestHandler` は `Retry` を立てないため、この経路を通るテストはない。

### 再現手順

1. `max_retry = 1` を設定し、`-service aws` で起動する。
2. Amazon Transcribe が `StartStreamTranscription` に 429 を返す状況で、音声ストリーミングを開始する。
3. `retry_interval_ms` の経過後に panic する。

## 設計方針

- リトライのタイマーが発火した分岐の `reader.Close()` を削除する。`Handle` がエラーを返した時点で `reader` は使われておらず、閉じる必要がない。
- 外部サービスとの接続に成功した後に切断されて再接続する分岐 (`ErrServerDisconnected` の分岐) の `reader.Close()` は、接続に成功した `reader` を閉じているため変えない。
- この修正の確認に、決めたエラーを返すテスト専用のサービスハンドラーは追加しない。`Handle` が固定の `SuzuError` を返すだけの実装はスタブであり、`AGENTS.md` の「モックやスタブは絶対に利用しないこと」に反する。既存の `ConnectErrorTestHandler` の `Retry` を立てて、この経路のテストに流用もしない。
- リトライ後に `Handle` が成功したときの結果の送信は、接続成功後の既存の経路であり、この修正では変えない。429 を返すテスト用のサーバーも、この issue では追加しない。

## 完了条件

- `max_retry` が `GetRetryCount()` より大きい状態で、`Handle` がリトライ対象の `*SuzuError` を返したとき、panic せず、`retry_interval_ms` の経過後に `Handle` が再度呼ばれる。
- 再度呼ばれた `Handle` が成功した場合、既存の経路で結果をクライアントに送信できる。
- 決めたエラーを返すテスト専用のサービスハンドラーと、429 を返すテスト用のサーバーを追加していない。
