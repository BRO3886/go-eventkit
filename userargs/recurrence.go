// Package userargs converts user-friendly argument shapes (typically from MCP
// tool inputs or CLI flags) into go-eventkit primitives like RecurrenceRule.
// It is cgo-free and builds on every platform — callers run the parsers
// before crossing into the platform-specific calendar/reminders packages.
package userargs

import (
	"errors"
	"fmt"
	"strings"

	eventkit "github.com/BRO3886/go-eventkit"
	"github.com/BRO3886/go-eventkit/dateparser"
)

// RecurrenceArgs is the input shape for ParseRecurrence. It mirrors the
// shape MCP/CLI consumers want to accept from the user — string frequency,
// short weekday codes ("MO", "Wed", "thursday"), and a single rule per
// event (multiple-rule recurrences are valid in EventKit but rare).
type RecurrenceArgs struct {
	// Frequency is one of "daily", "weekly", "monthly", "yearly".
	// Case-insensitive. Empty string returns nil from ParseRecurrence.
	Frequency string
	// Interval is the number of frequency units between occurrences
	// (e.g., Frequency="weekly", Interval=2 → every other week).
	// Values < 0 return an error. Value 0 is treated as the default of 1.
	Interval int
	// Count stops the recurrence after N occurrences. Mutually exclusive
	// with Until. Negative values return an error.
	Count int
	// Until is a date string (natural language or ISO 8601) parsed via
	// dateparser.ParseDate. Mutually exclusive with Count.
	Until string
	// ByDay is a slice of weekday codes ("MO", "Mon", "Monday", any case).
	// Accepted for weekly, monthly, and yearly frequencies. Rejected for daily.
	ByDay []string
}

// ParseRecurrence converts RecurrenceArgs into a slice of eventkit.RecurrenceRule
// suitable for calendar.CreateEventInput.RecurrenceRules or the equivalent
// reminders input.
//
// Returns nil when args is nil or args.Frequency is empty (caller can pass
// the result directly without a nil-check).
//
// Errors are returned for:
//   - Unknown Frequency
//   - Both Count and Until set
//   - Until that fails to parse via dateparser.ParseDate
//   - Unknown weekday code in ByDay
//   - A rule that fails eventkit.RecurrenceRule.Validate()
func ParseRecurrence(args *RecurrenceArgs) ([]eventkit.RecurrenceRule, error) {
	if args == nil || args.Frequency == "" {
		return nil, nil
	}
	if args.Count > 0 && args.Until != "" {
		return nil, errors.New("Count and Until are mutually exclusive")
	}
	if args.Count < 0 {
		return nil, fmt.Errorf("Count must be >= 0 (got %d)", args.Count)
	}
	if args.Interval < 0 {
		return nil, fmt.Errorf("Interval must be >= 0 (got %d)", args.Interval)
	}
	interval := args.Interval
	if interval == 0 {
		interval = 1
	}

	var rule eventkit.RecurrenceRule
	freq := strings.ToLower(strings.TrimSpace(args.Frequency))

	days, err := ParseWeekdays(args.ByDay)
	if err != nil {
		return nil, err
	}

	switch freq {
	case "daily":
		if len(days) > 0 {
			return nil, errors.New("ByDay is not valid for daily frequency")
		}
		rule = eventkit.Daily(interval)
	case "weekly":
		rule = eventkit.Weekly(interval, days...)
	case "monthly":
		rule = eventkit.Monthly(interval)
		if len(days) > 0 {
			rule.DaysOfTheWeek = make([]eventkit.RecurrenceDayOfWeek, 0, len(days))
			for _, d := range days {
				rule.DaysOfTheWeek = append(rule.DaysOfTheWeek, eventkit.RecurrenceDayOfWeek{DayOfTheWeek: d})
			}
		}
	case "yearly":
		rule = eventkit.Yearly(interval)
		if len(days) > 0 {
			rule.DaysOfTheWeek = make([]eventkit.RecurrenceDayOfWeek, 0, len(days))
			for _, d := range days {
				rule.DaysOfTheWeek = append(rule.DaysOfTheWeek, eventkit.RecurrenceDayOfWeek{DayOfTheWeek: d})
			}
		}
	default:
		return nil, fmt.Errorf("Frequency must be daily, weekly, monthly, or yearly (got %q)", args.Frequency)
	}

	if args.Until != "" {
		t, err := dateparser.ParseDate(args.Until)
		if err != nil {
			return nil, fmt.Errorf("invalid Until: %w", err)
		}
		rule = rule.Until(t)
	}
	if args.Count > 0 {
		rule = rule.Count(args.Count)
	}
	if err := rule.Validate(); err != nil {
		return nil, fmt.Errorf("invalid recurrence rule: %w", err)
	}
	return []eventkit.RecurrenceRule{rule}, nil
}

// weekdayLookup maps 2-letter, 3-letter, and full weekday names (lowercase,
// trimmed) to eventkit.Weekday values. Unexported to prevent external mutation
// of the shared map, which would be a data-race vector.
var weekdayLookup = map[string]eventkit.Weekday{
	"su": eventkit.Sunday, "sun": eventkit.Sunday, "sunday": eventkit.Sunday,
	"mo": eventkit.Monday, "mon": eventkit.Monday, "monday": eventkit.Monday,
	"tu": eventkit.Tuesday, "tue": eventkit.Tuesday, "tuesday": eventkit.Tuesday,
	"we": eventkit.Wednesday, "wed": eventkit.Wednesday, "wednesday": eventkit.Wednesday,
	"th": eventkit.Thursday, "thu": eventkit.Thursday, "thursday": eventkit.Thursday,
	"fr": eventkit.Friday, "fri": eventkit.Friday, "friday": eventkit.Friday,
	"sa": eventkit.Saturday, "sat": eventkit.Saturday, "saturday": eventkit.Saturday,
}

// ParseWeekday returns the eventkit.Weekday for s, treating the input
// case-insensitively and ignoring surrounding whitespace. Accepted forms:
// "MO", "Mon", "Monday" (and similarly for the other six days).
func ParseWeekday(s string) (eventkit.Weekday, error) {
	key := strings.ToLower(strings.TrimSpace(s))
	if w, ok := weekdayLookup[key]; ok {
		return w, nil
	}
	return eventkit.Sunday, fmt.Errorf("unknown weekday: %q", s)
}

// ParseWeekdays converts a slice of weekday codes into eventkit.Weekday values,
// preserving order and returning an error on the first unknown entry.
func ParseWeekdays(in []string) ([]eventkit.Weekday, error) {
	out := make([]eventkit.Weekday, 0, len(in))
	for _, s := range in {
		w, err := ParseWeekday(s)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}
