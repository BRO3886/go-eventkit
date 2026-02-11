//go:build darwin && integration

// Package main provides performance benchmarks for go-eventkit.
//
// Layer 2: Integration benchmarks measuring real EventKit round-trip times.
// Layer 3: AppleScript comparison showing speedup vs osascript.
//
// Run with: go run -tags integration ./scripts/benchmark.go
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/BRO3886/go-eventkit/calendar"
	"github.com/BRO3886/go-eventkit/reminders"
)

// benchResult holds timing results from a benchmark run.
type benchResult struct {
	name    string
	timings []time.Duration
	min     time.Duration
	median  time.Duration
	p95     time.Duration
	max     time.Duration
}

func runBenchmark(name string, iterations int, fn func() error) benchResult {
	timings := make([]time.Duration, 0, iterations)
	for i := range iterations {
		start := time.Now()
		if err := fn(); err != nil {
			log.Printf("  [%s] iteration %d error: %v", name, i, err)
			continue
		}
		timings = append(timings, time.Since(start))
	}

	if len(timings) == 0 {
		return benchResult{name: name}
	}

	sort.Slice(timings, func(i, j int) bool { return timings[i] < timings[j] })

	p95Idx := int(float64(len(timings)) * 0.95)
	if p95Idx >= len(timings) {
		p95Idx = len(timings) - 1
	}

	return benchResult{
		name:    name,
		timings: timings,
		min:     timings[0],
		median:  timings[len(timings)/2],
		p95:     timings[p95Idx],
		max:     timings[len(timings)-1],
	}
}

func benchAppleScript(name string, script string, iterations int) benchResult {
	// Warmup: 1 iteration.
	exec.Command("osascript", "-e", script).Output()

	timings := make([]time.Duration, 0, iterations)
	for i := range iterations {
		start := time.Now()
		cmd := exec.Command("osascript", "-e", script)
		if _, err := cmd.Output(); err != nil {
			log.Printf("  [%s] iteration %d error: %v", name, i, err)
			continue
		}
		timings = append(timings, time.Since(start))
	}

	if len(timings) == 0 {
		return benchResult{name: name}
	}

	sort.Slice(timings, func(i, j int) bool { return timings[i] < timings[j] })

	p95Idx := int(float64(len(timings)) * 0.95)
	if p95Idx >= len(timings) {
		p95Idx = len(timings) - 1
	}

	return benchResult{
		name:    name,
		timings: timings,
		min:     timings[0],
		median:  timings[len(timings)/2],
		p95:     timings[p95Idx],
		max:     timings[len(timings)-1],
	}
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "N/A"
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.1fµs", float64(d.Microseconds()))
	}
	return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000.0)
}

func passLabel(median time.Duration) string {
	if median == 0 {
		return "N/A"
	}
	if median <= 200*time.Millisecond {
		return "PASS"
	}
	return "SLOW"
}

func printResult(r benchResult) {
	fmt.Printf("  %-40s | %8s | %8s | %8s | %8s | %s\n",
		r.name,
		formatDuration(r.min),
		formatDuration(r.median),
		formatDuration(r.p95),
		formatDuration(r.max),
		passLabel(r.median),
	)
}

func printComparisonHeader() {
	fmt.Printf("  %-30s | %12s | %12s | %8s\n",
		"Operation", "go-eventkit", "AppleScript", "Speedup")
	fmt.Printf("  %s\n", strings.Repeat("-", 72))
}

func printComparison(name string, goResult, asResult benchResult) {
	speedup := "N/A"
	if goResult.median > 0 && asResult.median > 0 {
		speedup = fmt.Sprintf("%.0fx", float64(asResult.median)/float64(goResult.median))
	}
	fmt.Printf("  %-30s | %12s | %12s | %8s\n",
		name,
		formatDuration(goResult.median),
		formatDuration(asResult.median),
		speedup,
	)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("[benchmark] ")

	const readIterations = 100
	const writeIterations = 50
	const asIterations = 20

	// --- Initialize clients ---
	calClient, err := calendar.New()
	if err != nil {
		log.Fatalf("FATAL: calendar client: %v", err)
	}
	remClient, err := reminders.New()
	if err != nil {
		log.Fatalf("FATAL: reminders client: %v", err)
	}

	// --- Find writable calendar and list ---
	calendars, err := calClient.Calendars()
	if err != nil {
		log.Fatalf("FATAL: list calendars: %v", err)
	}
	var writableCalendar string
	for _, c := range calendars {
		if !c.ReadOnly {
			writableCalendar = c.Title
			break
		}
	}
	if writableCalendar == "" {
		log.Fatal("FATAL: no writable calendar found")
	}
	log.Printf("Using writable calendar: %q", writableCalendar)

	lists, err := remClient.Lists()
	if err != nil {
		log.Fatalf("FATAL: list reminder lists: %v", err)
	}
	var writableList string
	for _, l := range lists {
		if !l.ReadOnly {
			writableList = l.Title
			break
		}
	}
	if writableList == "" {
		log.Fatal("FATAL: no writable reminder list found")
	}
	log.Printf("Using writable list: %q", writableList)

	// ============================================================
	// Layer 2: Integration Benchmarks
	// ============================================================
	fmt.Println("\n=== Layer 2: Integration Benchmarks ===")
	fmt.Printf("  %-40s | %8s | %8s | %8s | %8s | %s\n",
		"Operation", "Min", "Median", "P95", "Max", "Pass?")
	fmt.Printf("  %s\n", strings.Repeat("-", 90))

	// --- Read benchmarks ---

	printResult(runBenchmark("calendar.Calendars()", readIterations, func() error {
		_, err := calClient.Calendars()
		return err
	}))

	now := time.Now()
	printResult(runBenchmark("calendar.Events(7 days)", readIterations, func() error {
		_, err := calClient.Events(now, now.Add(7*24*time.Hour))
		return err
	}))

	printResult(runBenchmark("calendar.Events(30 days)", readIterations, func() error {
		_, err := calClient.Events(now, now.Add(30*24*time.Hour))
		return err
	}))

	printResult(runBenchmark("calendar.Events(365 days)", readIterations, func() error {
		_, err := calClient.Events(now, now.Add(365*24*time.Hour))
		return err
	}))

	printResult(runBenchmark("reminders.Lists()", readIterations, func() error {
		_, err := remClient.Lists()
		return err
	}))

	printResult(runBenchmark("reminders.Reminders() [all]", readIterations, func() error {
		_, err := remClient.Reminders()
		return err
	}))

	printResult(runBenchmark("reminders.Reminders(WithList)", readIterations, func() error {
		_, err := remClient.Reminders(reminders.WithList(writableList))
		return err
	}))

	printResult(runBenchmark("reminders.Reminders(incomplete)", readIterations, func() error {
		_, err := remClient.Reminders(reminders.WithCompleted(false))
		return err
	}))

	// --- Write benchmarks: Calendar ---

	fmt.Printf("  %s\n", strings.Repeat("-", 90))

	// CreateEvent: create N events, cleanup after.
	var createEventIDs []string
	printResult(runBenchmark("calendar.CreateEvent()", writeIterations, func() error {
		start := time.Now().Add(24 * time.Hour)
		evt, err := calClient.CreateEvent(calendar.CreateEventInput{
			Title:     "[bench] Create Event",
			StartDate: start,
			EndDate:   start.Add(30 * time.Minute),
			Calendar:  writableCalendar,
		})
		if err == nil {
			createEventIDs = append(createEventIDs, evt.ID)
		}
		return err
	}))

	// Event by ID: pre-create 1 event, fetch by ID N times, cleanup.
	preEvent, err := calClient.CreateEvent(calendar.CreateEventInput{
		Title:     "[bench] Fetch By ID",
		StartDate: now.Add(24 * time.Hour),
		EndDate:   now.Add(25 * time.Hour),
		Calendar:  writableCalendar,
	})
	if err != nil {
		log.Printf("  WARN: pre-create for Event(ID) failed: %v", err)
	} else {
		printResult(runBenchmark("calendar.Event(id)", readIterations, func() error {
			_, err := calClient.Event(preEvent.ID)
			return err
		}))
	}

	// UpdateEvent: pre-create 1 event, update title N times, cleanup.
	updateEvent, err := calClient.CreateEvent(calendar.CreateEventInput{
		Title:     "[bench] Update Event",
		StartDate: now.Add(24 * time.Hour),
		EndDate:   now.Add(25 * time.Hour),
		Calendar:  writableCalendar,
	})
	if err != nil {
		log.Printf("  WARN: pre-create for UpdateEvent failed: %v", err)
	} else {
		counter := 0
		printResult(runBenchmark("calendar.UpdateEvent()", writeIterations, func() error {
			counter++
			title := fmt.Sprintf("[bench] Updated %d", counter)
			_, err := calClient.UpdateEvent(updateEvent.ID, calendar.UpdateEventInput{
				Title: &title,
			}, calendar.SpanThisEvent)
			return err
		}))
	}

	// DeleteEvent: pre-create N events, delete one per iteration.
	var deleteEventIDs []string
	for range writeIterations {
		start := now.Add(48 * time.Hour)
		evt, err := calClient.CreateEvent(calendar.CreateEventInput{
			Title:     "[bench] Delete Event",
			StartDate: start,
			EndDate:   start.Add(30 * time.Minute),
			Calendar:  writableCalendar,
		})
		if err != nil {
			log.Printf("  WARN: pre-create for DeleteEvent failed: %v", err)
			continue
		}
		deleteEventIDs = append(deleteEventIDs, evt.ID)
	}
	idx := 0
	printResult(runBenchmark("calendar.DeleteEvent()", len(deleteEventIDs), func() error {
		if idx >= len(deleteEventIDs) {
			return fmt.Errorf("no more events to delete")
		}
		err := calClient.DeleteEvent(deleteEventIDs[idx], calendar.SpanFutureEvents)
		idx++
		return err
	}))

	// --- Write benchmarks: Reminders ---

	fmt.Printf("  %s\n", strings.Repeat("-", 90))

	// CreateReminder.
	var createReminderIDs []string
	printResult(runBenchmark("reminders.CreateReminder()", writeIterations, func() error {
		r, err := remClient.CreateReminder(reminders.CreateReminderInput{
			Title:    "[bench] Create Reminder",
			ListName: writableList,
		})
		if err == nil {
			createReminderIDs = append(createReminderIDs, r.ID)
		}
		return err
	}))

	// Reminder by ID.
	preReminder, err := remClient.CreateReminder(reminders.CreateReminderInput{
		Title:    "[bench] Fetch By ID",
		ListName: writableList,
	})
	if err != nil {
		log.Printf("  WARN: pre-create for Reminder(ID) failed: %v", err)
	} else {
		printResult(runBenchmark("reminders.Reminder(id)", readIterations, func() error {
			_, err := remClient.Reminder(preReminder.ID)
			return err
		}))
	}

	// CompleteReminder: pre-create 1, complete/uncomplete N times.
	completeReminder, err := remClient.CreateReminder(reminders.CreateReminderInput{
		Title:    "[bench] Complete Toggle",
		ListName: writableList,
	})
	if err != nil {
		log.Printf("  WARN: pre-create for CompleteReminder failed: %v", err)
	} else {
		printResult(runBenchmark("reminders.CompleteReminder()", writeIterations, func() error {
			_, err := remClient.CompleteReminder(completeReminder.ID)
			return err
		}))

		printResult(runBenchmark("reminders.UncompleteReminder()", writeIterations, func() error {
			_, err := remClient.UncompleteReminder(completeReminder.ID)
			return err
		}))
	}

	// DeleteReminder: pre-create N, delete one per iteration.
	var deleteReminderIDs []string
	for range writeIterations {
		r, err := remClient.CreateReminder(reminders.CreateReminderInput{
			Title:    "[bench] Delete Reminder",
			ListName: writableList,
		})
		if err != nil {
			log.Printf("  WARN: pre-create for DeleteReminder failed: %v", err)
			continue
		}
		deleteReminderIDs = append(deleteReminderIDs, r.ID)
	}
	ridx := 0
	printResult(runBenchmark("reminders.DeleteReminder()", len(deleteReminderIDs), func() error {
		if ridx >= len(deleteReminderIDs) {
			return fmt.Errorf("no more reminders to delete")
		}
		err := remClient.DeleteReminder(deleteReminderIDs[ridx])
		ridx++
		return err
	}))

	// ============================================================
	// Layer 3: AppleScript Comparison
	// ============================================================
	fmt.Println("\n=== Layer 3: AppleScript Comparison ===")
	printComparisonHeader()

	// Fetch calendars.
	goCalendars := runBenchmark("go-eventkit: Calendars", asIterations, func() error {
		_, err := calClient.Calendars()
		return err
	})
	asCalendars := benchAppleScript("osascript: Calendars", `tell application "Calendar" to get name of every calendar`, asIterations)
	printComparison("Fetch calendars", goCalendars, asCalendars)

	// Fetch events.
	goEvents := runBenchmark("go-eventkit: Events", asIterations, func() error {
		_, err := calClient.Events(now, now.Add(7*24*time.Hour))
		return err
	})
	asEvents := benchAppleScript("osascript: Events", `tell application "Calendar" to get summary of every event of calendar 1`, asIterations)
	printComparison("Fetch events (7 days)", goEvents, asEvents)

	// Fetch reminders.
	goReminders := runBenchmark("go-eventkit: Reminders", asIterations, func() error {
		_, err := remClient.Reminders()
		return err
	})
	asReminders := benchAppleScript("osascript: Reminders", `tell application "Reminders" to get name of every reminder`, asIterations)
	printComparison("Fetch reminders", goReminders, asReminders)

	// ============================================================
	// Cleanup
	// ============================================================
	fmt.Println("\n--- Cleanup ---")
	cleanupCount := 0

	for _, id := range createEventIDs {
		if err := calClient.DeleteEvent(id, calendar.SpanFutureEvents); err == nil {
			cleanupCount++
		}
	}
	if preEvent != nil {
		if err := calClient.DeleteEvent(preEvent.ID, calendar.SpanFutureEvents); err == nil {
			cleanupCount++
		}
	}
	if updateEvent != nil {
		if err := calClient.DeleteEvent(updateEvent.ID, calendar.SpanFutureEvents); err == nil {
			cleanupCount++
		}
	}

	for _, id := range createReminderIDs {
		if err := remClient.DeleteReminder(id); err == nil {
			cleanupCount++
		}
	}
	if preReminder != nil {
		if err := remClient.DeleteReminder(preReminder.ID); err == nil {
			cleanupCount++
		}
	}
	if completeReminder != nil {
		if err := remClient.DeleteReminder(completeReminder.ID); err == nil {
			cleanupCount++
		}
	}

	fmt.Printf("  Cleaned up %d test items\n", cleanupCount)

	fmt.Println("\nDone.")
	os.Exit(0)
}
