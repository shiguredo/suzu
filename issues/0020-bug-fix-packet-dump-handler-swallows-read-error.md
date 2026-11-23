# /dump が音声の読み込みのエラーを握り潰して正常に終了する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-packet-dump-handler-swallows-read-error
- Polished: {YYYY-MM-DD}

## 目的

`/dump` で音声の読み込みがエラーになった場合に、そのエラーがアクセスログやクライアントへの応答から分かるようにする。

## 現状

- `packet_dump_handler.go` の `PacketDumpHandler.Handle` の goroutine は、音声の読み込み (`reader.Read`) がエラーを返すと、エラーの種類にかかわらず return する。defer の `w.Close()` で pipe が EOF で閉じられる。
- `handler.go` の `createSpeechHandler` は、pipe の EOF を正常な終了として扱うため、200 で終了する。エラーはログにもアクセスログの error にも残らない。
- `audio_streaming_header = true` で `optionReadPacketWithHeader` が返す `PAYLOAD-TOO-LARGE` のエラーも、同じく握り潰される。
- 同じ状況で、`test_handler.go` の `TestHandler.Handle` はエラーを返して終了する。

## 設計方針

- `io.EOF` 以外のエラーは `w.CloseWithError(err)` で `createSpeechHandler` に伝える。
- ctx のキャンセルによる終了は、これまでどおり正常な終了として扱う。
- type: error の結果を送る処理は、type: error の二重送信の issue の方針 (`createSpeechHandler` だけが書き込む) に合わせる。

## 完了条件

- `/dump` で音声の読み込みがエラーになると、アクセスログの error にその原因が出る。
- `PAYLOAD-TOO-LARGE` のエラーで、正常な終了として扱われない。
- 上記を確認するテストがある。
