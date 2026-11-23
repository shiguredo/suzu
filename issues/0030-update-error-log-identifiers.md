# ログに channel_id と connection_id を付け、ログのキーとメッセージの誤りを直す

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/update-error-log-identifiers
- Polished: {YYYY-MM-DD}

## 目的

外部サービスとのやりとりで出る error レベルのログから、どのセッションで起きたかを特定できるようにする。あわせて、ログのキーとメッセージの誤りと揺れを直す。

## 現状

- 次の error レベルのログには、`channel_id` と `connection_id` がない。
  - `amazon_transcribe_v2.go` の `AmazonTranscribeV2.Start` が起動する送信用の goroutine のログ (`session_id` だけ)
  - `speech_to_text.go` の `SpeechToText.Start` が起動する送信用の goroutine のログ (識別子が何もない)
- `handler.go` の `createSpeechHandler` の、`max_retry` が 1 未満で外部サービスから切断された場合のログは、`Err(ErrServerDisconnected).Err(err)` で `error` キーを 2 回出力する。
- `speech_to_text_handler.go` の `SpeechToTextHandler.Handle` の debug レベルのログに、`wrod` というキーの誤記がある。
- ログのメッセージの書式が揃っていない。多くは `CONNECTED` のような大文字のハイフン区切りだが、`retry`、`retry interrupted`、`Starting Amazon Transcribe stream`、`Started Amazon Transcribe stream`、`Starting Speech-to-Text streaming` などは異なる。

## 設計方針

- 外部サービスとのやりとりのログには、`channel_id`、`connection_id`、取得できる場合は `session_id` を付ける。識別子を付けたサブロガーを作って使い回す。
- `ErrServerDisconnected` と元のエラーは、別のキーで出力する。
- `wrod` を `word` に直す。
- メッセージを大文字のハイフン区切りに揃える。
- キーやメッセージの変更はログを集約しているパーサーに影響するため、`CHANGES.md` の `## develop` に記載する。

## 完了条件

- 外部サービスとのやりとりの error レベルのログに、`channel_id` と `connection_id` がある。
- 1 つのログで同じキーを 2 回出力しない。
- ログのメッセージの書式が揃っている。
