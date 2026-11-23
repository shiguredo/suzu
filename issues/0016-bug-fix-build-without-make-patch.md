# 追跡している oggwriter.go がパッチの適用結果より古く make patch なしではビルドできない

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-build-without-make-patch
- Polished: {YYYY-MM-DD}

## 目的

パッチから生成するファイルの扱いを 1 つにそろえ、`make test` と開発用のライブリロードが、チェックアウトした直後の状態から確実に動くようにする。

## 現状

- `oggwriter.go` と `util.go` は、どちらも `Makefile` の `patch` ターゲットで `_third_party/pion/` のファイルに `patch/` のパッチを当てて生成する。
- `util.go` は `.gitignore` で「自動生成」として追跡対象から外しているが、`oggwriter.go` は追跡されている。
- 追跡している `oggwriter.go` は、`patch/oggwriter.go.patch` を更新したときに再生成されておらず、パッチが追加する `NewWithoutHeader` と `(*OggWriter).WriteHeaders` を含まない。そのため、チェックアウトした直後の状態で `go build ./...` を実行すると、`handler.go` の `opus2ogg` が参照する `NewWithoutHeader` が見つからずに失敗する。
- `make patch` を実行すると、追跡している `oggwriter.go` が書き換わり、作業ツリーに差分が出る。
- `Makefile` の `all` と `release` は `patch` に依存しているが、`test` は依存していない。CI は `make test` の前に `make patch` を実行しているため通っている。
- `.air.toml` の `cmd` は `go build` を直接実行し、`make patch` を実行しない。

## 設計方針

- `oggwriter.go` も `util.go` と同じく生成物として扱い、追跡対象から外して `.gitignore` に追加する。
- `Makefile` の `test` を `patch` に依存させる。
- `.air.toml` のビルドの前にもパッチを当てる (`make patch` を実行するか、`cmd` を `make` 経由にする)。

## 完了条件

- チェックアウトした直後の状態から、`make test` と `make` が成功する。
- `make patch` を実行しても、作業ツリーに追跡ファイルの差分が出ない。
- air でのビルドが、チェックアウトした直後の状態から成功する。
