# 外部サービスに再接続すると保存中の Ogg ファイルが切り詰められる

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-ogg-file-truncated-on-reconnect
- Polished: {YYYY-MM-DD}

## 目的

`enable_ogg_file_output` で音声を保存しているセッションで外部サービスに再接続しても、それまでに保存した音声が失われないようにする。

## 現状

- `handler.go` の `opus2ogg` は、呼ばれるたびに `<sora-session-id>-<sora-connection-id>.ogg` を `os.Create` で開く。`os.Create` は既存のファイルを長さ 0 に切り詰める。
- `opus2ogg` は `AmazonTranscribeV2Handler.Handle` / `SpeechToTextHandler.Handle` から呼ばれ、`Handle` は外部サービスに接続し直すたびに呼ばれる。
  - `createSpeechHandler` は、`ErrServerDisconnected` で切断された場合とリトライの対象となる `SuzuError` の場合に、外側のループに戻って `Handle` を再び呼ぶ。
- そのため、`enable_ogg_file_output = true`、`max_retry` が 1 以上、`-service aws` または `gcp` のときに再接続すると、それまでに保存した音声が消える。
- 前の試行の goroutine は、ctx のキャンセル後に非同期で `OggWriter.Close` を実行する。`Close` はファイルの末尾からシークして最後のページを書き直すため、切り詰めた後の同じファイルに対して書き込むおそれがある。この競合は実測していない。
- echo を使っていた頃から同じコードで、HTTP サーバーの実装とは関係ない。

## 設計方針

- Ogg ファイルは接続の試行 (`Handle`) ごとではなく、リクエストごとに 1 回だけ開き、試行をまたいで書き続ける。
- 試行ごとに `OggWriter` を作り直す場合も、ファイルを切り詰めず、前の試行の `Close` が後の試行の書き込みと競合しないようにする。

## 完了条件

- 再接続の前後の音声が、同じ Ogg ファイルに失われずに保存される。
- 上記を、再接続の経路を通るテストで確認できる。
