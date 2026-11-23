# テストの失敗時のメッセージを日本語にし、書式の誤りを直す

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-test-failure-messages
- Polished: {YYYY-MM-DD}

## 目的

AGENTS.md の「テストのログメッセージは全て日本語にすること」に従い、テストが失敗したときに原因の値が正しく表示されるようにする。

## 現状

- 次のテストのメッセージが英語になっている。
  - `handler_test.go` の `TestOpusPacketReader` と `TestReadPacketWithHeader` (`should not receive packet: %v`、`should not receive error: %v` など)
  - `test_handler_test.go` の `TestSpeechHandler` (`READ-ERROR: %v\n`)
  - `options_test.go` の `TestPacketReaderOptionsOrder_SilentAfterHeader` と `TestOptionReadPacketWithHeader_PayloadTooLarge`
- `handler_test.go` の `assert.Fail(t, "...: %v", x)` は、testify の `Fail` が書式を展開しないため、`%v` がそのまま表示され、値が表示されない。`assert.Failf` を使う必要がある。
- `handler_test.go` の `TestReceiveFirstAudioData` の `t.Error(t, err)` は、`*testing.T` そのものを表示する。
- `test_handler_test.go` の `readDumpFile` は、書き込みのエラーを `fmt.Println` で標準出力に出しており、テストのログ (`t.Log`) を使っていない。

## 設計方針

- テストのメッセージをすべて日本語にする。
- 書式を使うメッセージは `assert.Failf` / `t.Errorf` を使う。
- `fmt.Println` は `t.Log` にする。

## 完了条件

- テストのメッセージがすべて日本語で、失敗時に原因の値が表示される。
