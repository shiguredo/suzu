# 言語コードのヘッダーがない場合と対応していない場合に 500 ではなく 400 を返す

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/change-invalid-language-code-status
- Polished: {YYYY-MM-DD}

## 目的

クライアントのリクエストの誤り (言語コードのヘッダーがない、対応していない言語コード) を、サーバーの内部エラーと区別できるステータスコードで返す。

## 現状

- `handler.go` の `createSpeechHandler` は、`languages.go` の `GetLanguageCode` がエラーを返すと、`newHTTPError(http.StatusInternalServerError)` で 500 を返す。
- `GetLanguageCode` がエラーを返すのは次の場合である。
  - `sora-audio-streaming-language-code` ヘッダーがない (`ErrMissingAudioStreamingLanguageCode`)
  - `aws` / `awsv2` で、Amazon Transcribe が対応していない言語コード (`ErrUnsupportedLanguageCode`)
  - 登録されていないサービス (`ErrUnsupportedService`)。これはサーバーの設定の誤りである。
- 500 はサーバーの内部エラーを表すため、アクセスログやメトリクスの code で、クライアントの誤りとサーバーの障害を区別できない。
- echo を使っていた頃から同じ挙動である。

## 設計方針

- `ErrMissingAudioStreamingLanguageCode` と `ErrUnsupportedLanguageCode` の場合は 400 を返す。
- `ErrUnsupportedService` は、これまでどおり 500 を返す (起動時の検証の issue で起動時に検出する)。
- 400 を返した場合の Sora の挙動 (再試行の有無など) が 500 の場合と変わらないかを、変更の前に確認する。
- `CHANGES.md` の `## develop` に、後方互換のない変更として記載する。

## 完了条件

- 言語コードのヘッダーがない場合と、対応していない言語コードの場合に 400 を返す。
- 上記を確認するテストがある。
