//go:build darwin

package calendar

/*
#include "bridge_darwin.h"
#include <stdlib.h>
*/
import "C"
import (
	"encoding/json"
	"fmt"
	"time"
	"unsafe"
)

// AttendeeWritesSupported reports whether this macOS exposes the private
// EventKit API for adding attendees. When false, [CreateEventInput.Attendees]
// and [UpdateEventInput.Attendees] cause the write to fail with
// [ErrUnsupportedFeature].
func (c *Client) AttendeeWritesSupported() bool {
	return C.ek_cal_attendee_selectors_available() == 1
}

// RSVPSupported reports whether this macOS exposes the private EventKit RSVP API.
func (c *Client) RSVPSupported() bool {
	return C.ek_cal_rsvp_selectors_available() == 1
}

// AvailabilitySupported reports whether this macOS exposes the private EventKit
// free/busy availability API. Note that even when true, the lookup only works
// for accounts whose server supports it (Exchange, Google Workspace) — iCloud
// does not.
func (c *Client) AvailabilitySupported() bool {
	return C.ek_cal_availability_selectors_available() == 1
}

// RespondToInvitation sets the current user's RSVP status on an event
// invitation. On a server-backed calendar this sends a reply to the organizer
// when saved. Returns [ErrNotFound] if the event does not exist and
// [ErrUnsupportedFeature] if RSVP is unavailable on this macOS.
func (c *Client) RespondToInvitation(eventID string, status ParticipantStatus) error {
	if !c.RSVPSupported() {
		return ErrUnsupportedFeature
	}
	cID := C.CString(eventID)
	defer C.free(unsafe.Pointer(cID))

	res := C.ek_cal_respond_to_event(cID, C.int(status))
	if res.error != nil {
		err := resultErr(res)
		if isNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("calendar: %w", err)
	}
	C.ek_cal_free(res.result)
	return nil
}

// RequestAvailability looks up free/busy spans for the given email addresses
// between start and end. It returns a map from address to its spans. Addresses
// with no data map to an empty slice.
//
// Only accounts whose server supports availability are queried; iCloud does
// not. Returns [ErrUnsupportedFeature] if the private API is missing, or an
// error if no account supports availability lookups.
func (c *Client) RequestAvailability(addresses []string, start, end time.Time) (map[string][]AvailabilitySpan, error) {
	if !c.AvailabilitySupported() {
		return nil, ErrUnsupportedFeature
	}
	if len(addresses) == 0 {
		return map[string][]AvailabilitySpan{}, nil
	}
	addrJSON, err := json.Marshal(addresses)
	if err != nil {
		return nil, fmt.Errorf("calendar: failed to marshal addresses: %w", err)
	}

	cStart := C.CString(start.UTC().Format("2006-01-02T15:04:05.000Z"))
	defer C.free(unsafe.Pointer(cStart))
	cEnd := C.CString(end.UTC().Format("2006-01-02T15:04:05.000Z"))
	defer C.free(unsafe.Pointer(cEnd))
	cAddr := C.CString(string(addrJSON))
	defer C.free(unsafe.Pointer(cAddr))

	res := C.ek_cal_request_availability(cStart, cEnd, cAddr)
	if res.error != nil {
		return nil, fmt.Errorf("calendar: %w", resultErr(res))
	}
	defer C.ek_cal_free(res.result)

	return parseAvailabilityJSON(C.GoString(res.result))
}

// PendingInvitations returns the event invitations awaiting the user's response
// (the Calendar.app notification inbox). Returns an empty slice when there are
// none or the private API is unavailable.
func (c *Client) PendingInvitations() ([]Invitation, error) {
	res := C.ek_cal_event_notifications()
	if res.error != nil {
		return nil, fmt.Errorf("calendar: %w", resultErr(res))
	}
	defer C.ek_cal_free(res.result)

	return parseInvitationsJSON(C.GoString(res.result))
}
