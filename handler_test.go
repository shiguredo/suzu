package suzu

import (
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

	t.Run("success", func(t *testing.T) {
		d := time.Duration(100) * time.Millisecond
		r := readDumpFile(t, "testdata/000.jsonl", 0)
		defer r.Close()

		c := Config{
			AudioStreamingHeader: false,
		}
		reader := NewOpusReader(c, d, r)

		for {
			buf := make([]byte, FrameSize)
			n, err := reader.Read(buf)
			if err != nil {
				assert.ErrorIs(t, err, io.EOF)
				break
			}
			assert.Equal(t, buf[:n], []byte{0, 0, 0})
		}
	})

	t.Run("read error", func(t *testing.T) {
		d := time.Duration(100) * time.Millisecond
		errPacketRead := errors.New("packet read error")

		r := NewErrReadCloser(errPacketRead)

		c := Config{
			AudioStreamingHeader: false,
		}
		reader := NewOpusReader(c, d, &r)

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

	t.Run("closed reader", func(t *testing.T) {
		d := time.Duration(100) * time.Millisecond
		r := readDumpFile(t, "testdata/dump.jsonl", 0)
		r.Close()

		c := Config{
			AudioStreamingHeader: false,
		}
		reader := NewOpusReader(c, d, r)

		for {
			buf := make([]byte, FrameSize)
			_, err := reader.Read(buf)
			if err != nil {
				assert.ErrorIs(t, err, io.ErrClosedPipe)
				break
			}
		}
	})

	t.Run("close reader", func(t *testing.T) {
		d := time.Duration(100) * time.Millisecond
		r := readDumpFile(t, "testdata/dump.jsonl", 0)
		go func() {
			time.Sleep(100 * time.Millisecond)
			r.Close()
		}()

		c := Config{
			AudioStreamingHeader: false,
		}
		reader := NewOpusReader(c, d, r)

		for {
			buf := make([]byte, FrameSize)
			_, err := reader.Read(buf)
			if err != nil {
				assert.ErrorIs(t, err, io.EOF)
				break
			}
		}
	})

}
