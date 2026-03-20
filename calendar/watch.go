package calendar

import (
	"context"
	"os"
	"syscall"
	"time"
)

// watchChangesFromFile reads bytes from f and sends a signal on the returned
// channel for each byte read. The channel is buffered (capacity 16); excess
// signals are dropped via a non-blocking send (callers re-fetch anyway).
// The channel is closed when ctx is cancelled or f returns an error/EOF.
// f is not closed by this function.
//
// The file descriptor is set to non-blocking mode so that reads can be
// interleaved with ctx.Done() checks. This avoids goroutine leaks when
// the context is cancelled but no data arrives on the pipe.
func watchChangesFromFile(ctx context.Context, f *os.File) <-chan struct{} {
	ch := make(chan struct{}, 16)
	// Set non-blocking so Read returns EAGAIN instead of blocking forever.
	syscall.SetNonblock(int(f.Fd()), true)
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
			if n > 0 {
				for i := 0; i < n; i++ {
					select {
					case ch <- struct{}{}:
					default: // drop: consumer will re-fetch anyway
					}
				}
				continue
			}
			if err != nil {
				// EAGAIN means no data available (non-blocking) — poll and retry.
				if isEAGAIN(err) {
					select {
					case <-ctx.Done():
						return
					case <-time.After(100 * time.Millisecond):
						continue
					}
				}
				// Real error (EOF, closed fd) — exit.
				return
			}
		}
	}()
	return ch
}

func isEAGAIN(err error) bool {
	if pe, ok := err.(*os.PathError); ok {
		err = pe.Err
	}
	return err == syscall.EAGAIN || err == syscall.EWOULDBLOCK
}
