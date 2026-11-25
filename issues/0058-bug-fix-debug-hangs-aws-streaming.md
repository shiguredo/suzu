# debug = true で Amazon Transcribe に接続するとハンドラーが戻らない

- Created: 2026-11-25
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-debug-hangs-aws-streaming
- Polished: {YYYY-MM-DD}

## 目的

`debug = true` で Amazon Transcribe を使うと、`/speech` のハンドラーが一切戻らず、解析結果も返らない状態を解消する。現在は `debug = true` の間、Amazon Transcribe を使うセッションが実質すべて停止し、クライアントが切断してもハンドラーと goroutine が残り続ける。

## 現状

### 症状

`debug = true` で `/speech` に音声を送ると、次のログまでしか出ず `Started Amazon Transcribe stream` に進まない。

```
{"level":"info","message":"Starting Amazon Transcribe stream"}
SDK 2026/11/25 15:28:05 DEBUG Request Signature: ...
```

その後、クライアントが切断してもハンドラーは戻らず、アクセスログの `POST` も出力されない。`debug = false` では発生しない。

### 原因

`NewAmazonTranscribeClientV2` は `debug` が有効なとき、`ClientLogMode` に `aws.LogRequestWithBody` と `aws.LogResponseWithBody` を立てている。この 2 つは AWS SDK の `RequestResponseLogger` ミドルウェアから `httputil.DumpRequestOut` / `httputil.DumpResponse` に `body = true` で渡され、ボディを `drainBody` で EOF までメモリに読み切る動きになる。

ストリーミング操作のリクエストボディは、`smithy-go` の `InitializeStreamWriter` が作る `io.Pipe` で、`opus2ogg` の goroutine が音声の受信のたびに書き込む。このパイプは音声が終わるまで EOF に到達しない。

さらに `opus2ogg` は、最初の音声パケットを受信した時点で `(*OggWriter).WriteHeaders` を呼び、Ogg のヘッダー 2 ページをパイプへ書き込む。`StartStreamTranscription` の内部では `DumpRequestOut` がリクエスト送信前の Deserialize フェーズでボディを読み始めるため、先に書き込まれた Ogg ヘッダーがダンプに吸い込まれ、その後は音声が流れないので EOF が来ず、`Start` が永久に返らない。

`handler.go` の `readPacket` は `ServiceHandler.Handle` の戻り値を待つ構造のため、`Start` が返らない間はクライアントの切断も検出できず、`createSpeechHandler` の `defer cancel()` にも到達しない。その結果、ハンドラーと `opus2ogg` などの goroutine がリークする。

goroutine ダンプでは、`(*AmazonTranscribeV2).Start` → `StartStreamTranscription` → `smithyhttp.RequestResponseLogger.HandleDeserialize` → `httputil.DumpRequestOut` → `drainBody` → `bytes.Buffer.ReadFrom` → `io.PipeReader.Read` の経路で停止し、`opus2ogg` 側は `(*OggWriter).writeHeaders` → `(*OggWriter).writeToStream` → `io.PipeWriter.Write` で止まっている。

### 以前動いていた理由

`ClientLogMode` の指定は `fb2cc7f` (2025-01-16) で追加された。当時は `opus2ogg` が `NewWith` の中で OggWriter を作っており、Ogg ヘッダーは `StartStreamTranscription` の呼び出しより後に書き込まれていた。`764538d` (2025-05-19) でヘッダーの書き込みが `NewWithoutHeader` と `(*OggWriter).WriteHeaders` に移り、`opus2ogg` の goroutine 内で最初の音声受信前に書かれるようになったため、ダンプと書き込みが競合する状態が生まれた。

## 設計方針

ストリーミングのボディを全量読み込むログ設定は、ストリーミング操作と両立しないため使わない。

- `ClientLogMode` から `aws.LogRequestWithBody` と `aws.LogResponseWithBody` を外し、`LogRequest` / `LogResponse` ではボディなしのダンプにする。`body = false` の場合は `DumpRequestOut` が `neverEnding` でダミーのボディを作るため、パイプは読まれない
- 署名、リクエスト・レスポンスのヘッダー、リトライ、イベントのログは残し、debug の用途を維持する
- 音声の内容は `dump_file` で確認できるため、ログにボディを出せなくなることによる損失は小さいと考える

## 完了条件

`debug = true` で Amazon Transcribe に接続して音声を送ったとき、`Started Amazon Transcribe stream` が出力され、解析結果がクライアントに届くこと。クライアントの切断後にハンドラーが戻り、goroutine がリークしないこと。

## 解決方法

- `amazon_transcribe_v2.go` の `NewAmazonTranscribeClientV2` で `clientLogMode` から `aws.LogRequestWithBody` と `aws.LogResponseWithBody` を外す
- `amazon_transcribe_v2_test.go` の `TestNewAmazonTranscribeClientV2` を、`ClientLogMode` が 0 でないことだけでなく、`aws.LogRequestWithBody` と `aws.LogResponseWithBody` が含まれないことを確認する内容に変える。実際の詰まりは AWS への実接続でしか発生しないため、単体テストで検証できるのはフラグの状態までとする
- `CHANGES.md` の `develop` に変更を追記する
- `config_example.ini` の `debug` は現在説明コメントが無いため、Amazon Transcribe のログにボディを含めない旨を補足するか、実装時に判断する
- `opus2ogg` のヘッダー書き込みのタイミング見直しは本 issue では扱わない。ログ設定に依存しない構造にする余地はあるが、別目的のため別 issue とする
