# /dump が音声の読み込みのエラーを握り潰して正常に終了する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-packet-dump-handler-swallows-read-error
- Polished: 2026-11-24

## 目的

`/dump` で音声の読み込みがエラーになった場合に、そのエラーがアクセスログやクライアントへの応答から分かるようにする。

## 現状

- `packet_dump_handler.go` の `PacketDumpHandler.Handle` の goroutine は、音声の読み込み (`reader.Read`) がエラーを返すと、エラーの種類にかかわらず return する。defer の `w.Close()` で pipe が EOF で閉じられる。
- `handler.go` の `createSpeechHandler` は、pipe の EOF を正常な終了として扱うため、200 で終了する。エラーはログにもアクセスログの error にも残らない。
- `audio_streaming_header = true` で `optionReadPacketWithHeader` が channel に送る `PAYLOAD-TOO-LARGE` のエラーも、同じく握り潰される。
- 同じ状況で、`test_handler.go` の `TestHandler.Handle` は pipe にエラーを伝えて終了するため、エラーは握り潰されない。
- ファイルを開くときのエラーと `OnResultFunc` のエラーは、既に `w.CloseWithError(err)` で `createSpeechHandler` に伝えている。

## 設計方針

- ctx が終了している場合は、読み込みのエラーにかかわらず正常な終了として扱い、`w.Close()` で pipe を EOF で閉じる。
- `io.EOF` の場合も正常な終了として扱う。
- それ以外のエラーは `w.CloseWithError(err)` で `createSpeechHandler` に伝える。
- `PacketDumpHandler` は type: error の結果を書き込まず、`createSpeechHandler` だけが書き込む (type: error の二重送信の issue (`issues/0011-bug-fix-duplicate-type-error-result.md`) の方針に合わせる)。

## 完了条件

- `io.EOF` 以外の音声の読み込みのエラー (ctx が終了している場合を除く) で、アクセスログの error にそのエラーが出る。
- 同じエラーで、クライアントへの応答に type: error の結果が含まれる。
- ctx が終了していない場合の `PAYLOAD-TOO-LARGE` のエラーで、正常な終了として扱われない。
- ctx が終了した場合は、アクセスログの error が出ず、クライアントに type: error の結果を送らない。
- 上記を確認するテストがある。
