# HTTP ハンドラーの終了後もリクエストボディを読み続ける

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-request-body-read-after-handler-return
- Polished: 2026-11-24

## 目的

net/http の Handler の契約を守り、`ServeHTTP` の終了後または終了と並行してリクエストボディを読まないようにする。

## 現状

- net/http の `Handler` の説明は、「ServeHTTP の終了後、または終了と並行して ResponseWriter を使うこと、Request.Body を読むことは正しくない」としている。
- `handler.go` の `readPacket` は、リクエストボディを読む goroutine を起動し、`closeOnDone` で ctx の終了時にボディを閉じる。
- `createSpeechHandler` が返すハンドラーが return すると、defer の `cancel()` で ctx が終了するが、goroutine の `Read` の終了や `closeOnDone` の `Close` の完了を待たない。そのため、`ServeHTTP` の終了後もボディの読み込みが続く場合がある。
- この問題はデータ競合ではないため、`go test -race` では検出されない。
- echo を使っていた頃から同じコードで、HTTP サーバーの実装とは関係ない。

## 設計方針

- `createSpeechHandler` が返すハンドラーは、ctx を作成した後のすべての return 経路で、ボディを読む goroutine が終了したことを待つ。待つ処理は `newOpusChannel` の直後に `defer` で登録し、return 経路の追加漏れを防ぐ。
- その `defer` では、`cancel()`、`r.Body` の Close、読み込み goroutine の終了待ちの順に実行する。既存の `defer cancel()` は残す。`cancel()` は複数回呼んでも安全である。
  - `cancel()` を先に呼ぶのは、読み込み goroutine が `opusCh` への送信でブロックしている場合に、`ctx.Done()` で終了させるためである。ボディを閉じるだけでは、送信でブロックしている goroutine は終了しない。
  - `closeOnDone` も ctx の終了時に `r.Body` を閉じる。net/http の `r.Body` の Close は複数回呼んでも安全である。
- `readPacket` は、読み込み goroutine が終了したときに閉じる channel を返す。`newOpusChannel` はその channel を呼び出し元へ返す。
  - `newOpusChannel` が返す `opusCh` の close は、`packetReaderOptions` の goroutine が `ctx.Done()` で先に終了すると `readPacket` の goroutine より前に起きる場合があるため、読み込み goroutine の終了の判定には使えない。
- 待つのはボディを読む `readPacket` の goroutine だけとし、`packetReaderOptions` の goroutine やサービスハンドラーの goroutine は待たない。ボディを読むのは `readPacket` の goroutine だけであり、ResponseWriter を使うのはハンドラーの goroutine だけである。
- 変更対象は `handler.go` の `readPacket`、`newOpusChannel`、`createSpeechHandler` が返すハンドラーとする。`readPacket` の戻り値の変更に伴い、`newOpusChannel` を呼ぶ既存のテストも更新する。

## 完了条件

- `createSpeechHandler` が返すハンドラーが return した時点で、ボディを読む `readPacket` の goroutine が終了している。
- 上記を確認するテストがある。テストでは、音声を 1 つ受信した後に return する経路 (例: `onResultFunc` のエラー) を使う。リクエストの `Body` に、1 つ目の `Read` で音声を返し、2 つ目の `Read` が Close されるまでブロックする `io.ReadCloser` を直接設定する。2 つ目の `Read` が開始したことを確認してから `onResultFunc` でエラーを返す。ボディの Close は複数回呼んでも安全にする。ハンドラーが return した時点で、2 つ目の `Read` が終了していることを確認する。
- `make test` と CI で実行している staticcheck が通る。
