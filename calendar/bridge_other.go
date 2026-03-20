//go:build !darwin

package calendar

import (
	"context"
	"time"
)

// New creates a new Calendar [Client] and requests calendar access.
//
// On non-darwin platforms, this always returns [ErrUnsupported].
func New() (*Client, error) {
	return nil, ErrUnsupported
}

// Calendars returns all calendars for events across all accounts.
func (c *Client) Calendars() ([]Calendar, error) { return nil, ErrUnsupported }

// Events returns events within the given time range.
func (c *Client) Events(start, end time.Time, opts ...ListOption) ([]Event, error) {
	return nil, ErrUnsupported
}

// Event returns a single event by its stable event identifier.
func (c *Client) Event(id string) (*Event, error) { return nil, ErrUnsupported }

// CreateEvent creates a new calendar event and returns it with its assigned ID.
func (c *Client) CreateEvent(input CreateEventInput) (*Event, error) { return nil, ErrUnsupported }

// UpdateEvent updates an existing event and returns the updated version.
func (c *Client) UpdateEvent(id string, input UpdateEventInput, span Span) (*Event, error) {
	return nil, ErrUnsupported
}

// DeleteEvent permanently removes an event.
func (c *Client) DeleteEvent(id string, span Span) error { return ErrUnsupported }

// DeleteEvents permanently removes multiple events in a single bridge call.
func (c *Client) DeleteEvents(ids []string, span Span) map[string]error {
	result := make(map[string]error)
	for _, id := range ids {
		result[id] = ErrUnsupported
	}
	return result
}

// CreateCalendar creates a new calendar and returns it with its assigned ID.
func (c *Client) CreateCalendar(input CreateCalendarInput) (*Calendar, error) {
	return nil, ErrUnsupported
}

// UpdateCalendar updates an existing calendar and returns the updated version.
func (c *Client) UpdateCalendar(id string, input UpdateCalendarInput) (*Calendar, error) {
	return nil, ErrUnsupported
}

// DeleteCalendar permanently removes a calendar and all its events.
func (c *Client) DeleteCalendar(id string) error { return ErrUnsupported }

// WatchChanges returns a channel that receives a value whenever the
// EventKit calendar database changes.
//
// Returns [ErrUnsupported] on non-darwin platforms.
func (c *Client) WatchChanges(ctx context.Context) (<-chan struct{}, error) {
	return nil, ErrUnsupported
}
