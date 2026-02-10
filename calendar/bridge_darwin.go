//go:build darwin

package calendar

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework EventKit -framework Foundation -framework AppKit
#include "bridge_darwin.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"strings"
	"time"
	"unsafe"
)

// New creates a new Calendar client and requests calendar access (TCC prompt).
// Returns an error if calendar access is denied or if not running on macOS.
func New() (*Client, error) {
	granted := C.ek_cal_request_access()
	if granted == 0 {
		cerr := C.ek_cal_last_error()
		if cerr != nil {
			msg := C.GoString(cerr)
			if strings.Contains(msg, "denied") {
				return nil, ErrAccessDenied
			}
			return nil, fmt.Errorf("calendar: %s", msg)
		}
		return nil, ErrAccessDenied
	}
	return &Client{}, nil
}

// Calendars returns all calendars for events.
func (c *Client) Calendars() ([]Calendar, error) {
	cstr := C.ek_cal_fetch_calendars()
	if cstr == nil {
		return nil, getLastError("failed to fetch calendars")
	}
	defer C.ek_cal_free(cstr)

	jsonStr := C.GoString(cstr)
	return parseCalendarsJSON(jsonStr)
}

// Events returns events within the given time range.
// Options can filter by calendar, search query, etc.
func (c *Client) Events(start, end time.Time, opts ...ListOption) ([]Event, error) {
	o := applyOptions(opts)

	cStart := C.CString(start.UTC().Format("2006-01-02T15:04:05.000Z"))
	defer C.free(unsafe.Pointer(cStart))
	cEnd := C.CString(end.UTC().Format("2006-01-02T15:04:05.000Z"))
	defer C.free(unsafe.Pointer(cEnd))

	var cCalID *C.char
	if o.calendarID != "" {
		cCalID = C.CString(o.calendarID)
		defer C.free(unsafe.Pointer(cCalID))
	} else if o.calendarName != "" {
		cCalID = C.CString(o.calendarName)
		defer C.free(unsafe.Pointer(cCalID))
	}

	var cSearch *C.char
	if o.searchQuery != "" {
		cSearch = C.CString(o.searchQuery)
		defer C.free(unsafe.Pointer(cSearch))
	}

	cstr := C.ek_cal_fetch_events(cStart, cEnd, cCalID, cSearch)
	if cstr == nil {
		return nil, getLastError("failed to fetch events")
	}
	defer C.ek_cal_free(cstr)

	jsonStr := C.GoString(cstr)
	return parseEventsJSON(jsonStr)
}

// Event returns a single event by ID (full ID or prefix).
func (c *Client) Event(id string) (*Event, error) {
	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))

	cstr := C.ek_cal_get_event(cID)
	if cstr == nil {
		err := getLastError("event not found: " + id)
		if strings.Contains(err.Error(), "not found") {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer C.ek_cal_free(cstr)

	jsonStr := C.GoString(cstr)
	return parseEventJSON(jsonStr)
}

// CreateEvent creates a new calendar event.
func (c *Client) CreateEvent(input CreateEventInput) (*Event, error) {
	jsonBytes, err := marshalCreateInput(input)
	if err != nil {
		return nil, fmt.Errorf("calendar: failed to marshal input: %w", err)
	}

	cJSON := C.CString(string(jsonBytes))
	defer C.free(unsafe.Pointer(cJSON))

	cstr := C.ek_cal_create_event(cJSON)
	if cstr == nil {
		return nil, getLastError("failed to create event")
	}
	defer C.ek_cal_free(cstr)

	jsonStr := C.GoString(cstr)
	return parseEventJSON(jsonStr)
}

// UpdateEvent updates an existing event.
// Span controls whether to update just this occurrence or future occurrences too.
func (c *Client) UpdateEvent(id string, input UpdateEventInput, span Span) (*Event, error) {
	jsonBytes, err := marshalUpdateInput(input)
	if err != nil {
		return nil, fmt.Errorf("calendar: failed to marshal input: %w", err)
	}

	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))
	cJSON := C.CString(string(jsonBytes))
	defer C.free(unsafe.Pointer(cJSON))

	cstr := C.ek_cal_update_event(cID, cJSON, C.int(span))
	if cstr == nil {
		err := getLastError("failed to update event: " + id)
		if strings.Contains(err.Error(), "not found") {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer C.ek_cal_free(cstr)

	jsonStr := C.GoString(cstr)
	return parseEventJSON(jsonStr)
}

// DeleteEvent removes an event.
// Span controls whether to delete just this occurrence or future occurrences too.
func (c *Client) DeleteEvent(id string, span Span) error {
	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))

	cstr := C.ek_cal_delete_event(cID, C.int(span))
	if cstr == nil {
		err := getLastError("failed to delete event: " + id)
		if strings.Contains(err.Error(), "not found") {
			return ErrNotFound
		}
		return err
	}
	defer C.ek_cal_free(cstr)
	return nil
}

// getLastError reads the last error from the ObjC bridge.
func getLastError(fallback string) error {
	cerr := C.ek_cal_last_error()
	if cerr != nil {
		return fmt.Errorf("calendar: %s", C.GoString(cerr))
	}
	return fmt.Errorf("calendar: %s", fallback)
}
