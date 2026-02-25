//go:build darwin

package reminders

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework EventKit -framework Foundation -framework AppKit
#include "bridge_darwin.h"
#include <stdlib.h>
*/
import "C"
import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"unsafe"
)

var remWatchMu sync.Mutex
var remWatchActive bool

// New creates a new Reminders [Client] and requests reminders access.
//
// On first call, macOS displays a TCC prompt requesting reminders access.
// Returns [ErrAccessDenied] if the user denies access.
// Returns [ErrUnsupported] on non-darwin platforms.
func New() (*Client, error) {
	granted := C.ek_rem_request_access()
	if granted == 0 {
		return nil, fmt.Errorf("%w: %s", ErrAccessDenied, getLastError())
	}
	return &Client{}, nil
}

// Lists returns all reminder lists across all accounts (iCloud, Exchange, etc.).
func (c *Client) Lists() ([]List, error) {
	cstr := C.ek_rem_fetch_lists()
	if cstr == nil {
		return nil, fmt.Errorf("reminders: %s", getLastError())
	}
	defer C.ek_rem_free(cstr)
	return parseListsJSON(C.GoString(cstr))
}

// Reminders returns reminders matching the given filter options.
// With no options, returns all reminders across all lists.
// Options can filter by list, completion status, search query, and due date range.
func (c *Client) Reminders(opts ...ListOption) ([]Reminder, error) {
	o := applyOptions(opts)

	var cList, cCompleted, cSearch, cBefore, cAfter *C.char

	if o.listName != "" {
		cList = C.CString(o.listName)
		defer C.free(unsafe.Pointer(cList))
	} else if o.listID != "" {
		// Use list ID as name — the bridge resolves by name (case-insensitive).
		// For ID-based filtering, we'd need a separate bridge function.
		// For now, pass it through — the bridge will try name match.
		cList = C.CString(o.listID)
		defer C.free(unsafe.Pointer(cList))
	}
	if o.completed != nil {
		if *o.completed {
			cCompleted = C.CString("true")
		} else {
			cCompleted = C.CString("false")
		}
		defer C.free(unsafe.Pointer(cCompleted))
	}
	if o.search != "" {
		cSearch = C.CString(o.search)
		defer C.free(unsafe.Pointer(cSearch))
	}
	if o.dueBefore != nil {
		s := o.dueBefore.UTC().Format("2006-01-02T15:04:05.000Z")
		cBefore = C.CString(s)
		defer C.free(unsafe.Pointer(cBefore))
	}
	if o.dueAfter != nil {
		s := o.dueAfter.UTC().Format("2006-01-02T15:04:05.000Z")
		cAfter = C.CString(s)
		defer C.free(unsafe.Pointer(cAfter))
	}

	cstr := C.ek_rem_fetch_reminders(cList, cCompleted, cSearch, cBefore, cAfter)
	if cstr == nil {
		return nil, fmt.Errorf("reminders: %s", getLastError())
	}
	defer C.ek_rem_free(cstr)
	return parseRemindersJSON(C.GoString(cstr))
}

// Reminder returns a single reminder by ID.
// Accepts a full identifier or a unique prefix (e.g., first 8 characters).
// Returns [ErrNotFound] if no reminder matches.
func (c *Client) Reminder(id string) (*Reminder, error) {
	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))

	cstr := C.ek_rem_get_reminder(cID)
	if cstr == nil {
		errMsg := getLastError()
		return nil, fmt.Errorf("%w: %s", ErrNotFound, errMsg)
	}
	defer C.ek_rem_free(cstr)
	return parseReminderJSON(C.GoString(cstr))
}

// CreateReminder creates a new reminder and returns it with its assigned ID.
// The reminder is saved to the EventKit store immediately.
func (c *Client) CreateReminder(input CreateReminderInput) (*Reminder, error) {
	jsonStr, err := marshalCreateInput(input)
	if err != nil {
		return nil, err
	}

	cJSON := C.CString(jsonStr)
	defer C.free(unsafe.Pointer(cJSON))

	cstr := C.ek_rem_create_reminder(cJSON)
	if cstr == nil {
		return nil, fmt.Errorf("reminders: %s", getLastError())
	}
	defer C.ek_rem_free(cstr)
	return parseReminderJSON(C.GoString(cstr))
}

// UpdateReminder updates an existing reminder and returns the updated version.
// Only non-nil fields in the input are modified. Returns [ErrNotFound] if the
// reminder does not exist.
func (c *Client) UpdateReminder(id string, input UpdateReminderInput) (*Reminder, error) {
	jsonStr, err := marshalUpdateInput(input)
	if err != nil {
		return nil, err
	}

	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))
	cJSON := C.CString(jsonStr)
	defer C.free(unsafe.Pointer(cJSON))

	cstr := C.ek_rem_update_reminder(cID, cJSON)
	if cstr == nil {
		return nil, fmt.Errorf("reminders: %s", getLastError())
	}
	defer C.ek_rem_free(cstr)
	return parseReminderJSON(C.GoString(cstr))
}

// DeleteReminder permanently deletes a reminder by ID.
// Returns [ErrNotFound] if the reminder does not exist.
func (c *Client) DeleteReminder(id string) error {
	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))

	cstr := C.ek_rem_delete_reminder(cID)
	if cstr == nil {
		return fmt.Errorf("reminders: %s", getLastError())
	}
	defer C.ek_rem_free(cstr)
	return nil
}

// CreateList creates a new reminder list and returns it with its assigned ID.
// The list is saved to the EventKit store immediately.
func (c *Client) CreateList(input CreateListInput) (*List, error) {
	if input.Source == "" {
		return nil, fmt.Errorf("reminders: source is required")
	}
	jsonStr, err := marshalCreateListInput(input)
	if err != nil {
		return nil, err
	}

	cJSON := C.CString(jsonStr)
	defer C.free(unsafe.Pointer(cJSON))

	cstr := C.ek_rem_create_list(cJSON)
	if cstr == nil {
		return nil, fmt.Errorf("reminders: %s", getLastError())
	}
	defer C.ek_rem_free(cstr)

	jsonResp := C.GoString(cstr)
	lists, err := parseListsJSON("[" + jsonResp + "]")
	if err != nil {
		return nil, err
	}
	if len(lists) == 0 {
		return nil, fmt.Errorf("reminders: unexpected empty response")
	}
	return &lists[0], nil
}

// UpdateList updates an existing reminder list and returns the updated version.
// Only non-nil fields in the input are modified.
// Returns [ErrNotFound] if the list does not exist.
// Returns [ErrImmutable] if the list is immutable.
func (c *Client) UpdateList(id string, input UpdateListInput) (*List, error) {
	jsonStr, err := marshalUpdateListInput(input)
	if err != nil {
		return nil, err
	}

	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))
	cJSON := C.CString(jsonStr)
	defer C.free(unsafe.Pointer(cJSON))

	cstr := C.ek_rem_update_list(cID, cJSON)
	if cstr == nil {
		errMsg := getLastError()
		if contains(errMsg, "not found") {
			return nil, ErrNotFound
		}
		if contains(errMsg, "immutable") {
			return nil, ErrImmutable
		}
		return nil, fmt.Errorf("reminders: %s", errMsg)
	}
	defer C.ek_rem_free(cstr)

	jsonResp := C.GoString(cstr)
	lists, err := parseListsJSON("[" + jsonResp + "]")
	if err != nil {
		return nil, err
	}
	if len(lists) == 0 {
		return nil, fmt.Errorf("reminders: unexpected empty response")
	}
	return &lists[0], nil
}

// DeleteList permanently removes a reminder list and all its reminders.
// Returns [ErrNotFound] if the list does not exist.
// Returns [ErrImmutable] if the list is immutable.
func (c *Client) DeleteList(id string) error {
	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))

	cstr := C.ek_rem_delete_list(cID)
	if cstr == nil {
		errMsg := getLastError()
		if contains(errMsg, "not found") {
			return ErrNotFound
		}
		if contains(errMsg, "immutable") {
			return ErrImmutable
		}
		return fmt.Errorf("reminders: %s", errMsg)
	}
	defer C.ek_rem_free(cstr)
	return nil
}

// contains checks if s contains substr (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// CompleteReminder marks a reminder as completed and returns the updated version.
// Sets [Reminder.Completed] to true and [Reminder.CompletionDate] to now.
// Returns [ErrNotFound] if the reminder does not exist.
func (c *Client) CompleteReminder(id string) (*Reminder, error) {
	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))

	cstr := C.ek_rem_complete_reminder(cID)
	if cstr == nil {
		return nil, fmt.Errorf("reminders: %s", getLastError())
	}
	defer C.ek_rem_free(cstr)
	return parseReminderJSON(C.GoString(cstr))
}

// UncompleteReminder marks a reminder as incomplete and returns the updated version.
// Sets [Reminder.Completed] to false and clears [Reminder.CompletionDate].
// Returns [ErrNotFound] if the reminder does not exist.
func (c *Client) UncompleteReminder(id string) (*Reminder, error) {
	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))

	cstr := C.ek_rem_uncomplete_reminder(cID)
	if cstr == nil {
		return nil, fmt.Errorf("reminders: %s", getLastError())
	}
	defer C.ek_rem_free(cstr)
	return parseReminderJSON(C.GoString(cstr))
}

func getLastError() string {
	cerr := C.ek_rem_last_error()
	if cerr != nil {
		return C.GoString(cerr)
	}
	return "unknown error"
}

// WatchChanges returns a channel that receives a value whenever the
// EventKit reminders database changes. Changes include writes by this
// process (CreateReminder, UpdateReminder, DeleteReminder, CreateList,
// etc.), iCloud sync, Reminders.app edits, and changes by other apps
// with reminders access.
//
// The channel is closed when ctx is cancelled or an internal read error
// occurs. After ctx cancellation, any pending signals already in the
// channel buffer are still readable.
//
// The channel carries no information about what specifically changed.
// Callers should re-fetch the data they care about after each signal.
// The channel is buffered (capacity 16); if the consumer falls behind,
// excess signals are dropped rather than blocking.
//
// Only one watcher may be active per process. A second call to
// WatchChanges while the first is active returns an error.
//
// Returns [ErrUnsupported] on non-darwin platforms.
func (c *Client) WatchChanges(ctx context.Context) (<-chan struct{}, error) {
	remWatchMu.Lock()
	if remWatchActive {
		remWatchMu.Unlock()
		return nil, errors.New("reminders: watcher already active")
	}
	if C.ek_rem_watch_start() == 0 {
		remWatchMu.Unlock()
		return nil, errors.New("reminders: failed to start watcher")
	}
	remWatchActive = true
	remWatchMu.Unlock()

	fd := int(C.ek_rem_watch_read_fd())
	f := os.NewFile(uintptr(fd), "ek-rem-watch-pipe")

	ch := make(chan struct{}, 16)
	go func() {
		defer func() {
			C.ek_rem_watch_stop()
			remWatchMu.Lock()
			remWatchActive = false
			remWatchMu.Unlock()
			close(ch)
		}()
		inner := watchChangesFromFile(ctx, f)
		for range inner {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}()
	return ch, nil
}
