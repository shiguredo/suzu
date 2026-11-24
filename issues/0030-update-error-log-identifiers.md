# ログに channel_id と connection_id を付け、ログのキーとメッセージの誤りを直す

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/update-error-log-identifiers
- Polished: 2026-11-25

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
  - `session_id` は外部サービスのセッション ID (AWS Transcribe の `SessionID`) とし、Sora の `sora-session-id` は使わない。`session_id` が空の場合は付けない。
- `ErrServerDisconnected` と元のエラーは、別のキーで出力する。`error` には元のエラーを、`server_disconnected` には `ErrServerDisconnected` を出力する。
- `wrod` を `word` に直す。
- メッセージを持つログのメッセージを、大文字の英字とハイフンだけで構成する形式に揃える。メッセージを持たないログ (`Send()` だけのログ) には、新しいメッセージを追加しない。
- `retry interrupted` は debug ログのメッセージであると同時に `createSpeechHandler` が返すエラーの文言でもある。debug ログのメッセージだけを `RETRY-INTERRUPTED` に変え、エラーの文言は変えない。
- キーやメッセージの変更はログを集約しているパーサーに影響するため、`CHANGES.md` の `## develop` に `[CHANGE]` として、変更するキーとメッセージ、およびパーサーへの影響を記載する。担当者の行を含める。

## 完了条件

- 外部サービスとのやりとりの error レベルのログに、`channel_id` と `connection_id` がある。`session_id` が取得できる場合は `session_id` もある。
- `handler.go` の `createSpeechHandler` の `max_retry` が 1 未満のログに、`error` と `server_disconnected` が別々に出力されている。
- 1 つのログで同じキーを 2 回出力しない。
- `wrod` キーが `word` になっている。
- メッセージを持つログのメッセージが、大文字の英字とハイフンだけで構成されている。メッセージを持たないログには、新しいメッセージを追加していない。
- `retry interrupted` のエラー文言は変わっていない。
- `CHANGES.md` の `## develop` に `[CHANGE]` のエントリがあり、変更するキーとメッセージ、およびパーサーへの影響と担当者の行が書かれている。
- `make test` と CI の staticcheck が通る。
