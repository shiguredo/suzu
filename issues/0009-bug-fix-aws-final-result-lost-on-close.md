# 音声の終了時に Amazon Transcribe のストリームの受信側も閉じてしまい最終結果を受け取れない

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-aws-final-result-lost-on-close
- Polished: {YYYY-MM-DD}

## 目的

クライアントが音声の送信を終えた後に Amazon Transcribe が返す確定結果を、取りこぼさずにクライアントへ送れるようにする。

## 現状

- `amazon_transcribe_v2.go` の `AmazonTranscribeV2.Start` が起動する送信用の goroutine は、音声の読み込みが EOF になるとループを抜け、defer で `stream.Close()` を呼ぶ。
- SDK (transcribestreaming v1.44.14) の `StartStreamTranscriptionEventStream.Close` は、「writer と reader の両方を閉じ、それ以降はイベントを送ることも受け取ることもできない」と説明されている。内部の `safeClose` は、`Writer.Close()` を最大 1 秒待った後、必ず `Reader.Close()` を呼ぶ。
- そのため、音声の終了後に AWS が返す確定結果が `amazon_transcribe_v2_handler.go` の `AmazonTranscribeV2Handler.Handle` の goroutine に届く前に、`stream.Events()` が閉じられる。
- AWS に接続できる環境での実測はしていない。Sora がリクエストボディを送り終えた後もレスポンスを読み続けるかどうかで、実際の影響が変わる。これも確認できていない。
- echo を使っていた頃から同じコードで、HTTP サーバーの実装とは関係ない。

## 設計方針

- 送信用の goroutine は、音声の送信を終えたら `stream.Writer.Close()` だけを呼び、受信側は閉じない。
- `stream.Close()` は、`Handle` の goroutine が `stream.Events()` を最後まで読み終えた後、または ctx がキャンセルされた後に呼ぶ。
- エラーで終了する経路では、これまでどおり `stream.Close()` で両方を閉じる。

## 完了条件

- 音声の送信を終えた後に `stream.Events()` に届いた結果を、クライアントに送信できる。
- ストリームを閉じる処理が、送信の終了と受信の終了の両方で漏れなく行われる。
