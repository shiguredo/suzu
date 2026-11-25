# Pion の util.go の取り込みとパッチをやめて標準ライブラリで置き換える

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/remove-pion-util-patch
- Polished: 2026-11-25

## 目的

1 つの関数のために取り込んでいる Pion の util.go と、そのパッチをなくし、`RandUint32` を標準ライブラリの `math/rand/v2` で置き換える。あわせて、util.go のためだけに直接依存している `github.com/pion/randutil` を直接依存から外す。

## 現状

- `Makefile` の `patch` は、`_third_party/pion/util.go` に `patch/util.go.patch` を当てて `util.go` を生成する。
- 生成した `util.go` のうち、Suzu が使うのは `RandUint32` だけである (パッチで生成する `oggwriter.go` の `NewWith` と `NewWithoutHeader` が Ogg のシリアル番号に使う)。`MathRandAlpha`、`FlattenErrs`、`multiError` は、util.go の外から参照されておらず、deadcode で到達しないと判定された。
- `RandUint32` のために `github.com/pion/randutil` を直接依存にしている。`github.com/pion/rtp` も `github.com/pion/randutil` を要求しており、`go mod tidy` の後も `github.com/pion/randutil` は `// indirect` として go.mod に残る。
- 生成した `util.go` のパッケージのコメントが `Package util ...` になっており、`go doc` で `suzu` パッケージの説明として表示される。

## 設計方針

- `oggwriter_ext.go` に、`math/rand/v2` の `rand.Uint32` を使う非公開の `randUint32` を追加する。`randUint32` は Suzu のコードで使うだけなので、0044 の公開シンボルを増やさないよう非公開にする。
- `patch/oggwriter.go.patch` の `serial: RandUint32()` を `serial: randUint32()` に置き換える。
- `_third_party/pion/util.go`、`patch/util.go.patch`、`Makefile` の `patch` の util.go の行、`.gitignore` の `util.go` を削除する。`.gitignore` の `# 自動生成` のコメントは、0016 が完了して `oggwriter.go` の記載が残る場合はそのままにし、他に自動生成の記載がなければ削除する。
- `oggwriter.go` が追跡されている場合 (0016 が未完了の場合) は、`make patch` で再生成した `oggwriter.go` をコミットし、`make patch` を実行しても追跡ファイルに差分が出ないようにする。0016 が先に完了して `oggwriter.go` が追跡対象から外れている場合は、`patch/oggwriter.go.patch` だけを更新する。
- 0016 が先に完了している場合は、`.air.toml` の `exclude_file` から `util.go` を削除する。0016 が後に完了する場合は、`util.go` は生成されないため `exclude_file` への追加は不要である。
- 0045 が先に完了している場合は、`_third_party/pion/README.md` から util.go の記載を削除する。0045 が後に完了する場合は、`util.go` は既にないため `oggwriter.go` の取り込み元だけを記録する。
- `go mod tidy` で `github.com/pion/randutil` を直接依存から外す。`github.com/pion/rtp` が要求するため、`// indirect` としては go.mod に残る。
- `NOTICE.md` は Pion WebRTC の MIT ライセンス表示で、ファイル単位の記載はないため変更しない。

## 完了条件

- `_third_party/pion/util.go` と `patch/util.go.patch` がない。
- `make patch` が `util.go` を生成しなくなり、`make` と `make test` が通る。
- `make patch` を実行しても、追跡しているファイルに差分が出ない (0016 が未完了の場合は `oggwriter.go` を再生成してコミットする)。
- `go.mod` の直接依存に `github.com/pion/randutil` がない (`github.com/pion/rtp` が要求するため、`// indirect` としては残り、`go.sum` の記載も残る)。
- `RandUint32` が `suzu` パッケージの公開シンボルに含まれない (`randUint32` は非公開)。
- `Makefile` と `.gitignore` に `util.go` への参照が残っていない。
- 0016 が先に完了している場合は `.air.toml` に、0045 が先に完了している場合は `_third_party/pion/README.md` に `util.go` への参照が残っていない。

## 関連 issue

- 0016 は `oggwriter.go` を追跡対象から外し、`.air.toml` の `exclude_file` に `oggwriter.go` と `util.go` を追加する。0016 が先に完了している場合は、本 issue で `.air.toml` の `exclude_file` から `util.go` を削除する。0016 が後に完了する場合は、`util.go` は生成されないため `exclude_file` への追加は不要である。
- 0044 は `suzu` パッケージの公開シンボルを `cmd/suzu/main.go` が使うものだけにし、`CHANGES.md` の `### misc` に `[CHANGE]` を記載する。本 issue で util.go を削除すると `RandUint32`、`MathRandAlpha`、`FlattenErrs` の公開がなくなる。これらの公開シンボルの削減は 0044 の `[CHANGE]` に含まれるため、本 issue では `CHANGES.md` を変更しない。
- 0045 は `_third_party/pion/README.md` に取り込み元を記録し、生成物のパッケージコメントを削除する。0045 が先に完了している場合は、本 issue で `README.md` から `util.go` の記載を削除する。0045 が後に完了する場合は、`util.go` は既にないため `oggwriter.go` の取り込み元だけを記録する。
