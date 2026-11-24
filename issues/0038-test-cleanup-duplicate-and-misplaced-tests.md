# 重複したテストと置き場所の合っていないテストを整理する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/refactor-cleanup-duplicate-and-misplaced-tests
- Polished: 2026-11-25

## 目的

shiguredo-test の「無駄なテストを絶対に追加しないこと」に従い、同じことを確認しているテストと、同じ構造のテーブルとループを減らし、テストを対象のコードのファイルに対応する場所に置く。あわせて、ケース名の重複、末尾の空白、誤記、効いていない設定や引数を直す。

## 現状

- 同じことを確認しているテスト
  - `healthcheck_handler_test.go` の `TestHealthcheckHandler` の HTTP/2 と HTTP/1.1 のケースは、プロトコルを見ないハンドラーを直接呼んでおり、同じことを 2 回確認している。`Config` に `Version` を設定していないため、期待値が `{"version":""}` になっており、バージョンを確認できていない。
  - `IsRetryTarget` の match / mismatch のテストが、`service_handler_test.go` の `TestIsRetryTarget`、`amazon_transcribe_v2_handler_test.go` の awsv2 のテスト、`speech_to_text_handler_test.go` に重複している。
    `test_handler_test.go` の `TestSpeechHandler` の「IsRetryTarget」は、`test` サービスが `RetryTargets` に一致してもリトライしないことの確認で、match の期待値だけが他のハンドラーと異なる。
    `aws` と `awsv2` は同じ `NewAmazonTranscribeV2Handler` を使う。
  - `amazon_transcribe_v2_handler_test.go` は、同じ構造のテーブルと同じループが 5 組ある。
- 置き場所が合っていないテスト
  - `test_handler_test.go` の `TestSpeechHandler` は、`handler.go` の `createSpeechHandler` のテストである (`IsRetryTarget` のサブテストを除く)。
  - `handler_test.go` の `TestOpusPacketReader` と `TestReadPacketWithHeader` は、`options.go` のテストで、同じ関数のテストが `options_test.go` にもある。
  - `handler_test.go` が使う `readDumpFile` は、`test_handler_test.go` にある。
- テストの中の誤りと不要な記述
  - テーブルのケース名の重複: `handler_test.go` の `TestReadPacketWithHeader` の「split data」が 5 つ、`service_handler_test.go` の「match」が 3 つ。
  - `amazon_transcribe_v2_handler_test.go` のケース名の末尾に空白がある (`"confidence > minimumConfidenceScore "` など)。
  - 変数名の誤記 `testCaces` (`healthcheck_handler_test.go`、`handler_test.go`)、`chnanelID` (`test_handler_test.go`)。
  - `test_handler_test.go` と `healthcheck_handler_test.go` の `Config` の `DumpFile`、`ListenPort`、`SkipBasicAuth`、`LogStdout`、`Debug` は、テストの結果に影響しない。`healthcheck_handler_test.go` の `TimeToWaitForOpusPacketMs` も、ヘルスチェックのハンドラーでは使われないため、テストの結果に影響しない。
  - `test_handler_test.go` の「silent packet」は、`NewServer(&config, "aws")` で作ったサーバーで `test` のハンドラーを呼んでおり、`aws` を指定する意味がない。
  - `handler_test.go` の `ErrReadCloser` は、`io.NopCloser` で包んだ標準の `testing/iotest.ErrReader` で置き換えられる (`test_handler_test.go` ではすでに使っている)。

## 設計方針

- 同じことを確認しているテストは 1 つにまとめる。`IsRetryTarget` は、`service_handler.go` の `isRetryTargetByConfig` の単体テストと、各ハンドラーの固有の判定 (AWS の例外の型、GCP の gRPC のコード) のテストに分ける。`test_handler_test.go` の `TestSpeechHandler` の「IsRetryTarget」は、`test` サービスが `RetryTargets` に一致してもリトライしないことの 1 ケースだけを `test_handler_test.go` に残す。
- `TestHealthcheckHandler` は 1 ケースにし、`Config.Version` に任意の値を設定して、その値が返ることを確認する。
- テストを対象のコードのファイルに対応するテストファイルに移す (`createSpeechHandler` は `handler_test.go`、`options.go` の関数は `options_test.go`)。`TestOpusPacketReader` は `options.go` の `optionSilentPacket` と `optionReadPacketWithHeader` のテストとして `options_test.go` に移す。`readDumpFile` は `handler_test.go` に移す。
- `amazon_transcribe_v2_handler_test.go` の `TestBuildMessageV2` の 5 組のテーブルとループを 1 組にまとめ、サブテスト名で条件を区別する。入力と期待値は変えない。
- ケース名の重複、末尾の空白、誤記、効いていない設定や引数を直す。`test_handler_test.go` の「silent packet」の `NewServer(&config, "aws")` は削除し、既存の `s` を使う。
- `handler_test.go` の `ErrReadCloser` と `NewErrReadCloser` を削除し、`io.NopCloser(iotest.ErrReader(err))` を使う。
- 移動の前後でテストの検証内容は変えない。

## 完了条件

- 同じことを確認しているテストがない。
- テストが対象のコードのファイルに対応する場所にあり、`readDumpFile` は `handler_test.go` にある。
- `TestBuildMessageV2` のテーブルとループが 1 組になっている。
- `TestHealthcheckHandler` が 1 ケースで、`Config.Version` に設定した値が返ることを確認している。
- `TestHandler.IsRetryTarget` のテストが `test_handler_test.go` に 1 ケースだけある。
- `ErrReadCloser` と `NewErrReadCloser` が残っていない。
- ケース名が一意で、誤記がない。
- 現状に挙げた効いていない設定と引数が削除されている。
- `make test` が通る。

## 関連 issue

- `0011-bug-fix-duplicate-type-error-result.md`、`0033-test-add-speech-handler-error-path-tests.md`、`0035-test-fix-test-failure-messages.md`、`0036-test-remove-connect-error-stub.md`、
  `0037-test-fix-nondeterministic-tests.md`、`0046-refactor-add-comments-to-old-modules.md`、`0047-refactor-resolve-stale-todos.md` は `TestSpeechHandler` のファイルパスを参照している。
  本 issue の完了後は `handler_test.go` になるため、テストはファイルパスではなくシンボル名で探す。
- `0035-test-fix-test-failure-messages.md`、`0037-test-fix-nondeterministic-tests.md`、`0049-remove-dead-code.md` は `readDumpFile` のファイルパスを参照している。本 issue の完了後は `handler_test.go` になるため、シンボル名で探す。
- `0039-test-add-fuzz-tests.md` と `0043-refactor-options-packet-reader.md` は `handler_test.go` の `TestReadPacketWithHeader` を参照している。本 issue の完了後は `options_test.go` になる。
- `0049-remove-dead-code.md` で `Config.Version` を削除した場合は、`TestHealthcheckHandler` はパッケージの `Version` を設定して確認する。
