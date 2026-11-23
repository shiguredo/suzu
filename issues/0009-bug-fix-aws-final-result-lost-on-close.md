# 音声の終了時に Amazon Transcribe のストリームの受信側も閉じてしまい最終結果を受け取れない

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-aws-final-result-lost-on-close
- Polished: 2026-11-23

## 目的

クライアントが音声の送信を終えた後に Amazon Transcribe が返す確定結果を、取りこぼさずにクライアントへ送れるようにする。

## 現状

- `amazon_transcribe_v2.go` の `AmazonTranscribeV2.Start` が起動する送信用の goroutine は、音声の読み込みが EOF になるとループを抜け、defer で `stream.Close()` を呼ぶ。
- SDK (transcribestreaming v1.44.14) の `StartStreamTranscriptionEventStream.Close` は、「writer と reader の両方を閉じ、それ以降はイベントを送ることも受け取ることもできない」と説明されている。内部の `safeClose` は、`Writer.Close()` の完了を最大 1 秒待った後、必ず `Reader.Close()` を呼ぶ。`Writer.Close()` は end-of-stream の送信を開始してすぐに戻るため、実際には待たずに `Reader.Close()` が呼ばれる。
- `stream.Events()` は `Reader.Close()` で閉じられる。そのため、音声の終了後に AWS が返す確定結果は、`amazon_transcribe_v2_handler.go` の `AmazonTranscribeV2Handler.Handle` の goroutine が受け取る前に `stream.Events()` が閉じられ、失われる。
- AWS に接続できる環境での実測はしていない。Sora がリクエストボディを送り終えた後もレスポンスを読み続けるかどうかで、実際の影響が変わる。これも確認できていない。
- echo を使っていた頃から同じコードで、HTTP サーバーの実装とは関係ない。

## 設計方針

- 送信用の goroutine は、音声の送信を終えたら `stream.Writer.Close()` だけを呼び、受信側は閉じない。`AudioStreamWriter.Close` は end-of-stream を送って writer を閉じるため、AWS が確定結果を返してレスポンスを終了するまで `stream.Events()` を読み続けられる。
- `stream.Close()` は、`Handle` の goroutine が `stream.Events()` を最後まで読み終えた後、または ctx がキャンセルされた後に呼ぶ。`AudioStreamWriter.Close` は複数同時呼び出しを許すため、送信用の goroutine の `Writer.Close()` と `Handle` の goroutine の `stream.Close()` が同時に走ってもよい。
- エラーで終了する経路では、これまでどおり `stream.Close()` で両方を閉じる。送信用の goroutine では、読み込みエラーまたは送信エラーでループを抜けた場合に `stream.Close()` を呼び、EOF と ctx のキャンセルでは `stream.Writer.Close()` だけを呼ぶ。

## 完了条件

- 音声の送信を終えた後も、送信用の goroutine が受信側を閉じないため、AWS が返す確定結果を `AmazonTranscribeV2Handler.Handle` の goroutine が受け取り、クライアントへ送信できる。
- `stream.Close()` が、`Handle` の goroutine が `stream.Events()` を最後まで読み終えた場合と ctx がキャンセルされた場合に呼ばれ、送信側と受信側の両方が閉じられる。
- 読み込みエラーまたは送信エラーで終了する経路でも、送信側と受信側の両方が閉じられる。
