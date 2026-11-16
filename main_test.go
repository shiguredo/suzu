package suzu

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
)

// TestMain はテストを実行する前に、グローバルロガーの出力先を testLogOutput に設定する
// ハンドラーの終了後も動き続ける goroutine がグローバルロガーを読むため、テストの途中で zlog.Logger に代入するとデータ競合になる
// そのため zlog.Logger への代入は、他の goroutine が動く前のここだけで行う
// ログを検証するテストは captureLog で出力先だけを差し替える
func TestMain(m *testing.M) {
	// zerolog の既定のロガーと同じく、標準エラー出力にタイムスタンプ付きで出力する
	zlog.Logger = zerolog.New(testLogOutput).With().Timestamp().Logger()
	m.Run()
}

// testLogOutput はテスト中のグローバルロガーの出力先
var testLogOutput = &swappableWriter{w: os.Stderr}

// swappableWriter は出力先を後から差し替えられる io.Writer
type swappableWriter struct {
	mu sync.Mutex
	w  io.Writer
}

// Write は現在の出力先に書き込む
func (s *swappableWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// swap は出力先を w に差し替え、差し替える前の出力先を返す
func (s *swappableWriter) swap(w io.Writer) io.Writer {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.w
	s.w = w
	return prev
}

// logBuffer はテストで出力されたログを保持する
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write はログをバッファに追記する
// 他のテストが残した goroutine のログを書き込んでいる最中に読む場合があるため、排他する
func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// String はこれまでに出力されたログをそのまま返す
func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// captureLog はテストの間だけグローバルロガーの出力先を logBuffer に差し替え、テストの終了時に元に戻す
// 他のテストが残した goroutine のログが混ざる場合があるため、検証は findLogs でメッセージを絞り込むか、内容で行うこと
// 出力先はパッケージで 1 つのため、t.Parallel を使うテストでは使わないこと
func captureLog(t *testing.T) *logBuffer {
	t.Helper()

	b := &logBuffer{}
	prev := testLogOutput.swap(b)
	t.Cleanup(func() {
		testLogOutput.swap(prev)
	})
	return b
}

// logEntries は captureLog で取得したログを 1 行ずつ JSON として解釈して返す
func logEntries(t *testing.T, logs *logBuffer) []map[string]any {
	t.Helper()

	var entries []map[string]any
	for line := range strings.SplitSeq(logs.String(), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("ログを JSON として解釈できなかった: %s: %v", line, err)
		}
		entries = append(entries, entry)
	}
	return entries
}

// findLogs は message が一致するログだけを返す
// アクセスログの message は HTTP メソッド
func findLogs(entries []map[string]any, message string) []map[string]any {
	var found []map[string]any
	for _, entry := range entries {
		if entry["message"] == message {
			found = append(found, entry)
		}
	}
	return found
}
