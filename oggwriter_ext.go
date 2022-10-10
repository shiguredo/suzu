package suzu

import "github.com/pion/rtp/codecs"

func (i *OggWriter) Write(opusPacket *codecs.OpusPacket) error {
	payload := opusPacket.Payload[0:]
	toc := payload[0]
	config := (toc & 0xf8) >> 3
	c := toc & 0x03

	var count uint64
	switch c {
	case 0:
		count = 1
	case 1, 2:
		count = 2
	case 3:
		m := payload[1] & 0x3f
		count = uint64(m)
	}

	// TODO: 値の決定に他の要素が必要ないか確認する
	granulePosition := GranulePosition(config)

	i.previousGranulePosition += count * granulePosition

	data := i.createPage(payload, pageHeaderTypeContinuationOfStream, i.previousGranulePosition, i.pageIndex)
	i.pageIndex++
	return i.writeToStream(data)
}

// config         | frame size | PCM samples
// ---------------+------------+------------
// 16, 20, 24, 28 | 2.5 ms     |  120
// 17, 21, 25, 29 |   5 ms     |  240
// 0, 4, 8        |  10 ms     |  480
// 12, 14         |  10 ms     |  480
// 18, 22, 26, 30 |  10 ms     |  480
// 1, 5, 9        |  20 ms     |  960
// 13, 15         |  20 ms     |  960
// 19, 23, 27, 31 |  20 ms     |  960
// 2, 6, 10       |  40 ms     | 1920
// 3, 7, 11       |  60 ms     | 2880

func GranulePosition(config byte) uint64 {
	// TODO: 値の決定に他の要素が必要ないか確認する
	var granulePosition uint64
	switch config {
	case 16, 20, 24, 28:
		granulePosition = 120
	case 17, 21, 25, 29:
		granulePosition = 240
	case 0, 4, 8, 12, 14, 18, 22, 26, 30:
		granulePosition = 480
	case 1, 5, 9, 13, 15, 19, 23, 27, 31:
		granulePosition = 960
	case 2, 6, 10:
		granulePosition = 1920
	case 3, 7, 11:
		granulePosition = 2880
	}

	return granulePosition
}
