# 重複したテストと置き場所の合っていないテストを整理する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/refactor-cleanup-duplicate-and-misplaced-tests
- Polished: {YYYY-MM-DD}

## 目的

shiguredo-test の「無駄なテストを絶対に追加しないこと」に従い、同じことを確認しているテストを減らし、テストを対象のコードのファイルに対応する場所に置く。

## 現状

- 同じことを確認しているテスト
  - `healthcheck_handler_test.go` の `TestHealthcheckHandler` の HTTP/2 と HTTP/1.1 のケースは、プロトコルを見ないハンドラーを直接呼んでおり、同じことを 2 回確認している。`Config` に `Version` を設定していないため、期待値が `{"version":""}` になっており、バージョンを確認できていない。
  - `IsRetryTarget` の match / mismatch のテストが、`service_handler_test.go` の `TestIsRetryTarget`、`amazon_transcribe_v2_handler_test.go` の awsv2 のテスト、`speech_to_text_handler_test.go`、`test_handler_test.go` の `TestSpeechHandler` の「IsRetryTarget」に重複している。`aws` と `awsv2` は同じ `NewAmazonTranscribeV2Handler` を使う。
  - `amazon_transcribe_v2_handler_test.go` は、同じ構造のテーブルと同じループが 5 組ある。
- 置き場所が合っていないテスト
  - `test_handler_test.go` の `TestSpeechHandler` は、`handler.go` の `createSpeechHandler` のテストである。
  - `handler_test.go` の `TestOpusPacketReader` と `TestReadPacketWithHeader` は、`options.go` のテストで、同じ関数のテストが `options_test.go` にもある。
  - `handler_test.go` が使う `readDumpFile` は、`test_handler_test.go` にある。
- テストの中の誤りと不要な記述
  - テーブルのケース名の重複: `handler_test.go` の `TestReadPacketWithHeader` の「split data」が 5 つ、`service_handler_test.go` の「match」が 3 つ。
  - `amazon_transcribe_v2_handler_test.go` のケース名の末尾に空白がある (`"confidence > minimumConfidenceScore "` など)。
  - 変数名の誤記 `testCaces` (`healthcheck_handler_test.go`、`handler_test.go`)、`chnanelID` (`test_handler_test.go`)。
  - `test_handler_test.go` と `healthcheck_handler_test.go` の `Config` の `DumpFile`、`ListenPort`、`SkipBasicAuth`、`LogStdout`、`Debug` は、テストの結果に影響しない。
  - `test_handler_test.go` の「silent packet」は、`NewServer(&config, "aws")` で作ったサーバーで `test` のハンドラーを呼んでおり、`aws` を指定する意味がない。
  - `handler_test.go` の `ErrReadCloser` は、標準の `testing/iotest.ErrReader` で置き換えられる (`test_handler_test.go` ではすでに使っている)。

## 設計方針

- 同じことを確認しているテストは 1 つにまとめる。`IsRetryTarget` は、`service_handler.go` の `isRetryTargetByConfig` の単体テストと、各ハンドラーの固有の判定 (AWS の例外の型、GCP の gRPC のコード) のテストに分ける。
- `TestHealthcheckHandler` は 1 ケースにし、`Version` を設定してバージョンが返ることを確認する。
- テストを対象のコードのファイルに対応するテストファイルに移す (`createSpeechHandler` は `handler_test.go`、`options.go` の関数は `options_test.go`)。
- ケース名の重複、末尾の空白、誤記、効いていない設定を直す。
- 移動の前後でテストの検証内容は変えない。

## 完了条件

- 同じことを確認しているテストがない。
- テストが対象のコードのファイルに対応する場所にある。
- ケース名が一意で、誤記がない。
