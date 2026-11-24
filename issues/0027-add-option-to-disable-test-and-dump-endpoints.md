# デバッグ用の /test と /dump を無効にする設定を追加する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/add-option-to-disable-test-and-dump-endpoints
- Polished: 2026-11-25

## 目的

本番の環境で、デバッグ用の `/test` と `/dump` を無効にできるようにする。

## 現状

- `server.go` の `NewServer` は、`-service` の値にかかわらず、`POST /test` と `POST /dump` を常に登録する。
- `/dump` は、受信した音声を `dump_file` に追記し続ける (`packet_dump_handler.go` の `PacketDumpHandler.Handle`)。ファイルの大きさに上限はなく、ファイルは 0644 で作成される。
- `config_example.ini` の例では `dump_file = ./dump.jsonl` が有効になっている。
- 既定ではクライアント認証がないため、待ち受けに到達できる誰でも `/dump` で書き込める。
- `doc/USE.md` は `/test` と `/dump` をデバッグ機能として説明している。

## 設計方針

- `config.go` の `Config` に `DisableDebugEndpoints` (`disable_debug_endpoints`) を追加する。既定値は `false` とし、これまでどおり `/test` と `/dump` を登録する。
- `server.go` の `NewServer` で、`DisableDebugEndpoints` が `true` の場合は `/test` と `/dump` の `handleRoute` を呼ばない。`handleRoute` はメソッドのルートと OPTIONS のルートを登録するため、両方のルートを登録しない。
- 無効にした場合、`/test` と `/dump` は登録していないパスと同じく 404 になる。メソッドにかかわらず 404 になり、OPTIONS も 404 になる。
- `-service` に `test` や `dump` を指定した場合も、`/speech` はこれまでどおりそのハンドラーを使う。設定は `/test` と `/dump` の登録だけを制御する。
- `config.go` の `ShowConfig` で `disable_debug_endpoints` を出力する。
- `config_example.ini` に `disable_debug_endpoints` の説明と既定値 (`false`) を追加する。
- `doc/USE.md` のデバッグ機能の節に、`disable_debug_endpoints` で `/test` と `/dump` を無効にできることと、無効にすると `dump_file` が使われないことを追加する。
- `CHANGES.md` の `## develop` に `[ADD]` として、設定項目と既定値を記載する (担当者の行を含む)。
- `server_test.go` に、無効時に `/test` と `/dump` が 404 になることを確認するテストを追加する。既定の挙動は既存の `TestServer` で確認している。

## 完了条件

- `disable_debug_endpoints = true` の場合、`/test` と `/dump` へのリクエストが、メソッドにかかわらず 404 になる。
- `disable_debug_endpoints` が未設定または `false` の場合、これまでどおり `POST /test` と `POST /dump` を受け付ける。
- 上記を確認するテストがある。
- `config_example.ini` と `doc/USE.md` に `disable_debug_endpoints` が記載されている。
- 起動時の `CONF` のログに `disable_debug_endpoints` が出る。
- `CHANGES.md` の `## develop` に `[ADD]` のエントリがある。
