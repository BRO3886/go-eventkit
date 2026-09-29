//go:build darwin && integration

// Package main is an integration check for all-day due dates against live
// macOS Reminders data. It works only inside a throwaway list, which it
// creates and deletes (it refuses to run if the list already exists).
//
// Reminders.app keeps two flags for an all-day due date: ZALLDAY (what
// EventKit's dueDateComponents reflect) and ZDISPLAYDATEISALLDAY (what the
// app uses to display it, e.g. as overdue). The second is recomputed only
// when the due value changes. When the Reminders store is readable, this
// script checks both flags with read-only sqlite.
//
// Run with: go run -tags integration ./scripts/integration_allday.go
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BRO3886/go-eventkit"
	"github.com/BRO3886/go-eventkit/reminders"
)

const testList = "ZZ Rem Fork Test"

func main() {
	log.SetFlags(0)
	log.SetPrefix("[allday-integration] ")
	// Registered first, so it runs last: after the list cleanup below.
	// Failures after the list exists use log.Panicf so the cleanup still runs.
	defer func() {
		if r := recover(); r != nil {
			exitCode = 1
		}
		os.Exit(exitCode)
	}()

	client, err := reminders.New()
	if err != nil {
		log.Fatalf("FATAL: %v", err)
	}

	lists, err := client.Lists()
	if err != nil {
		log.Fatalf("FATAL: %v", err)
	}
	source := ""
	for _, l := range lists {
		if l.Title == testList {
			log.Fatalf("FATAL: list %q already exists; delete it first", testList)
		}
		if source == "" && !l.ReadOnly && l.Source != "" {
			source = l.Source
		}
	}
	list, err := client.CreateList(reminders.CreateListInput{Title: testList, Source: source})
	if err != nil {
		log.Fatalf("FATAL: create list: %v", err)
	}
	defer func() {
		if err := client.DeleteList(list.ID); err != nil {
			log.Printf("WARN: delete list: %v", err)
		}
		lists, _ := client.Lists()
		for _, l := range lists {
			if l.Title == testList {
				log.Printf("FAIL: list %q still exists after delete", testList)
				exitCode = 1
				return
			}
		}
		log.Printf("cleanup: list %q deleted", testList)
	}()

	failed := 0
	check := func(name string, ok bool, detail string) {
		if ok {
			log.Printf("PASS: %s", name)
		} else {
			log.Printf("FAIL: %s: %s", name, detail)
			failed++
		}
	}

	now := time.Now()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, 1)

	// --- Broken state: timed midnight, then AppleScript all-day on the same date ---
	broken, err := client.CreateReminder(reminders.CreateReminderInput{Title: "broken", ListName: testList, DueDate: &day})
	if err != nil {
		log.Panicf("FATAL: create: %v", err)
	}
	if err := setAllDayViaAppleScript("broken", day); err != nil {
		log.Panicf("FATAL: AppleScript: %v", err)
	}
	a, d, known := storeFlags("broken")
	if known {
		check("AppleScript reproduces the broken state (1/0)", a == 1 && d == 0, fmt.Sprintf("store flags %d/%d", a, d))
	}
	r, err := client.Reminder(broken.ID)
	if err != nil {
		log.Panicf("FATAL: read: %v", err)
	}
	check("broken item does not read as all-day", !r.DueDateAllDay, "DueDateAllDay = true, but the app displays it as timed")

	// --- Repair: set all-day on the same date ---
	r, err = client.UpdateReminder(broken.ID, reminders.UpdateReminderInput{DueDate: &day, DueDateAllDay: true})
	if err != nil {
		log.Panicf("FATAL: update: %v", err)
	}
	check("repaired item reads as all-day", r.DueDateAllDay, "DueDateAllDay = false")
	check("repaired item keeps its date", r.DueDate != nil && sameDay(*r.DueDate, day), fmt.Sprintf("DueDate = %v", r.DueDate))
	if a, d, known := storeFlags("broken"); known {
		check("repair sets both store flags (1/1)", a == 1 && d == 1, fmt.Sprintf("store flags %d/%d", a, d))
	}

	// --- A healthy all-day item re-set to the same date stays healthy ---
	healthy, err := client.CreateReminder(reminders.CreateReminderInput{Title: "healthy", ListName: testList, DueDate: &day, DueDateAllDay: true})
	if err != nil {
		log.Panicf("FATAL: create: %v", err)
	}
	r, err = client.UpdateReminder(healthy.ID, reminders.UpdateReminderInput{DueDate: &day, DueDateAllDay: true})
	if err != nil {
		log.Panicf("FATAL: update: %v", err)
	}
	check("healthy all-day item stays all-day on the same date", r.DueDateAllDay && r.DueDate != nil && sameDay(*r.DueDate, day), fmt.Sprintf("%v %v", r.DueDateAllDay, r.DueDate))
	if a, d, known := storeFlags("healthy"); known {
		check("healthy item store flags (1/1)", a == 1 && d == 1, fmt.Sprintf("store flags %d/%d", a, d))
	}

	// --- Recurring broken item repairs too (it can never lose its due date) ---
	weekly := []eventkit.RecurrenceRule{eventkit.Weekly(1)}
	rec, err := client.CreateReminder(reminders.CreateReminderInput{Title: "recurring", ListName: testList, DueDate: &day, RecurrenceRules: weekly})
	if err != nil {
		log.Panicf("FATAL: create: %v", err)
	}
	if err := setAllDayViaAppleScript("recurring", day); err != nil {
		log.Panicf("FATAL: AppleScript: %v", err)
	}
	r, err = client.UpdateReminder(rec.ID, reminders.UpdateReminderInput{DueDate: &day, DueDateAllDay: true})
	if err != nil {
		log.Panicf("FATAL: update recurring: %v", err)
	}
	check("recurring item repaired and still recurring", r.DueDateAllDay && r.Recurring, fmt.Sprintf("allDay=%v recurring=%v", r.DueDateAllDay, r.Recurring))
	if a, d, known := storeFlags("recurring"); known {
		check("recurring item store flags (1/1)", a == 1 && d == 1, fmt.Sprintf("store flags %d/%d", a, d))
	}

	if failed > 0 {
		log.Printf("%d check(s) failed", failed)
		exitCode = 1
	} else {
		log.Printf("all checks passed")
	}
}

// exitCode is applied by main's first deferred call, after list cleanup.
var exitCode int

func sameDay(a, b time.Time) bool {
	a, b = a.In(time.Local), b.In(time.Local)
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

// setAllDayViaAppleScript sets "allday due date" to day on the named
// reminder in the test list, the way Reminders' scripting interface does.
func setAllDayViaAppleScript(name string, day time.Time) error {
	script := fmt.Sprintf(`
set d to current date
set day of d to 1
set year of d to %d
set month of d to %d
set day of d to %d
set time of d to 0
tell application "Reminders"
	set r to first reminder of list %q whose name is %q
	set allday due date of r to d
end tell`, day.Year(), int(day.Month()), day.Day(), testList, name)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, out)
	}
	return nil
}

// storeFlags reads ZALLDAY and ZDISPLAYDATEISALLDAY for the named reminder in
// the test list, read-only. known is false if the store can't be read.
func storeFlags(name string) (allDay, displayAllDay int, known bool) {
	home, _ := os.UserHomeDir()
	dbs, _ := filepath.Glob(filepath.Join(home, "Library/Group Containers/group.com.apple.reminders/Container_v1/Stores/Data-*.sqlite"))
	query := fmt.Sprintf(`select r.ZALLDAY, r.ZDISPLAYDATEISALLDAY from ZREMCDREMINDER r join ZREMCDBASELIST l on r.ZLIST = l.Z_PK
where l.ZNAME = '%s' and r.ZTITLE = '%s' and r.ZMARKEDFORDELETION = 0 and l.ZMARKEDFORDELETION = 0`, testList, name)
	// The app writes asynchronously; give it a moment.
	time.Sleep(time.Second)
	for _, db := range dbs {
		out, err := exec.Command("sqlite3", "-readonly", db, query).Output()
		if err != nil || strings.TrimSpace(string(out)) == "" {
			continue
		}
		if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d|%d", &allDay, &displayAllDay); err == nil {
			return allDay, displayAllDay, true
		}
	}
	return 0, 0, false
}
