package suzu

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPacketReaderOptions(t *testing.T) {
	t.Run("DisableSilentPacket: true, AudioStreamingHeader: false", func(t *testing.T) {
		c := Config{DisableSilentPacket: true, AudioStreamingHeader: false}
		options := newPacketReaderOptions(c)
		assert.Equal(t, 0, len(options))
	})
	t.Run("DisableSilentPacket = false, AudioStreamingHeader: false", func(t *testing.T) {
		c := Config{DisableSilentPacket: false, AudioStreamingHeader: false}
		options := newPacketReaderOptions(c)
		assert.Equal(t, 1, len(options))
	})
	t.Run("DisableSilentPacket: true, AudioStreamingHeader: true", func(t *testing.T) {
		c := Config{DisableSilentPacket: true, AudioStreamingHeader: true}
		options := newPacketReaderOptions(c)
		assert.Equal(t, 1, len(options))
	})
	t.Run("DisableSilentPacket: false, AudioStreamingHeader: true", func(t *testing.T) {
		c := Config{DisableSilentPacket: false, AudioStreamingHeader: true}
		options := newPacketReaderOptions(c)
		assert.Equal(t, 2, len(options))
	})
}

func TestPacketReaderOptionsOrder_SilentAfterHeader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := Config{
		AudioStreamingHeader:      true,
		DisableSilentPacket:       false,
		TimeToWaitForOpusPacketMs: 20,
	}

	in := make(chan opus)
	out := in
	for _, opt := range newPacketReaderOptions(c) {
		out = opt(ctx, c, out)
	}

	go func() {
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		deadline := time.After(60 * time.Millisecond)
		for {
			select {
			case <-deadline:
				close(in)
				return
			case <-ticker.C:
				in <- opus{Payload: []byte{0x01}}
			}
		}
	}()

	select {
	case got := <-out:
		fmt.Printf("got: %+v\n", got)
		if got.Err != nil {
			t.Fatalf("unexpected error: %v", got.Err)
		}
		if len(got.Payload) != 3 {
			t.Fatalf("expected silent packet without header, got length: %d", len(got.Payload))
		}
		if got.Payload[0] != 252 || got.Payload[1] != 255 || got.Payload[2] != 254 {
			t.Fatalf("expected silent packet, got payload: %v", got.Payload)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("expected silent packet before fragments stop")
	}
}
