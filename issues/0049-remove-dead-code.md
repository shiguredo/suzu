# 使われていないフィールド、引数、コメントアウトしたコード、不要な依存を削除する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/remove-dead-code
- Polished: {YYYY-MM-DD}

## 目的

読まれない値、到達しない分岐、残したままのコメントアウトしたコードを削除し、コードを読むときに実際の挙動と取り違えないようにする。挙動は変えない。

## 現状

- 使われていないフィールド
  - `amazon_transcribe_v2.go` の `AmazonTranscribeV2.Region` は、`NewAmazonTranscribeV2` で代入するだけで読まれない (`NewAmazonTranscribeClientV2` は `Config.AwsRegion` を使う)。`AmazonTranscribeV2.Debug` は代入も参照もされない。
  - `speech_to_text.go` の `SpeechToText.SampleRate` / `ChannelCount` は、`NewSpeechToText` で代入するが、`SpeechToText.Start` は `Config.SampleRate` / `Config.ChannelCount` を使う。
  - `config.go` の `Config.Version` は、パッケージの変数 `Version` の写しで、`healthcheck_handler.go` の `healthcheckHandler` だけが読む。
- 使われていない引数と、効果のない処理
  - `languages.go` の `GetLanguageCode` の引数 `f` は、唯一の呼び出し元 (`handler.go` の `createSpeechHandler`) が常に nil を渡すため、`if f != nil` の分岐に到達しない。
  - `server.go` の `NewServer` の `netip.ParseAddr(c.ListenAddr)` は、`config.go` の `validateConfig` と同じ確認である。
  - `errors.go` の `SuzuError.IsRetry` は、公開しているフィールド `Retry` を返すだけである。
  - `oggwriter_ext.go` の `(*OggWriter).Write` の `opusPacket.Payload[0:]` の `[0:]` は効果がない。
- コメントアウトしたコード
  - `speech_to_text.go` の `NewRecognitionConfig` の `SpeechContexts`
  - `handler_test.go` の `TestReceiveFirstAudioData` の `// testData := []byte(...)`
  - `test_handler_test.go` の `readDumpFile` の `//t.Log(err.Error())`
- 不要な依存
  - `service_handler.go` は `golang.org/x/exp/slices` を使っているが、`languages.go` はすでに標準の `slices` を使っている。`golang.org/x/exp` を使っているのはここだけである。

## 設計方針

- 上記をすべて削除する。`Config.Version` を削除する場合、`healthcheckHandler` は `Version` を直接参照する。
- `NewServer` の `netip.ParseAddr` を削除する場合、テストが `NewServer` に渡す `Config` の `ListenAddr` に不正な値がないことを確認する。
- `golang.org/x/exp/slices` を標準の `slices` に置き換え、`go mod tidy` で `golang.org/x/exp` を直接依存から外す。
- サービスハンドラーのリトライ回数のメソッドの戻り値、`createSpeechHandler` の効果のない `WriteHeader`、Basic 認証の設定項目は、それぞれの issue で扱う。

## 完了条件

- 上記の削除が終わり、既存のテストが通る。
- `go.mod` の直接依存に `golang.org/x/exp` がない。
