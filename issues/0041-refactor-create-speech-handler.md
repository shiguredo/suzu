# createSpeechHandler の重複と壊れやすいエラーの判定を整理する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/refactor-create-speech-handler
- Polished: 2026-11-25

## 目的

`handler.go` の `createSpeechHandler` の重複を減らし、エラーの型の変化で panic したり判定を誤ったりしない形にする。現在の生成元が返すエラーに対する外部から見える挙動 (クライアントが受け取る内容、アクセスログ、メトリクス) は変えない。`errors.As` / `errors.Is` で包まれたエラーも判定できるようにする。

## 現状

- 「type: error の結果を `json.Marshal` して書き込み、`flushResponse` する」処理が、`serviceHandler.Handle` が返した SuzuConfError、`reader.Read` が返した `max_retry` が 1 未満の切断、リトライの上限の超過、それ以外のエラーの 4 箇所にほぼ同じ形である。
  - `TranscriptionResult` は string のフィールドだけのため `json.Marshal` は失敗せず、各箇所の `json.Marshal` の失敗の分岐には到達しない。
- `channel_id` と `connection_id` を付けるログが `handler.go` の `createSpeechHandler` と `flushResponse` に 20 箇所以上あり、毎回 `Str` を書いている。
- リトライの待ちに `goto retry` を使い、select の中から外側のループに `continue` している。
- 外側のループの中で `defer cancelServiceHandler()` と `defer reader.Close()` を呼んでおり、再接続のたびに defer が積み上がる。
- エラーの判定が壊れやすい。
  - `SuzuError` の判定は `err.(*SuzuError)` の型アサーションで、`SuzuConfError` の判定は `errors.As` で、書き方がそろっていない。
  - `ErrServerDisconnected` の分岐は、`err.(interface{ Unwrap() []error })` を ok を確認せずに型アサーションし、`errs[0]` を元のエラーとみなしている。現在の生成元はすべて `errors.Join(err, ErrServerDisconnected)` で、元のエラーが 1 番目にあるため安全だが、`fmt.Errorf("%w", ...)` で包む実装が入ると型アサーションが失敗して panic する。`Unwrap()` が空のスライスを返した場合も `errs[0]` で panic する。
  - クライアントの切断は、`strings.Contains(err.Error(), "client disconnected")` の文字列の一致で判定している。Go 1.27 では `net/http/internal/http2` の `errClientDisconnected` になっており、コメントの `http.http2errClientDisconnected` は存在しない。`errClientDisconnected` も非公開のため、Suzu からは型や値では判定できない。
  - `fmt.Errorf("%s", "retry interrupted")` は、判定できるエラーの値になっていない。
- `w.WriteHeader(http.StatusOK)` を呼んでから `return nil` する箇所が 4 つある。ヘッダーを送信済みならラッパーが無視し、未送信なら net/http の既定と同じ 200 になるため、効果がない。echo の `c.NoContent(http.StatusOK)` を置き換えた名残である。

## 設計方針

- type: error を書き込む処理と、識別子を付けたサブロガーを、`createSpeechHandler` の中で 1 回だけ用意して使い回す。`flushResponse` も同じサブロガーを受け取る形にして、`handler.go` の該当するログで `Str("channel_id", ...)` と `Str("connection_id", ...)` を繰り返さない。
- リトライの待ちを `goto` を使わない形にし、select の中から外側のループへの `continue` も使わない形にする。試行ごとの処理を関数に切り出し、その関数の中で `defer` を使って後始末する。これにより、再接続のたびに defer が積み上がらないようにする。
- エラーの判定は `errors.As` / `errors.Is` にそろえる。
  - `ErrServerDisconnected` の元のエラーは、`errors.As` で `interface{ Unwrap() []error }` を取り出し、`Unwrap()` が空でないことを確認してから、最初の `ErrServerDisconnected` 以外の要素から取り出す。`fmt.Errorf("%w", ...)` で包まれていても `errors.As` は包みをたどる。`errs[0]` の順序に依存しない。要素が見つからない場合は `err` をそのまま元のエラーとして扱い、type: error のレスポンスとログの内容を変えない。
  - `retry interrupted` は `errors.New("retry interrupted")` で作った sentinel を返し、`errors.Is` で判定できるようにする。エラーの文言は `retry interrupted` のままにする。
- クライアントの切断の判定は、文字列の一致ではなく判定できる条件 (リクエストの ctx の終了など) で行えるかを確認する。Go 1.27 では `net/http/internal/http2` の `errClientDisconnected` は非公開で、Suzu からは型や値で判定できない。`r.Context()` の終了で判定する場合も、返すエラーは `reader.Read` が返した元のエラーとし、アクセスログの error を変えない。ctx の終了だけではクライアントの切断と他のキャンセルを区別できない場合は、文字列の一致を残し、その理由をコメントに書く。
- 効果のない `w.WriteHeader(http.StatusOK)` を削除する。
- type: error の書き込みは、0011 の完了後の実装に合わせる。`TranscriptionResult` は string のフィールドだけなので JSON のエンコードは失敗しない。`json.Marshal` を使う場合はその失敗の分岐を削除する。`json.Encoder` を使う場合は `Encode` の失敗は書き込みの失敗なので、その分岐は残す。いずれの場合も書き込みの失敗は扱う。
- `issues/0011-bug-fix-duplicate-type-error-result.md` の後に着手する。0011 は `createSpeechHandler` の type: error の書き込みと、サービスハンドラーからの type: error の送信を変える。0011 の完了後に、本 issue の「現状」が 0011 後の実装と一致しているか再確認する。

## 完了条件

- 上記の整理が終わり、既存のテストが通る。
- クライアントが受け取る内容、アクセスログ、メトリクスが、現在の生成元が返すエラーに対して変わらない。
- `errors.As` / `errors.Is` で包まれたエラーを判定でき、`ErrServerDisconnected` の元のエラーを取り出す処理が panic しない。
- `retry interrupted` が `errors.Is` で判定できる sentinel になっており、エラーの文言が `retry interrupted` のままである。
- `handler.go` の該当するログで `Str("channel_id", ...)` と `Str("connection_id", ...)` を繰り返していない。
