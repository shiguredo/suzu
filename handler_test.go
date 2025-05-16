package suzu

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

		t.Run("silent packet", func(t *testing.T) {
			d := time.Duration(500) * time.Millisecond
			r := readDumpFile(t, "testdata/000.jsonl", 3000*time.Millisecond)
			defer r.Close()

			reader := NewOpusReader(c, d, r)

			count := 0
			for {
				buf := make([]byte, FrameSize)
				n, err := reader.Read(buf)
				if err != nil {
					assert.ErrorIs(t, err, io.EOF)
					break
				}

				if count < 5 {
					// パケットを受信までは silent packet は 5 回分
					assert.Equal(t, []byte{252, 255, 254}, buf[:n])
				} else {
					// パケットを受信
					assert.Equal(t, []byte{0, 0, 0}, buf[:n])
					break
				}

				count += 1
			}
			assert.Equal(t, 5, count)
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
	})

	t.Run("disable silent packet", func(t *testing.T) {
		c := Config{
			AudioStreamingHeader: false,
			DisableSilentPacket:  true,
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

		t.Run("no silent packet", func(t *testing.T) {
			d := time.Duration(100) * time.Millisecond
			r := readDumpFile(t, "testdata/000.jsonl", 1000*time.Millisecond)
			defer r.Close()

			reader := NewOpusReader(c, d, r)

			count := 0
			for {
				buf := make([]byte, FrameSize)
				n, err := reader.Read(buf)
				if err != nil {
					assert.ErrorIs(t, err, io.EOF)
					break
				}
				// silent packet を無効にしているので、silent packet は来ない
				assert.Equal(t, []byte{0, 0, 0}, buf[:n])

				count += 1
			}
			// testdata/000.jsonl は 9 パケット分
			assert.Equal(t, 9, count)
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

func TestOggFileWriting(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		oggDir, err := os.MkdirTemp("", "ogg-")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(oggDir)

		c := Config{
			EnableOggFileOutput: true,
			OggDir:              oggDir,
		}

		header := soraHeader{
			SoraChannelID:    "ogg-test",
			SoraSessionID:    "C2TFB1QBDS4WD5SX317SWMJ6FM",
			SoraConnectionID: "1X0Z8JXZAD5A93X68M2S9NTC4G",
		}

		opusCh := make(chan opusChannel)
		defer close(opusCh)

		sampleRate := uint32(48000)
		channelCount := uint16(1)

		ctx := context.Background()
		reader, err := opus2ogg(ctx, opusCh, sampleRate, channelCount, c, header)
		if assert.NoError(t, err) {
			assert.NotNil(t, reader)
		}
		defer reader.Close()

		// ファイルへの書き込み待ち
		time.Sleep(100 * time.Millisecond)

		filename := fmt.Sprintf("%s-%s.ogg", header.SoraSessionID, header.SoraConnectionID)
		filePath := filepath.Join(oggDir, filename)
		_, err = os.Stat(filePath)
		assert.NoError(t, err)

		// Ogg ファイルのヘッダーを確認
		f, err := os.Open(filePath)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		buf := make([]byte, 4)
		n, err := f.Read(buf)
		assert.NoError(t, err)
		assert.Equal(t, []byte(`OggS`), buf[:n])
	})

	t.Run("disable_ogg_file_output", func(t *testing.T) {
		oggDir, err := os.MkdirTemp("", "ogg-")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(oggDir)

		c := Config{
			EnableOggFileOutput: false,
			OggDir:              oggDir,
		}

		header := soraHeader{
			SoraChannelID:    "ogg-test",
			SoraSessionID:    "C2TFB1QBDS4WD5SX317SWMJ6FM",
			SoraConnectionID: "1X0Z8JXZAD5A93X68M2S9NTC4G",
		}

		opusCh := make(chan opusChannel)
		defer close(opusCh)

		sampleRate := uint32(48000)
		channelCount := uint16(1)

		ctx := context.Background()
		reader, err := opus2ogg(ctx, opusCh, sampleRate, channelCount, c, header)
		assert.NoError(t, err)
		assert.NotNil(t, reader)
		defer reader.Close()

		filename := fmt.Sprintf("%s-%s.ogg", header.SoraSessionID, header.SoraConnectionID)
		filePath := filepath.Join(oggDir, filename)
		_, err = os.Stat(filePath)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("no permission", func(t *testing.T) {
		oggDir, err := os.MkdirTemp("", "ogg-")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(oggDir)

		// 書き込み権限を剥奪
		if err := os.Chmod(oggDir, 0000); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.Chmod(oggDir, 0700); err != nil {
				t.Fatal(err)
			}
		}()

		c := Config{
			EnableOggFileOutput: true,
			OggDir:              oggDir,
		}

		header := soraHeader{
			SoraChannelID:    "ogg-test",
			SoraSessionID:    "C2TFB1QBDS4WD5SX317SWMJ6FM",
			SoraConnectionID: "1X0Z8JXZAD5A93X68M2S9NTC4G",
		}

		opusCh := make(chan opusChannel)
		defer close(opusCh)

		sampleRate := uint32(48000)
		channelCount := uint16(1)

		ctx := context.Background()
		reader, err := opus2ogg(ctx, opusCh, sampleRate, channelCount, c, header)
		assert.ErrorIs(t, err, os.ErrPermission)
		assert.Nil(t, reader)
	})

	t.Run("directory does not exist", func(t *testing.T) {
		oggDir, err := os.MkdirTemp("", "ogg-")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(oggDir)

		c := Config{
			EnableOggFileOutput: true,
			// 既存のディレクトリ名に 0 を付与して存在しないディレクトリを指定する
			OggDir: oggDir + "0",
		}

		header := soraHeader{
			SoraChannelID:    "ogg-test",
			SoraSessionID:    "C2TFB1QBDS4WD5SX317SWMJ6FM",
			SoraConnectionID: "1X0Z8JXZAD5A93X68M2S9NTC4G",
		}

		opusCh := make(chan opusChannel)
		defer close(opusCh)

		sampleRate := uint32(48000)
		channelCount := uint16(1)

		ctx := context.Background()
		reader, err := opus2ogg(ctx, opusCh, sampleRate, channelCount, c, header)
		assert.ErrorIs(t, err, os.ErrNotExist)
		assert.Nil(t, reader)
	})
}
