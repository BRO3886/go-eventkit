//go:build darwin && integration

// Package main provides an integration test script for the calendar package.
// It exercises the real EventKit bridge against live macOS Calendar data.
//
// Run with: go run -tags integration ./scripts/integration_test.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/BRO3886/go-eventkit"
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

	// --- Test 16: Create event with daily recurrence ---
	dailyRecStart := testStart.Add(7 * time.Hour)
	dailyRecEnd := dailyRecStart.Add(30 * time.Minute)
	dailyRecEvent, err := client.CreateEvent(calendar.CreateEventInput{
		Title:     "[go-eventkit test] Daily Recurring",
		StartDate: dailyRecStart,
		EndDate:   dailyRecEnd,
		Calendar:  "Home",
		Notes:     "Created by go-eventkit integration test. Safe to delete.",
		RecurrenceRules: []eventkit.RecurrenceRule{
			eventkit.Daily(1).Count(5),
		},
	})
	check("Create event with daily recurrence", err)

	var dailyRecEventID string
	if err == nil {
		dailyRecEventID = dailyRecEvent.ID
		log.Printf("  Created daily recurring event: %q, Recurring=%v", dailyRecEvent.Title, dailyRecEvent.Recurring)
		if !dailyRecEvent.Recurring {
			log.Printf("  FAIL: Recurring should be true")
			failed++
		}
		if len(dailyRecEvent.RecurrenceRules) != 1 {
			log.Printf("  FAIL: RecurrenceRules count = %d, want 1", len(dailyRecEvent.RecurrenceRules))
			failed++
		} else {
			rule := dailyRecEvent.RecurrenceRules[0]
			if rule.Frequency != eventkit.FrequencyDaily {
				log.Printf("  FAIL: Frequency = %d, want %d (daily)", rule.Frequency, eventkit.FrequencyDaily)
				failed++
			}
			if rule.Interval != 1 {
				log.Printf("  FAIL: Interval = %d, want 1", rule.Interval)
				failed++
			}
			if rule.End == nil || rule.End.OccurrenceCount != 5 {
				log.Printf("  FAIL: End = %+v, want OccurrenceCount=5", rule.End)
				failed++
			}
			log.Printf("  Verified recurrence rule: daily, interval=1, count=5")
		}
	}

	// --- Test 17: Create event with weekly recurrence on Mon/Wed/Fri ---
	weeklyRecStart := testStart.Add(8 * time.Hour)
	weeklyRecEnd := weeklyRecStart.Add(30 * time.Minute)
	endDate := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, time.Local).Add(30 * 24 * time.Hour)
	weeklyRecEvent, err := client.CreateEvent(calendar.CreateEventInput{
		Title:     "[go-eventkit test] Weekly MWF Meeting",
		StartDate: weeklyRecStart,
		EndDate:   weeklyRecEnd,
		Calendar:  "Home",
		Notes:     "Created by go-eventkit integration test. Safe to delete.",
		RecurrenceRules: []eventkit.RecurrenceRule{
			eventkit.Weekly(2, eventkit.Monday, eventkit.Wednesday, eventkit.Friday).Until(endDate),
		},
	})
	check("Create event with weekly recurrence (MWF every 2 weeks)", err)

	var weeklyRecEventID string
	if err == nil {
		weeklyRecEventID = weeklyRecEvent.ID
		log.Printf("  Created weekly recurring event: %q, Recurring=%v", weeklyRecEvent.Title, weeklyRecEvent.Recurring)
		if len(weeklyRecEvent.RecurrenceRules) == 1 {
			rule := weeklyRecEvent.RecurrenceRules[0]
			if rule.Frequency != eventkit.FrequencyWeekly {
				log.Printf("  FAIL: Frequency = %d, want %d (weekly)", rule.Frequency, eventkit.FrequencyWeekly)
				failed++
			}
			if rule.Interval != 2 {
				log.Printf("  FAIL: Interval = %d, want 2", rule.Interval)
				failed++
			}
			if len(rule.DaysOfTheWeek) != 3 {
				log.Printf("  FAIL: DaysOfTheWeek count = %d, want 3", len(rule.DaysOfTheWeek))
				failed++
			}
			if rule.End == nil || rule.End.EndDate == nil {
				log.Printf("  FAIL: End should have EndDate")
				failed++
			}
			log.Printf("  Verified recurrence rule: weekly(2), MWF, until=%v", endDate)
		}
	}

	// --- Test 18: Create event with structured location ---
	locEvent, err := client.CreateEvent(calendar.CreateEventInput{
		Title:     "[go-eventkit test] Location Event",
		StartDate: testStart.Add(9 * time.Hour),
		EndDate:   testStart.Add(10 * time.Hour),
		Calendar:  "Home",
		Notes:     "Created by go-eventkit integration test. Safe to delete.",
		StructuredLocation: &eventkit.StructuredLocation{
			Title:     "Apple Park",
			Latitude:  37.3349,
			Longitude: -122.0090,
			Radius:    150.0,
		},
	})
	check("Create event with structured location", err)

	var locEventID string
	if err == nil {
		locEventID = locEvent.ID
		log.Printf("  Created location event: %q", locEvent.Title)
		if locEvent.StructuredLocation == nil {
			log.Printf("  WARN: StructuredLocation is nil on returned event (EventKit may not populate immediately)")
		} else {
			sl := locEvent.StructuredLocation
			log.Printf("  StructuredLocation: title=%q, lat=%f, long=%f, radius=%f",
				sl.Title, sl.Latitude, sl.Longitude, sl.Radius)
		}
	}

	// --- Test 19: Read back location event and verify coordinates ---
	if locEventID != "" {
		readBackLoc, err := client.Event(locEventID)
		check("Read back location event", err)
		if err == nil && readBackLoc.StructuredLocation != nil {
			sl := readBackLoc.StructuredLocation
			if sl.Latitude < 37.33 || sl.Latitude > 37.34 {
				log.Printf("  FAIL: Latitude = %f, expected ~37.3349", sl.Latitude)
				failed++
			} else {
				log.Printf("  Verified latitude: %f", sl.Latitude)
			}
			if sl.Longitude < -122.01 || sl.Longitude > -122.00 {
				log.Printf("  FAIL: Longitude = %f, expected ~-122.009", sl.Longitude)
				failed++
			} else {
				log.Printf("  Verified longitude: %f", sl.Longitude)
			}
		}
	}

	// --- Test 20: Update event to add recurrence rule ---
	if createdID != "" {
		addRules := []eventkit.RecurrenceRule{eventkit.Daily(1).Count(3)}
		updated, err := client.UpdateEvent(createdID, calendar.UpdateEventInput{
			RecurrenceRules: &addRules,
		}, calendar.SpanThisEvent)
		check("Update event: add recurrence rule", err)
		if err == nil {
			if !updated.Recurring {
				log.Printf("  FAIL: Recurring should be true after adding rule")
				failed++
			} else {
				log.Printf("  Event now recurring: %v", updated.Recurring)
			}
		}
	}

	// --- Test 21: Update event to remove recurrence ---
	if createdID != "" {
		emptyRules := []eventkit.RecurrenceRule{}
		updated, err := client.UpdateEvent(createdID, calendar.UpdateEventInput{
			RecurrenceRules: &emptyRules,
		}, calendar.SpanThisEvent)
		check("Update event: remove recurrence (empty slice)", err)
		if err == nil {
			if updated.Recurring {
				log.Printf("  FAIL: Recurring should be false after removing rules")
				failed++
			} else {
				log.Printf("  Event no longer recurring: %v", updated.Recurring)
			}
		}
	}

	// --- Test 22: Delete single occurrence of recurring event (SpanThisEvent) ---
	if dailyRecEventID != "" {
		err := client.DeleteEvent(dailyRecEventID, calendar.SpanThisEvent)
		check("Delete single occurrence of recurring event", err)
		if err == nil {
			log.Printf("  Deleted single occurrence of daily recurring event")
		}
	}

	// --- Test 23: Get non-existent event ---
	_, err = client.Event("non-existent-event-id-12345")
	if err != nil {
		log.Printf("PASS: Get non-existent event returns error: %v", err)
		passed++
	} else {
		log.Printf("FAIL: Get non-existent event should return error")
		failed++
	}

	// --- Test 25: Create calendar ---
	// Discover a writable source from existing calendars.
	var writableSource string
	for _, c := range calendars {
		if !c.ReadOnly && c.Source != "" {
			writableSource = c.Source
			break
		}
	}
	testCal, err := client.CreateCalendar(calendar.CreateCalendarInput{
		Title:  "[go-eventkit test] Integration Cal",
		Color:  "#FF6961",
		Source: writableSource,
	})
	check("Create calendar", err)

	var testCalID string
	if err == nil {
		testCalID = testCal.ID
		log.Printf("  Created calendar: %q (ID: %s)", testCal.Title, testCal.ID[:8]+"...")
		log.Printf("  Color: %s, Source: %s, ReadOnly: %v", testCal.Color, testCal.Source, testCal.ReadOnly)
	}

	// --- Test 26: Verify new calendar appears in list ---
	if testCalID != "" {
		cals, err := client.Calendars()
		check("Verify new calendar in list", err)
		if err == nil {
			found := false
			for _, c := range cals {
				if c.ID == testCalID {
					found = true
					log.Printf("  Found new calendar in list: %q", c.Title)
					break
				}
			}
			if !found {
				log.Printf("  FAIL: New calendar not found in list")
				failed++
			}
		}
	}

	// --- Test 27: Update calendar (rename + recolor) ---
	if testCalID != "" {
		newTitle := "[go-eventkit test] Renamed Cal"
		newColor := "#00FF00"
		updatedCal, err := client.UpdateCalendar(testCalID, calendar.UpdateCalendarInput{
			Title: &newTitle,
			Color: &newColor,
		})
		check("Update calendar (rename + recolor)", err)
		if err == nil {
			if updatedCal.Title != newTitle {
				log.Printf("  FAIL: Title not updated: got %q, want %q", updatedCal.Title, newTitle)
				failed++
			} else {
				log.Printf("  Renamed calendar to: %q", updatedCal.Title)
			}
		}
	}

	// --- Test 28: Create event in new calendar ---
	var calTestEventID string
	if testCalID != "" {
		calTestEvent, err := client.CreateEvent(calendar.CreateEventInput{
			Title:     "[go-eventkit test] Event in New Cal",
			StartDate: testStart.Add(11 * time.Hour),
			EndDate:   testStart.Add(12 * time.Hour),
			Calendar:  "[go-eventkit test] Renamed Cal",
			Notes:     "Created by go-eventkit integration test. Safe to delete.",
		})
		check("Create event in new calendar", err)
		if err == nil {
			calTestEventID = calTestEvent.ID
			log.Printf("  Created event in new calendar: %q", calTestEvent.Title)
		}
	}

	// --- Test 29: Delete event in new calendar before deleting calendar ---
	if calTestEventID != "" {
		err := client.DeleteEvent(calTestEventID, calendar.SpanFutureEvents)
		check("Delete event in new calendar", err)
	}

	// --- Test 30: Delete calendar ---
	if testCalID != "" {
		err := client.DeleteCalendar(testCalID)
		check("Delete calendar", err)
		if err == nil {
			log.Printf("  Deleted calendar: %s", testCalID[:8]+"...")
		}
	}

	// --- Test 31: Verify deleted calendar is gone ---
	if testCalID != "" {
		cals, err := client.Calendars()
		check("Verify deleted calendar is gone", err)
		if err == nil {
			found := false
			for _, c := range cals {
				if c.ID == testCalID {
					found = true
				}
			}
			if found {
				log.Printf("  FAIL: Deleted calendar still in list")
				failed++
			} else {
				log.Printf("  Deleted calendar confirmed gone")
			}
		}
	}

	// --- Cleanup: Delete all test events ---
	log.Println("\n--- Cleanup ---")
	cleanupIDs := []string{createdID, workEventID, familyEventID, tzEventID, urlEventID, alertEventID, weeklyRecEventID, locEventID}
	for _, id := range cleanupIDs {
		if id == "" {
			continue
		}
		err := client.DeleteEvent(id, calendar.SpanFutureEvents)
		if err != nil {
			log.Printf("WARN: Failed to delete event %s: %v", id, err)
		} else {
			log.Printf("  Deleted event: %s", id)
		}
	}

	// --- Test 24: Verify deleted event is gone ---
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

	// --- Test 32: WatchChanges — signal on event write ---
	log.Println("\n--- Test 32: WatchChanges signal on event write ---")
	{
		ctx32, cancel32 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel32()
		changes32, err := client.WatchChanges(ctx32)
		check("WatchChanges start", err)
		if err == nil {
			// Create an event to trigger notification.
			start32 := time.Now().Add(24 * time.Hour).Truncate(time.Hour)
			ev32, cerr := client.CreateEvent(calendar.CreateEventInput{
				Title:     "WatchChanges test event",
				StartDate: start32,
				EndDate:   start32.Add(time.Hour),
			})
			if cerr != nil {
				log.Printf("WARN: CreateEvent for watch test: %v", cerr)
			} else {
				defer client.DeleteEvent(ev32.ID, calendar.SpanThisEvent)
			}
			select {
			case _, ok := <-changes32:
				if ok {
					log.Printf("  Received change signal (expected)")
				} else {
					log.Printf("  FAIL: channel closed before signal")
					failed++
					passed-- // undo the check increment
				}
			case <-ctx32.Done():
				log.Printf("  FAIL: timeout waiting for change signal")
				failed++
				passed-- // undo the check increment
			}
			cancel32()
		}
	}

	// --- Test 33: WatchChanges — channel closes on ctx cancel ---
	log.Println("\n--- Test 33: WatchChanges channel closes on ctx cancel ---")
	{
		ctx33, cancel33 := context.WithCancel(context.Background())
		changes33, err := client.WatchChanges(ctx33)
		check("WatchChanges start for cancel test", err)
		if err == nil {
			cancel33()
			// Write a dummy event to unblock the pipe read.
			select {
			case <-changes33:
			case <-time.After(2 * time.Second):
			}
			// Channel should now be closed.
			select {
			case _, ok := <-changes33:
				if !ok {
					log.Printf("  Channel closed after cancel (expected)")
				} else {
					log.Printf("  Got signal after cancel (draining...)")
					// May get one pending signal; wait for close.
					select {
					case _, ok2 := <-changes33:
						if !ok2 {
							log.Printf("  Channel closed after draining (expected)")
						} else {
							log.Printf("  FAIL: channel still open after cancel")
							failed++
							passed--
						}
					case <-time.After(2 * time.Second):
						log.Printf("  FAIL: timeout waiting for channel close")
						failed++
						passed--
					}
				}
			case <-time.After(2 * time.Second):
				log.Printf("  FAIL: timeout waiting for channel close after cancel")
				failed++
				passed--
			}
		}
	}

	// --- Test 34: WatchChanges — double call returns error ---
	log.Println("\n--- Test 34: WatchChanges double call returns error ---")
	{
		ctx34a, cancel34a := context.WithCancel(context.Background())
		defer cancel34a()
		changes34, err := client.WatchChanges(ctx34a)
		check("WatchChanges first call", err)
		if err == nil {
			_, err2 := client.WatchChanges(context.Background())
			if err2 != nil {
				log.Printf("  Second call returned error (expected): %v", err2)
			} else {
				log.Printf("  FAIL: second call should have returned error")
				failed++
				passed--
			}
			cancel34a()
			// Drain to let goroutine exit.
			for range changes34 {
			}
		}
	}

	// --- Test 35: WatchChanges — restart after first watcher stopped ---
	log.Println("\n--- Test 35: WatchChanges restart after stop ---")
	{
		ctx35a, cancel35a := context.WithCancel(context.Background())
		changes35a, err := client.WatchChanges(ctx35a)
		check("WatchChanges first call for restart test", err)
		if err == nil {
			cancel35a()
			for range changes35a {
			} // wait for goroutine to exit

			ctx35b, cancel35b := context.WithCancel(context.Background())
			defer cancel35b()
			_, err2 := client.WatchChanges(ctx35b)
			if err2 != nil {
				log.Printf("  FAIL: restart failed: %v", err2)
				failed++
				passed--
			} else {
				log.Printf("  Restart succeeded (expected)")
				cancel35b()
			}
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
