# Pion の util.go の取り込みとパッチをやめて標準ライブラリで置き換える

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/remove-pion-util-patch
- Polished: {YYYY-MM-DD}

## 目的

1 つの関数のために取り込んでいる Pion の util.go と、そのパッチ、依存ライブラリをなくす。

## 現状

- `Makefile` の `patch` は、`_third_party/pion/util.go` に `patch/util.go.patch` を当てて `util.go` を生成する。
- 生成した `util.go` のうち、Suzu が使うのは `RandUint32` だけである (パッチで生成する `oggwriter.go` の `NewWithoutHeader` などが Ogg のシリアル番号に使う)。`MathRandAlpha`、`FlattenErrs`、`multiError` は、deadcode で到達しないと判定された。
- `RandUint32` のために `github.com/pion/randutil` を直接依存にしている。
- 生成した `util.go` のパッケージのコメントが `Package util ...` になっており、`go doc` で `suzu` パッケージの説明として表示される。

## 設計方針

- `RandUint32` を、標準の `math/rand/v2` を使う関数で置き換える。置き換える関数は Suzu のコード (`oggwriter_ext.go` など) に置き、`patch/oggwriter.go.patch` が参照する名前を変える必要があればパッチも更新する。
- `_third_party/pion/util.go`、`patch/util.go.patch`、`Makefile` の `patch` の util.go の行、`.gitignore` の `util.go` を削除する。
- `go mod tidy` で `github.com/pion/randutil` を依存から外す。
- `NOTICE.md` の記載が util.go に関わる場合は更新する。

## 完了条件

- `util.go` を生成しなくてもビルドとテストが通る。
- `go.mod` に `github.com/pion/randutil` がない。
