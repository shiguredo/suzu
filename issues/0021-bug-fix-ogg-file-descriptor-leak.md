# Ogg ファイルの出力を有効にするとファイルディスクリプタを閉じずに終了する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-ogg-file-descriptor-leak
- Polished: 2026-11-24

## 目的

`enable_ogg_file_output = true` のとき、セッションの終了時に Ogg ファイルを確実に閉じ、ファイルの末尾 (EOS のページ) を正しく書き終える。

## 現状

- `handler.go` の `opus2ogg` の goroutine は、`NewWithoutHeader` が成功した後のどの経路でも `oggWriter.CloseWithError` で pipe を閉じてから、defer の `o.Close()` を実行する。`o.Close()` のエラーは捨てている。
- `OggWriter.Close` (パッチの適用後の `oggwriter.go`) は、ファイルの最後のページを読み直して EOS のページとして `writeToStream` で書き直し、最後に `fd.Close()` を呼ぶ。
  - `writeToStream` の書き込み先は、ファイルと pipe をまとめた `io.MultiWriter` である。ファイルへの書き込みは成功するが、pipe は先に閉じているため `io.MultiWriter` 全体としてはエラーになり、`fd.Close()` に到達しない。
  - `OggWriter.Close` のシークは `lastPayloadSize+pageHeaderSize+1` で、1 セグメントのページを前提にしている。ペイロードが 255 バイト以上のページでは、シーク位置が実際のページの先頭より後ろになり、EOS のページを書く位置がずれる。読み出しの位置は、シークのずれと `+1` の仮定が打ち消し合って正しいが、書き込みの位置はずれる。
  - 音声を 1 つも受信せずに終わった場合は、ファイルは空で、空のファイルに対する `Seek` が失敗し、同じく `fd.Close()` に到達しない。
- 実測では、Ogg ファイルの出力を有効にした 10 リクエストの後、`.ogg` のファイルディスクリプタが 8 個開いたままだった。GC のファイナライザーで後から閉じられるとみられるが、閉じられるまでの間は開いたままになる。
- `NewWithoutHeader` が失敗した場合は、作成したファイルを閉じずに return する。
- echo を使っていた頃から同じコードで、HTTP サーバーの実装とは関係ない。

### 再現手順

1. `enable_ogg_file_output = true` と `ogg_dir` に任意のディレクトリを設定し、`-service aws` または `gcp` で起動する。
2. `/speech` に音声を送り、音声の送信を終了してリクエストを終了する。
3. `.ogg` のファイルディスクリプタが閉じられていることを `lsof` や `/proc/<pid>/fd` で確認する。閉じられていない場合は、GC が走るまで開いたままになる。

## 設計方針

- EOS のページの書き直しはファイルにだけ行い、閉じた pipe に書き込まない。`OggWriter.Close` は `i.stream` (ファイルと pipe をまとめた `io.MultiWriter`) ではなく `i.fd` に書く。
- `OggWriter.Close` のシークと読み出しのオフセットは、最後のページのセグメント数 (`lastPayloadSize/255+1`) を使って計算する。
- `OggWriter.Close` がエラーを返す場合も、ファイルは必ず閉じる。`fd.Close()` のエラーも返す。
- `opus2ogg` が作成したファイルは、`OggWriter` の生成に失敗した場合も含めて、すべての経路で閉じる。
- `opus2ogg` の defer で `o.Close()` のエラーを捨てずにログに出す。
- `OggWriter.Close` を変更するため、`patch/oggwriter.go.patch` と生成される `oggwriter.go` の両方を更新する。`oggwriter.go` は `make patch` で生成されるため、パッチを更新せずに `oggwriter.go` だけを編集しない。
- `0012-bug-fix-ogg-file-truncated-on-reconnect.md` では、ファイルを `createSpeechHandler` で開き、リクエストの終了時に 1 回だけ閉じる。その場合は `OggWriter.Close` はファイルを閉じず、`createSpeechHandler` が閉じる。この issue では現状の `opus2ogg` がファイルを開くため、`OggWriter.Close` がファイルを閉じる。

## 完了条件

- Ogg ファイルの出力を有効にしたセッションが終わると、ファイルディスクリプタが閉じられている。
- 音声を 1 つ以上受信した場合、保存した Ogg ファイルの最後のページが EOS になっている。ペイロードが 255 バイト以上のページでも、最後のページが正しい位置に書かれている。
- 上記を確認するテストがある。モックやスタブは使わない。
