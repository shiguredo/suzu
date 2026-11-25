# 放置された TODO コメントを整理する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/refactor-resolve-stale-todos
- Polished: 2026-11-25

## 目的

コードとテストに残っている TODO コメントを、対応するか、issue にするか、削除するかに振り分け、コードに「いつかやる」が残らないようにする。

## 現状

次の TODO コメントが残っている (2026-11-20 時点。パッチで生成するファイルを除く)。

`amazon_transcribe_v2.go` の `context.TODO()` は TODO コメントではないため、本 issue の対象外とする。これは `0014-bug-fix-aws-http-transport-created-per-stream.md` で `context.Background()` にする。

- `handler.go`
  - `ErrServerDisconnected` の「分かりにくい場合はエラー名を変更する」
  - `createSpeechHandler` の「context.WithCancelCause(ctx) に変更する」
  - `createSpeechHandler` の「sora-audio-sample-rate などのヘッダーから取得する」
  - `createSpeechHandler` のクライアントの切断の分岐の「エラーレベルを見直す」
  - `createSpeechHandler` の再接続の分岐の「必要な場合は連続のリトライを避けるために少し待つ処理を追加する」
- `speech_to_text.go` の `SpeechToText.Start` の「適切な StatusCode に変更する」(2 箇所)
- `logger.go` の `prettyFormat`
  - 「各色を定数に置き換える」「他の logLevel が必要な場合は追加する」
  - 「Caller をファイル名と行番号だけの表示で出力する」: 直前の `FormatCaller` で `[config.go:102]` の形式になっており、実現済み。関数の先頭の出力例 (`config.go:102 > CONF`) も現在の出力と合っていない。
  - 「name=value が無い場合に | を消す方法がわからなかった」
  - 「カンマ区切りを同実現するかわからなかった」(「同実現」は誤記)
- `oggwriter_ext.go` の `(*OggWriter).Write` と `GranulePosition` の「値の決定に他の要素が必要ないか確認する」
- `TestSpeechHandler` (2026-11-20 時点では `test_handler_test.go`) の「テストデータは固定のため、すべてのメッセージを確認する」

## 設計方針

- 各 TODO コメントを、次のいずれかに振り分ける。
  - すぐに対応できるものは対応する (実現済みの TODO コメントの削除、出力例の更新、色の定数化など)。
  - 対応に設計判断や調査が必要なものは、根拠と完了条件を書いて別の issue にし、コードの TODO コメントは削除する。
  - 不要になったものは削除する。
- TODO コメントの位置やシンボル名は、0041、0044、0045 などのリファクタリングで変わる可能性がある。実装時は TODO コメントの文言で検索し、新しい名前で読み替える。
- 別の issue にする場合は、`issues/`、`issues/closed/`、`CHANGES.md` に重複がないことを確認する。
- 振り分けの結果は、この issue の解決方法に記録する。

## 完了条件

- パッチで生成するファイル以外のコードとテストに TODO コメントがない。
- `amazon_transcribe_v2.go` の `context.TODO()` は TODO コメントではないため、残っていてよい (0014 で扱う)。
- 別の issue にしたものは、根拠と完了条件を書いた issue として起票されている。

## 関連 issue

- `0014-bug-fix-aws-http-transport-created-per-stream.md` は `amazon_transcribe_v2.go` の `context.TODO()` を `context.Background()` にする。本 issue は TODO コメントだけを対象にする。
- `0038-test-cleanup-duplicate-and-misplaced-tests.md` は `TestSpeechHandler` を `handler_test.go` に移す。本 issue はテストをファイルパスではなくシンボル名で参照する。
- `0041-refactor-create-speech-handler.md` は `createSpeechHandler` の試行ごとの処理を関数に切り出す。TODO コメントの位置が変わるため、文言で検索する。
- `0044-refactor-reduce-exported-symbols.md` は `ErrServerDisconnected` と `GranulePosition` を非公開にする。`0045-refactor-oggwriter-patch-documentation.md` は `(*OggWriter).Write` を改名する。TODO コメントの対象は新しい名前で読み替える。
- `0046-refactor-add-comments-to-old-modules.md` はコメントを追加するが、TODO コメントは削除しない。
