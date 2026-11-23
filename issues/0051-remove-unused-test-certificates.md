# どこからも使われていないテスト用の証明書と秘密鍵を削除する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/remove-unused-test-certificates
- Polished: {YYYY-MM-DD}

## 目的

使われていない秘密鍵をリポジトリに置いたままにしない。

## 現状

- `cert/server/` と `cert/client/` に、証明書と秘密鍵の 6 ファイルがコミットされている。秘密鍵は 3 つ (`server.key`、`user.key`、`invalid.key`) ある。
- いずれも自己署名のテスト用で、どの信頼にもつながっていない (CN は localhost やテスト用の CA、有効期限は 2032 年)。他のプロジェクトのテスト用の証明書を取り込んだものである。
- コード、テスト、ドキュメント、CI のどこからも参照されていない (rg で確認)。
- shiguredo-no-secrets は秘密鍵をリポジトリに残さない対象としている。secret scanning でも誤検知の原因になる。
- `cert/client/ca.pem` を `tls_verify_cacert_path` に流用すると、リポジトリにある `user.key` を持つ誰でもクライアント認証を通れる。

## 設計方針

- `cert/` を削除する。
- TLS とクライアント認証のテストは、テストの中で証明書を生成する (サーバーの起動と停止、TLS のテストの issue で扱う)。

## 完了条件

- リポジトリに `cert/` がない。
