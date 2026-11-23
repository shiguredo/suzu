# 実時間と実行環境に依存するテストを決定的にする

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-nondeterministic-tests
- Polished: {YYYY-MM-DD}

## 目的

CI の負荷や開発者の環境によって結果が変わりうるテストを、決定的に結果が決まる形にする。shiguredo-test の「フレーキーテストを見逃さないこと」に従い、フレーキーになる前に対処する。

## 現状

- `handler_test.go` の `TestOpusPacketReader` の「silent packet」と「no silent packet」は、`readDumpFile` で 1100 ms と 1000 ms の待ちを入れ、その間に無音パケットが何回届くかを数えている。
  - 「silent packet」は無音パケットの間隔が 500 ms で、2 回目の無音パケット (1000 ms) とデータ (1100 ms) の差は 100 ms しかない。
  - 2 つのテストで 18.9 秒かかり、パッケージ全体 (約 22 秒) の大半を占める。
- `handler_test.go` の `TestOggFileWriting` の「success」は、`time.Sleep(100 * time.Millisecond)` でファイルへの書き込みを待つ。間に合わないと `data[:4]` の範囲外参照で panic し、アサーションの失敗ではなくテストのバイナリごと終了する。
- `options_test.go` の `TestPacketReaderOptionsOrder_SilentAfterHeader` は、20 ms の無音パケットのタイマーが 50 ms 以内に届く前提で書かれている。送信側の goroutine は、テストの終了後も `in` への送信で止まる場合がある。
- `test_handler_test.go` の `TestSpeechHandler` の「silent packet」は、150 ms の送信の間隔と 100 ms のタイマーの関係に依存している。また `testdata/dump.jsonl` のペイロードは `silentPacket` と同じバイト列のため、無音パケットが挿入されたかどうかを区別できない。
- テストの goroutine の中で呼ぶ `t.Error` (`handler_test.go` の `TestOggFileWriting`、`test_handler_test.go` の `readDumpFile`) は、テストの終了後に呼ばれると panic する。
- `amazon_transcribe_v2_test.go` の `TestNewAmazonTranscribeClientV2_WithoutProfile_AppliesHTTPClientAndLogMode` は、`HOME` や `AWS_PROFILE` を差し替えていないため、`config.LoadDefaultConfig` が開発者の `~/.aws/config` と環境変数を読む。

## 設計方針

- 待ち時間で順序を作る代わりに、チャネルや `assert.Eventually` で状態の変化を待つ。
- 無音パケットの挿入を確認するテストは、無音パケットと区別できるペイロード (`testdata/000.jsonl` など) を使う。
- goroutine の中では `t.Error` を呼ばず、結果をチャネルでテストの goroutine に返す。goroutine はテストの終了までに必ず終了させる。
- AWS の設定を読むテストは、`t.Setenv` で `HOME`、`AWS_CONFIG_FILE`、`AWS_SHARED_CREDENTIALS_FILE`、`AWS_PROFILE` を `t.TempDir()` などに向けて環境から切り離す。
- テストのためにコードを変える必要が出た場合は、変更の前にユーザーに確認する。

## 完了条件

- 上記のテストが、固定の待ち時間に依存せずに通る。
- `go test -race -count=20` で上記のテストが失敗しない。
- 上記のテストの結果が、開発者の AWS の設定の有無で変わらない。
