# CI とリリースのワークフロー、Makefile を規約と現状に合わせて更新する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/update-build-and-ci-workflows
- Polished: 2026-11-25

## 目的

CI が整形の漏れを検出できるようにし、ワークフローとビルドの設定を shiguredo-github-actions の規約と現状に合わせる。

## 現状

- `.github/workflows/ci.yaml`
  - `go fmt .` は整形して書き戻すだけで、未整形のファイルがあっても終了コードは 0 である。対象もリポジトリの直下のパッケージだけである。
  - ci のジョブに `permissions` と `timeout-minutes` がない。
- `.github/workflows/release.yml`
  - `go install github.com/tcnksm/ghr@latest` は版を固定していない。
  - release のジョブに `permissions` (ghr が使う `contents: write`) の明示と `timeout-minutes` がない。
  - `Patch` のステップは、続く `make release` が `patch` に依存しているため重複している。
  - canary のタグ (`*-canary.*`) も、通常のリリースと同じく draft として作られ、prerelease にならない。
- 両方のワークフロー
  - `actions/checkout` は v6.0.2、`actions/setup-go` は v6.4.0 で、より新しい版がある。
  - ci と release のジョブは `ubuntu-24.04` を使っている。shiguredo-github-actions は「ubuntu-slim を使えない場合は、その理由を明確にしてから別の runner を選ぶこと」と定めているが、理由が残っていない。ubuntu-slim で要件を満たせるかは、まだ実際のジョブで確認していない。
    - ubuntu-slim のイメージ (actions/runner-images の `ubuntu-slim-Readme.md`) には `patch` コマンドが含まれていない。`make patch` を実行するには `patch` を追加でインストールする必要がある。`make`、`gcc`、`git`、`gzip` は含まれている。
    - ubuntu-slim は 1 vCPU、5 GB のコンテナで、GitHub のドキュメントは「典型的な重量級の CI/CD ビルドには向かない」としている。
- `Makefile`
  - `.PHONY` に `release` がない。
  - `release` に `-trimpath` がなく、ビルドした環境の絶対パスがバイナリに埋め込まれる。
- `CHANGES.md`
  - `## develop` の `### misc` に、CI と Makefile の変更がまだ記載されていない。

## 設計方針

- 整形の検査は `gofmt -l .` の出力が空であることを確認する形にし、リポジトリ全体を対象にする。出力が空でない場合は CI を失敗させる。
- 各ジョブに必要最小限の `permissions` と、ジョブの通常の実行時間に余裕を持たせた `timeout-minutes` を設定する。ci は `contents: read`、release は `contents: write` にする。
- `ghr` の版を固定する。確認時点の最新は `v0.18.6` なので、`go install github.com/tcnksm/ghr@v0.18.6` にする。
- 重複した `Patch` のステップを削除する。canary のタグ (`*-canary.*`) では `ghr` に `-prerelease` を渡し、通常のタグでは `-draft` を渡す。
- actions は `/update-actions` で最新の版に更新し、コミットハッシュの固定とバージョンのコメントの形式を維持する。
- ubuntu-slim で ci と release のジョブの要件を満たせるかを確認し、満たせるなら ubuntu-slim にする。ubuntu-slim を使う場合は `patch` をインストールするステップを追加する。満たせない場合は理由をワークフローの `runs-on` 付近のコメントに残す。
- `Makefile` の `.PHONY` に `release` を追加し、`release` の 2 つの `go build` の両方に `-trimpath` を付ける。
- `CHANGES.md` の `## develop` の `### misc` に、CI と Makefile の変更を `[UPDATE]` として追記する。

## 完了条件

- `gofmt -l .` の出力が空でない場合に CI が失敗する。対象はリポジトリ全体の Go ファイルである。
- ci と release のジョブに、必要最小限の `permissions` (ci は `contents: read`、release は `contents: write`) と、ジョブの通常の実行時間に余裕を持たせた `timeout-minutes` がある。
- `ghr` の版が `@latest` ではなく、具体的なバージョン (確認時点では `v0.18.6`) に固定されている。
- release ワークフローから重複した `Patch` のステップが削除されている。
- canary のタグ (`*-canary.*`) で作られるリリースが prerelease になり、通常のタグは draft のままになる。
- actions の参照が `/update-actions` で最新版に更新され、コミットハッシュ固定 + バージョンコメントの形式が維持されている。
- ubuntu-slim を使うジョブは、`patch` をインストールするステップがあり、そのジョブが ubuntu-slim で成功する。ubuntu-slim を使わないジョブは、その理由がワークフローの `runs-on` 付近のコメントに残っている。
- `Makefile` の `.PHONY` に `release` が追加されている。
- `make release` の 2 つの `go build` の両方に `-trimpath` が付いており、`strings dist/suzu* | grep -F "$(pwd)"` が何も出力しない。
- `CHANGES.md` の `## develop` の `### misc` に、CI と Makefile の変更が `[UPDATE]` として追記されている。
