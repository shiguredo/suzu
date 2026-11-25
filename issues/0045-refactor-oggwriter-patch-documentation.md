# Pion の oggwriter のパッチと拡張に説明を付け、取り込み元と公開メソッド名を整理する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/refactor-oggwriter-patch-documentation
- Polished: 2026-11-25

## 目的

Pion の oggwriter を取り込んで変更している理由と取り込み元を記録し、上流の修正を取り込むときの基準を分かるようにする。あわせて、生成物のパッケージコメントと、`oggwriter_ext.go` の `(*OggWriter).Write` の名前を整理し、`go doc` と `staticcheck` で扱いやすい状態にする。

## 現状

- `patch/oggwriter.go.patch` で追加する公開関数 `NewWithoutHeader` と `(*OggWriter).WriteHeaders` にコメントがない。ヘッダーの書き出しを遅らせる理由 (`handler.go` の `opus2ogg` は、最初の音声を受信したときに Ogg のヘッダーを書く) は、パッチで追加する関数と `oggwriter_ext.go` には書かれていない。`server_test.go` の `TestServerOggFileOutput` には、外部サービスへ接続する前に最初の音声を待つことが書かれているが、Ogg のヘッダーを遅らせる理由は書かれていない。
- ヘッダーを遅らせるのは、`opus2ogg` が返す Ogg ストリームを外部サービスへのリクエストボディに使い、`receiveFirstAudioData` (`handler.go`) が最初の音声データを読むまで外部サービスへ接続しないためである。ヘッダーを先に書くと、`receiveFirstAudioData` がヘッダーだけを最初の音声データとして返し、音声が届く前に外部サービスへ接続してしまう。
- `_third_party/pion/` のファイルを、上流のどの版から取り込んだかが記録されていない。現在の `go.mod` と `go.sum` には `github.com/pion/webrtc` がないため、これらからは版を辿れない。
- `_third_party/pion/oggwriter.go` と `util.go` は、`github.com/pion/webrtc/v4` の v4.0.1 の `pkg/media/oggwriter/oggwriter.go` と `internal/util/util.go` と一致する。`_third_party/pion/oggwriter.go` を更新した 2024-10-24 のコミットの `go.mod` と `go.sum` には `github.com/pion/webrtc/v4 v4.0.1` がある。
- 生成される `oggwriter.go` のパッケージのコメントが `Package oggwriter implements OGG media container writer` のままで、`go doc` で `suzu` パッケージの説明として表示される。`util.go` も同じく `Package util ...` になっている。`staticcheck.conf` の `-ST1000` がこれを隠している。
- `oggwriter_ext.go` の `(*OggWriter).Write` は、`io.Writer` の `Write` と同じ名前で、異なるシグネチャ (`*codecs.OpusPacket` を受け取る) である。
- `oggwriter_ext.go` にもコメントがほとんどない。

## 設計方針

- パッチで追加する `NewWithoutHeader` と `(*OggWriter).WriteHeaders` に、追加した理由と使い方のコメントを付ける。`NewWithoutHeader` は Ogg のヘッダーを書かずに `OggWriter` を作り、`WriteHeaders` は最初の音声データを受信したときに 1 回だけ呼ぶことを書く。ヘッダーを遅らせる理由 (音声が届くまで外部サービスへ接続しない) も書く。
- `oggwriter_ext.go` の `(*OggWriter).Write` と `GranulePosition` に、役割と理由のコメントを付ける。`Write` は、Suzu が Sora から Opus パケットを直接受け取るため、RTP パケットを受け取る `WriteRTP` ではなく Opus パケットを受け取る必要があることを書く。`GranulePosition` は、Opus の TOC の config から 1 フレームの PCM サンプル数を求めることを書く。`GranulePosition` が別の issue で非公開になる場合は、その後の名前でコメントを付ける。
- 取り込み元の上流のリポジトリ、版 (タグまたはコミット)、取り込んだ日を、`_third_party/pion/README.md` に記録する。`oggwriter.go` は `github.com/pion/webrtc/v4` の v4.0.1 の `pkg/media/oggwriter/oggwriter.go`、`util.go` は `internal/util/util.go` から 2024-10-24 に取り込んだことを書く。`util.go` を生成するパッチをなくす issue が先に完了した場合は、`oggwriter.go` の取り込み元だけを記録する。
- 生成されるファイルのパッケージのコメントを、パッチで削除する。`util.go` を生成するパッチをなくす issue が先に完了した場合は、`oggwriter.go` だけを対象にする。
- `suzu` パッケージのパッケージコメントを、`doc.go` などに追加する。`Package suzu` で始まる日本語のコメントにし、`staticcheck` の ST1000 がパッケージのコメントがない場合も報告するため、生成物のパッケージコメントを削除しただけでは `-ST1000` を外せないことを踏まえる。
- 生成物のパッケージコメントをすべて削除し、`suzu` パッケージのパッケージコメントを追加したら、`staticcheck.conf` の `-ST1000` を外し、`staticcheck` が通ることを確認する。
- `(*OggWriter).Write` を `WriteOpusPacket` など、`io.Writer` の `Write` と紛らわしくない名前にする。`handler.go` の `opus2ogg` の呼び出し元も更新する。
- `(*OggWriter).Write` の改名は公開メソッドの後方互換のない変更なので、`CHANGES.md` の `## develop` の `### misc` に `[CHANGE]` として記載する。
- `oggwriter.go` を追跡対象から外す issue が先に完了した場合は、`patch/oggwriter.go.patch` だけを更新する。追跡している場合は、`make patch` で再生成した `oggwriter.go` も更新する。`go doc` は `make patch` の後で確認する。

## 完了条件

- パッチで追加する `NewWithoutHeader` と `(*OggWriter).WriteHeaders` に、追加した理由と使い方のコメントがある。
- `oggwriter_ext.go` の `(*OggWriter).Write` (改名後はその名前) と `GranulePosition` (別の issue で名前が変わった場合はその名前) に、役割と理由のコメントがある。
- `(*OggWriter).Write` が `io.Writer` の `Write` と紛らわしくない名前に変わり、`handler.go` の `opus2ogg` の呼び出し元が更新されている。
- `_third_party/pion/README.md` に、上流のリポジトリ、版 (v4.0.1)、取り込んだ日 (2024-10-24)、取り込み元のファイルパスが記録されている。
- `make patch` の後で `go doc` を実行し、`suzu` パッケージの説明として oggwriter / util の説明が表示されず、`suzu` パッケージの説明が表示される。
- `staticcheck.conf` の `-ST1000` を外した状態で `staticcheck` が通る。
- `make patch` で再生成した `oggwriter.go` に、パッチの変更が反映されている。
- `CHANGES.md` の `## develop` の `### misc` に、`(*OggWriter).Write` の改名が `[CHANGE]` として記載されている。
- `make` と `make test` が通る。
