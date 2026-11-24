# Amazon Transcribe への接続のエラーを設定の不備 (SuzuConfError) として広く扱いすぎている

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-aws-errors-classified-as-conf-error
- Polished: 2026-11-24

## 目的

設定の不備ではないエラーを `SuzuConfError` として扱わないようにし、元のエラーを `errors.Is` / `errors.As` で判定できるようにする。あわせて、リクエストの ctx がキャンセルまたは期限切れになった場合は、`SuzuConfError` と `SuzuError` のどちらにもせず、`context.Canceled` は `createSpeechHandler` の既存の `context.Canceled` の経路で終了し、`context.DeadlineExceeded` は 500 で終了する。

## 現状

- `amazon_transcribe_v2.go` の `AmazonTranscribeV2.Start` は、`StartStreamTranscription` のエラーが `awshttp.ResponseError` でない場合、`smithy.OperationError` であれば `NewSuzuConfError` で `SuzuConfError` にする。コメントは「smithy.OperationError の場合は、リトライしない」としている。
- AWS SDK は `StartStreamTranscription` の操作のエラーを `smithy.OperationError` で包んで返すため、`awshttp.ResponseError` 以外の操作のエラーがすべて「設定の不備」として扱われる。
- SDK のソースコードと実際のエラーを確認した範囲では、`StartStreamTranscription` は次のエラーを返す。
  - リクエストを送る前に ctx がキャンセルされた場合: `smithy.OperationError` (原因は `context.Canceled`)。現在は `SuzuConfError` になる。
  - 認証情報の取得に失敗した場合: `smithy.OperationError` (原因のエラーは、認証情報のプロバイダーによって型付きの場合とそうでない場合がある)。
  - `aws_region` が未設定の場合: `smithy.OperationError` (原因のエラーは `fmt.wrapError` で、型付きのエラーではない)。
  - リクエスト中に ctx がキャンセルされた場合: `awshttp.ResponseError` (ステータスコード 0)。現在は `SuzuError` になり、`createSpeechHandler` がステータスコード 0 を 500 に変換する。
  - リクエストを送る前、またはリクエスト中に ctx の期限が切れた場合: `smithy.OperationError` または `awshttp.ResponseError` (原因は `context.DeadlineExceeded`)。現在は `SuzuConfError` または `SuzuError` になる。
  - 外部サービスに到達できない場合: `awshttp.ResponseError` (ステータスコード 0)。現在は `SuzuError` になり、`createSpeechHandler` がステータスコード 0 を 500 に変換する。
- `errors.go` の `NewSuzuConfError` は、元のエラーの文言 (`err.Error()`) だけを保持し、`SuzuConfError` には `Unwrap` がない。
  - そのため、`SuzuConfError` になった元のエラーが `context.Canceled` または `context.DeadlineExceeded` であっても、`handler.go` の `createSpeechHandler` の `errors.Is(err, context.Canceled)` が成り立たない。
  - `SuzuError` にも `Unwrap` がないため、`awshttp.ResponseError` になったキャンセルや期限切れも同じく `errors.Is` が成り立たない。
- `SuzuConfError` は、`createSpeechHandler` で error レベルのログを出し、type: error の結果をクライアントに送ってから、エラーを返して終了する経路に入る。
- `SuzuError` は、`createSpeechHandler` で error レベルのログを出し、`Code` のステータスコードを書き込んで終了する経路に入る。`Code` が 0 の場合は 500 になる。

## 設計方針

- `errors.go` の `SuzuConfError` に `Err error` フィールドを追加し、`Unwrap() error` で `Err` を返す。`NewSuzuConfError` は `Message` に `err.Error()` を、`Err` に `err` を設定する。`Error()` はこれまで通り `Message` を返す。
- `SuzuConfError` は `speech_to_text.go` の GCP の設定不備でも使われる。`Unwrap` の追加は GCP のエラーにも有効で、GCP の分類は変更しない。
- `amazon_transcribe_v2.go` の `AmazonTranscribeV2.Start` で、`StartStreamTranscription` のエラーを分類する前に `errors.Is(err, context.Canceled)` または `errors.Is(err, context.DeadlineExceeded)` を確認し、成り立つ場合は `err` をそのまま返す。
  - この確認は `awshttp.ResponseError` と `smithy.OperationError` の判定より前に行う。
  - `smithy.OperationError` と `awshttp.ResponseError` は `Unwrap` を持つため、リクエスト前の `smithy.OperationError` とリクエスト中の `awshttp.ResponseError` のどちらでも `errors.Is` が成り立つ。
  - これにより、`context.Canceled` は `createSpeechHandler` の既存の `context.Canceled` の経路 (ステータスコード 200、type: error の結果を送らない) に入り、`context.DeadlineExceeded` は既存の経路に入らず 500 になる。
- `NewSuzuConfError` を呼ぶのは、設定の不備と判断できるエラーに限る。
  - SDK は認証情報の取得の失敗と `aws_region` の未設定を `smithy.OperationError` の原因のエラーとして返すが、原因のエラーは型付きの場合とそうでない場合があり、型だけでは設定の不備を一意に判別できない。
  - 確認した範囲では、`context.Canceled` と `context.DeadlineExceeded` を除く `smithy.OperationError` は、認証情報の取得の失敗、`aws_region` の未設定、エンドポイントの解決の失敗などの設定の不備である。
  - これ以外の `smithy.OperationError` が返る可能性は残るが、ctx の終了以外は設定の不備として扱い、その理由をコメントに書く。
- `errors.Is(err, context.Canceled)` も `errors.Is(err, context.DeadlineExceeded)` も成り立たず、原因が `*smithy.SerializationError` または `*smithy.DeserializationError` でもない `smithy.OperationError` を `SuzuConfError` にする。
- `smithy.OperationError` の原因が `*smithy.SerializationError` または `*smithy.DeserializationError` の場合は、入力や応答の不備であり、設定の不備ではないため、`err` をそのまま返す。
- `context.DeadlineExceeded` は設定の不備ではないため、`SuzuConfError` にしない。`createSpeechHandler` は `context.Canceled` だけを特別扱いするため、`context.DeadlineExceeded` は 500 になる。
- `errors.Is(err, context.Canceled)` も `errors.Is(err, context.DeadlineExceeded)` も成り立たない `awshttp.ResponseError` は、これまで通り `SuzuError` にする。ステータスコード 0 の場合は `createSpeechHandler` が 500 にする。
- コードのコメントには、`smithy.OperationError` の原因が `context.Canceled` または `context.DeadlineExceeded` の場合は設定の不備ではないこと、認証情報の取得の失敗と `aws_region` の未設定は設定の不備として扱うこと、`*smithy.SerializationError` と `*smithy.DeserializationError` は設定の不備ではないことを、日本語で書く。

## 完了条件

- `SuzuConfError` から `errors.Is` / `errors.As` で `NewSuzuConfError` に渡したエラーを判定できる。
- リクエストの ctx が `context.Canceled` で終了した場合 (リクエスト前とリクエスト中の両方) に、ステータスコード 200 で終了し、type: error の結果を送らない。
- リクエストの ctx が `context.DeadlineExceeded` で終了した場合 (リクエスト前とリクエスト中の両方) に、`SuzuConfError` と `SuzuError` のどちらにもならず、ステータスコード 500 で終了し、type: error の結果を送らない。
- 認証情報の取得の失敗と `aws_region` の未設定の場合に、`SuzuConfError` になる。
- 外部サービスに到達できない場合に、`SuzuConfError` にならない (`SuzuError` になる)。
- 上記を確認するテストがある。テストでは、少なくとも次を確認する。
  - `SuzuConfError.Unwrap` が `NewSuzuConfError` に渡したエラーを返す。
  - リクエスト前とリクエスト中の `context.Canceled` と `context.DeadlineExceeded` で、`AmazonTranscribeV2.Start` がそれぞれのエラーを含むエラーを返し、`SuzuConfError` と `SuzuError` のどちらにもならない。
  - 認証情報の取得の失敗と `aws_region` の未設定で、`AmazonTranscribeV2.Start` が `SuzuConfError` を返す。
  - 外部サービスに到達できない場合に、`AmazonTranscribeV2.Start` が `SuzuError` を返し、`SuzuConfError` にならない。
  - `createSpeechHandler` に `context.DeadlineExceeded` を返すサービスハンドラーを登録し、ステータスコード 500 で type: error の結果を送らないことを確認する。
  - `make test` で通る。
- `createSpeechHandler` の `context.Canceled` 経路のテストは `issues/0033-test-add-speech-handler-error-path-tests.md` の範囲とし、この issue では `AmazonTranscribeV2.Start` が返すエラーの分類と `context.DeadlineExceeded` の 500 を確認する。
