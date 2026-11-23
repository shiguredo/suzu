# 音声の終了時に Speech-to-Text のクライアントを閉じてしまい最終結果を受け取れない

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-gcp-final-result-lost-on-close
- Polished: {YYYY-MM-DD}

## 目的

クライアントが音声の送信を終えた後に Google Cloud Speech-to-Text が返す最終結果を、取りこぼさずにクライアントへ送れるようにする。あわせて、ストリームの正常な終了をエラーとして扱わないようにする。

## 現状

- `speech_to_text.go` の `SpeechToText.Start` が起動する送信用の goroutine は、音声の読み込みが EOF になると終了し、defer で `stream.CloseSend()`、`client.Close()`、`r.Close()` の順に実行する。
- `CloseSend` は送信側だけを閉じる半クローズだが、直後の `client.Close()` が gRPC の接続そのものを閉じる。そのため、半クローズの後に GCP が返す確定結果を、`speech_to_text_handler.go` の `SpeechToTextHandler.Handle` の goroutine が `stream.Recv()` で受け取れない。
  - ローカルの gRPC サーバーで同じ順序を再現すると、`Recv` は `rpc error: code = Canceled desc = grpc: the client connection is closing` を返し、半クローズの 100ms 後に返した最終結果を受け取れなかった。`client.Close()` を呼ばない場合は受け取れた。
- `Handle` の goroutine は `Recv` のエラーを error レベルで出力し、type: error の結果をクライアントに送る。gRPC が正常な終了として返す `io.EOF` も同じ扱いになる。
- Sora がリクエストボディを送り終えた後もレスポンスを読み続けるかどうかで、実際の影響が変わる。これは確認できていない。
- echo を使っていた頃から同じコードで、HTTP サーバーの実装とは関係ない。

## 設計方針

- 送信用の goroutine は、音声の送信を終えたら `CloseSend` だけを呼ぶ。
- `client.Close()` は、`Handle` の goroutine が `Recv` の終了を確認した後に呼ぶ。クライアントの所有権を受信側に移す。
- `Recv` が返す `io.EOF` は正常な終了として扱い、error レベルのログも type: error の結果も出さずに pipe を閉じる。
- ctx がキャンセルされた場合は、これまでどおりすぐに終了する。

## 完了条件

- 音声の送信を終えた後に GCP が返す結果を、クライアントに送信できる。
- `Recv` が `io.EOF` を返した場合に、error レベルのログと type: error の結果を出さない。
- 上記を、ローカルで起動した gRPC サーバーを接続先にしたテストで確認できる。
