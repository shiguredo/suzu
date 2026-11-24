# 認証情報ファイルを .gitignore で除外する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-ignore-credentials-file
- Polished: 2026-11-25

## 目的

開発中に置いた AWS の認証情報ファイルを、誤ってリポジトリにコミットしないようにする。

## 現状

- `config_example.ini` の例では、`aws_credential_file = ./credentials` と `aws_profile = default` が有効になっている。`doc/USE.md` と `doc/DEV.md` の手順どおりに `config_example.ini` をコピーすると、作業ディレクトリ (開発時はリポジトリの直下) の `credentials` を読む。
- `.gitignore` は `config.ini` を除外しているが、`credentials` は除外していない。そのため `git add -A` などで認証情報ファイルがステージされる。

## 設計方針

- `.gitignore` に `credentials` を追加する。

## 完了条件

- リポジトリの直下に `credentials` を置いても、`git status` に表示されない。
