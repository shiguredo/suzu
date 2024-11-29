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
	c := Config{
		AudioStreamingHeader: false,
	}

	t.Run("success", func(t *testing.T) {
		d := time.Duration(3000) * time.Millisecond
		r := readDumpFile(t, "testdata/000.jsonl", 0)
		defer r.Close()

		reader := NewOpusReader(c, d, r)

		for {
			buf := make([]byte, FrameSize)
			n, err := reader.Read(buf)
			if err != nil {
				assert.ErrorIs(t, err, io.EOF)
				break
			}
			assert.Equal(t, []byte{0, 0, 0}, buf[:n])
		}
	})

	t.Run("read error", func(t *testing.T) {
		d := time.Duration(3000) * time.Millisecond
		errPacketRead := errors.New("packet read error")

		r := NewErrReadCloser(errPacketRead)

		reader := NewOpusReader(c, d, &r)

		for {
			buf := make([]byte, FrameSize)
			n, err := reader.Read(buf)
			if err != nil {
				assert.ErrorIs(t, err, errPacketRead)
				break
			}
			assert.Equal(t, []byte{255, 255, 254}, buf[:n])
		}
	})

	t.Run("closed reader", func(t *testing.T) {
		d := time.Duration(3000) * time.Millisecond
		r := readDumpFile(t, "testdata/dump.jsonl", 0)
		r.Close()

		reader := NewOpusReader(c, d, r)

		for {
			buf := make([]byte, FrameSize)
			_, err := reader.Read(buf)
			if err != nil {
				assert.ErrorIs(t, io.ErrClosedPipe, err)
				break
			}
		}
	})

	t.Run("close reader", func(t *testing.T) {
		d := time.Duration(3000) * time.Millisecond
		r := readDumpFile(t, "testdata/dump.jsonl", 0)
		go func() {
			time.Sleep(3000 * time.Millisecond)
			r.Close()
		}()

		reader := NewOpusReader(c, d, r)

		for {
			buf := make([]byte, FrameSize)
			_, err := reader.Read(buf)
			if err != nil {
				assert.ErrorIs(t, io.EOF, err)
				break
			}
		}
	})

}

func TestReadPacketWithHeader(t *testing.T) {
	testCaces := []struct {
		Name   string
		Data   [][]byte
		Expect [][]byte
	}{
		{
			Name: "success",
			Data: [][]byte{
				{
					0, 5, 236, 96, 167, 215, 194, 192,
					0, 0, 0, 0, 0, 0, 0, 0,
					0, 0, 0, 3,
					252, 255, 254,
				},
				{
					0, 5, 236, 96, 167, 215, 194, 193,
					0, 0, 0, 0, 0, 0, 0, 0,
					0, 0, 0, 3,
					252, 255, 255,
				},
			},
			Expect: [][]byte{
				{
					252, 255, 254,
				},
				{
					252, 255, 255,
				},
			},
		},
		{
			Name: "multiple data",
			Data: [][]byte{
				{
					0, 5, 236, 96, 167, 215, 194, 192,
					0, 0, 0, 0, 0, 0, 0, 0,
					0, 0, 0, 3,
					252, 255, 254,
					0, 5, 236, 96, 167, 215, 194, 193,
					0, 0, 0, 0, 0, 0, 0, 0,
					0, 0, 0, 3,
					252, 255, 255,
				},
			},
			Expect: [][]byte{
				{
					252, 255, 254,
				},
				{
					252, 255, 255,
				},
			},
		},
		{
			Name: "split data",
			Data: [][]byte{
				{
					0, 5, 236, 96, 167, 215, 194, 192,
					0, 0, 0, 0, 0, 0, 0, 0,
					0, 0, 0, 3,
				},
				{
					252, 255, 254,
				},
			},
			Expect: [][]byte{
				{
					252, 255, 254,
				},
			},
		},
		{
			Name: "split data",
			Data: [][]byte{
				{
					0, 5, 236, 96, 167, 215, 194, 192,
					0, 0, 0, 0, 0, 0, 0, 0,
				},
				{
					0, 0, 0, 3,
					252, 255, 254,
				},
			},
			Expect: [][]byte{
				{
					252, 255, 254,
				},
			},
		},
		{
			Name: "split data",
			Data: [][]byte{
				{
					0, 5, 236, 96, 167, 215, 194, 192,
				},
				{
					0, 0, 0, 0, 0, 0, 0, 0,
					0, 0, 0, 3,
					252, 255, 254,
				},
			},
			Expect: [][]byte{
				{
					252, 255, 254,
				},
			},
		},
		{
			Name: "split data",
			Data: [][]byte{
				{
					0, 5, 236, 96, 167, 215, 194, 192,
					0, 0, 0, 0, 0, 0, 0, 0,
				},
				{
					0, 0, 0, 3,
					252, 255, 254,
					0, 5, 236, 96, 167, 215, 194, 193,
					0, 0, 0, 0, 0, 0, 0, 0,
					0, 0, 0, 3,
					252, 255, 255,
				},
			},
			Expect: [][]byte{
				{
					252, 255, 254,
				},
				{
					252, 255, 255,
				},
			},
		},
		{
			Name: "split data",
			Data: [][]byte{
				{
					0, 5, 236, 96, 167, 215, 194, 192,
					0, 0, 0, 0, 0, 0, 0, 0,
					0, 0, 0, 3,
					252, 255, 254,
					0, 5, 236, 96, 167, 215, 194, 193,
					0, 0, 0, 0, 0, 0, 0, 0,
					0, 0, 0, 3,
				},
				{
					252, 255, 255,
				},
			},
			Expect: [][]byte{
				{
					252, 255, 254,
				},
				{
					252, 255, 255,
				},
			},
		},
	}

	for _, tc := range testCaces {
		t.Run(tc.Name, func(t *testing.T) {
			reader, writer := io.Pipe()
			defer reader.Close()

			go func() {
				defer writer.Close()
				for _, data := range tc.Data {
					_, err := writer.Write(data)
					if err != nil {
						if assert.ErrorIs(t, err, io.EOF) {
							break
						}
						t.Error(t, err)
						return
					}
				}
			}()

			r := readPacketWithHeader(reader)

			i := 0
			for {
				buf := make([]byte, HeaderLength+MaxPayloadLength)
				n, err := r.Read(buf)
				if err != nil {
					if assert.ErrorIs(t, err, io.EOF) {
						break
					}
					t.Error(t, err)
					return
				}

				assert.Equal(t, tc.Expect[i], buf[:n])

				i += 1
			}

		})
	}
}
