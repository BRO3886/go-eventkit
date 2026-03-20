//go:build darwin && integration

// Package main provides an integration test script for the reminders package.
// It exercises the real EventKit bridge against live macOS Reminders data.
//
// Run with: go run -tags integration ./scripts/integration_reminders.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/BRO3886/go-eventkit"
	"github.com/BRO3886/go-eventkit/reminders"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("[reminders-integration] ")

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
	client, err := reminders.New()
	if err != nil {
		log.Fatalf("FATAL: Failed to create client (TCC denied?): %v", err)
	}
	log.Println("PASS: Client created successfully")
	passed++

	// --- Test 2: List all reminder lists ---
	lists, err := client.Lists()
	check("List all reminder lists", err)
	if err == nil {
		log.Printf("  Found %d lists:", len(lists))
		for _, l := range lists {
			log.Printf("    - %s (ID: %s, Source: %s, Count: %d, ReadOnly: %v)",
				l.Title, truncateID(l.ID), l.Source, l.Count, l.ReadOnly)
		}
	}

	// --- Test 3: Get all reminders (no filter) ---
	allReminders, err := client.Reminders()
	check("Fetch all reminders (no filter)", err)
	if err == nil {
		log.Printf("  Found %d total reminders", len(allReminders))
	}

	// --- Test 4: Get incomplete reminders only ---
	incompleteReminders, err := client.Reminders(reminders.WithCompleted(false))
	check("Fetch incomplete reminders only", err)
	if err == nil {
		log.Printf("  Found %d incomplete reminders", len(incompleteReminders))
		for _, r := range incompleteReminders {
			if r.Completed {
				log.Printf("  FAIL: Reminder %q is completed but filter was incomplete-only", r.Title)
				failed++
				break
			}
		}
	}

	// --- Test 5: Get completed reminders only ---
	completedReminders, err := client.Reminders(reminders.WithCompleted(true))
	check("Fetch completed reminders only", err)
	if err == nil {
		log.Printf("  Found %d completed reminders", len(completedReminders))
		for _, r := range completedReminders {
			if !r.Completed {
				log.Printf("  FAIL: Reminder %q is not completed but filter was completed-only", r.Title)
				failed++
				break
			}
		}
	}

	// --- Determine default list name ---
	defaultList := "Reminders"
	if len(lists) > 0 {
		defaultList = lists[0].Title
	}
	log.Printf("  Using default list: %q", defaultList)

	// --- Test 6: Create a reminder ---
	dueDate := time.Now().Add(48 * time.Hour)
	created, err := client.CreateReminder(reminders.CreateReminderInput{
		Title:    "[go-eventkit test] Integration Test Reminder",
		Notes:    "Created by go-eventkit integration test. Safe to delete.",
		ListName: defaultList,
		DueDate:  &dueDate,
		Priority: reminders.PriorityMedium,
	})
	check("Create reminder in "+defaultList+" list", err)

	var createdID string
	if err == nil {
		createdID = created.ID
		log.Printf("  Created reminder: %q (ID: %s)", created.Title, truncateID(created.ID))
		log.Printf("  List: %s, Priority: %s, Completed: %v", created.List, created.Priority, created.Completed)
		if created.DueDate != nil {
			log.Printf("  DueDate: %v", created.DueDate.Format(time.RFC3339))
		}
	}

	// --- Test 7: Get reminder by ID ---
	if createdID != "" {
		fetched, err := client.Reminder(createdID)
		check("Get reminder by ID", err)
		if err == nil {
			if fetched.Title != created.Title {
				log.Printf("  WARN: Title mismatch: got %q, want %q", fetched.Title, created.Title)
			}
			log.Printf("  Fetched reminder matches: %q", fetched.Title)
		}
	}

	// --- Test 8: Get reminder by ID prefix ---
	if createdID != "" && len(createdID) > 8 {
		prefix := createdID[:8]
		fetched, err := client.Reminder(prefix)
		check("Get reminder by ID prefix", err)
		if err == nil {
			if fetched.ID != createdID {
				log.Printf("  WARN: ID mismatch: got %q, want %q", fetched.ID, createdID)
			}
			log.Printf("  Found reminder by prefix %q: %q", prefix, fetched.Title)
		}
	}

	// --- Test 9: Search reminders ---
	searchResults, err := client.Reminders(reminders.WithSearch("Integration Test"))
	check("Search reminders for 'Integration Test'", err)
	if err == nil {
		log.Printf("  Found %d reminders matching search", len(searchResults))
		found := false
		for _, r := range searchResults {
			if r.ID == createdID {
				found = true
			}
		}
		if createdID != "" && !found {
			log.Printf("  WARN: Created reminder not found in search results")
		}
	}

	// --- Test 10: Filter by list name ---
	if len(lists) > 0 {
		listName := lists[0].Title
		listReminders, err := client.Reminders(reminders.WithList(listName))
		check(fmt.Sprintf("Filter reminders by list (%s)", listName), err)
		if err == nil {
			log.Printf("  Found %d reminders in %q", len(listReminders), listName)
			for _, r := range listReminders {
				if r.List != listName {
					log.Printf("  FAIL: Reminder %q is in list %q, expected %q", r.Title, r.List, listName)
					failed++
					break
				}
			}
		}
	}

	// --- Test 11: Update reminder ---
	if createdID != "" {
		newTitle := "[go-eventkit test] Updated Reminder"
		newNotes := "Updated by integration test"
		newPriority := reminders.PriorityHigh

		updated, err := client.UpdateReminder(createdID, reminders.UpdateReminderInput{
			Title:    &newTitle,
			Notes:    &newNotes,
			Priority: &newPriority,
		})
		check("Update reminder", err)
		if err == nil {
			if updated.Title != newTitle {
				log.Printf("  FAIL: Title not updated: got %q, want %q", updated.Title, newTitle)
				failed++
			} else {
				log.Printf("  Updated reminder title to: %q", updated.Title)
			}
			log.Printf("  Updated priority: %s", updated.Priority)
		}
	}

	// --- Test 12: Complete reminder ---
	if createdID != "" {
		completed, err := client.CompleteReminder(createdID)
		check("Complete reminder", err)
		if err == nil {
			if !completed.Completed {
				log.Printf("  FAIL: Reminder not marked as completed")
				failed++
			} else {
				log.Printf("  Reminder completed: %v, CompletionDate: %v", completed.Completed, completed.CompletionDate)
			}
		}
	}

	// --- Test 13: Uncomplete reminder ---
	if createdID != "" {
		uncompleted, err := client.UncompleteReminder(createdID)
		check("Uncomplete reminder", err)
		if err == nil {
			if uncompleted.Completed {
				log.Printf("  FAIL: Reminder still marked as completed")
				failed++
			} else {
				log.Printf("  Reminder uncompleted: %v", uncompleted.Completed)
			}
		}
	}

	// --- Test 14: Create reminder with alarm ---
	alarmDate := time.Now().Add(72 * time.Hour)
	alarmReminder, err := client.CreateReminder(reminders.CreateReminderInput{
		Title:    "[go-eventkit test] Alarm Reminder",
		ListName: defaultList,
		Notes:    "Created by go-eventkit integration test. Safe to delete.",
		Alarms: []reminders.Alarm{
			{AbsoluteDate: &alarmDate},
		},
	})
	check("Create reminder with alarm", err)

	var alarmReminderID string
	if err == nil {
		alarmReminderID = alarmReminder.ID
		log.Printf("  Created alarm reminder: %q, HasAlarms=%v, Alarms=%d",
			alarmReminder.Title, alarmReminder.HasAlarms, len(alarmReminder.Alarms))
	}

	// --- Test 15: Create reminder with URL ---
	urlReminder, err := client.CreateReminder(reminders.CreateReminderInput{
		Title:    "[go-eventkit test] URL Reminder",
		ListName: defaultList,
		URL:      "https://example.com/test",
		Notes:    "Created by go-eventkit integration test. Safe to delete.",
	})
	check("Create reminder with URL", err)

	var urlReminderID string
	if err == nil {
		urlReminderID = urlReminder.ID
		log.Printf("  Created URL reminder: %q, URL=%s", urlReminder.Title, urlReminder.URL)
	}

	// --- Test 16: Create reminder with relative offset alarm ---
	relAlarmReminder, err := client.CreateReminder(reminders.CreateReminderInput{
		Title:    "[go-eventkit test] Relative Alarm",
		ListName: defaultList,
		DueDate:  &dueDate,
		Notes:    "Created by go-eventkit integration test. Safe to delete.",
		Alarms: []reminders.Alarm{
			{RelativeOffset: -30 * time.Minute},
		},
	})
	check("Create reminder with relative offset alarm", err)

	var relAlarmID string
	if err == nil {
		relAlarmID = relAlarmReminder.ID
		log.Printf("  Created relative alarm reminder: %q", relAlarmReminder.Title)
	}

	// --- Test 17: Filter by due date range ---
	now := time.Now()
	futureDate := now.Add(96 * time.Hour)
	dueDateReminders, err := client.Reminders(reminders.WithDueAfter(now), reminders.WithDueBefore(futureDate))
	check("Filter reminders by due date range", err)
	if err == nil {
		log.Printf("  Found %d reminders due in next 96 hours", len(dueDateReminders))
	}

	// --- Test 18: Get non-existent reminder ---
	_, err = client.Reminder("non-existent-reminder-id-12345")
	if err != nil {
		log.Printf("PASS: Get non-existent reminder returns error: %v", err)
		passed++
	} else {
		log.Printf("FAIL: Get non-existent reminder should return error")
		failed++
	}

	// --- Test 20: Create reminder with daily recurrence ---
	recDailyReminder, err := client.CreateReminder(reminders.CreateReminderInput{
		Title:    "[go-eventkit test] Daily Recurring",
		ListName: defaultList,
		DueDate:  &dueDate,
		Notes:    "Created by go-eventkit integration test. Safe to delete.",
		RecurrenceRules: []eventkit.RecurrenceRule{
			eventkit.Daily(1).Count(5),
		},
	})
	check("Create reminder with daily recurrence", err)

	var recDailyID string
	if err == nil {
		recDailyID = recDailyReminder.ID
		log.Printf("  Created recurring reminder: %q, Recurring=%v, Rules=%d",
			recDailyReminder.Title, recDailyReminder.Recurring, len(recDailyReminder.RecurrenceRules))
		if !recDailyReminder.Recurring {
			log.Printf("  FAIL: Expected recurring=true")
			failed++
		} else {
			passed++
		}
		if len(recDailyReminder.RecurrenceRules) != 1 {
			log.Printf("  FAIL: Expected 1 recurrence rule, got %d", len(recDailyReminder.RecurrenceRules))
		} else if recDailyReminder.RecurrenceRules[0].Frequency != eventkit.FrequencyDaily {
			log.Printf("  FAIL: Expected daily frequency, got %d", recDailyReminder.RecurrenceRules[0].Frequency)
		}
	}

	// --- Test 21: Create reminder with weekly recurrence ---
	endDate := time.Now().Add(90 * 24 * time.Hour)
	recWeeklyReminder, err := client.CreateReminder(reminders.CreateReminderInput{
		Title:    "[go-eventkit test] Weekly Recurring",
		ListName: defaultList,
		DueDate:  &dueDate,
		Notes:    "Created by go-eventkit integration test. Safe to delete.",
		RecurrenceRules: []eventkit.RecurrenceRule{
			eventkit.Weekly(2, eventkit.Monday, eventkit.Friday).Until(endDate),
		},
	})
	check("Create reminder with weekly recurrence", err)

	var recWeeklyID string
	if err == nil {
		recWeeklyID = recWeeklyReminder.ID
		log.Printf("  Created weekly recurring reminder: %q", recWeeklyReminder.Title)
		if len(recWeeklyReminder.RecurrenceRules) == 1 {
			rule := recWeeklyReminder.RecurrenceRules[0]
			if rule.Frequency != eventkit.FrequencyWeekly {
				log.Printf("  FAIL: Expected weekly frequency")
				failed++
			} else {
				passed++
			}
			if rule.Interval != 2 {
				log.Printf("  FAIL: Expected interval=2, got %d", rule.Interval)
			}
		}
	}

	// --- Test 22: Update reminder to add recurrence ---
	if createdID != "" {
		addRules := []eventkit.RecurrenceRule{eventkit.Daily(1).Count(3)}
		updatedRec, err := client.UpdateReminder(createdID, reminders.UpdateReminderInput{
			RecurrenceRules: &addRules,
		})
		check("Update reminder: add recurrence rule", err)
		if err == nil {
			log.Printf("  Updated reminder with recurrence: Recurring=%v, Rules=%d",
				updatedRec.Recurring, len(updatedRec.RecurrenceRules))
		}
	}

	// --- Test 23: Update reminder to remove recurrence ---
	if createdID != "" {
		emptyRules := []eventkit.RecurrenceRule{}
		updatedNoRec, err := client.UpdateReminder(createdID, reminders.UpdateReminderInput{
			RecurrenceRules: &emptyRules,
		})
		check("Update reminder: remove recurrence rules", err)
		if err == nil {
			log.Printf("  Removed recurrence: Recurring=%v, Rules=%d",
				updatedNoRec.Recurring, len(updatedNoRec.RecurrenceRules))
		}
	}

	// --- Test 24: Create reminder list ---
	// Discover a writable source from existing lists.
	var writableSource string
	for _, l := range lists {
		if !l.ReadOnly && l.Source != "" {
			writableSource = l.Source
			break
		}
	}
	testList, err := client.CreateList(reminders.CreateListInput{
		Title:  "[go-eventkit test] Integration List",
		Color:  "#FF6961",
		Source: writableSource,
	})
	check("Create reminder list", err)

	var testListID string
	if err == nil {
		testListID = testList.ID
		log.Printf("  Created list: %q (ID: %s)", testList.Title, truncateID(testList.ID))
		log.Printf("  Color: %s, Source: %s, Count: %d, ReadOnly: %v", testList.Color, testList.Source, testList.Count, testList.ReadOnly)
	}

	// --- Test 25: Verify new list appears in lists ---
	if testListID != "" {
		allLists, err := client.Lists()
		check("Verify new list in lists", err)
		if err == nil {
			found := false
			for _, l := range allLists {
				if l.ID == testListID {
					found = true
					log.Printf("  Found new list: %q", l.Title)
					break
				}
			}
			if !found {
				log.Printf("  FAIL: New list not found in lists")
				failed++
			}
		}
	}

	// --- Test 26: Update list (rename + recolor) ---
	if testListID != "" {
		newTitle := "[go-eventkit test] Renamed List"
		newColor := "#00FF00"
		updatedList, err := client.UpdateList(testListID, reminders.UpdateListInput{
			Title: &newTitle,
			Color: &newColor,
		})
		check("Update list (rename + recolor)", err)
		if err == nil {
			if updatedList.Title != newTitle {
				log.Printf("  FAIL: Title not updated: got %q, want %q", updatedList.Title, newTitle)
				failed++
			} else {
				log.Printf("  Renamed list to: %q", updatedList.Title)
			}
		}
	}

	// --- Test 27: Create reminder in new list ---
	var listTestReminderID string
	if testListID != "" {
		listTestReminder, err := client.CreateReminder(reminders.CreateReminderInput{
			Title:    "[go-eventkit test] Reminder in New List",
			ListName: "[go-eventkit test] Renamed List",
			Notes:    "Created by go-eventkit integration test. Safe to delete.",
		})
		check("Create reminder in new list", err)
		if err == nil {
			listTestReminderID = listTestReminder.ID
			log.Printf("  Created reminder in new list: %q", listTestReminder.Title)
		}
	}

	// --- Test 28: Delete reminder in new list before deleting list ---
	if listTestReminderID != "" {
		err := client.DeleteReminder(listTestReminderID)
		check("Delete reminder in new list", err)
	}

	// --- Test 29: Delete list ---
	if testListID != "" {
		err := client.DeleteList(testListID)
		check("Delete list", err)
		if err == nil {
			log.Printf("  Deleted list: %s", truncateID(testListID))
		}
	}

	// --- Test 30: Verify deleted list is gone ---
	if testListID != "" {
		allLists, err := client.Lists()
		check("Verify deleted list is gone", err)
		if err == nil {
			found := false
			for _, l := range allLists {
				if l.ID == testListID {
					found = true
				}
			}
			if found {
				log.Printf("  FAIL: Deleted list still in lists")
				failed++
			} else {
				log.Printf("  Deleted list confirmed gone")
			}
		}
	}

	// --- Cleanup: Delete all test reminders ---
	log.Println("\n--- Cleanup ---")
	cleanupIDs := []string{createdID, alarmReminderID, urlReminderID, relAlarmID, recDailyID, recWeeklyID}
	for _, id := range cleanupIDs {
		if id == "" {
			continue
		}
		err := client.DeleteReminder(id)
		if err != nil {
			log.Printf("WARN: Failed to delete reminder %s: %v", truncateID(id), err)
		} else {
			log.Printf("  Deleted reminder: %s", truncateID(id))
		}
	}

	// --- Test 19: Verify deleted reminder is gone ---
	if createdID != "" {
		_, err := client.Reminder(createdID)
		if err != nil {
			log.Printf("PASS: Deleted reminder not found (expected)")
			passed++
		} else {
			log.Printf("FAIL: Deleted reminder still accessible")
			failed++
		}
	}

	// drainWatch drains a watch channel with a bounded timeout.
	drainWatch := func(ch <-chan struct{}, d time.Duration) {
		timer := time.NewTimer(d)
		defer timer.Stop()
		for {
			select {
			case _, ok := <-ch:
				if !ok {
					return
				}
			case <-timer.C:
				return
			}
		}
	}

	// --- Test 31: WatchChanges — signal on reminder write ---
	log.Println("\n--- Test 31: WatchChanges signal on reminder write ---")
	{
		ctx31, cancel31 := context.WithTimeout(context.Background(), 5*time.Second)
		changes31, err := client.WatchChanges(ctx31)
		check("WatchChanges start", err)
		if err == nil {
			// Create a reminder to trigger notification.
			rem31, cerr := client.CreateReminder(reminders.CreateReminderInput{
				Title: "WatchChanges test reminder",
			})
			if cerr != nil {
				log.Printf("WARN: CreateReminder for watch test: %v", cerr)
			} else {
				defer client.DeleteReminder(rem31.ID)
			}
			select {
			case _, ok := <-changes31:
				if ok {
					log.Printf("  Received change signal (expected)")
				} else {
					log.Printf("  FAIL: channel closed before signal")
					failed++
					passed--
				}
			case <-ctx31.Done():
				log.Printf("  FAIL: timeout waiting for change signal")
				failed++
				passed--
			}
			cancel31()
			drainWatch(changes31, 3*time.Second)
		}
	}

	// --- Test 32: WatchChanges — channel closes on ctx cancel ---
	log.Println("\n--- Test 32: WatchChanges channel closes on ctx cancel ---")
	{
		ctx32, cancel32 := context.WithCancel(context.Background())
		changes32, err := client.WatchChanges(ctx32)
		check("WatchChanges start for cancel test", err)
		if err == nil {
			cancel32()
			drainWatch(changes32, 3*time.Second)
			select {
			case _, ok := <-changes32:
				if !ok {
					log.Printf("  Channel closed after cancel (expected)")
				} else {
					log.Printf("  FAIL: channel still open after cancel + drain")
					failed++
					passed--
				}
			default:
				log.Printf("  Channel drained (goroutine may still be exiting)")
			}
		}
	}

	// --- Test 33: WatchChanges — double call returns error ---
	log.Println("\n--- Test 33: WatchChanges double call returns error ---")
	{
		ctx33a, cancel33a := context.WithCancel(context.Background())
		changes33, err := client.WatchChanges(ctx33a)
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
			cancel33a()
			drainWatch(changes33, 3*time.Second)
		}
	}

	// --- Test 34: WatchChanges — restart after first watcher stopped ---
	log.Println("\n--- Test 34: WatchChanges restart after stop ---")
	{
		ctx34a, cancel34a := context.WithCancel(context.Background())
		changes34a, err := client.WatchChanges(ctx34a)
		check("WatchChanges first call for restart test", err)
		if err == nil {
			cancel34a()
			drainWatch(changes34a, 3*time.Second)

			ctx34b, cancel34b := context.WithCancel(context.Background())
			changes34b, err2 := client.WatchChanges(ctx34b)
			if err2 != nil {
				log.Printf("  FAIL: restart failed: %v", err2)
				failed++
				passed--
			} else {
				log.Printf("  Restart succeeded (expected)")
				cancel34b()
				drainWatch(changes34b, 3*time.Second)
			}
		}
	}

	// --- Summary ---
	fmt.Printf("\n=== Reminders Integration Test Results ===\n")
	fmt.Printf("Passed: %d\n", passed)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Total:  %d\n", passed+failed)
	if failed > 0 {
		os.Exit(1)
	}
}

func truncateID(id string) string {
	if len(id) > 8 {
		return id[:8] + "..."
	}
	return id
}
