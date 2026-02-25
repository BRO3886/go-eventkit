//go:build darwin

// consumer watches for EventKit calendar changes and prints a diff of what
// events appeared, disappeared, or were updated since the last fetch.
//
// Run:
//
//	go run ./scripts/watch-demo/consumer
//
// Then in a separate terminal, run the producer:
//
//	go run ./scripts/watch-demo/producer
//
// Cross-process EKEventStoreChangedNotification delivery requires the main
// thread's CFRunLoop to be active. This binary locks the main goroutine to
// the OS main thread and pumps the CFRunLoop there; all Go work runs in a
// separate goroutine.
package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework CoreFoundation
#import <Foundation/Foundation.h>
#include <CoreFoundation/CoreFoundation.h>

// Pump the main CFRunLoop for up to timeoutSecs seconds (or until a source fires).
static void pump_run_loop(double timeoutSecs) {
    CFRunLoopRunInMode(kCFRunLoopDefaultMode, timeoutSecs, false);
}
*/
import "C"

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/BRO3886/go-eventkit/calendar"
)

// snapshot is a lightweight in-memory record of an event.
type snapshot struct {
	title     string
	startDate time.Time
	endDate   time.Time
	calendar  string
}

func main() {
	// Lock the main goroutine to the OS main thread so that CFRunLoopRun below
	// runs on the actual main thread — required for cross-process
	// EKEventStoreChangedNotification delivery.
	runtime.LockOSThread()

	log.SetFlags(0)

	client, err := calendar.New()
	if err != nil {
		log.Fatalf("calendar.New: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Fetch window: next 7 days.
	fetchWindow := func() (time.Time, time.Time) {
		now := time.Now()
		return now, now.Add(7 * 24 * time.Hour)
	}

	// Build an in-memory snapshot map keyed by event ID.
	buildSnapshot := func() (map[string]snapshot, error) {
		start, end := fetchWindow()
		events, err := client.Events(start, end)
		if err != nil {
			return nil, err
		}
		m := make(map[string]snapshot, len(events))
		for _, e := range events {
			m[e.ID] = snapshot{
				title:     e.Title,
				startDate: e.StartDate,
				endDate:   e.EndDate,
				calendar:  e.Calendar,
			}
		}
		return m, nil
	}

	// Diff two snapshots and print what changed.
	diff := func(before, after map[string]snapshot) {
		added := 0
		removed := 0
		changed := 0

		for id, a := range after {
			b, existed := before[id]
			if !existed {
				fmt.Printf("  + ADDED    %q  [%s]  %s – %s\n",
					a.title, a.calendar,
					a.startDate.Format("Mon Jan 2 15:04"),
					a.endDate.Format("15:04"))
				added++
				continue
			}
			if a.title != b.title || !a.startDate.Equal(b.startDate) || !a.endDate.Equal(b.endDate) {
				fmt.Printf("  ~ CHANGED  %q  [%s]  %s – %s\n",
					a.title, a.calendar,
					a.startDate.Format("Mon Jan 2 15:04"),
					a.endDate.Format("15:04"))
				changed++
			}
		}

		for id, b := range before {
			if _, stillExists := after[id]; !stillExists {
				fmt.Printf("  - REMOVED  %q  [%s]  %s\n",
					b.title, b.calendar,
					b.startDate.Format("Mon Jan 2 15:04"))
				removed++
			}
		}

		if added+removed+changed == 0 {
			fmt.Println("  (no visible changes in 7-day window)")
		}
	}

	// Take initial snapshot before subscribing.
	prev, err := buildSnapshot()
	if err != nil {
		log.Fatalf("initial fetch: %v", err)
	}
	fmt.Printf("[consumer] watching next 7 days (%d events). Ctrl+C to stop.\n\n", len(prev))

	changes, err := client.WatchChanges(ctx)
	if err != nil {
		log.Fatalf("WatchChanges: %v", err)
	}

	// Process change signals in a goroutine; main thread pumps the run loop.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range changes {
			fmt.Printf("[%s] change detected — re-fetching...\n", time.Now().Format(time.TimeOnly))
			curr, err := buildSnapshot()
			if err != nil {
				log.Printf("  fetch error: %v", err)
				continue
			}
			diff(prev, curr)
			prev = curr
			fmt.Println()
		}
		fmt.Println("[consumer] stopped.")
	}()

	// Pump the main CFRunLoop so cross-process EKEventStoreChangedNotification
	// can be delivered to this process. Returns every 0.1s to check for exit.
	for {
		C.pump_run_loop(0.1)
		select {
		case <-done:
			return
		default:
		}
	}
}
