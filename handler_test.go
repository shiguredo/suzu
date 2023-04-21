package suzu

import (
	"context"
	"errors"
	"io"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSilentPacketReader(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		d := time.Duration(100) * time.Millisecond
		r := readDumpFile(t, "testdata/dump.jsonl", 0)
		defer r.Close()

		reader, err := readerWithSilentPacketFromOpusReader(ctx, d, r)
		assert.NoError(t, err)

		for {
			buf := make([]byte, FrameSize)
			n, err := reader.Read(buf)
			if err != nil {
				assert.ErrorIs(t, err, io.EOF)
				break
			}
			// TODO: silent packet と読み込んだパケットを見分けるためにテストデータを変更して、期待値も変更する
			assert.Equal(t, buf[:n], []byte{252, 255, 254})
		}
	})

	t.Run("read error", func(t *testing.T) {
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		d := time.Duration(100) * time.Millisecond
		errPacketRead := errors.New("packet read error")
		r := iotest.ErrReader(errPacketRead)

		reader, err := readerWithSilentPacketFromOpusReader(ctx, d, r)
		assert.NoError(t, err)

		for {
			buf := make([]byte, FrameSize)
			n, err := reader.Read(buf)
			if err != nil {
				assert.ErrorIs(t, err, errPacketRead)
				break
			}
			assert.Equal(t, buf[:n], []byte{255, 255, 254})
		}
	})

	//t.Run("closed reader", func(t *testing.T) {
	//	ctx := context.Background()
	//	ctx, cancel := context.WithCancelCause(ctx)
	//	defer cancel(nil)
	//	d := time.Duration(100) * time.Millisecond
	//	r := readDumpFile(t, "testdata/dump.jsonl", 0)
	//	go func() {
	//		r.Close()
	//	}()

	//	reader, err := readerWithSilentPacketFromOpusReader(ctx, d, r)
	//	if assert.NoError(t, err) {
	//		cancel(err)
	//	}

	//	for {
	//		buf := make([]byte, FrameSize)
	//		_, err := reader.Read(buf)
	//		if err != nil {
	//			assert.ErrorIs(t, err, io.ErrClosedPipe)
	//			break
	//		}
	//	}
	//})

}
