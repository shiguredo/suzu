package suzu

import (
	"context"
	"fmt"
	"io"
	"time"
)

func TestHandler(ctx context.Context, body io.Reader, args HandlerArgs) (<-chan Response, error) {
	ch := handleTest(ctx, body, args.Config)
	return ch, nil
}

func handleTest(ctx context.Context, r io.Reader, c Config) <-chan Response {
	ch := make(chan Response)

	go func() {
		defer close(ch)

		// TODO: エラー処理
		t, _ := time.ParseDuration(c.TimeToWaitForOpusPacket)

		reader := NewReaderWithTimer(r)
		resultCh := reader.Read(ctx, t)

		for {
			select {
			case <-ctx.Done():
				return
			case res := <-resultCh:
				if err := res.Error; err != nil {
					ch <- Response{
						Error: res.Error,
					}
					return
				}
				ch <- Response{
					ChannelID: &[]string{"ch_0"}[0],
					Message:   fmt.Sprintf("n: %d", len(res.Message)),
				}
			}

		}
	}()

	return ch
}
