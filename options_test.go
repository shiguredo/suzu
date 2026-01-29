package suzu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPacketReaderOptions(t *testing.T) {
	t.Run("DisableSilentPacket: true, AudioStreamingHeader: false", func(t *testing.T) {
		c := Config{DisableSilentPacket: true, AudioStreamingHeader: false}
		options := NewPacketReaderOptions(c)
		assert.Equal(t, 0, len(options))
	})
	t.Run("DisableSilentPacket = false, AudioStreamingHeader: false", func(t *testing.T) {
		c := Config{DisableSilentPacket: false, AudioStreamingHeader: false}
		options := NewPacketReaderOptions(c)
		assert.Equal(t, 1, len(options))
	})
	t.Run("DisableSilentPacket: true, AudioStreamingHeader: true", func(t *testing.T) {
		c := Config{DisableSilentPacket: true, AudioStreamingHeader: true}
		options := NewPacketReaderOptions(c)
		assert.Equal(t, 1, len(options))
	})
	t.Run("DisableSilentPacket: false, AudioStreamingHeader: true", func(t *testing.T) {
		c := Config{DisableSilentPacket: false, AudioStreamingHeader: true}
		options := NewPacketReaderOptions(c)
		assert.Equal(t, 2, len(options))
	})
}
