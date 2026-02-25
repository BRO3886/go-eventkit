//go:build darwin

package calendar

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework EventKit -framework Foundation -framework AppKit -framework CoreLocation
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
	"time"
	"unsafe"
)

var calWatchMu sync.Mutex
var calWatchActive bool

// New creates a new Calendar [Client] and requests calendar access.
//
// On first call, macOS displays a TCC prompt requesting calendar access.
// Returns [ErrAccessDenied] if the user denies access.
// Returns [ErrUnsupported] on non-darwin platforms.
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

// Calendars returns all calendars for events across all accounts
// (iCloud, Google, Exchange, local, subscribed, birthdays).
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
// EventKit requires a bounded date range — this method cannot fetch all events.
// Options can filter by calendar name, calendar ID, or search query.
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

// Event returns a single event by its stable event identifier
// (EKEvent.eventIdentifier). Returns [ErrNotFound] if no event matches.
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

// CreateEvent creates a new calendar event and returns it with its assigned ID.
// The event is saved to the EventKit store immediately.
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

// UpdateEvent updates an existing event and returns the updated version.
// Only non-nil fields in the input are modified. The span parameter controls
// whether the change applies to just this occurrence or all future occurrences
// of a recurring event. Returns [ErrNotFound] if the event does not exist.
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

// DeleteEvent permanently removes an event.
// The span parameter controls whether the deletion applies to just this
// occurrence or all future occurrences of a recurring event.
// Returns [ErrNotFound] if the event does not exist.
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

// CreateCalendar creates a new calendar and returns it with its assigned ID.
// The calendar is saved to the EventKit store immediately.
func (c *Client) CreateCalendar(input CreateCalendarInput) (*Calendar, error) {
	if input.Source == "" {
		return nil, fmt.Errorf("calendar: source is required")
	}
	jsonBytes, err := marshalCreateCalendarInput(input)
	if err != nil {
		return nil, fmt.Errorf("calendar: failed to marshal input: %w", err)
	}

	cJSON := C.CString(string(jsonBytes))
	defer C.free(unsafe.Pointer(cJSON))

	cstr := C.ek_cal_create_calendar(cJSON)
	if cstr == nil {
		return nil, getLastError("failed to create calendar")
	}
	defer C.ek_cal_free(cstr)

	jsonStr := C.GoString(cstr)
	cals, err := parseCalendarsJSON("[" + jsonStr + "]")
	if err != nil {
		return nil, err
	}
	if len(cals) == 0 {
		return nil, fmt.Errorf("calendar: unexpected empty response")
	}
	return &cals[0], nil
}

// UpdateCalendar updates an existing calendar and returns the updated version.
// Only non-nil fields in the input are modified.
// Returns [ErrNotFound] if the calendar does not exist.
// Returns [ErrImmutable] if the calendar is immutable.
func (c *Client) UpdateCalendar(id string, input UpdateCalendarInput) (*Calendar, error) {
	jsonBytes, err := marshalUpdateCalendarInput(input)
	if err != nil {
		return nil, fmt.Errorf("calendar: failed to marshal input: %w", err)
	}

	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))
	cJSON := C.CString(string(jsonBytes))
	defer C.free(unsafe.Pointer(cJSON))

	cstr := C.ek_cal_update_calendar(cID, cJSON)
	if cstr == nil {
		err := getLastError("failed to update calendar: " + id)
		if strings.Contains(err.Error(), "not found") {
			return nil, ErrNotFound
		}
		if strings.Contains(err.Error(), "immutable") {
			return nil, ErrImmutable
		}
		return nil, err
	}
	defer C.ek_cal_free(cstr)

	jsonStr := C.GoString(cstr)
	cals, err := parseCalendarsJSON("[" + jsonStr + "]")
	if err != nil {
		return nil, err
	}
	if len(cals) == 0 {
		return nil, fmt.Errorf("calendar: unexpected empty response")
	}
	return &cals[0], nil
}

// DeleteCalendar permanently removes a calendar and all its events.
// Returns [ErrNotFound] if the calendar does not exist.
// Returns [ErrImmutable] if the calendar is immutable.
func (c *Client) DeleteCalendar(id string) error {
	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))

	cstr := C.ek_cal_delete_calendar(cID)
	if cstr == nil {
		err := getLastError("failed to delete calendar: " + id)
		if strings.Contains(err.Error(), "not found") {
			return ErrNotFound
		}
		if strings.Contains(err.Error(), "immutable") {
			return ErrImmutable
		}
		return err
	}
	defer C.ek_cal_free(cstr)
	return nil
}

// WatchChanges returns a channel that receives a value whenever the
// EventKit calendar database changes. Changes include writes by this
// process (CreateEvent, UpdateEvent, DeleteEvent, CreateCalendar, etc.),
// iCloud sync, Calendar.app edits, Exchange push, and changes by other
// apps with calendar access.
//
// The channel is closed when ctx is cancelled or an internal read error
// occurs. After ctx cancellation, any pending signals already in the
// channel buffer are still readable.
//
// The channel carries no information about what specifically changed.
// Callers should re-fetch the data they care about after each signal.
// The channel is buffered (capacity 16); if the consumer falls behind,
// excess signals are dropped rather than blocking — this is safe because
// callers re-fetch anyway.
//
// Only one watcher may be active per process. A second call to
// WatchChanges while the first is active returns an error.
//
// Returns [ErrUnsupported] on non-darwin platforms.
func (c *Client) WatchChanges(ctx context.Context) (<-chan struct{}, error) {
	calWatchMu.Lock()
	if calWatchActive {
		calWatchMu.Unlock()
		return nil, errors.New("calendar: watcher already active")
	}
	if C.ek_cal_watch_start() == 0 {
		calWatchMu.Unlock()
		return nil, errors.New("calendar: failed to start watcher")
	}
	calWatchActive = true
	calWatchMu.Unlock()

	fd := int(C.ek_cal_watch_read_fd())
	f := os.NewFile(uintptr(fd), "ek-cal-watch-pipe")

	ch := make(chan struct{}, 16)
	go func() {
		defer func() {
			C.ek_cal_watch_stop()
			calWatchMu.Lock()
			calWatchActive = false
			calWatchMu.Unlock()
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

// getLastError reads the last error from the ObjC bridge.
func getLastError(fallback string) error {
	cerr := C.ek_cal_last_error()
	if cerr != nil {
		return fmt.Errorf("calendar: %s", C.GoString(cerr))
	}
	return fmt.Errorf("calendar: %s", fallback)
}
