# 外部サービスに再接続すると保存中の Ogg ファイルが切り詰められる

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-ogg-file-truncated-on-reconnect
- Polished: 2026-11-23

## 目的

`enable_ogg_file_output` で音声を保存しているセッションで外部サービスに再接続しても、それまでに保存した音声が失われないようにする。

## 現状

- `handler.go` の `opus2ogg` は、呼ばれるたびに `<sora-session-id>-<sora-connection-id>.ogg` を `os.Create` で開く。`os.Create` は既存のファイルを長さ 0 に切り詰める。
- `opus2ogg` は `AmazonTranscribeV2Handler.Handle` / `SpeechToTextHandler.Handle` から呼ばれ、`Handle` は外部サービスに接続し直すたびに呼ばれる。
  - `createSpeechHandler` は、`ErrServerDisconnected` で切断された場合とリトライの対象となる `SuzuError` の場合に、外側のループに戻って `Handle` を再び呼ぶ。
  - `SuzuError` の経路は、`Handle` がエラーとともに nil の `reader` を返すため、現状は `reader.Close()` で panic する (`0004-bug-fix-retry-nil-reader-panic.md` で修正予定)。
- そのため、`enable_ogg_file_output = true`、`max_retry` が 1 以上、`-service aws`、`awsv2`、または `gcp` のときに再接続すると、それまでに保存した音声が消える。`awsv2` は `aws` と同じハンドラーに登録されている。
- 前の試行の goroutine は、ctx のキャンセル後に非同期で `OggWriter.Close` を実行する。`Close` はファイルの末尾からシークして最後のページを書き直し、最後に `fd.Close()` でファイルを閉じる。そのため、切り詰めた後の同じファイルに対して書き込むおそれがあり、ファイルを試行間で共有すると前の試行の `Close` がファイルを閉じてしまう。この競合は実測していない。
- echo を使っていた頃から同じコードで、HTTP サーバーの実装とは関係ない。

### 再現手順

1. `enable_ogg_file_output = true`、`ogg_dir` に任意のディレクトリ、`max_retry` を 1 以上に設定し、`-service aws`、`awsv2`、または `gcp` で起動する。
2. 音声を送信し、外部サービスに接続して Ogg ファイルに音声が書き込まれることを確認する。
3. 外部サービスから切断されて再接続する (`ErrServerDisconnected` が発生する) と、`opus2ogg` が同じファイルを `os.Create` で開き直し、それまでの音声が消える。

## 設計方針

- Ogg ファイルは接続の試行 (`Handle`) ごとではなく、リクエストごとに 1 回だけ開き、試行をまたいで書き続ける。ファイルを開くのは `createSpeechHandler` とし、`Handle` / `opus2ogg` に渡す。
- ファイルを開くのは、`EnableOggFileOutput` が有効で、`opus2ogg` を使う `aws` / `awsv2` / `gcp` のときだけとする。`test` / `dump` では Ogg ファイルを作成しない。
- リクエストの最初のオープンでは、既存の動作と同じくファイルを切り詰めてよい。試行をまたいで開き直さないため、再接続で切り詰められることはない。
- ファイルの Close はリクエストの終了時に 1 回だけ行う。試行ごとの `OggWriter.Close` は、EOS のページを書き直したうえで、ファイルを閉じないようにする (`0021-bug-fix-ogg-file-descriptor-leak.md` の Close の扱いと整合させる)。
- `OggWriter` を変更する場合は、`patch/oggwriter.go.patch` と生成される `oggwriter.go` の両方を更新する。
- 試行ごとに `OggWriter` を作り直す場合も、前の試行の `Close` が後の試行の書き込みと競合しないようにする。試行ごとに `OggWriter` を作り直すと、ファイルには試行ごとの Ogg ストリーム (`OpusHead` / `OpusTags`) が並ぶ chained Ogg になるが、音声が失われなければよい。
- ファイルを開くときは `O_APPEND` を使わず、切り詰めるか、切り詰めずに開いて末尾から書き込む。`OggWriter.Close` がファイルの末尾からシークして最後のページを書き直すため、`O_APPEND` で開くと `Seek` の挙動が不定で、書き直しが追記になるおそれがある。
- `0005-bug-fix-ogg-file-path-traversal.md` のファイル名の検証が入っている場合は、ファイルを開く場所を移しても検証を維持する。

## 完了条件

- 再接続の前後の音声が、同じ Ogg ファイルに失われずに保存される。
- 上記を、同じリクエストの複数回の試行として `opus2ogg` を同じファイルに対して呼ぶテストで確認できる。モックやスタブは使わない。
