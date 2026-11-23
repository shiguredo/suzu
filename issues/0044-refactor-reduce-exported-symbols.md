# パッケージの外に公開する必要のないシンボルを非公開にする

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/refactor-reduce-exported-symbols
- Polished: {YYYY-MM-DD}

## 目的

`suzu` パッケージの公開範囲を、`cmd/suzu/main.go` が使うものに絞り、変更してよい範囲を明確にする。

## 現状

- `cmd/suzu/main.go` が `suzu` パッケージから使うのは、次だけである。
  - `Version`、`NewConfig`、`Config`、`InitLogger`、`NewLogger`、`ShowConfig`、`NewServer`、`Server.Start`、`Server.StartExporter`、`NewServiceHandlerFuncs.GetNames`
- それ以外に、約 40 個のシンボルが公開されている。たとえば次のもの。
  - `FrameSize`、`ErrServerDisconnected`、`TranscriptionResult`、`NewSuzuErrorResponse` (`handler.go`)
  - `SuzuError`、`SuzuConfError`、`NewSuzuConfError` (`errors.go`)
  - `GetLanguageCode` と `Err` で始まるエラーの値 (`languages.go`)
  - `HeaderLength`、`MaxPayloadLength`、`ErrPayloadTooLarge` (`options.go`)
  - 各サービスハンドラーの型、コンストラクター、結果の型 (`AmazonTranscribeV2Handler`、`SpeechToTextHandler`、`TestHandler`、`PacketDumpHandler` など)
  - `GranulePosition` (`oggwriter_ext.go`)
- `NewAmazonTranscribeV2Handler` などの公開しているコンストラクターが、非公開の `serviceHandlerInterface` を返している。
- `NewServiceHandlerFuncs` は、公開されていて変更できるグローバルな map で、テストが上書きしている。
- `GranulePosition` は、1 フレームのサンプル数を返しており、名前と中身が合っていない。
- パッチで生成する `oggwriter.go` と `util.go` の公開シンボルは、上流のコードのため対象外とする。

## 設計方針

- `cmd/suzu/main.go` が使うもの以外を非公開にする。
- サービス名の一覧は、`NewServiceHandlerFuncs` を公開せずに取得できる関数にする。
- `GranulePosition` は、中身に合った非公開の名前にする。
- 公開範囲の変更は、`suzu` パッケージを import している外部のコードに影響するため、`CHANGES.md` の `## develop` の `### misc` に記載する。

## 完了条件

- `suzu` パッケージの公開シンボルが、`cmd/suzu/main.go` が使うものだけになる (生成物を除く)。
- 既存のテストが通る。
