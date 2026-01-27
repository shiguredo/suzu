package suzu

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Read 時にエラーを返す ReadCloser
type ErrReadCloser struct {
	Error error
}

func NewErrReadCloser(err error) ErrReadCloser {
	return ErrReadCloser{
		Error: err,
	}
}

func (e *ErrReadCloser) Read(p []byte) (n int, err error) {
	return 0, e.Error
}

func (e *ErrReadCloser) Close() error {
	return nil
}

func TestOpusPacketReader(t *testing.T) {
	t.Run("enable silent packet", func(t *testing.T) {
		c := Config{}

		t.Run("success", func(t *testing.T) {
			c.TimeToWaitForOpusPacketMs = 3000
			r := readDumpFile(t, "testdata/000.jsonl", 0)
			defer r.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			middlewareFuncs := []middlewareFunc{
				middlewareSilentPacket,
			}
			opusCh := newOpusChannel(ctx, c, r, middlewareFuncs)

			for {
				select {
				case <-ctx.Done():
					return
				case opus := <-opusCh:
					switch m := opus.(type) {
					case error:
						assert.ErrorIs(t, m, io.EOF)
						return
					case []byte:
						assert.Equal(t, []byte{0, 0, 0}, m)
					}
				}
			}
		})

		t.Run("silent packet", func(t *testing.T) {
			// 000.jsonl による 1 パケットを読み込む前に、2 回 silent packet を channel で受信する想定
			c.TimeToWaitForOpusPacketMs = 500
			// 1 パケットを読み込むまでに 1100 ms かかるように設定して、silent packet が 2 回送信されることを確認する
			r := readDumpFile(t, "testdata/000.jsonl", 1100*time.Millisecond)
			defer r.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			middlewareFuncs := []middlewareFunc{
				middlewareSilentPacket,
			}
			opusCh := newOpusChannel(ctx, c, r, middlewareFuncs)

			count := 0
		L:
			for {
				select {
				case <-ctx.Done():
					break L
				case opus := <-opusCh:
					switch m := opus.(type) {
					case error:
						assert.ErrorIs(t, m, io.EOF)
						break L
					case []byte:
						if count%3 == 2 {
							// 3 回中 1 回は 000.jsonl のパケットを受信する
							assert.Equal(t, []byte{0, 0, 0}, m)
						} else {
							// silent packet
							assert.Equal(t, []byte{252, 255, 254}, m)
						}
						count += 1
					}
				}
			}
			// パケット数の確認（受信パケット: 1 + silent packet: 2 * 000.jsonl の行数: 9 = 27）
			assert.Equal(t, 27, count)
		})

		t.Run("read error", func(t *testing.T) {
			c.TimeToWaitForOpusPacketMs = 500
			errPacketRead := errors.New("packet read error")
			r := NewErrReadCloser(errPacketRead)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			middlewareFuncs := []middlewareFunc{
				middlewareSilentPacket,
			}
			opusCh := newOpusChannel(ctx, c, &r, middlewareFuncs)

		L:
			for {
				select {
				case <-ctx.Done():
					break L
				case opus := <-opusCh:
					switch m := opus.(type) {
					case error:
						assert.ErrorIs(t, m, errPacketRead)
						break L
					case []byte:
						assert.Fail(t, "should not receive packet: %v", m)
					}
				}
			}
		})

		t.Run("closed reader", func(t *testing.T) {
			c.TimeToWaitForOpusPacketMs = 500
			r := readDumpFile(t, "testdata/000.jsonl", 0)
			// すでに閉じている場合の動作を確認する
			r.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			middlewareFuncs := []middlewareFunc{
				middlewareSilentPacket,
			}
			opusCh := newOpusChannel(ctx, c, r, middlewareFuncs)

			count := 0
		L:
			for {
				select {
				case <-ctx.Done():
					break L
				case opus := <-opusCh:
					switch m := opus.(type) {
					case error:
						assert.ErrorIs(t, m, io.ErrClosedPipe)
						break L
					case []byte:
						count += 1
					}
				}
			}
			assert.Equal(t, 0, count)
		})
	})

	t.Run("disable silent packet", func(t *testing.T) {
		c := Config{}
		//
		t.Run("success", func(t *testing.T) {
			c.TimeToWaitForOpusPacketMs = 3000
			r := readDumpFile(t, "testdata/000.jsonl", 0)
			defer r.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// silent packet 無効化
			middlewareFuncs := []middlewareFunc{}
			opusCh := newOpusChannel(ctx, c, r, middlewareFuncs)

			for {
				select {
				case <-ctx.Done():
					return
				case opus := <-opusCh:
					switch m := opus.(type) {
					case error:
						assert.ErrorIs(t, m, io.EOF)
						return
					case []byte:
						assert.Equal(t, []byte{0, 0, 0}, m)
					}
				}
			}
		})

		t.Run("no silent packet", func(t *testing.T) {
			c.TimeToWaitForOpusPacketMs = 100
			r := readDumpFile(t, "testdata/000.jsonl", 1000*time.Millisecond)
			defer r.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// silent packet 無効化
			middlewareFuncs := []middlewareFunc{}
			opusCh := newOpusChannel(ctx, c, r, middlewareFuncs)

			count := 0
		L:
			for {
				select {
				case <-ctx.Done():
					break L
				case opus := <-opusCh:
					switch m := opus.(type) {
					case error:
						assert.ErrorIs(t, m, io.EOF)
						break L
					case []byte:
						// silent packet を無効にしているので、silent packet は来ない
						assert.Equal(t, []byte{0, 0, 0}, m)
					}

					count += 1
				}
			}

			// testdata/000.jsonl は 9 パケット分
			assert.Equal(t, 9, count)
		})

		t.Run("read error", func(t *testing.T) {
			c.TimeToWaitForOpusPacketMs = 500
			errPacketRead := errors.New("packet read error")
			r := NewErrReadCloser(errPacketRead)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			middlewareFuncs := []middlewareFunc{}
			opusCh := newOpusChannel(ctx, c, &r, middlewareFuncs)

		L:
			for {
				select {
				case <-ctx.Done():
					break L
				case opus := <-opusCh:
					switch m := opus.(type) {
					case error:
						assert.ErrorIs(t, m, errPacketRead)
						break L
					case []byte:
						assert.Fail(t, "should not receive packet: %v", m)
					}
				}
			}
		})

		t.Run("closed reader", func(t *testing.T) {
			c.TimeToWaitForOpusPacketMs = 500
			r := readDumpFile(t, "testdata/000.jsonl", 0)
			// すでに閉じている場合の動作を確認する
			r.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			middlewareFuncs := []middlewareFunc{}
			opusCh := newOpusChannel(ctx, c, r, middlewareFuncs)

		L:
			for {
				select {
				case <-ctx.Done():
					break L
				case opus := <-opusCh:
					switch m := opus.(type) {
					case error:
						assert.ErrorIs(t, m, io.ErrClosedPipe)
						break L
					case []byte:
						assert.Failf(t, "should not receive packet: %v", string(m))
					}
				}
			}
		})
	})
}
