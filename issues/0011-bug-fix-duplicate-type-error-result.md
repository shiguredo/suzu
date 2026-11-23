# 同じ type: error の結果をクライアントに 2 回送信する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-duplicate-type-error-result
- Polished: {YYYY-MM-DD}

## 目的

外部サービスや音声の読み込みでエラーが起きたときに、クライアントへ送る type: error の結果を 1 回だけにする。

## 現状

- type: error の結果を書き込む処理が、サービスハンドラーと `handler.go` の `createSpeechHandler` の両方にある。
  - `test_handler.go` の `TestHandler.Handle` は、音声の読み込みが EOF 以外のエラーになると、`encoder.Encode(NewSuzuErrorResponse(err))` で type: error を pipe に書き込んでから `w.CloseWithError(err)` で閉じる。
  - `speech_to_text_handler.go` の `SpeechToTextHandler.Handle` は、`stream.Recv()` がエラーを返すと、同じく type: error を書き込んでから `CloseWithError` で閉じる。`OnResultFunc` がエラーを返した場合も同じ。
  - `amazon_transcribe_v2_handler.go` の `AmazonTranscribeV2Handler.Handle` も、`OnResultFunc` がエラーを返した場合は同じ。
- `createSpeechHandler` は、pipe から読んだ type: error をそのままクライアントに転送する。続く `reader.Read` でエラーを受け取ると、`ErrServerDisconnected` の分岐 (`max_retry` が 1 未満の場合、リトライの上限を超えた場合) とそれ以外の分岐で、`json.Marshal(NewSuzuErrorResponse(err))` で同じ type: error をもう一度書き込む。
- 実測では、ボディの読み込みがエラーになる `/test` のリクエストで、レスポンスに `{"reason":"packet read error","type":"error"}` が 2 行入った。
  - 1 行目は `json.Encoder` で書くため末尾に改行があるが、2 行目は `json.Marshal` で書くため改行がない。
- GCP では、リトライの対象となるエラーでも先に type: error を送るため、再接続に成功するセッションでもクライアントにエラーが通知される。
- 既存のテスト `test_handler_test.go` の `TestSpeechHandler` の「packet read error」は 1 行目しか確認していないため、二重の送信を検出できない。
- echo を使っていた頃から同じコードで、HTTP サーバーの実装とは関係ない。

## 設計方針

- type: error の結果をクライアントに書き込むのは `createSpeechHandler` だけにする。サービスハンドラーは `CloseWithError` でエラーを返すだけにする。
- `createSpeechHandler` が書き込む type: error も、結果と同じく末尾に改行を付ける。
- リトライの対象となるエラーでは、リトライするかどうかを判断する前に type: error を送らない。

## 完了条件

- 各エラーの経路で、クライアントが受け取る type: error が 1 行だけで、末尾に改行がある。
- リトライの対象となるエラーで再接続した場合、type: error を送らない。
- レスポンスのすべての行を確認して、type: error が 1 行だけであることを検証するテストがある。
