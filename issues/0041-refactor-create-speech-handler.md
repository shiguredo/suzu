# createSpeechHandler の重複と壊れやすいエラーの判定を整理する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/refactor-create-speech-handler
- Polished: {YYYY-MM-DD}

## 目的

`handler.go` の `createSpeechHandler` の重複を減らし、エラーの型の変化で panic したり判定を誤ったりしない形にする。外部から見える挙動は変えない。

## 現状

- 「type: error の結果を `json.Marshal` して書き込み、`flushResponse` する」処理が、SuzuConfError、`max_retry` が 1 未満の切断、リトライの上限の超過、それ以外の切断の 4 箇所にほぼ同じ形である。
  - `TranscriptionResult` は string のフィールドだけのため `json.Marshal` は失敗せず、各箇所の `json.Marshal` の失敗の分岐には到達しない。
- `channel_id` と `connection_id` を付けるログが 20 箇所以上あり、毎回 `Str` を書いている。
- リトライの待ちに `goto retry` を使い、select の中から外側のループに `continue` している。
- 外側のループの中で `defer cancelServiceHandler()` と `defer reader.Close()` を呼んでおり、再接続のたびに defer が積み上がる。
- エラーの判定が壊れやすい。
  - `SuzuError` の判定は `err.(*SuzuError)` の型アサーションで、`SuzuConfError` の判定は `errors.As` で、書き方がそろっていない。
  - `ErrServerDisconnected` の分岐は、`err.(interface{ Unwrap() []error })` を ok を確認せずに型アサーションし、`errs[0]` を元のエラーとみなしている。現在の生成元はすべて `errors.Join` のため安全だが、`fmt.Errorf("%w", ...)` で包む実装が入ると panic する。
  - クライアントの切断は、`strings.Contains(err.Error(), "client disconnected")` の文字列の一致で判定している。コメントの `http.http2errClientDisconnected` は Go 1.27 には存在しない名前である。
  - `fmt.Errorf("%s", "retry interrupted")` は、判定できるエラーの値になっていない。
- `w.WriteHeader(http.StatusOK)` を呼んでから `return nil` する箇所が 4 つある。ヘッダーを送信済みならラッパーが無視し、未送信なら net/http の既定と同じ 200 になるため、効果がない。echo の `c.NoContent(http.StatusOK)` を置き換えた名残である。

## 設計方針

- type: error を書き込む処理と、識別子を付けたサブロガーを、関数の中で 1 回だけ用意して使い回す。
- リトライの待ちを `goto` を使わない形にし、試行ごとの後始末を試行の単位で行う。
- エラーの判定は `errors.As` / `errors.Is` にそろえる。元のエラーを取り出す処理は ok を確認する。`retry interrupted` は判定できるエラーの値にする。
- クライアントの切断の判定は、文字列の一致ではなく判定できる条件 (リクエストの ctx の終了など) で行えるかを確認し、できない場合は理由をコメントに書く。
- 効果のない `w.WriteHeader(http.StatusOK)` と、到達しない `json.Marshal` の失敗の分岐を削除する。
- type: error の二重送信の issue と変更する箇所が重なるため、その issue の後に着手する。

## 完了条件

- 上記の整理が終わり、既存のテストが通る。
- クライアントが受け取る内容、アクセスログ、メトリクスが変わらない。
