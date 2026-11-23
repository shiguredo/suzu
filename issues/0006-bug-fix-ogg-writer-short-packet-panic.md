# 1 バイトの音声パケットで Ogg への書き込みが panic してプロセスが終了する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-ogg-writer-short-packet-panic
- Polished: 2026-11-23

## 目的

クライアントから受信した音声パケットの内容によって Suzu のプロセス全体が終了しないようにする。現状は 1 バイトのパケット 1 つで、処理中のすべてのセッションが切断される。

## 現状

- `oggwriter_ext.go` の `(*OggWriter).Write` は、TOC バイトの下位 2 ビット (code) が 3 のとき、長さを確認せずに `payload[1]` を参照してフレーム数を取り出している。
- 呼び出し元の `handler.go` の `opus2ogg` は、`codecs.OpusPacket.Unmarshal` を通してから `Write` を呼ぶ。pion/rtp の `Unmarshal` が拒否するのは長さ 0 のパケットだけで、1 バイトのパケットは通る。
- `opus2ogg` は専用の goroutine で動き、recover していない。HTTP ハンドラーの recover (`middleware.go` の `serveWithRecover`) は別の goroutine の panic を捕まえられないため、panic するとプロセスが終了する。
- 到達条件
  - `opus2ogg` が最初の音声パケットを書き込むのは、Ogg のヘッダー 2 ページを pipe の読み手が読み終えた後。2 ページ目を読むのは、`AmazonTranscribeV2.Start` / `SpeechToText.Start` が外部サービスへの接続に成功した後に起動する送信用の goroutine のため、外部サービスへの接続に成功した後に到達する。外部サービスに到達できない状態では落ちないことを確認済み。
  - `audio_streaming_header = true` では、ヘッダーの length を 1 にして、先頭バイトの下位 2 ビットが 3 のペイロードを送ると確実に到達する。
  - `audio_streaming_header = false` では、リクエストボディの 1 回の読み込みが 1 バイトになり、そのバイトの下位 2 ビットが 3 の場合に到達する。
- `aws` と `gcp` のサービスが `opus2ogg` を使う。`test` と `dump` は使わない。
- echo を使っていた頃から同じコードで、HTTP サーバーの実装とは関係ない。

### 再現手順

1. `NewWithoutHeader` で作った `OggWriter` に、`codecs.OpusPacket{Payload: []byte{0x03}}` を渡して `Write` を呼ぶ。
2. `runtime error: index out of range [1] with length 1` で panic する。

## 設計方針

- `Write` は、長さが 0 のペイロードでは `payload[0]` を読まず、長さが 2 バイト未満かつ code が 3 のペイロードでは `payload[1]` を読まず、どちらも panic せずにエラーを返す。長さが 1 以上で code が 0 か 1 か 2 のパケットは、現状どおりエラーにしない。
- `opus2ogg` は `Write` のエラーを、既存の経路と同じく `oggWriter.CloseWithError` で読み手に伝える。外部サービスへの送信が終わり、`createSpeechHandler` の既存のエラー処理で終了する。長さ 0 のペイロードは `opus2ogg` まで届くことがある。`opus2ogg` 内の `Unmarshal` が長さ 0 を拒否するので、`Write` は呼ばれない。`opus2ogg` と `createSpeechHandler` は変更しない。
- Go 標準の fuzzing で、任意のペイロードで `Write` が panic しないことを確認する。対象は nil の `*codecs.OpusPacket` ではなく、ペイロードのバイト列であり、長さ 0 を含む。シードには `{0x03}`、`silentPacket` の値、`{0x00}`、空のバイト列を入れる。

## 完了条件

- 長さが 0 のパケットと、1 バイトで code 3 のパケットを `Write` に渡しても panic せず、エラーが返る。
- 上記の単体テストと、任意のペイロードで panic しないことを確認する fuzz テストがある。
