package reminders

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"
)

func TestWatchChanges_SignalOnWrite(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := watchChangesFromFile(ctx, r)

	if _, err := w.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	select {
	case _, ok := <-ch:
		if !ok {
			t.Fatal("channel closed unexpectedly")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for signal")
	}
}

func TestWatchChanges_CtxCancel(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	ch := watchChangesFromFile(ctx, r)
	cancel()

	// Write a byte to unblock the Read call so the goroutine can observe ctx.Done().
	_, _ = w.Write([]byte{1})

	select {
	case _, ok := <-ch:
		if ok {
			// A signal arrived before cancel took effect — drain and check next.
			select {
			case _, ok2 := <-ch:
				if ok2 {
					t.Fatal("channel should be closed after cancel")
				}
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for channel close after cancel")
			}
		}
		// ok == false means channel already closed — pass
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for channel close after cancel")
	}
}

func TestWatchChanges_PipeClose(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	ch := watchChangesFromFile(context.Background(), r)
	w.Close()

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("channel should be closed after pipe EOF")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for channel close after pipe EOF")
	}
}

func TestWatchChanges_Coalescing(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := watchChangesFromFile(ctx, r)
	if _, err := w.Write(bytes.Repeat([]byte{1}, 100)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)

	count := 0
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				goto done
			}
			count++
		default:
			goto done
		}
	}
done:
	if count > 16 {
		t.Fatalf("expected at most 16 coalesced signals, got %d", count)
	}
	if count == 0 {
		t.Fatal("expected at least one signal")
	}
}
