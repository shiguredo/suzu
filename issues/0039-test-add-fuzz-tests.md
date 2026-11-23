# パケットの読み込み、メトリクスのラベル、ResponseWriter のラッパーに fuzz テストを追加する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/add-fuzz-tests
- Polished: {YYYY-MM-DD}

## 目的

入力の組み合わせが多く、単体テストのケースでは網羅しきれない処理の性質を、最小限のテストで確認する。

## 現状

- プロジェクトに PBT や fuzz のテストがない。
- `options.go` の `optionReadPacketWithHeader` は、任意の位置で分割されて届くヘッダー付きのパケットを組み立て直す。`handler_test.go` の `TestReadPacketWithHeader` は、決めた分割のケースだけを確認している。
- `metrics.go` のラベルの値は、時系列の数が増えすぎないように、`url` を登録したパスか `unknown`、`method` を 9 種類か `unknown` に制限している。`server_test.go` は決めたリクエストでだけ確認している。
- `middleware.go` の `responseWriter` は、`WriteHeader`、`Write`、`FlushError` の呼び出しの順序によって、記録するステータスコードとヘッダーの送信済みの状態が変わる。`middleware_test.go` は決めた順序だけを確認している。
- Ogg への書き込みと末尾スラッシュの除去の fuzz テストは、それぞれの不具合の issue で扱う。

## 設計方針

- 依存を増やさないため、Go 標準の fuzzing (`testing.F`) を使う。
- 次の性質を確認する。
  - `optionReadPacketWithHeader`: 任意のペイロードの列をヘッダー付きで連結し、任意の位置で分割して流しても、元のペイロードの列がそのまま出てくる。
  - メトリクスのラベル: 任意のメソッドとパスのリクエストを `newMainHandler` に通しても、`url` は登録したパスか `unknown`、`method` は 9 種類か `unknown` になる。
  - `responseWriter`: `WriteHeader`、`Write`、`FlushError` を任意の順序で呼んでも、記録したステータスコードが、`httptest.ResponseRecorder` に実際に書き込まれたステータスコードと一致する。
- `go test` の通常の実行では、シードのケースだけを実行する。

## 完了条件

- 上記の 3 つの fuzz テストがあり、`make test` でシードのケースが通る。
