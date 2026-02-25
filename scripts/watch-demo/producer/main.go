//go:build darwin

// producer creates a calendar event, waits, updates it, waits, then deletes it.
// Run it while the consumer is running to see live change diffs.
//
// Run:
//
//	go run ./scripts/watch-demo/producer
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/BRO3886/go-eventkit/calendar"
)

func main() {
	log.SetFlags(0)

	client, err := calendar.New()
	if err != nil {
		log.Fatalf("calendar.New: %v", err)
	}

	pause := func(msg string) {
		fmt.Printf("[producer] %s\n", msg)
		time.Sleep(2 * time.Second)
	}

	// --- CREATE ---
	start := time.Now().Add(2 * time.Hour).Truncate(time.Hour)
	end := start.Add(30 * time.Minute)

	pause("creating event...")
	event, err := client.CreateEvent(calendar.CreateEventInput{
		Title:     "WatchChanges demo event",
		StartDate: start,
		EndDate:   end,
		Notes:     "Created by producer script",
	})
	if err != nil {
		log.Fatalf("CreateEvent: %v", err)
	}
	fmt.Printf("[producer] created %q (ID: %s)\n\n", event.Title, event.ID[:8]+"...")

	// --- UPDATE ---
	pause("updating title...")
	newTitle := "WatchChanges demo event (updated)"
	updated, err := client.UpdateEvent(event.ID, calendar.UpdateEventInput{
		Title: &newTitle,
	}, calendar.SpanThisEvent)
	if err != nil {
		log.Fatalf("UpdateEvent: %v", err)
	}
	fmt.Printf("[producer] updated to %q\n\n", updated.Title)

	// --- DELETE ---
	pause("deleting event...")
	if err := client.DeleteEvent(event.ID, calendar.SpanThisEvent); err != nil {
		log.Fatalf("DeleteEvent: %v", err)
	}
	fmt.Printf("[producer] deleted %q\n\n", updated.Title)

	fmt.Println("[producer] done.")
}
