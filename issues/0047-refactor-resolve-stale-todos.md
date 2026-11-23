# 放置された TODO を整理する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/refactor-resolve-stale-todos
- Polished: {YYYY-MM-DD}

## 目的

コードに残っている TODO を、対応するか、issue にするか、削除するかに振り分け、コードに「いつかやる」が残らないようにする。

## 現状

次の TODO が残っている (パッチで生成するファイルを除く)。

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
- `test_handler_test.go` の `TestSpeechHandler` の「テストデータは固定のため、すべてのメッセージを確認する」

## 設計方針

- 各 TODO を、次のいずれかに振り分ける。
  - すぐに対応できるものは対応する (実現済みの TODO の削除、出力例の更新、色の定数化など)。
  - 対応に設計判断や調査が必要なものは、根拠と完了条件を書いて別の issue にし、コードの TODO は削除する。
  - 不要になったものは削除する。
- 振り分けの結果は、この issue の解決方法に記録する。

## 完了条件

- パッチで生成するファイル以外のコードに TODO がない。
- 別の issue にしたものは、issue として起票されている。
