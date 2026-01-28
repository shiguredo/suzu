package suzu

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

			packetReaderOptions := []packetReaderOption{
				optionSilentPacket,
			}
			opusCh := newOpusChannel(ctx, c, r, packetReaderOptions)

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

			packetReaderOptions := []packetReaderOption{
				optionSilentPacket,
			}
			opusCh := newOpusChannel(ctx, c, r, packetReaderOptions)

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

			packetReaderOptions := []packetReaderOption{
				optionSilentPacket,
			}
			opusCh := newOpusChannel(ctx, c, &r, packetReaderOptions)

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

			packetReaderOptions := []packetReaderOption{
				optionSilentPacket,
			}
			opusCh := newOpusChannel(ctx, c, r, packetReaderOptions)

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

		t.Run("success", func(t *testing.T) {
			c.TimeToWaitForOpusPacketMs = 3000
			r := readDumpFile(t, "testdata/000.jsonl", 0)
			defer r.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// silent packet 無効化
			packetReaderOptions := []packetReaderOption{}
			opusCh := newOpusChannel(ctx, c, r, packetReaderOptions)

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
			packetReaderOptions := []packetReaderOption{}
			opusCh := newOpusChannel(ctx, c, r, packetReaderOptions)

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

			packetReaderOptions := []packetReaderOption{}
			opusCh := newOpusChannel(ctx, c, &r, packetReaderOptions)

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

			packetReaderOptions := []packetReaderOption{}
			opusCh := newOpusChannel(ctx, c, r, packetReaderOptions)

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
	t.Run("enable audio streaming header", func(t *testing.T) {
		c := Config{}

		t.Run("success", func(t *testing.T) {
			r := readDumpFile(t, "testdata/header.jsonl", 0)
			defer r.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			packetReaderOptions := []packetReaderOption{
				optionReadPacketWithHeader,
			}
			opusCh := newOpusChannel(ctx, c, r, packetReaderOptions)

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
						assert.Equal(t, []byte{0xfc, 0xff, 0xfe}, m)
					}
				}
			}
		})

		t.Run("read error", func(t *testing.T) {
			errPacketRead := errors.New("packet read error")
			r := NewErrReadCloser(errPacketRead)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			packetReaderOptions := []packetReaderOption{
				optionReadPacketWithHeader,
			}
			opusCh := newOpusChannel(ctx, c, &r, packetReaderOptions)

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
			r := readDumpFile(t, "testdata/header.jsonl", 0)
			// すでに閉じている場合の動作を確認する
			r.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			packetReaderOptions := []packetReaderOption{
				optionReadPacketWithHeader,
			}
			opusCh := newOpusChannel(ctx, c, r, packetReaderOptions)

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
			ch := make(chan any)

			go func() {
				defer close(ch)

				for _, data := range tc.Data {
					ch <- data
				}
			}()

			c := Config{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			packetCh := optionReadPacketWithHeader(ctx, c, ch)

			i := 0
			for packet := range packetCh {
				switch p := packet.(type) {
				case error:
					assert.Fail(t, "should not receive error: %v", p)
				case []byte:
					assert.Equal(t, tc.Expect[i], p)
					i += 1
				}
			}

			assert.Equal(t, len(tc.Expect), i)
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

		opusCh := make(chan any)
		defer close(opusCh)

		// 音声データの受信をシミュレート
		go func() {
			opusCh <- []byte{0}
		}()

		sampleRate := uint32(48000)
		channelCount := uint16(1)

		ctx := context.Background()
		reader, err := opus2ogg(ctx, opusCh, sampleRate, channelCount, c, header)
		if assert.NoError(t, err) {
			assert.NotNil(t, reader)
		}
		defer reader.Close()

		// Ogg データは送信しないため、ここでは音声データの受信をシミュレートする
		go func() {
			for {
				buf := make([]byte, 1024)
				_, err := reader.Read(buf)
				if err != nil {
					if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
						return
					}
					t.Error(err)
					return
				}
			}
		}()

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

		var data []byte
		for {
			buf := make([]byte, 1024)
			n, err := f.Read(buf)
			if err != nil && err != io.EOF {
				t.Fatal(err)
			}
			if n == 0 {
				break
			}
			data = append(data, buf[:n]...)
		}
		// OggS で始まることの確認
		assert.Equal(t, []byte(`OggS`), data[:4])
		// OpusHeader, OpusTags が 1 つだけ含まれることの確認
		assert.Equal(t, 1, strings.Count(string(data), "OpusHead"))
		assert.Equal(t, 1, strings.Count(string(data), "OpusTags"))
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

		opusCh := make(chan any)
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

		opusCh := make(chan any)
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

		opusCh := make(chan any)
		defer close(opusCh)

		sampleRate := uint32(48000)
		channelCount := uint16(1)

		ctx := context.Background()
		reader, err := opus2ogg(ctx, opusCh, sampleRate, channelCount, c, header)
		assert.ErrorIs(t, err, os.ErrNotExist)
		assert.Nil(t, reader)
	})
}

func TestReceiveFirstAudioData(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// testData := []byte(`012`)
		testData := [][]byte{
			[]byte(`012`),
			[]byte(`345`),
			[]byte(`678`),
		}

		reader, writer := io.Pipe()
		defer reader.Close()

		go func() {
			defer writer.Close()
			for _, data := range testData {
				_, err := writer.Write(data)
				if err != nil {
					if assert.ErrorIs(t, err, io.EOF) {
						// EOF の場合は終了
						return
					}

					t.Error(t, err)
					return
				}
			}
		}()

		for _, data := range testData {
			audioData, err := receiveFirstAudioData(reader)
			if assert.NoError(t, err) {
				assert.NotNil(t, audioData)
				assert.Equal(t, data, audioData)
			}
		}
	})
}
