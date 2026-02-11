// Package calendar provides native macOS Calendar bindings via EventKit.
//
// It exposes idiomatic Go types and a Client API for reading and writing
// calendar events, backed by Apple's EventKit framework for in-process,
// sub-200ms access. No AppleScript, no subprocesses.
//
// EventKit sees all configured accounts — iCloud, Google (CalDAV), Exchange,
// local, and subscribed calendars. Attendees and organizer fields are read-only
// (Apple limitation).
//
// # Platform Support
//
// This package requires macOS (darwin). On other platforms, [New] returns
// [ErrUnsupported]. Types and constants are importable on all platforms for
// use in cross-platform code.
//
// # Permissions
//
// On first call to [New], macOS displays a TCC (Transparency, Consent, and
// Control) prompt requesting calendar access. The prompt shows the terminal
// application name, not the Go binary. If denied, [New] returns
// [ErrAccessDenied]. Permissions can be managed in System Settings >
// Privacy & Security > Calendars.
//
// # Usage
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

	"github.com/BRO3886/go-eventkit"
)

// Sentinel errors returned by Client methods. Use [errors.Is] to check:
//
//	if errors.Is(err, calendar.ErrNotFound) { ... }
var (
	// ErrUnsupported is returned by [New] on non-darwin platforms.
	ErrUnsupported = errors.New("calendar: only supported on macOS (darwin)")

	// ErrAccessDenied is returned by [New] when the user denies calendar
	// access via the macOS TCC prompt.
	ErrAccessDenied = errors.New("calendar: access denied")

	// ErrNotFound is returned by [Client.Event], [Client.UpdateEvent], and
	// [Client.DeleteEvent] when no event matches the given ID.
	ErrNotFound = errors.New("calendar: not found")

	// ErrImmutable is returned by [Client.UpdateCalendar] and
	// [Client.DeleteCalendar] when the target calendar is immutable
	// (e.g., subscribed or birthday calendars).
	ErrImmutable = errors.New("calendar: calendar is immutable")
)

// Client provides access to macOS Calendar via EventKit.
//
// Create a Client with [New]. All methods are safe to call from a single
// goroutine. For concurrent usage from multiple goroutines, see the
// concurrency notes in the package documentation.
type Client struct{}

// Calendar represents a calendar container (e.g., "Work", "Personal", "Holidays").
// Calendars belong to a source (iCloud, Google, Exchange, etc.) and contain events.
type Calendar struct {
	// ID is the stable calendar identifier (EKCalendar.calendarIdentifier).
	ID string `json:"id"`
	// Title is the display name of the calendar.
	Title string `json:"title"`
	// Type indicates the calendar backend (local, CalDAV, Exchange, etc.).
	Type CalendarType `json:"type"`
	// Color is the calendar's display color as a hex string (e.g., "#FF6961").
	Color string `json:"color"`
	// Source is the account name this calendar belongs to (e.g., "iCloud", "Gmail").
	Source string `json:"source"`
	// ReadOnly is true for calendars that cannot be modified (subscriptions, birthdays).
	ReadOnly bool `json:"readOnly"`
}

// Event represents a calendar event (EKEvent).
type Event struct {
	// ID is the stable event identifier (EKEvent.eventIdentifier).
	// This ID is stable across recurrence edits, unlike calendarItemIdentifier.
	ID string `json:"id"`
	// Title is the event's display title.
	Title string `json:"title"`
	// StartDate is when the event begins.
	StartDate time.Time `json:"startDate"`
	// EndDate is when the event ends.
	EndDate time.Time `json:"endDate"`
	// AllDay is true for events that span entire days without specific times.
	AllDay bool `json:"allDay"`
	// Location is the event's location string. May be empty.
	Location string `json:"location"`
	// Notes is the event's plain-text notes. EventKit does not support rich text.
	Notes string `json:"notes"`
	// URL is an optional URL associated with the event (e.g., a meeting link).
	URL string `json:"url"`
	// Calendar is the display name of the calendar this event belongs to.
	Calendar string `json:"calendar"`
	// CalendarID is the identifier of the calendar this event belongs to.
	CalendarID string `json:"calendarID"`
	// Status is the event's confirmation status (none, confirmed, tentative, canceled).
	Status EventStatus `json:"status"`
	// Availability indicates the user's availability during this event.
	Availability Availability `json:"availability"`
	// Organizer is the display name of the event organizer. Read-only from EventKit.
	Organizer string `json:"organizer"`
	// Attendees lists the event participants. Read-only — EventKit cannot
	// add or modify attendees (Apple limitation since 2013).
	Attendees []Attendee `json:"attendees"`
	// Recurring is true if this event is part of a recurrence series.
	Recurring bool `json:"recurring"`
	// RecurrenceRules contains the recurrence patterns for this event.
	// Empty if the event is not recurring.
	RecurrenceRules []eventkit.RecurrenceRule `json:"recurrenceRules,omitempty"`
	// IsDetached is true when this occurrence of a recurring event was
	// modified independently from the series.
	IsDetached bool `json:"isDetached"`
	// OccurrenceDate is the original date for this occurrence of a recurring
	// event, even if the occurrence has been moved to a different date.
	OccurrenceDate *time.Time `json:"occurrenceDate,omitempty"`
	// StructuredLocation contains geographic coordinates and geofence data.
	// Nil if no structured location is set. The Title field may differ from Location.
	StructuredLocation *eventkit.StructuredLocation `json:"structuredLocation,omitempty"`
	// Alerts lists the notification alerts configured for this event.
	Alerts []Alert `json:"alerts"`
	// CreatedAt is when the event was first created.
	CreatedAt time.Time `json:"createdAt"`
	// ModifiedAt is when the event was last modified.
	ModifiedAt time.Time `json:"modifiedAt"`
	// TimeZone is the IANA timezone identifier for this event (e.g., "America/New_York").
	TimeZone string `json:"timeZone"`
}

// Attendee represents a participant in an event.
//
// Attendees are read-only from EventKit — Apple does not provide an API to
// add, remove, or modify event participants.
type Attendee struct {
	// Name is the attendee's display name.
	Name string `json:"name"`
	// Email is the attendee's email address.
	Email string `json:"email"`
	// Status is the attendee's response status (pending, accepted, declined, etc.).
	Status ParticipantStatus `json:"status"`
}

// Alert represents a notification alert before an event.
type Alert struct {
	// RelativeOffset is the time before the event to fire the alert.
	// Negative values mean before the event (e.g., -15*time.Minute for 15 minutes before).
	// Positive values mean after the event start.
	RelativeOffset time.Duration `json:"relativeOffset"`
}

// EventStatus indicates the confirmation status of an event.
// Values correspond to Apple's EKEventStatus enum.
type EventStatus int

const (
	StatusNone      EventStatus = 0 // No status set.
	StatusConfirmed EventStatus = 1 // Event is confirmed.
	StatusTentative EventStatus = 2 // Event is tentatively accepted.
	StatusCanceled  EventStatus = 3 // Event has been canceled.
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

// Availability indicates the user's free/busy status during an event.
// Values correspond to Apple's EKEventAvailability enum.
type Availability int

const (
	AvailabilityNotSupported Availability = -1 // Availability not supported by the calendar source.
	AvailabilityBusy         Availability = 0  // User is busy during this event.
	AvailabilityFree         Availability = 1  // User is free during this event.
	AvailabilityTentative    Availability = 2  // User is tentatively busy.
	AvailabilityUnavailable  Availability = 3  // User is unavailable.
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

// CalendarType indicates the backend type of a calendar.
// Values correspond to Apple's EKCalendarType enum.
type CalendarType int

const (
	CalendarTypeLocal        CalendarType = 0 // Locally stored calendar.
	CalendarTypeCalDAV       CalendarType = 1 // CalDAV calendar (iCloud, Google, etc.).
	CalendarTypeExchange     CalendarType = 2 // Microsoft Exchange calendar.
	CalendarTypeSubscription CalendarType = 3 // Subscribed .ics calendar (read-only).
	CalendarTypeBirthday     CalendarType = 4 // Birthday calendar from Contacts (read-only).
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

// ParticipantStatus indicates an attendee's RSVP response status.
// Values correspond to Apple's EKParticipantStatus enum.
type ParticipantStatus int

const (
	ParticipantStatusUnknown   ParticipantStatus = 0 // Response status is unknown.
	ParticipantStatusPending   ParticipantStatus = 1 // Attendee has not yet responded.
	ParticipantStatusAccepted  ParticipantStatus = 2 // Attendee accepted the invitation.
	ParticipantStatusDeclined  ParticipantStatus = 3 // Attendee declined the invitation.
	ParticipantStatusTentative ParticipantStatus = 4 // Attendee tentatively accepted.
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

// Span controls whether a write operation (update, delete) affects a single
// occurrence or all future occurrences of a recurring event.
// For non-recurring events, the span value has no effect.
// Values correspond to Apple's EKSpan enum.
type Span int

const (
	// SpanThisEvent affects only this occurrence of a recurring event.
	SpanThisEvent Span = 0
	// SpanFutureEvents affects this and all future occurrences of a recurring event.
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

// ListOption configures filtering for [Client.Events].
// Multiple options can be combined; all filters are applied together (AND logic).
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

// CreateEventInput contains the fields for creating a new event via
// [Client.CreateEvent].
//
// Title, StartDate, and EndDate are required. All other fields are optional.
type CreateEventInput struct {
	// Title is the event title (required).
	Title string `json:"title"`
	// StartDate is when the event begins (required).
	StartDate time.Time `json:"startDate"`
	// EndDate is when the event ends (required).
	EndDate time.Time `json:"endDate"`
	// AllDay marks the event as an all-day event. When true, only the date
	// portion of StartDate and EndDate is used.
	AllDay bool `json:"allDay"`
	// Location is the event's location string.
	Location string `json:"location"`
	// Notes is plain-text notes for the event.
	Notes string `json:"notes"`
	// URL is an optional URL to associate with the event.
	URL string `json:"url"`
	// Calendar is the name of the calendar to create the event in.
	// If empty, the system default calendar is used.
	Calendar string `json:"calendar"`
	// Alerts configures notification alerts before the event.
	Alerts []Alert `json:"alerts"`
	// TimeZone is the IANA timezone for the event (e.g., "Asia/Tokyo").
	// If empty, the system timezone is used.
	TimeZone string `json:"timeZone"`
	// RecurrenceRules sets the recurrence pattern(s) for the event.
	// Most events have zero or one rule. Multiple rules are supported by
	// EventKit but uncommon.
	RecurrenceRules []eventkit.RecurrenceRule `json:"recurrenceRules,omitempty"`
	// StructuredLocation sets geographic coordinates for the event.
	// If set, this takes precedence over Location for map integrations.
	// The plain Location string is set independently.
	StructuredLocation *eventkit.StructuredLocation `json:"structuredLocation,omitempty"`
}

// UpdateEventInput contains fields for updating an existing event via
// [Client.UpdateEvent].
//
// Only non-nil pointer fields are modified. Nil fields are left unchanged.
// To clear a string field, set it to a pointer to an empty string.
type UpdateEventInput struct {
	Title     *string    `json:"title,omitempty"`
	StartDate *time.Time `json:"startDate,omitempty"`
	EndDate   *time.Time `json:"endDate,omitempty"`
	AllDay    *bool      `json:"allDay,omitempty"`
	Location  *string    `json:"location,omitempty"`
	Notes     *string    `json:"notes,omitempty"`
	URL       *string    `json:"url,omitempty"`
	// Calendar moves the event to a different calendar by name.
	Calendar *string `json:"calendar,omitempty"`
	// Alerts replaces all existing alerts. Pass an empty slice to remove all alerts.
	Alerts   *[]Alert `json:"alerts,omitempty"`
	TimeZone *string  `json:"timeZone,omitempty"`
	// RecurrenceRules replaces all existing recurrence rules.
	// Pass an empty slice to make the event non-recurring.
	// Pass nil to leave recurrence unchanged.
	RecurrenceRules *[]eventkit.RecurrenceRule `json:"recurrenceRules,omitempty"`
	// StructuredLocation updates the geographic location. Set to non-nil to
	// update, leave nil to keep unchanged.
	StructuredLocation *eventkit.StructuredLocation `json:"structuredLocation,omitempty"`
}

// CreateCalendarInput contains the fields for creating a new calendar via
// [Client.CreateCalendar].
//
// Title and Source are required. Color is optional.
type CreateCalendarInput struct {
	// Title is the display name for the new calendar (required).
	Title string
	// Source is the account name to create the calendar in (required).
	// Use [Client.Calendars] to discover available source names (e.g., "iCloud",
	// "siddverma1999@gmail.com"). Not all sources support calendar creation.
	Source string
	// Color is the calendar's display color as a hex string (e.g., "#FF6961").
	// If empty, the system default color is used.
	Color string
}

// UpdateCalendarInput contains fields for updating an existing calendar via
// [Client.UpdateCalendar].
//
// Only non-nil pointer fields are modified. Nil fields are left unchanged.
type UpdateCalendarInput struct {
	// Title renames the calendar.
	Title *string
	// Color changes the calendar's display color (hex string, e.g., "#FF6961").
	Color *string
}

// applyOptions applies ListOption functions and returns the resulting options.
func applyOptions(opts []ListOption) listOptions {
	var o listOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
