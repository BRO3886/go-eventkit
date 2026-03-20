//go:build darwin && integration

// Package main provides a concurrency smoke test for the inline error returns refactor.
// It exercises error paths from multiple goroutines simultaneously to verify that
// error messages are returned correctly and not lost or swapped between threads.
//
// Run with: go run -tags integration ./scripts/integration_concurrent.go
package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/BRO3886/go-eventkit/calendar"
	"github.com/BRO3886/go-eventkit/reminders"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("[concurrent] ")

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

	// --- Setup ---
	calClient, err := calendar.New()
	if err != nil {
		log.Fatalf("FATAL: Failed to create calendar client: %v", err)
	}
	remClient, err := reminders.New()
	if err != nil {
		log.Fatalf("FATAL: Failed to create reminders client: %v", err)
	}

	// --- Test 1: Concurrent "not found" errors (calendar) ---
	// 20 goroutines all request a nonexistent event simultaneously.
	// With the old TLS pattern, some would get "unknown error" instead of "not found".
	log.Println("\n--- Test 1: Concurrent calendar Event(nonexistent) ---")
	{
		const n = 20
		var wg sync.WaitGroup
		errs := make([]error, n)
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func(idx int) {
				defer wg.Done()
				_, errs[idx] = calClient.Event(fmt.Sprintf("nonexistent-id-%d", idx))
			}(i)
		}
		wg.Wait()

		allCorrect := true
		for i, e := range errs {
			if e == nil {
				log.Printf("  goroutine %d: got nil error (expected error)", i)
				allCorrect = false
			} else if !strings.Contains(e.Error(), "not found") {
				log.Printf("  goroutine %d: wrong error: %v", i, e)
				allCorrect = false
			}
		}
		if allCorrect {
			check("Concurrent Event(nonexistent) — all 20 got 'not found'", nil)
		} else {
			check("Concurrent Event(nonexistent)", fmt.Errorf("some goroutines got wrong error"))
		}
	}

	// --- Test 2: Concurrent "not found" errors (reminders) ---
	log.Println("\n--- Test 2: Concurrent reminders Reminder(nonexistent) ---")
	{
		const n = 20
		var wg sync.WaitGroup
		errs := make([]error, n)
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func(idx int) {
				defer wg.Done()
				_, errs[idx] = remClient.Reminder(fmt.Sprintf("nonexistent-id-%d", idx))
			}(i)
		}
		wg.Wait()

		allCorrect := true
		for i, e := range errs {
			if e == nil {
				log.Printf("  goroutine %d: got nil error (expected error)", i)
				allCorrect = false
			} else if !strings.Contains(e.Error(), "not found") {
				log.Printf("  goroutine %d: wrong error: %v", i, e)
				allCorrect = false
			}
		}
		if allCorrect {
			check("Concurrent Reminder(nonexistent) — all 20 got 'not found'", nil)
		} else {
			check("Concurrent Reminder(nonexistent)", fmt.Errorf("some goroutines got wrong error"))
		}
	}

	// --- Test 3: Concurrent reads (calendar) ---
	// 10 goroutines fetch calendars simultaneously — should all succeed.
	log.Println("\n--- Test 3: Concurrent Calendars() reads ---")
	{
		const n = 10
		var wg sync.WaitGroup
		results := make([]int, n)
		errs := make([]error, n)
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func(idx int) {
				defer wg.Done()
				cals, err := calClient.Calendars()
				errs[idx] = err
				results[idx] = len(cals)
			}(i)
		}
		wg.Wait()

		allCorrect := true
		for i, e := range errs {
			if e != nil {
				log.Printf("  goroutine %d: error: %v", i, e)
				allCorrect = false
			}
		}
		// All should return the same count
		if allCorrect && results[0] > 0 {
			for i := 1; i < n; i++ {
				if results[i] != results[0] {
					log.Printf("  goroutine %d: got %d calendars, expected %d", i, results[i], results[0])
					allCorrect = false
				}
			}
		}
		if allCorrect {
			check(fmt.Sprintf("Concurrent Calendars() — all 10 returned %d calendars", results[0]), nil)
		} else {
			check("Concurrent Calendars()", fmt.Errorf("inconsistent results"))
		}
	}

	// --- Test 4: Concurrent reads (reminders) ---
	log.Println("\n--- Test 4: Concurrent Lists() reads ---")
	{
		const n = 10
		var wg sync.WaitGroup
		results := make([]int, n)
		errs := make([]error, n)
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func(idx int) {
				defer wg.Done()
				lists, err := remClient.Lists()
				errs[idx] = err
				results[idx] = len(lists)
			}(i)
		}
		wg.Wait()

		allCorrect := true
		for i, e := range errs {
			if e != nil {
				log.Printf("  goroutine %d: error: %v", i, e)
				allCorrect = false
			}
		}
		if allCorrect && results[0] > 0 {
			for i := 1; i < n; i++ {
				if results[i] != results[0] {
					log.Printf("  goroutine %d: got %d lists, expected %d", i, results[i], results[0])
					allCorrect = false
				}
			}
		}
		if allCorrect {
			check(fmt.Sprintf("Concurrent Lists() — all 10 returned %d lists", results[0]), nil)
		} else {
			check("Concurrent Lists()", fmt.Errorf("inconsistent results"))
		}
	}

	// --- Test 5: Mixed concurrent reads + error paths ---
	log.Println("\n--- Test 5: Mixed concurrent operations ---")
	{
		const n = 30
		var wg sync.WaitGroup
		errs := make([]error, n)
		wg.Add(n)

		now := time.Now()
		weekLater := now.Add(7 * 24 * time.Hour)

		for i := 0; i < n; i++ {
			go func(idx int) {
				defer wg.Done()
				switch idx % 3 {
				case 0:
					// Read calendars
					_, errs[idx] = calClient.Calendars()
				case 1:
					// Read events
					_, errs[idx] = calClient.Events(now, weekLater)
				case 2:
					// Error path: nonexistent event
					_, err := calClient.Event(fmt.Sprintf("mixed-nonexistent-%d", idx))
					if err != nil && strings.Contains(err.Error(), "not found") {
						errs[idx] = nil // expected error
					} else if err == nil {
						errs[idx] = fmt.Errorf("expected error, got nil")
					} else {
						errs[idx] = fmt.Errorf("wrong error: %v", err)
					}
				}
			}(i)
		}
		wg.Wait()

		allCorrect := true
		for i, e := range errs {
			if e != nil {
				log.Printf("  goroutine %d (op %d): %v", i, i%3, e)
				allCorrect = false
			}
		}
		if allCorrect {
			check("Mixed concurrent (10 reads + 10 events + 10 errors) — all correct", nil)
		} else {
			check("Mixed concurrent operations", fmt.Errorf("some operations failed"))
		}
	}

	// --- Results ---
	fmt.Printf("\n=== Concurrent Test Results ===\n")
	fmt.Printf("Passed: %d\nFailed: %d\nTotal:  %d\n", passed, failed, passed+failed)
	if failed > 0 {
		os.Exit(1)
	}
}
