# 言語コードのヘッダーがない場合と対応していない言語コードの場合に 500 ではなく 400 を返す

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/change-invalid-language-code-status
- Polished: 2026-11-25

## 目的

クライアントのリクエストの誤り (言語コードのヘッダーがない、対応していない言語コード) を、サーバーの内部エラーと区別できるステータスコードで返す。

## 現状

- `handler.go` の `createSpeechHandler` は、`languages.go` の `GetLanguageCode` がエラーを返すと、`newHTTPError(http.StatusInternalServerError)` で 500 を返す。
- `createSpeechHandler` が呼ぶときの `GetLanguageCode` は `f` が nil のため、エラーを返すのは次の場合である。
  - `sora-audio-streaming-language-code` ヘッダーがない (`ErrMissingAudioStreamingLanguageCode`)
  - `aws` / `awsv2` で、Amazon Transcribe が対応していない言語コード (`ErrUnsupportedLanguageCode`)。このエラーは `fmt.Errorf("%w: %s", ...)` で包んで返される。
  - 登録されていないサービス (`ErrUnsupportedService`)。これはサーバーの設定の誤りである。このエラーも `fmt.Errorf("%w: %s", ...)` で包んで返される。
- 500 はサーバーの内部エラーを表すため、アクセスログの status やメトリクスの code で、クライアントの誤りとサーバーの障害を区別できない。
- echo を使っていた頃から同じ挙動である。
- 現状の `test_handler_test.go` の「missing sora-audio-streaming-language-code header」は 500 を期待しており、「unsupported language code」は `test` サービスを使っているため、対応していない言語コードでもエラーにならず 200 で終了する。

## 設計方針

- `GetLanguageCode` が返すエラーを `errors.Is` で判定し、`ErrMissingAudioStreamingLanguageCode` と `ErrUnsupportedLanguageCode` の場合は 400 を返す。`ErrUnsupportedLanguageCode` と `ErrUnsupportedService` は包まれて返るため、直接比較では判定できない。
- `ErrUnsupportedService` は、これまでどおり 500 を返す (起動時の検証の issue `issues/0025-change-validate-config-at-startup.md` で起動時に検出する)。ただし、`GetLanguageCode` はヘッダーがない場合を先に判定するため、登録されていないサービスでもヘッダーがない場合は `ErrMissingAudioStreamingLanguageCode` の 400 になる。
- Sora は音声ストリーミングゲートウェイへの接続ができなかった場合と、接続時に 5xx 系のステータスコードが返された場合にリトライし、4xx 系ではリトライしない (Sora のドキュメントの「リトライ条件」: https://sora-doc.shiguredo.jp/AUDIO_STREAMING)。そのため、500 から 400 に変えると Sora はリトライしなくなる。言語コードのヘッダーがない場合と対応していない言語コードの場合は、リトライしても結果が変わらないため、リトライしない挙動にする。
- `test_handler_test.go` の「missing sora-audio-streaming-language-code header」は 400 を期待するように変更し、「unsupported language code」は `aws` サービスで実行して 400 を確認するテストに変更する。
- `CHANGES.md` の `## develop` に、次の内容を後方互換のない変更として記載する。
  - 言語コードのヘッダーがない場合と、対応していない言語コードの場合に 400 を返す。
  - Sora は 4xx ではリトライしないため、これらの場合はリトライされなくなる。

## 完了条件

- 言語コードのヘッダーがない場合と、対応していない言語コードの場合に 400 を返す。
- `ErrUnsupportedService` の場合は 500 のままである。
- `createSpeechHandler` を直接呼ぶテストで、少なくとも次を確認する。
  - 言語コードのヘッダーがない場合に 400 を返す。
  - `aws` サービスで対応していない言語コードの場合に 400 を返す。
  - ヘッダーに `ja-JP` を設定し、登録されていないサービスを指定した場合に、`ErrUnsupportedService` で 500 を返す。
- `CHANGES.md` の `## develop` に、後方互換のない変更として記載されている。
