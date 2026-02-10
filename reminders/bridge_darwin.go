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
	"fmt"
	"unsafe"
)

// New creates a new Reminders client.
// Requests full reminder access on first use (TCC prompt).
// Returns ErrAccessDenied if the user denies access.
func New() (*Client, error) {
	granted := C.ek_rem_request_access()
	if granted == 0 {
		return nil, fmt.Errorf("%w: %s", ErrAccessDenied, getLastError())
	}
	return &Client{}, nil
}

// Lists returns all reminder lists.
func (c *Client) Lists() ([]List, error) {
	cstr := C.ek_rem_fetch_lists()
	if cstr == nil {
		return nil, fmt.Errorf("reminders: %s", getLastError())
	}
	defer C.ek_rem_free(cstr)
	return parseListsJSON(C.GoString(cstr))
}

// Reminders returns reminders matching the given options.
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

// Reminder returns a single reminder by ID (full or prefix).
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

// CreateReminder creates a new reminder.
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

// UpdateReminder updates an existing reminder.
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

// DeleteReminder deletes a reminder by ID.
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

// CompleteReminder marks a reminder as completed.
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

// UncompleteReminder marks a reminder as incomplete.
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
