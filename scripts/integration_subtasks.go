//go:build darwin && integration

// Package main is an integration check for subtasks (parent/child
// reminders) against live macOS Reminders data. It writes only reminders it
// creates, titled "ZZ subtask ...", in an existing scratch list (default
// "claude-sync-test"; override with SUBTASK_TEST_LIST), and deletes them
// afterwards. It refuses to run if such reminders are already there.
//
// EventKit can't see subtasks, so every write is also checked in the
// Reminders store with read-only sqlite (ZPARENTREMINDER, plus the CloudKit
// parent identifier that iCloud sync uses).
//
// Run with: go run -tags integration ./scripts/integration_subtasks.go
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BRO3886/go-eventkit/reminders"
)

const prefix = "ZZ subtask "

func main() {
	log.SetFlags(0)
	log.SetPrefix("[subtask-integration] ")
	// Registered first, so it runs last: after the cleanup below. Failures
	// after the first reminder exists use log.Panicf so the cleanup runs.
	defer func() {
		if r := recover(); r != nil {
			exitCode = 1
		}
		os.Exit(exitCode)
	}()

	testList := os.Getenv("SUBTASK_TEST_LIST")
	if testList == "" {
		testList = "claude-sync-test"
	}

	client, err := reminders.New()
	if err != nil {
		log.Fatalf("FATAL: %v", err)
	}
	lists, err := client.Lists()
	if err != nil {
		log.Fatalf("FATAL: %v", err)
	}
	found := false
	for _, l := range lists {
		if l.Title == testList {
			found = true
			if l.IsShared || l.SharedToMe {
				log.Fatalf("FATAL: list %q is shared; use an unshared scratch list", testList)
			}
		}
	}
	if !found {
		log.Fatalf("FATAL: list %q not found", testList)
	}
	if mine := ours(client, testList); len(mine) > 0 {
		log.Fatalf("FATAL: %d %q reminders already in %q; delete them first", len(mine), prefix, testList)
	}

	defer func() {
		ids := []string{}
		for _, r := range ours(client, testList) {
			ids = append(ids, r.ID)
		}
		if len(ids) > 0 {
			for id, err := range client.DeleteReminders(ids) {
				log.Printf("WARN: delete %s: %v", id, err)
			}
		}
		if left := ours(client, testList); len(left) > 0 {
			log.Printf("FAIL: %d test reminders still in %q after delete", len(left), testList)
			exitCode = 1
			return
		}
		log.Printf("cleanup: %d test reminders deleted from %q", len(ids), testList)
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
	create := func(title string) *reminders.Reminder {
		r, err := client.CreateReminder(reminders.CreateReminderInput{Title: prefix + title, ListName: testList})
		if err != nil {
			log.Panicf("FATAL: create %q: %v", title, err)
		}
		return r
	}
	setParent := func(child *reminders.Reminder, parentID string) (*reminders.Reminder, error) {
		return client.UpdateReminder(child.ID, reminders.UpdateReminderInput{ParentID: &parentID})
	}
	// freshParent re-reads the reminder in a new process, so the check can't
	// pass on a value that only lives in this process's cache.
	freshParent := func(id string) string {
		out, err := exec.Command("go", "run", "-tags", "integration", "./scripts/integration_subtasks.go", "-read-parent", id).Output()
		if err != nil {
			log.Panicf("FATAL: fresh read of %s: %v", id, err)
		}
		return strings.TrimSpace(string(out))
	}

	parent := create("parent")
	other := create("other parent")
	childA := create("child A")
	childB := create("child B")
	if _, err := client.CompleteReminder(childB.ID); err != nil {
		log.Panicf("FATAL: complete: %v", err)
	}

	// --- Nest an open and a completed reminder ---
	r, err := setParent(childA, parent.ID)
	check("nest open child", err == nil && r.ParentID == parent.ID, fmt.Sprintf("err=%v parentID=%q", err, parentIDOf(r)))
	r, err = setParent(childB, parent.ID[:8])
	check("nest completed child by ID prefix", err == nil && r.ParentID == parent.ID, fmt.Sprintf("err=%v parentID=%q", err, parentIDOf(r)))
	check("store: child A under parent", storeParent(testList, "child A") == prefix+"parent", storeParent(testList, "child A"))
	check("store: child B under parent", storeParent(testList, "child B") == prefix+"parent", storeParent(testList, "child B"))
	check("store: CloudKit parent identifier set", storeCKParentSet(testList, "child A"), "ZCKPARENTREMINDERIDENTIFIER is empty")
	check("fresh process reads parentID", freshParent(childA.ID) == parent.ID, freshParent(childA.ID))
	p, err := client.Reminder(parent.ID)
	check("parent stays top-level", err == nil && p.ParentID == "", fmt.Sprintf("err=%v parentID=%q", err, parentIDOf(p)))

	// --- One level only, no self-parenting ---
	_, err = setParent(other, childA.ID)
	check("refuse a subtask as parent", err != nil && strings.Contains(err.Error(), "only one level"), fmt.Sprintf("err=%v", err))
	_, err = setParent(parent, other.ID)
	check("refuse nesting a reminder that has subtasks", err != nil && strings.Contains(err.Error(), "only one level"), fmt.Sprintf("err=%v", err))
	_, err = setParent(other, other.ID)
	check("refuse self as parent", err != nil && strings.Contains(err.Error(), "own parent"), fmt.Sprintf("err=%v", err))
	_, err = setParent(other, "00000000-0000-0000-0000-000000000000")
	check("refuse unknown parent", err != nil && strings.Contains(err.Error(), "not found"), fmt.Sprintf("err=%v", err))
	check("store: refused writes changed nothing", storeParent(testList, "other parent") == "" && storeParent(testList, "parent") == "",
		storeParent(testList, "other parent")+"/"+storeParent(testList, "parent"))

	// --- Re-parent directly ---
	r, err = setParent(childA, other.ID)
	check("re-parent to another reminder", err == nil && r.ParentID == other.ID, fmt.Sprintf("err=%v parentID=%q", err, parentIDOf(r)))
	check("store: child A under other parent", storeParent(testList, "child A") == prefix+"other parent", storeParent(testList, "child A"))

	// --- Outdent ---
	r, err = setParent(childA, "")
	check("outdent", err == nil && r.ParentID == "" && r.List == testList, fmt.Sprintf("err=%v parentID=%q", err, parentIDOf(r)))
	check("store: child A top-level and not deleted", storeParent(testList, "child A") == "" && storeAlive(testList, "child A"), storeParent(testList, "child A"))
	r, err = setParent(childA, "")
	check("outdent a top-level reminder is a no-op", err == nil && r.ParentID == "", fmt.Sprintf("err=%v", err))
	check("child B still under parent", storeParent(testList, "child B") == prefix+"parent", storeParent(testList, "child B"))

	if failed > 0 {
		log.Printf("%d check(s) failed", failed)
		exitCode = 1
	} else {
		log.Printf("all checks passed")
	}
}

func init() {
	// Subprocess mode for freshParent: print one reminder's parentID.
	if len(os.Args) == 3 && os.Args[1] == "-read-parent" {
		client, err := reminders.New()
		if err != nil {
			log.Fatal(err)
		}
		r, err := client.Reminder(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(r.ParentID)
		os.Exit(0)
	}
}

// exitCode is applied by main's first deferred call, after cleanup.
var exitCode int

func parentIDOf(r *reminders.Reminder) string {
	if r == nil {
		return "<nil>"
	}
	return r.ParentID
}

// ours returns this script's reminders (open and completed) in the list.
func ours(client *reminders.Client, list string) []reminders.Reminder {
	all, err := client.Reminders(reminders.WithList(list))
	if err != nil {
		log.Panicf("FATAL: list reminders: %v", err)
	}
	var mine []reminders.Reminder
	for _, r := range all {
		if strings.HasPrefix(r.Title, prefix) {
			mine = append(mine, r)
		}
	}
	return mine
}

// storeQuery runs a read-only query against the Reminders store that has
// rows for it. Without the store, persistence can't be verified, so the run
// must not pass.
func storeQuery(query string) string {
	home, _ := os.UserHomeDir()
	dbs, _ := filepath.Glob(filepath.Join(home, "Library/Group Containers/group.com.apple.reminders/Container_v1/Stores/Data-*.sqlite"))
	// The app writes asynchronously; give it a moment.
	time.Sleep(time.Second)
	for _, db := range dbs {
		out, err := exec.Command("sqlite3", "-readonly", db, query).Output()
		if err != nil {
			continue
		}
		if s := strings.TrimSpace(string(out)); s != "" {
			return s
		}
	}
	log.Panicf("FATAL: no rows in the Reminders store for: %s (needs Full Disk Access for the terminal)", query)
	return ""
}

const childRow = `from ZREMCDREMINDER c join ZREMCDBASELIST l on c.ZLIST = l.Z_PK
where l.ZNAME = '%s' and c.ZTITLE = '%s' and c.ZMARKEDFORDELETION = 0 and l.ZMARKEDFORDELETION = 0`

// storeParent is the title of the named reminder's parent in the store, or
// "" if it is top-level.
func storeParent(list, title string) string {
	q := fmt.Sprintf(`select 'p:' || coalesce((select p.ZTITLE from ZREMCDREMINDER p where p.Z_PK = c.ZPARENTREMINDER), '') `+childRow,
		list, prefix+title)
	return strings.TrimPrefix(storeQuery(q), "p:")
}

func storeCKParentSet(list, title string) bool {
	q := fmt.Sprintf(`select coalesce(c.ZCKPARENTREMINDERIDENTIFIER, '') != '' `+childRow, list, prefix+title)
	return storeQuery(q) == "1"
}

func storeAlive(list, title string) bool {
	q := fmt.Sprintf(`select count(*) `+childRow, list, prefix+title)
	return storeQuery(q) == "1"
}
