# デバッグ用の /test と /dump を無効にする設定を追加する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/add-option-to-disable-test-and-dump-endpoints
- Polished: {YYYY-MM-DD}

## 目的

本番の環境で、デバッグ用の `/test` と `/dump` を無効にできるようにする。

## 現状

- `server.go` の `NewServer` は、`-service` の値にかかわらず、`POST /test` と `POST /dump` を常に登録する。
- `/dump` は、受信した音声を `dump_file` に追記し続ける (`packet_dump_handler.go` の `PacketDumpHandler.Handle`)。ファイルの大きさに上限はなく、ファイルは 0644 で作成される。
- `config_example.ini` の例では `dump_file = ./dump.jsonl` が有効になっている。
- 既定ではクライアント認証がないため、待ち受けに到達できる誰でも `/dump` で書き込める。
- `doc/USE.md` は `/test` と `/dump` をデバッグ機能として説明している。

## 設計方針

- `/test` と `/dump` を登録しないようにする設定 (例: `disable_debug_endpoints`) を追加する。既定ではこれまでどおり登録する。
- 無効にした場合、`/test` と `/dump` は登録していないパスと同じく 404 になる。
- `config_example.ini` と `doc/USE.md` に設定を記載し、`CHANGES.md` の `## develop` に `[ADD]` として記載する。

## 完了条件

- 設定を有効にすると、`/test` と `/dump` が 404 になる。
- 既定では、これまでどおり `/test` と `/dump` を受け付ける。
- 上記を確認するテストがある。
