package suzu

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"
)

const (
	// timestamp(64), sequence number(64), length(32)
	HeaderLength     = 20
	MaxPayloadLength = 0xffff

	ErrPayloadTooLarge = "PAYLOAD-TOO-LARGE: %d"
	ErrBufferTooLarge  = "BUFFER-TOO-LARGE: %d"
)

// パケット読み込み時のオプション関数の型定義
// opus channel を受け取り、オプション処理を行った後の opus channel を返す
type packetReaderOption func(ctx context.Context, c Config, ch chan opus) chan opus

// パケット読み込み時のオプション関数群を生成する
func newPacketReaderOptions(c Config) []packetReaderOption {
	options := []packetReaderOption{}

	if c.AudioStreamingHeader {
		options = append(options, optionReadPacketWithHeader)
	}

	if !c.DisableSilentPacket {
		options = append(options, optionSilentPacket)
	}

	return options
}

func optionSilentPacket(ctx context.Context, c Config, opusCh chan opus) chan opus {
	ch := make(chan opus)

	go func() {
		defer close(ch)

		// 無音パケットを送信する間隔
		d := time.Duration(c.TimeToWaitForOpusPacketMs) * time.Millisecond
		timer := time.NewTimer(d)

		for {
			var opusPacket opus
			select {
			case <-timer.C:
				// サイレントパケットはヘッダー無しで送出する
				payload := silentPacket()
				opusPacket = opus{Payload: payload}
			case req, ok := <-opusCh:
				if !ok {
					return
				}

				opusPacket = req
			}

			// 受信したデータ、または、無音パケットを送信する
			select {
			case <-ctx.Done():
				return
			case ch <- opusPacket:
			}

			// 受信したらタイマーをリセットする
			timer.Reset(d)
		}
	}()

	return ch
}

// パケット読み込み時のヘッダー処理オプション関数
func optionReadPacketWithHeader(ctx context.Context, c Config, opusCh chan opus) chan opus {
	ch := make(chan opus)

	go func() {
		defer close(ch)

		length := 0
		payloadLength := 0
		var payload []byte

		for {
			select {
			case <-ctx.Done():
				return
			case req, ok := <-opusCh:
				if !ok {
					return
				}
				if req.Err != nil {
					select {
					case <-ctx.Done():
						return
					case ch <- opus{Err: req.Err}:
						return
					}
				}

				packet := req.Payload
				payload = append(payload, packet...)
				length += len(packet)

				// ヘッダー分のデータが揃っていないので、次の読み込みへ
				if length < HeaderLength {
					continue
				}

				// timestamp(64), sequence number(64), length(32)
				h := payload[:HeaderLength]
				p := payload[HeaderLength:]

				payloadLength = int(binary.BigEndian.Uint32(h[16:HeaderLength]))
				// payloadLength が大きすぎる場合はエラーを返す
				if payloadLength > MaxPayloadLength {
					select {
					case <-ctx.Done():
						return
					case ch <- opus{Err: fmt.Errorf(ErrPayloadTooLarge, payloadLength)}:
						return
					}
				}

				// payload が足りないので、次の読み込みへ
				if length < (HeaderLength + payloadLength) {
					// パケット全体が大きすぎる場合はエラーを返す
					if length > HeaderLength+MaxPayloadLength {
						select {
						case <-ctx.Done():
							return
						case ch <- opus{Err: fmt.Errorf(ErrBufferTooLarge, length)}:
							return
						}
					}
					continue
				}

				select {
				case <-ctx.Done():
					return
				case ch <- opus{Payload: p[:payloadLength]}:
				}

				payload = p[payloadLength:]
				length = len(payload)

				// 全てのデータを書き込んだ場合は次の読み込みへ
				if length == 0 {
					continue
				}

				// 次の frame が含まれている場合
				for {
					// ヘッダー分のデータが揃っていないので、次の読み込みへ
					if length < HeaderLength {
						break
					}

					h = payload[:HeaderLength]
					p = payload[HeaderLength:]

					payloadLength = int(binary.BigEndian.Uint32(h[16:HeaderLength]))
					// payloadLength が大きすぎる場合はエラーを返す
					if payloadLength > MaxPayloadLength {
						select {
						case <-ctx.Done():
							return
						case ch <- opus{Err: fmt.Errorf(ErrPayloadTooLarge, payloadLength)}:
							return
						}
					}

					// payload が足りないので、次の読み込みへ
					if length < (HeaderLength + payloadLength) {
						// パケット全体が大きすぎる場合はエラーを返す
						if length > HeaderLength+MaxPayloadLength {
							select {
							case <-ctx.Done():
								return
							case ch <- opus{Err: fmt.Errorf(ErrBufferTooLarge, length)}:
								return
							}
						}
						break
					}

					// データが足りているので payloadLength まで書き込む
					select {
					case <-ctx.Done():
						return
					case ch <- opus{Payload: p[:payloadLength]}:
					}

					// 残りの処理へ
					payload = p[payloadLength:]
					length = len(payload)
				}
			}
		}
	}()

	return ch
}
