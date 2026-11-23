# Pion の oggwriter に当てるパッチと拡張に説明と取り込み元の情報を付ける

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/refactor-oggwriter-patch-documentation
- Polished: {YYYY-MM-DD}

## 目的

Pion の oggwriter を取り込んで変更している理由と取り込み元を記録し、上流の修正を取り込むときの基準を分かるようにする。

## 現状

- `patch/oggwriter.go.patch` で追加する公開関数 `NewWithoutHeader` と `(*OggWriter).WriteHeaders` にコメントがない。ヘッダーの書き出しを遅らせる理由 (`handler.go` の `opus2ogg` は、最初の音声を受信したときに Ogg のヘッダーを書く) がどこにも書かれていない。
- `_third_party/pion/` のファイルを、上流のどの版から取り込んだかが記録されていない。`go.mod` に pion/webrtc はないため、版を辿れない。
- 生成される `oggwriter.go` のパッケージのコメントが `Package oggwriter implements OGG media container writer` のままで、`go doc` で `suzu` パッケージの説明として表示される。`util.go` も同じく `Package util ...` になっている。`staticcheck.conf` の `-ST1000` がこれを隠している。
- `oggwriter_ext.go` の `(*OggWriter).Write` は、`io.Writer` の `Write` と同じ名前で、異なるシグネチャ (`*codecs.OpusPacket` を受け取る) である。
- `oggwriter_ext.go` にもコメントがほとんどない。

## 設計方針

- パッチで追加する関数に、追加した理由と使い方のコメントを付ける。
- 取り込み元の上流のリポジトリ、版 (タグまたはコミット)、取り込んだ日を、`_third_party/pion/` の README などに記録する。
- 生成されるファイルのパッケージのコメントを、パッチで削除する。`util.go` を生成するパッチをなくす issue が先に完了した場合は、`oggwriter.go` だけを対象にする。両方が片付いたら `staticcheck.conf` の `-ST1000` を外す。
- `(*OggWriter).Write` を、`io.Writer` と紛らわしくない名前 (`WriteOpusPacket` など) にする。

## 完了条件

- パッチで追加する関数と `oggwriter_ext.go` に、理由の分かるコメントがある。
- 取り込み元の上流の版が記録されている。
- `go doc` で `suzu` パッケージの説明として oggwriter / util の説明が表示されない。
