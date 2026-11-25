# どこからも使われていないテスト用の証明書と秘密鍵を削除する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/remove-unused-test-certificates
- Polished: 2026-11-25

## 目的

使われていないテスト用の証明書と秘密鍵をリポジトリに置いたままにしない。

## 現状

- `cert/server/` と `cert/client/` に、証明書と秘密鍵の 7 ファイルがコミットされている。秘密鍵は 3 つ (`server.key`、`user.key`、`invalid.key`) ある。
- 証明書はいずれもテスト用で、CN は localhost、テスト用の CA、テスト用のユーザー、有効期限は 2032 年である。`cert/client/ca.pem` は自己署名の CA 証明書で、`server.pem`、`user.pem`、`invalid.pem` はテスト用の CA が署名した証明書である。他のプロジェクトのテスト用の証明書を取り込んだものである。
- コード、テスト、ドキュメント、CI のどこからも参照されていない (rg で確認)。
- shiguredo-no-secrets は秘密鍵をリポジトリに残さない対象としている。secret scanning でも誤検知の原因になる。
- `cert/client/ca.pem` を `tls_verify_cacert_path` に流用すると、リポジトリにある `user.key` を持つ誰でもクライアント認証を通れる。

## 設計方針

- `cert/` を削除する。
- TLS とクライアント認証のテストは、テストの中で証明書を生成する。これは 0032 の「サーバーの起動と停止、TLS、クライアント認証、Exporter の TLS のテストを追加する」で扱う。
- 現行のコードとテストは `cert/` を参照していないため、0032 の完了を待たずに `cert/` を削除できる。

## 完了条件

- リポジトリに `cert/` がない。
