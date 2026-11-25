# 実装のない Basic 認証の設定項目を削除する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/remove-unused-basic-auth-config
- Polished: 2026-11-25

## 目的

設定しても何も起きない設定項目を削除し、Basic 認証が掛かっていると誤解されないようにする。

## 現状

- `config.go` の `Config` に `SkipBasicAuth` (`skip_basic_auth`)、`BasicAuthUsername` (`basic_auth_username`)、`BasicAuthPassword` (`basic_auth_password`) がある。
- これらのフィールドは、Go のコードのどこからも読まれていない。`basic_auth_username` と `basic_auth_password` を設定しても Basic 認証は掛からない。echo を使っていた頃の `server.go` にも、Basic 認証のミドルウェアはなかった。
- `config_example.ini` に `skip_basic_auth = true` がある。`basic_auth_username` と `basic_auth_password` は `config_example.ini` にない。
- `healthcheck_handler_test.go` と `test_handler_test.go` の `Config` に `SkipBasicAuth: true` がある。
- `config.go` の `NewConfig` が使う `ini.StrictMapTo` は、`Config` にないキーを無視する。そのため、フィールドを削除しても、これらのキーが残っている既存の config.ini で起動できる (確認済み)。

## 設計方針

- 3 つのフィールド、`config_example.ini` の `skip_basic_auth`、テストの `SkipBasicAuth: true` を削除する。
- `config_test.go` に、`listen_addr` と削除した 3 つのキー (`skip_basic_auth`、`basic_auth_username`、`basic_auth_password`) を含む一時的な config.ini を `NewConfig` に渡し、エラーにならないことを確認するテストを追加する。`listen_addr` は `NewConfig` の `validateConfig` が必須にしているため含める。
- `CHANGES.md` の `## develop` に `[UPDATE]` で記載する。設定しても効果がなかったこと、残っていても起動に影響しないことを書き、担当者の行 (`- @ユーザー名`) を付ける。削除する設定項目は `skip_basic_auth`、`basic_auth_username`、`basic_auth_password` である。

## 完了条件

- `Config` と `config_example.ini` に Basic 認証の設定項目がない。
- `config_test.go` に、`listen_addr` と削除した 3 つのキーを含む一時的な config.ini を `NewConfig` に渡してエラーにならないことを確認するテストがある。
- `CHANGES.md` の `## develop` に、削除する設定項目と、設定しても効果がなかったこと、残っていても起動に影響しないことを書いた `[UPDATE]` のエントリがある。
- `make test` が通る。

## 関連 issue

- `0054-doc-fix-config-example.md` は `Config` のすべての設定項目を `config_example.ini` に記載することを完了条件にしている。本 issue を先に完了させ、削除する設定項目を 0054 が追加しないようにする。
