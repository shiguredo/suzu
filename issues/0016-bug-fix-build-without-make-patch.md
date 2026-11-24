# 追跡している oggwriter.go がパッチの適用結果より古く make patch なしではビルドできない

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-build-without-make-patch
- Polished: 2026-11-24

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

- `oggwriter.go` も `util.go` と同じく生成物として扱い、`git rm --cached oggwriter.go` で追跡対象から外して `.gitignore` に追加する。
- `Makefile` の `test` を `patch` に依存させる。
- `go build ./...` を直接実行する場合は、先に `make patch` で生成物を作る必要がある。`make` と `make test` は `patch` に依存するため、チェックアウトした直後の状態から成功する。
- `.air.toml` の `cmd` を `make patch && go build -race -o bin/suzu-dev cmd/suzu/main.go` にする。`cmd` を `make` だけにすると `Makefile` の `all` が `bin/suzu` を生成し、`.air.toml` の `bin = "bin/suzu-dev"` と一致しない。
- `.air.toml` の `include_ext` に `patch` を追加し、`exclude_file` に `oggwriter.go` と `util.go` を追加する。`make patch` は `patch -o` で生成物を毎回書き換えるため、生成物を監視対象から外さないと air の再ビルドが連鎖する。

## 完了条件

- チェックアウトした直後の状態から、`make patch` を手動で実行しなくても `make test` と `make` が成功する。
- `make patch` を実行しても、`git status` に追跡ファイルの差分が出ない。
- air を起動するだけで、チェックアウトした直後の状態からビルドが成功する。
- air で Go ファイルを変更したときに再ビルドされ、生成物の書き換えで再ビルドが連鎖しない。
