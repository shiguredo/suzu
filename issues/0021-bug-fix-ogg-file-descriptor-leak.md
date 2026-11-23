# Ogg ファイルの出力を有効にするとファイルディスクリプタを閉じずに終了する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-ogg-file-descriptor-leak
- Polished: {YYYY-MM-DD}

## 目的

`enable_ogg_file_output = true` のとき、セッションの終了時に Ogg ファイルを確実に閉じ、ファイルの末尾 (EOS のページ) を正しく書き終える。

## 現状

- `handler.go` の `opus2ogg` の goroutine は、どの経路でも `oggWriter.CloseWithError` で pipe を閉じてから、defer の `o.Close()` を実行する。`o.Close()` のエラーは捨てている。
- `OggWriter.Close` (パッチの適用後の `oggwriter.go`) は、ファイルの最後のページを読み直して EOS のページとして `writeToStream` で書き直し、最後に `fd.Close()` を呼ぶ。
  - `writeToStream` の書き込み先は、ファイルと pipe をまとめた `io.MultiWriter` である。pipe は先に閉じているため書き込みが必ず失敗し、`fd.Close()` に到達しない。
  - 音声を 1 つも受信せずに終わった場合は、空のファイルに対する `Seek` が失敗し、同じく `fd.Close()` に到達しない。
- 実測では、Ogg ファイルの出力を有効にした 10 リクエストの後、`.ogg` のファイルディスクリプタが 8 個開いたままだった。GC のファイナライザーで後から閉じられるとみられるが、閉じられるまでの間は開いたままになる。
- `NewWithoutHeader` が失敗した場合は、作成したファイルを閉じずに return する。
- echo を使っていた頃から同じコードで、HTTP サーバーの実装とは関係ない。

## 設計方針

- EOS のページの書き直しはファイルにだけ行い、閉じた pipe に書き込まない。
- `OggWriter.Close` がエラーを返す場合も、ファイルは必ず閉じる。
- `opus2ogg` が作成したファイルは、`OggWriter` の生成に失敗した場合も含めて、すべての経路で閉じる。
- `Close` のエラーは捨てずにログに出す。

## 完了条件

- Ogg ファイルの出力を有効にしたセッションが終わると、ファイルディスクリプタが閉じられている。
- 保存した Ogg ファイルの最後のページが EOS になっている。
- 上記を確認するテストがある。
