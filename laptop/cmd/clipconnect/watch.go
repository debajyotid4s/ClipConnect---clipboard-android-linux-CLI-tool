package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/clip"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/clip/backend"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/server"
)

// watchClipboard polls the OS clipboard and sends changed clips to paired phones.
// echoGuard is shared with the incoming handler so a clip applied from the
// laptop (phone side) is not echoed back; clip.Guard is mutex-protected and safe
// to share across goroutines.
func watchClipboard(ctx context.Context, be backend.Backend, srv *server.Server, echoGuard *clip.Guard) {
	var lastClip string
	var lastTime time.Time

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			text, err := be.Get(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "clipboard get: %v\n", err)
				continue
			}
			if clip.IsEmptyOrTooLarge(text) || clip.IsSensitive(text) {
				continue
			}
			if t.Sub(lastTime) < 150*time.Millisecond {
				continue
			}
			// Echo guard: skip a clip the laptop just applied to this machine.
			if echoGuard.Record(text) {
				continue
			}
			if text == lastClip {
				continue
			}
			lastClip = text
			lastTime = t
			if sendErr := srv.Send(text); sendErr != nil {
				fmt.Fprintf(os.Stderr, "send clip: %v\n", sendErr)
			}
		}
	}
}
