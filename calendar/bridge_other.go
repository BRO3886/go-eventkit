//go:build !darwin

package calendar

import "time"

// New creates a new Calendar client.
// On non-darwin platforms, this always returns ErrUnsupported.
func New() (*Client, error) {
	return nil, ErrUnsupported
}

// Calendars returns all calendars for events.
func (c *Client) Calendars() ([]Calendar, error) { return nil, ErrUnsupported }

// Events returns events within the given time range.
func (c *Client) Events(start, end time.Time, opts ...ListOption) ([]Event, error) {
	return nil, ErrUnsupported
}

// Event returns a single event by ID.
func (c *Client) Event(id string) (*Event, error) { return nil, ErrUnsupported }

// CreateEvent creates a new calendar event.
func (c *Client) CreateEvent(input CreateEventInput) (*Event, error) { return nil, ErrUnsupported }

// UpdateEvent updates an existing event.
func (c *Client) UpdateEvent(id string, input UpdateEventInput, span Span) (*Event, error) {
	return nil, ErrUnsupported
}

// DeleteEvent removes an event.
func (c *Client) DeleteEvent(id string, span Span) error { return ErrUnsupported }
