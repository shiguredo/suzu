package suzu

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Version は VERSION ファイルを埋め込んだ値から作っている
// VERSION ファイルは末尾に改行を含むため、取り除かずに使うと
// ヘルスチェックのレスポンスや -V の出力に改行が混ざる
func TestVersion(t *testing.T) {
	// 空の場合は VERSION ファイルの埋め込みに失敗している
	assert.NotEmpty(t, Version, "バージョンが空です")

	// 前後に空白や改行が残っていないことを確認する
	assert.Equal(t, strings.TrimSpace(Version), Version, "バージョンの前後に空白や改行が含まれています")
}
