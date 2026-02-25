package reminders

import (
	"context"
	"os"
)

// watchChangesFromFile reads bytes from f and sends a signal on the returned
// channel for each byte read. The channel is buffered (capacity 16); excess
// signals are dropped via a non-blocking send (callers re-fetch anyway).
// The channel is closed when ctx is cancelled or f returns an error/EOF.
// f is not closed by this function.
func watchChangesFromFile(ctx context.Context, f *os.File) <-chan struct{} {
	ch := make(chan struct{}, 16)
	go func() {
		defer close(ch)
		buf := make([]byte, 64)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			n, err := f.Read(buf)
			if err != nil || n == 0 {
				return
			}
			for i := 0; i < n; i++ {
				select {
				case ch <- struct{}{}:
				default: // drop: consumer will re-fetch anyway
				}
			}
		}
	}()
	return ch
}
