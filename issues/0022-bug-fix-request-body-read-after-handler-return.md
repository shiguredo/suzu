# HTTP ハンドラーの終了後もリクエストボディを読み続ける

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-request-body-read-after-handler-return
- Polished: {YYYY-MM-DD}

## 目的

net/http の Handler の契約を守り、`ServeHTTP` の終了後にリクエストボディを読まないようにする。

## 現状

- net/http の `Handler` の説明は、「ServeHTTP の終了後、または終了と並行して ResponseWriter を使うこと、Request.Body を読むことは正しくない」としている。
- `handler.go` の `readPacket` は、リクエストボディを読む goroutine を起動し、`closeOnDone` で ctx の終了時にボディを閉じる。
- `createSpeechHandler` が return すると、defer の `cancel()` で ctx が終了するが、goroutine の `Read` の終了や `closeOnDone` の `Close` の完了を待たない。そのため、`ServeHTTP` の終了後もボディの読み込みが続く場合がある。
- `go test -race` ではデータ競合は検出されていない。
- echo を使っていた頃から同じコードで、HTTP サーバーの実装とは関係ない。

## 設計方針

- `createSpeechHandler` は、return する前に、ボディを読む goroutine が終了したことを待つ。
- 待つ前にボディを閉じて、ブロックしている `Read` を終わらせる。

## 完了条件

- `createSpeechHandler` が return した時点で、ボディを読む goroutine が終了している。
- 上記を確認するテストがある。
