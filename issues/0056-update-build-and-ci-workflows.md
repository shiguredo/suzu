# CI とリリースのワークフロー、Makefile を規約と現状に合わせて更新する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/update-build-and-ci-workflows
- Polished: {YYYY-MM-DD}

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
  - ci と release のジョブは `ubuntu-24.04` を使っている。shiguredo-github-actions は「ubuntu-slim を使えない場合は、その理由を明確にしてから別の runner を選ぶこと」と定めているが、理由が残っていない。ubuntu-slim で要件を満たせるかは確認していない。
- `Makefile`
  - `.PHONY` に `release` がない。
  - `release` に `-trimpath` がなく、ビルドした環境の絶対パスがバイナリに埋め込まれる。

## 設計方針

- 整形の検査は `gofmt -l` の出力が空であることを確認する形にし、リポジトリ全体を対象にする。
- 各ジョブに必要最小限の `permissions` と `timeout-minutes` を設定する。
- `ghr` の版を固定する。
- 重複した `Patch` のステップを削除する。canary のタグは prerelease として作る。
- actions は `/update-actions` で最新の版に更新し、コミットハッシュの固定とバージョンのコメントの形式を維持する。
- ubuntu-slim で ci と release のジョブの要件を満たせるかを確認し、満たせるなら ubuntu-slim にする。満たせない場合は理由を残す。
- `Makefile` の `.PHONY` に `release` を追加し、`release` に `-trimpath` を付ける。

## 完了条件

- 未整形のファイルがあると CI が失敗する。
- 各ジョブに `permissions` と `timeout-minutes` がある。
- `ghr` の版が固定されている。
- runner の選定が規約に沿っている。
- `make release` の成果物に、ビルドした環境の絶対パスが含まれない。
