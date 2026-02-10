// Package reminders provides native macOS Reminders bindings via EventKit.
//
// This package uses cgo + Objective-C to access Apple's EventKit framework
// directly, providing in-process, sub-200ms access to reminders data.
// All operations (reads AND writes) go through EventKit — no AppleScript
// or subprocess overhead.
//
// Usage:
//
//	client, err := reminders.New()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// List all reminder lists
//	lists, err := client.Lists()
//
//	// Get reminders from a specific list
//	items, err := client.Reminders(reminders.WithList("Shopping"))
//
//	// Create a reminder
//	r, err := client.CreateReminder(reminders.CreateReminderInput{
//	    Title:    "Buy milk",
//	    ListName: "Shopping",
//	})
package reminders

import (
	"errors"
	"time"
)

// Client provides access to macOS Reminders via EventKit.
type Client struct{}

// Sentinel errors.
var (
	ErrUnsupported  = errors.New("reminders: not supported on this platform")
	ErrAccessDenied = errors.New("reminders: access denied")
	ErrNotFound     = errors.New("reminders: not found")
)

// Reminder represents a single reminder item.
type Reminder struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Notes          string     `json:"notes,omitempty"`
	List           string     `json:"list"`
	ListID         string     `json:"listID"`
	DueDate        *time.Time `json:"dueDate,omitempty"`
	RemindMeDate   *time.Time `json:"remindMeDate,omitempty"`
	CompletionDate *time.Time `json:"completionDate,omitempty"`
	CreatedAt      *time.Time `json:"createdAt,omitempty"`
	ModifiedAt     *time.Time `json:"modifiedAt,omitempty"`
	Priority       Priority   `json:"priority"`
	Completed      bool       `json:"completed"`
	Flagged        bool       `json:"flagged"`
	URL            string     `json:"url,omitempty"`
	HasAlarms      bool       `json:"hasAlarms"`
	Alarms         []Alarm    `json:"alarms,omitempty"`
}

// List represents a Reminders list.
type List struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Color    string `json:"color,omitempty"`
	Source   string `json:"source,omitempty"`
	Count    int    `json:"count"`
	ReadOnly bool   `json:"readOnly"`
}

// Alarm represents a reminder alarm/notification.
type Alarm struct {
	AbsoluteDate   *time.Time    `json:"absoluteDate,omitempty"`
	RelativeOffset time.Duration `json:"relativeOffset,omitempty"`
}

// Priority represents the priority level of a reminder.
// Values match Apple's EventKit: 0=none, 1-4=high, 5=medium, 6-9=low.
type Priority int

const (
	PriorityNone   Priority = 0
	PriorityHigh   Priority = 1
	PriorityMedium Priority = 5
	PriorityLow    Priority = 9
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

// ParsePriority converts a string to a Priority value.
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

// ListOption configures how reminders are listed.
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

func applyOptions(opts []ListOption) *listOptions {
	o := &listOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// CreateReminderInput holds the parameters for creating a reminder.
type CreateReminderInput struct {
	Title        string
	Notes        string
	ListName     string     // List name (uses default if empty)
	DueDate      *time.Time // Due date (optional)
	RemindMeDate *time.Time // Alarm date (optional)
	Priority     Priority
	URL          string
	Alarms       []Alarm // Additional alarms (optional)
}

// UpdateReminderInput holds the parameters for updating a reminder.
// Nil pointer fields are not updated. Use empty string to clear string fields.
type UpdateReminderInput struct {
	Title        *string
	Notes        *string
	ListName     *string // Move to a different list
	DueDate      *time.Time
	ClearDueDate bool // Set to true to remove the due date
	RemindMeDate *time.Time
	Priority     *Priority
	Completed    *bool
	Flagged      *bool
	URL          *string
	Alarms       *[]Alarm // Replace all alarms
}
