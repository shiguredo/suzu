# リトライ対象のエラーで外部サービスへの接続をリトライすると panic する

- Created: 2026-11-17
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-retry-nil-reader-panic
- Polished: {YYYY-MM-DD}

## 目的

外部サービスへの接続がリトライ対象のエラー (Amazon Transcribe の 429) で失敗したとき、`max_retry` に従ってリトライできるようにする。現状はリトライのタイマーが発火した時点で panic し、リトライが一度も実行されない。

## 現状

- `handler.go` の `createSpeechHandler` は、`serviceHandler.Handle` がエラーを返したとき、そのエラーが `*SuzuError` で `IsRetry()` が真、かつ `s.config.MaxRetry > serviceHandler.GetRetryCount()` なら、`retry_interval_ms` だけ待ってからリトライする。
- リトライのタイマーが発火した `case <-retryTimer.C:` の分岐で `reader.Close()` を呼んでいる。この `reader` は `Handle` がエラーと一緒に返した値で、エラーのときは nil の `*io.PipeReader` になる。
  - たとえば `AmazonTranscribeV2Handler.Handle` は、`at.Start` が失敗すると `return nil, err` を返す。
- `(*io.PipeReader).Close` は内部で `r.pipe` を参照するため、nil に対して呼ぶと nil ポインタ参照で panic する。
- リトライ対象の `SuzuError` (`Retry: true`) を返すのは、`AmazonTranscribeV2.Start` が `StartStreamTranscription` で HTTP ステータスコード 429 を受け取った場合だけ。
- そのため `max_retry` を 1 以上に設定して Amazon Transcribe が 429 を返すと、リトライは行われず、panic を recover したミドルウェアが 500 を返す (レスポンスヘッダーを送信済みなら 200 のまま終了する)。
  - アクセスログの error と `PANIC-RECOVERED` のログに `runtime error: invalid memory address or nil pointer dereference` が出る。
- `max_retry` の既定値は 0 のため、既定の設定では発生しない。
- echo を使っていた頃から同じコードで、HTTP サーバーの実装とは関係なく発生する。
- テストで使っている `ConnectErrorTestHandler` は `Retry` を立てないため、この経路を通るテストはない。

### 再現手順

1. `max_retry = 1` を設定し、`-service aws` で起動する。
2. Amazon Transcribe が `StartStreamTranscription` に 429 を返す状況で、音声ストリーミングを開始する。テストでは、`Handle` が `nil` と `&SuzuError{Code: 429, Retry: true}` を返すサービスハンドラーを登録すると再現できる。
3. `retry_interval_ms` の経過後に panic する。

## 設計方針

- リトライのタイマーが発火した分岐の `reader.Close()` を削除する。`Handle` がエラーを返した時点で `reader` は使われておらず、閉じる必要がない。
- 外部サービスとの接続に成功した後に切断されて再接続する分岐 (`ErrServerDisconnected` の分岐) の `reader.Close()` は、接続に成功した `reader` を閉じているため変えない。
- リトライ対象のエラーを 1 回返した後に接続に成功するサービスハンドラーを使い、リトライが実行されて結果を受信できることを確認するテストを追加する。

## 完了条件

- `max_retry` が 1 以上で、`Handle` がリトライ対象の `SuzuError` を返した場合に panic せず、`retry_interval_ms` の経過後に `Handle` が再度呼ばれる。
- リトライ後に接続に成功した場合、結果をクライアントに送信できる。
- 上記を確認するテストがある。
