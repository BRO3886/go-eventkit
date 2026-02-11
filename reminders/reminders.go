// Package reminders provides native macOS Reminders bindings via EventKit.
//
// This package uses cgo + Objective-C to access Apple's EventKit framework
// directly, providing in-process, sub-200ms access to reminders data.
// All operations (reads AND writes) go through EventKit — no AppleScript
// or subprocess overhead.
//
// EventKit sees all configured accounts (iCloud, Exchange, etc.) and provides
// access to all reminder lists and items.
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
// Control) prompt requesting reminders access. The prompt shows the terminal
// application name, not the Go binary. If denied, [New] returns
// [ErrAccessDenied]. Permissions can be managed in System Settings >
// Privacy & Security > Reminders.
//
// # Known Limitations
//
// The [Reminder.Flagged] field is always false — Apple's EventKit framework
// does not expose the "flagged" property despite it being visible in the
// Reminders app.
//
// # Usage
//
//	client, err := reminders.New()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// List all reminder lists
//	lists, err := client.Lists()
//
//	// Get incomplete reminders from a specific list
//	items, err := client.Reminders(
//	    reminders.WithList("Shopping"),
//	    reminders.WithCompleted(false),
//	)
//
//	// Create a reminder with a due date
//	due := time.Now().Add(24 * time.Hour)
//	r, err := client.CreateReminder(reminders.CreateReminderInput{
//	    Title:    "Buy milk",
//	    ListName: "Shopping",
//	    DueDate:  &due,
//	    Priority: reminders.PriorityHigh,
//	})
//
//	// Mark it as done
//	client.CompleteReminder(r.ID)
package reminders

import (
	"errors"
	"time"

	"github.com/BRO3886/go-eventkit"
)

// Client provides access to macOS Reminders via EventKit.
//
// Create a Client with [New]. All methods are safe to call from a single
// goroutine. For concurrent usage from multiple goroutines, see the
// concurrency notes in the package documentation.
type Client struct{}

// Sentinel errors returned by Client methods. Use [errors.Is] to check:
//
//	if errors.Is(err, reminders.ErrNotFound) { ... }
var (
	// ErrUnsupported is returned by [New] on non-darwin platforms.
	ErrUnsupported = errors.New("reminders: not supported on this platform")

	// ErrAccessDenied is returned by [New] when the user denies reminders
	// access via the macOS TCC prompt.
	ErrAccessDenied = errors.New("reminders: access denied")

	// ErrNotFound is returned by [Client.Reminder], [Client.UpdateReminder],
	// [Client.DeleteReminder], [Client.CompleteReminder], and
	// [Client.UncompleteReminder] when no reminder matches the given ID.
	ErrNotFound = errors.New("reminders: not found")

	// ErrImmutable is returned by [Client.UpdateList] and
	// [Client.DeleteList] when the target list is immutable.
	ErrImmutable = errors.New("reminders: list is immutable")
)

// Reminder represents a single reminder item (EKReminder).
type Reminder struct {
	// ID is the reminder's unique identifier (EKCalendarItem.calendarItemIdentifier).
	// Can be used with [Client.Reminder] for lookup (full ID or prefix).
	ID string `json:"id"`
	// Title is the reminder's display title.
	Title string `json:"title"`
	// Notes is plain-text notes. EventKit does not support rich text.
	Notes string `json:"notes,omitempty"`
	// List is the display name of the reminder list this item belongs to.
	List string `json:"list"`
	// ListID is the identifier of the reminder list.
	ListID string `json:"listID"`
	// DueDate is when the reminder is due. Nil if no due date is set.
	// In EventKit, due dates use NSDateComponents (date-only, no time component
	// unless explicitly set).
	DueDate *time.Time `json:"dueDate,omitempty"`
	// RemindMeDate is when the reminder notification fires. Independent of DueDate.
	RemindMeDate *time.Time `json:"remindMeDate,omitempty"`
	// CompletionDate is when the reminder was marked complete. Nil if incomplete.
	CompletionDate *time.Time `json:"completionDate,omitempty"`
	// CreatedAt is when the reminder was first created.
	CreatedAt *time.Time `json:"createdAt,omitempty"`
	// ModifiedAt is when the reminder was last modified.
	ModifiedAt *time.Time `json:"modifiedAt,omitempty"`
	// Priority is the reminder's priority level (0=none, 1=high, 5=medium, 9=low).
	Priority Priority `json:"priority"`
	// Completed is true if the reminder has been marked as done.
	Completed bool `json:"completed"`
	// Flagged indicates whether the reminder is flagged. Note: this is always
	// false because Apple's EventKit does not expose the flagged property.
	Flagged bool `json:"flagged"`
	// URL is an optional URL associated with the reminder.
	URL string `json:"url,omitempty"`
	// Recurring is true if this reminder has recurrence rules.
	Recurring bool `json:"recurring"`
	// RecurrenceRules contains the recurrence patterns for this reminder.
	// Empty if the reminder is not recurring. EKReminder inherits recurrence
	// support from EKCalendarItem.
	RecurrenceRules []eventkit.RecurrenceRule `json:"recurrenceRules,omitempty"`
	// HasAlarms is true if the reminder has any alarms configured.
	HasAlarms bool `json:"hasAlarms"`
	// Alarms lists the notification alarms configured for this reminder.
	Alarms []Alarm `json:"alarms,omitempty"`
}

// List represents a Reminders list (EKCalendar of entity type Reminder).
type List struct {
	// ID is the list's unique identifier (EKCalendar.calendarIdentifier).
	ID string `json:"id"`
	// Title is the display name of the list.
	Title string `json:"title"`
	// Color is the list's display color as a hex string (e.g., "#FF6961").
	Color string `json:"color,omitempty"`
	// Source is the account name this list belongs to (e.g., "iCloud").
	Source string `json:"source,omitempty"`
	// Count is the number of reminders in this list.
	Count int `json:"count"`
	// ReadOnly is true if the list cannot be modified.
	ReadOnly bool `json:"readOnly"`
}

// Alarm represents a reminder notification alert.
//
// An alarm is either absolute (fires at a specific time) or relative
// (fires a duration before the due date). Set one or the other, not both.
type Alarm struct {
	// AbsoluteDate fires the alarm at this exact time. Nil for relative alarms.
	AbsoluteDate *time.Time `json:"absoluteDate,omitempty"`
	// RelativeOffset fires the alarm this duration before the due date.
	// Negative values mean before the due date (e.g., -30*time.Minute).
	// Zero for absolute alarms.
	RelativeOffset time.Duration `json:"relativeOffset,omitempty"`
}

// Priority represents the priority level of a reminder.
//
// Apple's EventKit uses a 0-9 integer scale that maps to three user-visible
// levels. The constants below represent the canonical values for each level.
// When reading, values 1-4 all display as "high" and 6-9 as "low" in Apple's
// Reminders app.
type Priority int

const (
	PriorityNone   Priority = 0 // No priority set.
	PriorityHigh   Priority = 1 // High priority (values 1-4 in EventKit).
	PriorityMedium Priority = 5 // Medium priority.
	PriorityLow    Priority = 9 // Low priority (values 6-9 in EventKit).
)

// String returns a human-readable representation of the priority.
func (p Priority) String() string {
	switch {
	case p == PriorityNone:
		return "none"
	case p >= 1 && p <= 4:
		return "high"
	case p == 5:
		return "medium"
	case p >= 6 && p <= 9:
		return "low"
	default:
		return "none"
	}
}

// ParsePriority converts a string to a [Priority] value.
// Accepts: "high"/"h"/"1", "medium"/"med"/"m"/"5", "low"/"l"/"9".
// Any other string returns [PriorityNone].
func ParsePriority(s string) Priority {
	switch s {
	case "high", "h", "1":
		return PriorityHigh
	case "medium", "med", "m", "5":
		return PriorityMedium
	case "low", "l", "9":
		return PriorityLow
	default:
		return PriorityNone
	}
}

// ListOption configures filtering for [Client.Reminders].
// Multiple options can be combined; all filters are applied together (AND logic).
// If the same option is provided multiple times, the last value wins.
type ListOption func(*listOptions)

type listOptions struct {
	listName  string
	listID    string
	completed *bool
	search    string
	dueBefore *time.Time
	dueAfter  *time.Time
}

// WithList filters reminders by list name.
func WithList(name string) ListOption {
	return func(o *listOptions) { o.listName = name }
}

// WithListID filters reminders by list identifier.
func WithListID(id string) ListOption {
	return func(o *listOptions) { o.listID = id }
}

// WithCompleted filters reminders by completion status.
// Pass true for completed only, false for incomplete only.
func WithCompleted(completed bool) ListOption {
	return func(o *listOptions) { o.completed = &completed }
}

// WithSearch filters reminders by search query (title and notes).
func WithSearch(query string) ListOption {
	return func(o *listOptions) { o.search = query }
}

// WithDueBefore filters reminders due before the given date.
func WithDueBefore(t time.Time) ListOption {
	return func(o *listOptions) { o.dueBefore = &t }
}

// WithDueAfter filters reminders due after the given date.
func WithDueAfter(t time.Time) ListOption {
	return func(o *listOptions) { o.dueAfter = &t }
}

// CreateListInput contains the fields for creating a new reminder list via
// [Client.CreateList].
//
// Title is required. Source and Color are optional.
type CreateListInput struct {
	// Title is the display name for the new list (required).
	Title string
	// Source is the account name to create the list in (e.g., "iCloud").
	// If empty, the default source for new reminders is used.
	Source string
	// Color is the list's display color as a hex string (e.g., "#FF6961").
	// If empty, the system default color is used.
	Color string
}

// UpdateListInput contains fields for updating an existing reminder list via
// [Client.UpdateList].
//
// Only non-nil pointer fields are modified. Nil fields are left unchanged.
type UpdateListInput struct {
	// Title renames the list.
	Title *string
	// Color changes the list's display color (hex string, e.g., "#FF6961").
	Color *string
}

func applyOptions(opts []ListOption) *listOptions {
	o := &listOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// CreateReminderInput holds the parameters for creating a reminder via
// [Client.CreateReminder].
//
// Only Title is required. All other fields are optional.
type CreateReminderInput struct {
	// Title is the reminder's display title (required).
	Title string
	// Notes is optional plain-text notes.
	Notes string
	// ListName is the name of the list to create the reminder in.
	// If empty, the system default reminders list is used.
	ListName string
	// DueDate sets when the reminder is due. Nil for no due date.
	DueDate *time.Time
	// RemindMeDate sets when the notification alarm fires. Independent of DueDate.
	RemindMeDate *time.Time
	// Priority sets the reminder's priority level.
	Priority Priority
	// URL associates a URL with the reminder.
	URL string
	// Alarms adds notification alarms to the reminder.
	Alarms []Alarm
	// RecurrenceRules sets the recurrence pattern(s) for the reminder.
	// Most reminders have zero or one rule.
	RecurrenceRules []eventkit.RecurrenceRule
}

// UpdateReminderInput holds the parameters for updating a reminder via
// [Client.UpdateReminder].
//
// Only non-nil pointer fields are modified. Nil fields are left unchanged.
// To clear a string field, set it to a pointer to an empty string.
type UpdateReminderInput struct {
	// Title updates the reminder's display title.
	Title *string
	// Notes updates the reminder's notes.
	Notes *string
	// ListName moves the reminder to a different list by name.
	ListName *string
	// DueDate updates the due date. See also ClearDueDate.
	DueDate *time.Time
	// ClearDueDate removes the due date entirely when set to true.
	// Takes precedence over DueDate if both are set.
	ClearDueDate bool
	// RemindMeDate updates when the notification alarm fires.
	RemindMeDate *time.Time
	// Priority updates the priority level.
	Priority *Priority
	// Completed marks the reminder as completed (true) or incomplete (false).
	// For a dedicated API, see [Client.CompleteReminder] and [Client.UncompleteReminder].
	Completed *bool
	// Flagged updates the flagged state. Note: EventKit does not expose the
	// flagged property, so this field has no effect.
	Flagged *bool
	// URL updates the associated URL.
	URL *string
	// Alarms replaces all existing alarms. Pass an empty slice to remove all alarms.
	Alarms *[]Alarm
	// RecurrenceRules replaces all existing recurrence rules.
	// Pass an empty slice to make the reminder non-recurring.
	// Pass nil to leave recurrence unchanged.
	RecurrenceRules *[]eventkit.RecurrenceRule
}
