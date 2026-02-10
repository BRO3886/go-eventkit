// Package calendar provides native macOS Calendar bindings via EventKit.
//
// It exposes idiomatic Go types and a Client API for reading and writing
// calendar events, backed by Apple's EventKit framework for in-process,
// sub-200ms access. No AppleScript, no subprocesses.
//
// Usage:
//
//	client, err := calendar.New()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// List all calendars
//	calendars, err := client.Calendars()
//
//	// Fetch events for the next week
//	now := time.Now()
//	events, err := client.Events(now, now.Add(7*24*time.Hour))
//
//	// Create an event
//	event, err := client.CreateEvent(calendar.CreateEventInput{
//	    Title:     "Team standup",
//	    StartDate: time.Date(2026, 2, 12, 10, 0, 0, 0, time.Local),
//	    EndDate:   time.Date(2026, 2, 12, 10, 30, 0, 0, time.Local),
//	    Calendar:  "Work",
//	})
package calendar

import (
	"errors"
	"time"
)

// Sentinel errors.
var (
	// ErrUnsupported is returned on non-darwin platforms.
	ErrUnsupported = errors.New("calendar: only supported on macOS (darwin)")

	// ErrAccessDenied is returned when the user denies calendar access (TCC).
	ErrAccessDenied = errors.New("calendar: access denied")

	// ErrNotFound is returned when an event or calendar is not found.
	ErrNotFound = errors.New("calendar: not found")
)

// Client provides access to macOS Calendar via EventKit.
type Client struct{}

// Calendar represents a calendar (e.g., "Work", "Personal", "Holidays").
type Calendar struct {
	ID       string       `json:"id"`
	Title    string       `json:"title"`
	Type     CalendarType `json:"type"`
	Color    string       `json:"color"`
	Source   string       `json:"source"`
	ReadOnly bool        `json:"readOnly"`
}

// Event represents a calendar event.
type Event struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	StartDate    time.Time    `json:"startDate"`
	EndDate      time.Time    `json:"endDate"`
	AllDay       bool         `json:"allDay"`
	Location     string       `json:"location"`
	Notes        string       `json:"notes"`
	URL          string       `json:"url"`
	Calendar     string       `json:"calendar"`
	CalendarID   string       `json:"calendarID"`
	Status       EventStatus  `json:"status"`
	Availability Availability `json:"availability"`
	Organizer    string       `json:"organizer"`
	Attendees    []Attendee   `json:"attendees"`
	Recurring    bool         `json:"recurring"`
	Alerts       []Alert      `json:"alerts"`
	CreatedAt    time.Time    `json:"createdAt"`
	ModifiedAt   time.Time    `json:"modifiedAt"`
	TimeZone     string       `json:"timeZone"`
}

// Attendee represents a participant in an event (read-only from EventKit).
type Attendee struct {
	Name   string            `json:"name"`
	Email  string            `json:"email"`
	Status ParticipantStatus `json:"status"`
}

// Alert represents a reminder alert before an event.
type Alert struct {
	// RelativeOffset is the time before the event to fire the alert.
	// Negative values mean before the event (e.g., -15*time.Minute).
	RelativeOffset time.Duration `json:"relativeOffset"`
}

// EventStatus indicates the confirmation status of an event.
type EventStatus int

const (
	StatusNone      EventStatus = 0
	StatusConfirmed EventStatus = 1
	StatusTentative EventStatus = 2
	StatusCanceled  EventStatus = 3
)

// String returns a human-readable representation of the event status.
func (s EventStatus) String() string {
	switch s {
	case StatusNone:
		return "none"
	case StatusConfirmed:
		return "confirmed"
	case StatusTentative:
		return "tentative"
	case StatusCanceled:
		return "canceled"
	default:
		return "unknown"
	}
}

// Availability indicates the user's availability during an event.
type Availability int

const (
	AvailabilityNotSupported Availability = -1
	AvailabilityBusy         Availability = 0
	AvailabilityFree         Availability = 1
	AvailabilityTentative    Availability = 2
	AvailabilityUnavailable  Availability = 3
)

// String returns a human-readable representation of the availability.
func (a Availability) String() string {
	switch a {
	case AvailabilityNotSupported:
		return "notSupported"
	case AvailabilityBusy:
		return "busy"
	case AvailabilityFree:
		return "free"
	case AvailabilityTentative:
		return "tentative"
	case AvailabilityUnavailable:
		return "unavailable"
	default:
		return "unknown"
	}
}

// CalendarType indicates the type of a calendar.
type CalendarType int

const (
	CalendarTypeLocal        CalendarType = 0
	CalendarTypeCalDAV       CalendarType = 1
	CalendarTypeExchange     CalendarType = 2
	CalendarTypeBirthday     CalendarType = 4
	CalendarTypeSubscription CalendarType = 5
)

// String returns a human-readable representation of the calendar type.
func (t CalendarType) String() string {
	switch t {
	case CalendarTypeLocal:
		return "local"
	case CalendarTypeCalDAV:
		return "caldav"
	case CalendarTypeExchange:
		return "exchange"
	case CalendarTypeBirthday:
		return "birthday"
	case CalendarTypeSubscription:
		return "subscription"
	default:
		return "unknown"
	}
}

// ParticipantStatus indicates an attendee's response status.
type ParticipantStatus int

const (
	ParticipantStatusUnknown   ParticipantStatus = 0
	ParticipantStatusPending   ParticipantStatus = 1
	ParticipantStatusAccepted  ParticipantStatus = 2
	ParticipantStatusDeclined  ParticipantStatus = 3
	ParticipantStatusTentative ParticipantStatus = 4
)

// String returns a human-readable representation of the participant status.
func (s ParticipantStatus) String() string {
	switch s {
	case ParticipantStatusUnknown:
		return "unknown"
	case ParticipantStatusPending:
		return "pending"
	case ParticipantStatusAccepted:
		return "accepted"
	case ParticipantStatusDeclined:
		return "declined"
	case ParticipantStatusTentative:
		return "tentative"
	default:
		return "unknown"
	}
}

// Span controls whether an operation affects a single occurrence or future occurrences of a recurring event.
type Span int

const (
	// SpanThisEvent affects only this occurrence of a recurring event.
	SpanThisEvent Span = 0
	// SpanFutureEvents affects this and all future occurrences.
	SpanFutureEvents Span = 1
)

// String returns a human-readable representation of the span.
func (s Span) String() string {
	switch s {
	case SpanThisEvent:
		return "thisEvent"
	case SpanFutureEvents:
		return "futureEvents"
	default:
		return "unknown"
	}
}

// ListOption configures event listing behavior.
type ListOption func(*listOptions)

type listOptions struct {
	calendarName string
	calendarID   string
	searchQuery  string
}

// WithCalendar filters events by calendar name.
func WithCalendar(name string) ListOption {
	return func(o *listOptions) {
		o.calendarName = name
	}
}

// WithCalendarID filters events by calendar identifier.
func WithCalendarID(id string) ListOption {
	return func(o *listOptions) {
		o.calendarID = id
	}
}

// WithSearch filters events by a search query (matches title, location, notes).
func WithSearch(query string) ListOption {
	return func(o *listOptions) {
		o.searchQuery = query
	}
}

// CreateEventInput contains the fields needed to create a new event.
type CreateEventInput struct {
	Title     string    `json:"title"`
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
	AllDay    bool      `json:"allDay"`
	Location  string    `json:"location"`
	Notes     string    `json:"notes"`
	URL       string    `json:"url"`
	Calendar  string    `json:"calendar"`
	Alerts    []Alert   `json:"alerts"`
	TimeZone  string    `json:"timeZone"`
}

// UpdateEventInput contains the fields that can be updated on an event.
// Nil pointer fields are not changed.
type UpdateEventInput struct {
	Title    *string    `json:"title,omitempty"`
	StartDate *time.Time `json:"startDate,omitempty"`
	EndDate   *time.Time `json:"endDate,omitempty"`
	AllDay    *bool      `json:"allDay,omitempty"`
	Location  *string    `json:"location,omitempty"`
	Notes     *string    `json:"notes,omitempty"`
	URL       *string    `json:"url,omitempty"`
	Calendar  *string    `json:"calendar,omitempty"`
	Alerts    *[]Alert   `json:"alerts,omitempty"`
	TimeZone  *string    `json:"timeZone,omitempty"`
}

// applyOptions applies ListOption functions and returns the resulting options.
func applyOptions(opts []ListOption) listOptions {
	var o listOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
