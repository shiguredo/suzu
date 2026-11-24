# テストの失敗時のメッセージを日本語にし、書式の誤りを直す

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/fix-test-failure-messages
- Polished: 2026-11-25

## 目的

AGENTS.md の「テストのログメッセージは全て日本語にすること」に従い、テストが失敗したときに原因の値が正しく表示されるようにする。

## 現状

- 次のテストのメッセージが英語になっている。
  - `handler_test.go` の `TestOpusPacketReader` (`should not receive packet: %v`) と `TestReadPacketWithHeader` (`should not receive error: %v`)
  - `test_handler_test.go` の `TestSpeechHandler` (`READ-ERROR: %v\n`)
  - `options_test.go` の `TestPacketReaderOptionsOrder_SilentAfterHeader` (`unexpected error: %v`、`expected silent packet without header, got length: %d`、`expected silent packet, got payload: %v`、`expected silent packet before fragments stop`) と `TestOptionReadPacketWithHeader_PayloadTooLarge` (`expected error, got closed channel`、`expected error, got nil`)
  - `handler_test.go` の `parseOggPagesForTest` が返すエラー (`invalid ogg page header length: %d`、`invalid ogg page signature at offset: %d`、`invalid ogg segment table length: %d`、`invalid ogg payload length: %d`)。`assert.NoError` で失敗したときに表示される。
- `handler_test.go` の `assert.Fail(t, "should not receive packet: %v", opus)`、`assert.Failf(t, "should not receive packet: %v", string(opus.Payload))`、`assert.Fail(t, "should not receive error: %v", packet.Err)` は、testify v1.12.1 の `Fail` / `Failf` が第 2 引数の書式を展開しないため、`%v` がリテラルのまま表示される。値は `Messages` に別途表示されるが、メッセージ内には埋め込まれない。値をメッセージに埋め込むには `fmt.Sprintf` を使うか、`t.Errorf` を使う必要がある。
- `handler_test.go` の `TestReceiveFirstAudioData` の goroutine の中で呼ばれている `t.Error(t, err)` は、`*testing.T` そのものを表示する。
- `test_handler_test.go` の `readDumpFile` は、書き込みのエラーを `fmt.Println(err.Error())` で標準出力に出しており、テストのログ (`t.Log`) を使っていない。`t.Log` の呼び出しはコメントアウトされている (`//t.Log(err.Error())`)。この書き込みエラーは、`r.Close()` による停止で発生する `io.ErrClosedPipe` で、正常な終了でも必ず出る (`io.Pipe` の `Write` は、読み込み側が閉じられると `io.ErrClosedPipe` を返す)。`readDumpFile` の goroutine はテストの終了後にも動くため、`t.Log` を呼ぶと panic する可能性がある。

## 設計方針

- 対象は、テストコードが出力するメッセージ (`t.Log` / `t.Error` / `t.Fatal` / `assert.Fail` に渡す文言と、テスト用ヘルパーが返す診断用のエラー) とする。テスト対象に渡す入力のエラー値、テスト対象や標準ライブラリが返すエラーの文言、`t.Run` のテスト名は対象外とする。
- テストのメッセージをすべて日本語にする。
- 値をメッセージに埋め込む書式 (`%v` など) は、`fmt.Sprintf` で組み立てて `assert.Fail` に渡すか、`t.Errorf` を使う。`assert.Fail` / `assert.Failf` の第 2 引数は書式を展開しないため、`%v` をそのまま渡さない。
- `t.Error(t, err)` は `t.Errorf("...: %v", err)` にする。
- `t.Logf` は自動で改行するため、メッセージ末尾の `\n` は削除する。
- `readDumpFile` の `fmt.Println` は `t.Logf` にする。ただし、`r.Close()` による正常な停止で発生する `io.ErrClosedPipe` は `errors.Is(err, io.ErrClosedPipe)` で判定して記録しない。それ以外の書き込みエラーだけを `t.Logf("...: %v", err)` で記録する。

## 完了条件

- `make test` が通る。
- 設計方針で対象としたメッセージが日本語になり、`assert.Fail` / `assert.Failf` の第 2 引数にリテラルの `%v` が残らない。
- `t.Logf` のメッセージ末尾に不要な `\n` が残らない。
- `readDumpFile` が書き込みエラーを標準出力に直接出さず、`io.ErrClosedPipe` 以外の書き込みエラーを `t.Logf` で記録する。
- `TestReceiveFirstAudioData` の `t.Error` が `*testing.T` ではなくエラーを表示する。

## 関連 issue

- `0037-test-fix-nondeterministic-tests.md` は、テストの goroutine の中で呼ぶ `t.Error` がテストの終了後に panic する問題と、`TestPacketReaderOptionsOrder_SilentAfterHeader` の実時間依存を扱う。本 issue では `readDumpFile` の `t.Log` が同じ panic を起こさないように `io.ErrClosedPipe` を記録しない範囲に限定し、goroutine の全面的な見直しとテストの決定的化は 0037 に任せる。
- `0038-test-cleanup-duplicate-and-misplaced-tests.md` は、`TestOpusPacketReader`、`TestReadPacketWithHeader`、`TestSpeechHandler`、`readDumpFile` の置き場所を変える。本 issue のメッセージの修正は、移動の前後どちらでも必要になる。
