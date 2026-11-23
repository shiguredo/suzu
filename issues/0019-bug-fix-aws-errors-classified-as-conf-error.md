# Amazon Transcribe への接続のエラーを設定の不備 (SuzuConfError) として広く扱いすぎている

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-aws-errors-classified-as-conf-error
- Polished: {YYYY-MM-DD}

## 目的

設定の不備ではないエラーを `SuzuConfError` として扱わないようにし、元のエラーを `errors.Is` / `errors.As` で判定できるようにする。

## 現状

- `amazon_transcribe_v2.go` の `AmazonTranscribeV2.Start` は、`StartStreamTranscription` のエラーが `awshttp.ResponseError` でない場合、`smithy.OperationError` であれば `NewSuzuConfError` で `SuzuConfError` にする。コメントは「smithy.OperationError の場合は、リトライしない」としている。
- AWS SDK は操作のエラーを `smithy.OperationError` で包んで返すため、`ResponseError` 以外の操作のエラーがすべて「設定の不備」として扱われる。
- `errors.go` の `NewSuzuConfError` は、元のエラーの文言 (`err.Error()`) だけを保持し、`SuzuConfError` には `Unwrap` がない。そのため、元のエラーが `context.Canceled` であっても、`handler.go` の `createSpeechHandler` の `errors.Is(err, context.Canceled)` が成り立たない。
- `SuzuConfError` は、`createSpeechHandler` で error レベルのログを出し、type: error の結果をクライアントに送ってから終了する経路に入る。
- どのような状況で `ResponseError` 以外の `OperationError` が返るか (認証情報の取得の失敗、リクエストの ctx のキャンセルなど) は、すべては確認できていない。外部サービスに到達できない場合は `ResponseError` (ステータスコード 0) になることを確認している。

## 設計方針

- `SuzuConfError` は元のエラーを保持し、`Unwrap` で返せるようにする。
- `SuzuConfError` にするのは、認証情報の取得の失敗など、設定の不備と判断できるエラーに限る。ctx のキャンセルは、`createSpeechHandler` の既存の `context.Canceled` の経路で扱う。
- どのエラーを設定の不備として扱うかを、SDK が返すエラーの型を確認したうえでコメントに書く。

## 完了条件

- `SuzuConfError` から `errors.Is` / `errors.As` で元のエラーを判定できる。
- リクエストの ctx がキャンセルされた場合に、type: error の結果を送らずに終了する。
- 上記を確認するテストがある。
