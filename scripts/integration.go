//go:build darwin && integration

// Package main provides an integration test script for the calendar package.
// It exercises the real EventKit bridge against live macOS Calendar data.
//
// Run with: go run -tags integration ./scripts/integration_test.go
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/BRO3886/go-eventkit/calendar"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("[integration] ")

	passed := 0
	failed := 0

	check := func(name string, err error) {
		if err != nil {
			log.Printf("FAIL: %s: %v", name, err)
			failed++
		} else {
			log.Printf("PASS: %s", name)
			passed++
		}
	}

	// --- Test 1: Create client (TCC access) ---
	client, err := calendar.New()
	if err != nil {
		log.Fatalf("FATAL: Failed to create client (TCC denied?): %v", err)
	}
	log.Println("PASS: Client created successfully")
	passed++

	// --- Test 2: List all calendars ---
	calendars, err := client.Calendars()
	check("List calendars", err)
	if err == nil {
		log.Printf("  Found %d calendars:", len(calendars))
		for _, c := range calendars {
			log.Printf("    - %s (ID: %s, Type: %s, Source: %s, Color: %s, ReadOnly: %v)",
				c.Title, c.ID[:8]+"...", c.Type, c.Source, c.Color, c.ReadOnly)
		}
	}

	// --- Test 3: Create event in Home calendar ---
	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	testStart := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 10, 0, 0, 0, time.Local)
	testEnd := testStart.Add(30 * time.Minute)

	created, err := client.CreateEvent(calendar.CreateEventInput{
		Title:     "[go-eventkit test] Integration Test Event",
		StartDate: testStart,
		EndDate:   testEnd,
		Location:  "Test Location",
		Notes:     "Created by go-eventkit integration test. Safe to delete.",
		Calendar:  "Home",
		Alerts:    []calendar.Alert{{RelativeOffset: -15 * time.Minute}},
	})
	check("Create event in Home calendar", err)

	var createdID string
	if err == nil {
		createdID = created.ID
		log.Printf("  Created event: %q (ID: %s)", created.Title, created.ID)
		log.Printf("  Calendar: %s, Start: %v, End: %v", created.Calendar, created.StartDate, created.EndDate)
		log.Printf("  Location: %s, Alerts: %d", created.Location, len(created.Alerts))
	}

	// --- Test 4: Get event by ID ---
	if createdID != "" {
		fetched, err := client.Event(createdID)
		check("Get event by ID", err)
		if err == nil {
			if fetched.Title != created.Title {
				log.Printf("  WARN: Title mismatch: got %q, want %q", fetched.Title, created.Title)
			}
			if fetched.Location != "Test Location" {
				log.Printf("  WARN: Location mismatch: got %q, want %q", fetched.Location, "Test Location")
			}
			log.Printf("  Fetched event matches: %q", fetched.Title)
		}
	}

	// --- Test 5: Fetch events in date range ---
	rangeStart := testStart.Add(-time.Hour)
	rangeEnd := testEnd.Add(time.Hour)
	events, err := client.Events(rangeStart, rangeEnd)
	check("Fetch events in date range", err)
	if err == nil {
		log.Printf("  Found %d events in range %v to %v", len(events), rangeStart.Format(time.RFC3339), rangeEnd.Format(time.RFC3339))
		found := false
		for _, e := range events {
			if e.ID == createdID {
				found = true
				log.Printf("  Found our test event in range results")
			}
		}
		if !found && createdID != "" {
			log.Printf("  WARN: Our test event not found in range results")
		}
	}

	// --- Test 6: Fetch events with calendar filter ---
	homeEvents, err := client.Events(rangeStart, rangeEnd, calendar.WithCalendar("Home"))
	check("Fetch events with calendar filter (Home)", err)
	if err == nil {
		log.Printf("  Found %d Home events", len(homeEvents))
		for _, e := range homeEvents {
			if e.Calendar != "Home" {
				log.Printf("  FAIL: Event %q has calendar %q, expected Home", e.Title, e.Calendar)
				failed++
			}
		}
	}

	// --- Test 7: Fetch events with search filter ---
	searchEvents, err := client.Events(rangeStart, rangeEnd, calendar.WithSearch("Integration Test"))
	check("Fetch events with search filter", err)
	if err == nil {
		log.Printf("  Found %d events matching 'Integration Test'", len(searchEvents))
	}

	// --- Test 8: Update event ---
	if createdID != "" {
		newTitle := "[go-eventkit test] Updated Event"
		newLocation := "Updated Location"
		newNotes := "Updated by integration test"

		updated, err := client.UpdateEvent(createdID, calendar.UpdateEventInput{
			Title:    &newTitle,
			Location: &newLocation,
			Notes:    &newNotes,
		}, calendar.SpanThisEvent)
		check("Update event", err)
		if err == nil {
			if updated.Title != newTitle {
				log.Printf("  FAIL: Title not updated: got %q, want %q", updated.Title, newTitle)
				failed++
			} else {
				log.Printf("  Updated event title to: %q", updated.Title)
			}
			if updated.Location != newLocation {
				log.Printf("  FAIL: Location not updated: got %q, want %q", updated.Location, newLocation)
				failed++
			}
		}
	}

	// --- Test 9: Create event in Work calendar ---
	workStart := testStart.Add(2 * time.Hour)
	workEnd := workStart.Add(time.Hour)
	workEvent, err := client.CreateEvent(calendar.CreateEventInput{
		Title:     "[go-eventkit test] Work Meeting",
		StartDate: workStart,
		EndDate:   workEnd,
		Calendar:  "Work",
		Notes:     "Created by go-eventkit integration test. Safe to delete.",
	})
	check("Create event in Work calendar", err)

	var workEventID string
	if err == nil {
		workEventID = workEvent.ID
		log.Printf("  Created work event: %q in %s calendar", workEvent.Title, workEvent.Calendar)
	}

	// --- Test 10: Create all-day event in Family calendar ---
	allDayStart := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, time.UTC)
	allDayEnd := allDayStart.Add(24 * time.Hour)
	familyEvent, err := client.CreateEvent(calendar.CreateEventInput{
		Title:     "[go-eventkit test] Family All-Day",
		StartDate: allDayStart,
		EndDate:   allDayEnd,
		AllDay:    true,
		Calendar:  "Family",
		Notes:     "Created by go-eventkit integration test. Safe to delete.",
	})
	check("Create all-day event in Family calendar", err)

	var familyEventID string
	if err == nil {
		familyEventID = familyEvent.ID
		log.Printf("  Created all-day event: %q, AllDay=%v", familyEvent.Title, familyEvent.AllDay)
		if !familyEvent.AllDay {
			log.Printf("  WARN: AllDay flag not set on returned event")
		}
	}

	// --- Test 11: Verify events across multiple calendars ---
	broadStart := testStart.Add(-2 * time.Hour)
	broadEnd := workEnd.Add(2 * time.Hour)
	allEvents, err := client.Events(broadStart, broadEnd)
	check("Fetch events across all calendars", err)
	if err == nil {
		calMap := make(map[string]int)
		for _, e := range allEvents {
			calMap[e.Calendar]++
		}
		log.Printf("  Events by calendar:")
		for cal, count := range calMap {
			log.Printf("    - %s: %d events", cal, count)
		}
	}

	// --- Test 12: Create event with timezone ---
	tzEvent, err := client.CreateEvent(calendar.CreateEventInput{
		Title:     "[go-eventkit test] Tokyo Meeting",
		StartDate: time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 9, 0, 0, 0, time.UTC),
		EndDate:   time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 10, 0, 0, 0, time.UTC),
		Calendar:  "Home",
		TimeZone:  "Asia/Tokyo",
		Notes:     "Created by go-eventkit integration test. Safe to delete.",
	})
	check("Create event with timezone (Asia/Tokyo)", err)

	var tzEventID string
	if err == nil {
		tzEventID = tzEvent.ID
		log.Printf("  Created timezone event: %q, TimeZone=%s", tzEvent.Title, tzEvent.TimeZone)
	}

	// --- Test 13: Create event with URL ---
	urlEvent, err := client.CreateEvent(calendar.CreateEventInput{
		Title:     "[go-eventkit test] URL Event",
		StartDate: testStart.Add(3 * time.Hour),
		EndDate:   testStart.Add(4 * time.Hour),
		Calendar:  "Home",
		URL:       "https://meet.example.com/test",
		Notes:     "Created by go-eventkit integration test. Safe to delete.",
	})
	check("Create event with URL", err)

	var urlEventID string
	if err == nil {
		urlEventID = urlEvent.ID
		log.Printf("  Created URL event: %q, URL=%s", urlEvent.Title, urlEvent.URL)
	}

	// --- Test 14: Create event with multiple alerts ---
	alertEvent, err := client.CreateEvent(calendar.CreateEventInput{
		Title:     "[go-eventkit test] Alert Event",
		StartDate: testStart.Add(5 * time.Hour),
		EndDate:   testStart.Add(6 * time.Hour),
		Calendar:  "Home",
		Alerts: []calendar.Alert{
			{RelativeOffset: -15 * time.Minute},
			{RelativeOffset: -1 * time.Hour},
			{RelativeOffset: -24 * time.Hour},
		},
		Notes: "Created by go-eventkit integration test. Safe to delete.",
	})
	check("Create event with multiple alerts", err)

	var alertEventID string
	if err == nil {
		alertEventID = alertEvent.ID
		log.Printf("  Created alert event: %q, Alerts=%d", alertEvent.Title, len(alertEvent.Alerts))
	}

	// --- Test 15: Move event between calendars ---
	if createdID != "" {
		workCal := "Work"
		moved, err := client.UpdateEvent(createdID, calendar.UpdateEventInput{
			Calendar: &workCal,
		}, calendar.SpanThisEvent)
		check("Move event from Home to Work", err)
		if err == nil {
			log.Printf("  Moved event to calendar: %s", moved.Calendar)
		}
	}

	// --- Test 16: Get non-existent event ---
	_, err = client.Event("non-existent-event-id-12345")
	if err != nil {
		log.Printf("PASS: Get non-existent event returns error: %v", err)
		passed++
	} else {
		log.Printf("FAIL: Get non-existent event should return error")
		failed++
	}

	// --- Cleanup: Delete all test events ---
	log.Println("\n--- Cleanup ---")
	cleanupIDs := []string{createdID, workEventID, familyEventID, tzEventID, urlEventID, alertEventID}
	for _, id := range cleanupIDs {
		if id == "" {
			continue
		}
		err := client.DeleteEvent(id, calendar.SpanThisEvent)
		if err != nil {
			log.Printf("WARN: Failed to delete event %s: %v", id, err)
		} else {
			log.Printf("  Deleted event: %s", id)
		}
	}

	// --- Test 17: Verify deleted event is gone ---
	if createdID != "" {
		_, err := client.Event(createdID)
		if err != nil {
			log.Printf("PASS: Deleted event not found (expected)")
			passed++
		} else {
			log.Printf("FAIL: Deleted event still accessible")
			failed++
		}
	}

	// --- Summary ---
	fmt.Printf("\n=== Integration Test Results ===\n")
	fmt.Printf("Passed: %d\n", passed)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Total:  %d\n", passed+failed)
	if failed > 0 {
		os.Exit(1)
	}
}
