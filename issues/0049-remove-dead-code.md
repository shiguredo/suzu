# 使われていないフィールド、引数、効果のない処理、コメントアウトしたコード、不要な依存を削除する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/remove-dead-code
- Polished: 2026-11-25

## 目的

読まれない値、到達しない分岐、効果のない処理、残したままのコメントアウトしたコードを削除し、コードを読むときに実際の挙動と取り違えないようにする。あわせて、不要な依存を減らす。`NewConfig` を経由する起動時の挙動は変えない。

## 現状

- 使われていないフィールド
  - `amazon_transcribe_v2.go` の `AmazonTranscribeV2.Region` は、`NewAmazonTranscribeV2` で代入するだけで読まれない (`NewAmazonTranscribeClientV2` は `Config.AwsRegion` を使う)。`AmazonTranscribeV2.Debug` は代入も参照もされない。この 2 つは 0042 の完了条件にも含まれる。
  - `speech_to_text.go` の `SpeechToText.SampleRate` / `ChannelCount` は、`NewSpeechToText` で代入するが、`SpeechToText.Start` は `Config.SampleRate` / `Config.ChannelCount` を使う。フィールドを削除すると、`NewSpeechToText` の `sampleRate` / `channelCount` 引数も使われなくなる。
  - `config.go` の `Config.Version` は、パッケージの変数 `Version` の写しで、`healthcheck_handler.go` の `healthcheckHandler` と `healthcheck_handler_test.go` の `TestHealthcheckHandler` が読む。`NewConfig` の `setDefaultsConfig` で `Version` を代入している。
- 使われていない引数と、効果のない処理
  - `languages.go` の `GetLanguageCode` の引数 `f` は、唯一の呼び出し元 (`handler.go` の `createSpeechHandler`) が常に nil を渡すため、`if f != nil` の分岐に到達しない。
  - `server.go` の `NewServer` の `netip.ParseAddr(c.ListenAddr)` は、`config.go` の `validateConfig` と同じ確認である。`NewServer` を直接呼ぶ場合はこの確認がなくなるが、`cmd/suzu/main.go` は `NewConfig` を経由するため、起動時の挙動は変わらない。
  - `errors.go` の `SuzuError.IsRetry` は、公開しているフィールド `Retry` を返すだけである。`handler.go` の `createSpeechHandler` は `err.IsRetry()` を呼んでいる。
  - `oggwriter_ext.go` の `(*OggWriter).Write` の `opusPacket.Payload[0:]` の `[0:]` は効果がない。
- コメントアウトしたコード
  - `speech_to_text.go` の `NewRecognitionConfig` の `SpeechContexts` の `&speechpb.SpeechContext{ Phrases: []string{} }`。`SpeechContexts: []*speechpb.SpeechContext{}` の代入は残す。
  - `handler_test.go` の `TestReceiveFirstAudioData` の `// testData := []byte(...)`
  - `test_handler_test.go` の `readDumpFile` の `//t.Log(err.Error())`
- 不要な依存
  - `service_handler.go` は `golang.org/x/exp/slices` を使っているが、`languages.go` はすでに標準の `slices` を使っている。`golang.org/x/exp` を使っているのはここだけである。

## 設計方針

- 上記をすべて削除する。
- `AmazonTranscribeV2.Region` と `Debug` は、0042 が先に完了していれば既に削除されているため、その場合は残りの項目だけを実施する。
- `SpeechToText.SampleRate` / `ChannelCount` を削除し、`NewSpeechToText` の `sampleRate` / `channelCount` 引数も削除する。`speech_to_text_handler.go` の `NewSpeechToText(h.Config, h.LanguageCode, int32(h.SampleRate), int32(h.ChannelCount))` を `NewSpeechToText(h.Config, h.LanguageCode)` に変更する。
- `Config.Version` を削除し、`setDefaultsConfig` の `config.Version = Version` を削除する。`healthcheckHandler` はパッケージの `Version` を直接参照する。`healthcheck_handler_test.go` の `TestHealthcheckHandler` は、期待値に `s.config.Version` ではなくパッケージの `Version` を使う。
- `GetLanguageCode` の引数 `f` と `if f != nil` の分岐を削除し、`handler.go` の呼び出しを `GetLanguageCode(serviceType, h.SoraAudioStreamingLanguageCode)` に変更する。
- `NewServer` の `netip.ParseAddr(c.ListenAddr)` と `net/netip` の import を削除する。テストが `NewServer` に渡す `Config` の `ListenAddr` がすべて有効な値 (`127.0.0.1`) であることは確認済みである。
- `SuzuError.IsRetry` を削除し、`handler.go` の `err.IsRetry()` を `err.Retry` に変更する。
- `(*OggWriter).Write` の `opusPacket.Payload[0:]` を `opusPacket.Payload` に変更する。
- `NewRecognitionConfig` の `SpeechContexts` のコメントアウトした `&speechpb.SpeechContext{...}` を削除し、`SpeechContexts: []*speechpb.SpeechContext{}` の代入は残す。
- `handler_test.go` の `// testData := []byte(...)` と、`test_handler_test.go` の `readDumpFile` の `//t.Log(err.Error())` を削除する。
- `golang.org/x/exp/slices` を標準の `slices` に置き換え、`go mod tidy` で `golang.org/x/exp` を直接依存から外す。
- サービスハンドラーのリトライ回数のメソッドの戻り値、`createSpeechHandler` の効果のない `WriteHeader`、Basic 認証の設定項目は、それぞれ 0042、0041、0048 で扱う。

## 完了条件

- 上記の削除が終わり、既存のテストが通る。
- `SpeechToText` に `SampleRate` / `ChannelCount` がなく、`NewSpeechToText` が `Config` と `LanguageCode` だけを受け取る。
- `Config.Version` がなく、`healthcheckHandler` と `TestHealthcheckHandler` がパッケージの `Version` を参照している。
- `SuzuError.IsRetry` がなく、`handler.go` が `SuzuError.Retry` を参照している。
- `go.mod` の直接依存に `golang.org/x/exp` がない。

## 関連 issue

- 0042 は `AmazonTranscribeV2` から `Region`、`Debug` などを削除する。0042 が先に完了した場合、本 issue の該当項目は実施済みとして扱う。
- 0044 は `GetLanguageCode`、`SuzuError`、`AmazonTranscribeV2`、`SpeechToText` などを非公開にする。0044 が先に完了した場合は、本 issue のシンボル名を新しい名前で読み替える。
- 0045 は `(*OggWriter).Write` を `io.Writer` の `Write` と紛らわしくない名前に改名する。0045 が先に完了した場合は、改名後の名前で読み替える。
- 0038 は `TestHealthcheckHandler` で `Config.Version` を設定する完了条件を持ち、`readDumpFile` を `handler_test.go` に移す。0038 が先に完了して `Config.Version` を設定するテストになっている場合も、本 issue で `Config.Version` を削除するため、パッケージの `Version` を使うように変更する。0038 が先に完了した場合は、`readDumpFile` は `handler_test.go` にある。
- 0035 は `readDumpFile` の書き込みエラーを `fmt.Println` から `t.Logf` に変える。0035 が先に完了した場合は、`//t.Log(err.Error())` が既に削除されている場合がある。
