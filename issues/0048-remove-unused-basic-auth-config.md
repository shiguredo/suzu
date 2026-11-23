# 実装のない Basic 認証の設定項目を削除する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/remove-unused-basic-auth-config
- Polished: {YYYY-MM-DD}

## 目的

設定しても何も起きない設定項目を削除し、Basic 認証が掛かっていると誤解されないようにする。

## 現状

- `config.go` の `Config` に `SkipBasicAuth` (`skip_basic_auth`)、`BasicAuthUsername` (`basic_auth_username`)、`BasicAuthPassword` (`basic_auth_password`) がある。
- これらのフィールドは、Go のコードのどこからも読まれていない。`basic_auth_username` と `basic_auth_password` を設定しても Basic 認証は掛からない。echo を使っていた頃の `server.go` にも、Basic 認証のミドルウェアはなかった。
- `config_example.ini` に `skip_basic_auth = true` がある。
- `healthcheck_handler_test.go` と `test_handler_test.go` の `Config` に `SkipBasicAuth: true` がある。
- `config.go` の `NewConfig` が使う `ini.StrictMapTo` は、`Config` にないキーを無視する。そのため、フィールドを削除しても、これらのキーが残っている既存の config.ini で起動できる (確認済み)。

## 設計方針

- 3 つのフィールド、`config_example.ini` の `skip_basic_auth`、テストの `SkipBasicAuth: true` を削除する。
- `CHANGES.md` の `## develop` に、設定項目の削除として記載する。設定しても効果がなかったこと、残っていても起動に影響しないことを書く。

## 完了条件

- `Config` と `config_example.ini` に Basic 認証の設定項目がない。
- これらのキーが残っている config.ini で起動できることを確認するテストがある。
