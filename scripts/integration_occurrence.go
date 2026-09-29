//go:build darwin && integration

// Package main exercises occurrence-targeted writes on recurring events
// against real EventKit. It creates a throwaway iCloud calendar, runs every
// check inside it, and deletes the calendar on exit.
//
// Run with: go run -tags integration ./scripts/integration_occurrence.go
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/BRO3886/go-eventkit"
	"github.com/BRO3886/go-eventkit/calendar"
)

const testCalendar = "ZZ Ical Fork Test"

func main() {
	log.SetFlags(0)
	log.SetPrefix("[occurrence] ")

	client, err := calendar.New()
	if err != nil {
		log.Fatalf("FATAL: client: %v", err)
	}

	cal, err := client.CreateCalendar(calendar.CreateCalendarInput{Title: testCalendar, Source: "iCloud"})
	if err != nil {
		log.Fatalf("FATAL: create calendar: %v", err)
	}
	failed := run(client)
	if err := client.DeleteCalendar(cal.ID); err != nil {
		log.Printf("FAIL: delete test calendar %s: %v", cal.ID, err)
		failed++
	}
	if failed > 0 {
		log.Printf("%d check(s) failed", failed)
		os.Exit(1)
	}
	log.Println("all checks passed")
}

func run(client *calendar.Client) int {
	failed := 0
	check := func(name string, err error) {
		if err != nil {
			log.Printf("FAIL: %s: %v", name, err)
			failed++
		} else {
			log.Printf("PASS: %s", name)
		}
	}

	// Weekly series of 6, starting a week from now at 09:00 local.
	base := time.Now().AddDate(0, 0, 7)
	first := time.Date(base.Year(), base.Month(), base.Day(), 9, 0, 0, 0, time.Local)
	series, err := client.CreateEvent(calendar.CreateEventInput{
		Title:                 "Series",
		StartDate:             first,
		EndDate:               first.Add(time.Hour),
		Calendar:              testCalendar,
		SuppressDefaultAlarms: true,
		RecurrenceRules:       []eventkit.RecurrenceRule{eventkit.Weekly(1).Count(6)},
	})
	if err != nil {
		check("create series", err)
		return failed
	}
	id := series.ID
	week := func(n int) time.Time { return first.AddDate(0, 0, 7*n) }

	list := func() []calendar.Event {
		evs, err := client.Events(first.Add(-time.Hour), week(6), calendar.WithCalendar(testCalendar))
		if err != nil {
			log.Fatalf("FATAL: list: %v", err)
		}
		return evs
	}
	titles := func() map[string]string {
		out := map[string]string{}
		for _, e := range list() {
			out[e.StartDate.Format("01-02")] = e.Title
		}
		return out
	}
	day := func(n int) string { return week(n).Format("01-02") }

	// 1. Update occurrence #3 (index 2) with SpanThisEvent: only it changes.
	_, err = client.UpdateEventOccurrence(id, week(2), calendar.UpdateEventInput{Title: ptr("Third")}, calendar.SpanThisEvent)
	check("update occurrence 3", err)
	got := titles()
	check("only occurrence 3 renamed", expect(got, map[string]string{
		day(0): "Series", day(1): "Series", day(2): "Third", day(3): "Series", day(4): "Series", day(5): "Series",
	}))

	// 2. Detach the first occurrence, then delete occurrence #4 (index 3).
	_, err = client.UpdateEventOccurrence(id, week(0), calendar.UpdateEventInput{Title: ptr("First")}, calendar.SpanThisEvent)
	check("detach occurrence 1", err)
	err = client.DeleteEventOccurrence(id, week(3), calendar.SpanThisEvent)
	check("delete occurrence 4 after detaching 1", err)
	got = titles()
	check("occurrence 4 gone, others intact", expect(got, map[string]string{
		day(0): "First", day(1): "Series", day(2): "Third", day(4): "Series", day(5): "Series",
	}))

	// 3. Deleting an occurrence that no longer exists fails loudly.
	err = client.DeleteEventOccurrence(id, week(3), calendar.SpanThisEvent)
	if !errors.Is(err, calendar.ErrNotFound) {
		check("delete missing occurrence returns ErrNotFound", fmt.Errorf("got %v", err))
	} else {
		check("delete missing occurrence returns ErrNotFound", nil)
	}

	// 4. SpanFutureEvents from occurrence #5 leaves earlier ones alone.
	_, err = client.UpdateEventOccurrence(id, week(4), calendar.UpdateEventInput{Title: ptr("Later")}, calendar.SpanFutureEvents)
	check("update future from occurrence 5", err)
	got = titles()
	check("future span starts at occurrence 5", expect(got, map[string]string{
		day(0): "First", day(1): "Series", day(2): "Third", day(4): "Later", day(5): "Later",
	}))

	return failed
}

func expect(got, want map[string]string) error {
	if len(got) != len(want) {
		return fmt.Errorf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			return fmt.Errorf("got %v, want %v", got, want)
		}
	}
	return nil
}

func ptr[T any](v T) *T { return &v }
