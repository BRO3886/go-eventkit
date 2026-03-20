// Package eventkit provides shared types for Apple's EventKit framework.
//
// This package defines recurrence rules, structured locations, and related
// types that are shared between the [calendar] and [reminders] subpackages.
// These types correspond to Apple's EKRecurrenceRule, EKStructuredLocation,
// and related EventKit classes.
//
// # Recurrence Rules
//
// Recurrence rules define how calendar events and reminders repeat.
// Use the convenience constructors [Daily], [Weekly], [Monthly], and [Yearly]
// to create rules, then chain [RecurrenceRule.Until] or [RecurrenceRule.Count]
// to set an end condition:
//
//	rule := eventkit.Daily(1).Count(30)          // Every day, 30 times
//	rule := eventkit.Weekly(2, eventkit.Monday)   // Every 2 weeks on Monday
//	rule := eventkit.Monthly(1, 1, 15)            // 1st and 15th of every month
//
// # Structured Locations
//
// [StructuredLocation] represents a geographic location with optional
// coordinates and geofence radius. Used by calendar events for map
// integrations and geofence alerts.
package eventkit

import (
	"fmt"
	"time"
)

// RecurrenceRule defines how an event or reminder repeats.
// Corresponds to EKRecurrenceRule (subset of iCalendar RRULE).
type RecurrenceRule struct {
	// Frequency is how often the item repeats (daily, weekly, monthly, yearly).
	Frequency RecurrenceFrequency `json:"frequency"`
	// Interval is the number of frequency units between occurrences.
	// E.g., Frequency=Weekly + Interval=2 means every 2 weeks.
	Interval int `json:"interval"`
	// DaysOfTheWeek specifies which days of the week the item occurs on.
	// Only relevant for weekly or monthly frequency. Nil means not constrained.
	DaysOfTheWeek []RecurrenceDayOfWeek `json:"daysOfTheWeek,omitempty"`
	// DaysOfTheMonth specifies which days of the month (1-31, or -1 to -31
	// for counting from end). Only relevant for monthly frequency.
	DaysOfTheMonth []int `json:"daysOfTheMonth,omitempty"`
	// MonthsOfTheYear specifies which months (1-12). Only relevant for yearly frequency.
	MonthsOfTheYear []int `json:"monthsOfTheYear,omitempty"`
	// WeeksOfTheYear specifies which weeks (1-53, or -1 to -53).
	// Only relevant for yearly frequency.
	WeeksOfTheYear []int `json:"weeksOfTheYear,omitempty"`
	// DaysOfTheYear specifies which days of the year (1-366, or -1 to -366).
	// Only relevant for yearly frequency.
	DaysOfTheYear []int `json:"daysOfTheYear,omitempty"`
	// SetPositions filters the set of occurrences within a period.
	// E.g., [-1] with DaysOfTheWeek=[Mon-Fri] means "last weekday of the month".
	SetPositions []int `json:"setPositions,omitempty"`
	// End defines when the recurrence stops. Nil means it recurs forever.
	End *RecurrenceEnd `json:"end,omitempty"`
}

// Validate checks whether the recurrence rule satisfies Apple's EventKit
// constraints. It returns a descriptive error for the first violation found,
// or nil if the rule is valid.
func (r RecurrenceRule) Validate() error {
	if r.Interval < 1 {
		return fmt.Errorf("interval must be >= 1, got %d", r.Interval)
	}

	if len(r.DaysOfTheWeek) > 0 && r.Frequency != FrequencyWeekly && r.Frequency != FrequencyMonthly && r.Frequency != FrequencyYearly {
		return fmt.Errorf("daysOfTheWeek is only valid for weekly, monthly, or yearly frequency, got %s", r.Frequency)
	}

	if len(r.DaysOfTheMonth) > 0 && r.Frequency != FrequencyMonthly && r.Frequency != FrequencyYearly {
		return fmt.Errorf("daysOfTheMonth is only valid for monthly or yearly frequency, got %s", r.Frequency)
	}

	if len(r.MonthsOfTheYear) > 0 && r.Frequency != FrequencyYearly {
		return fmt.Errorf("monthsOfTheYear is only valid for yearly frequency, got %s", r.Frequency)
	}

	if len(r.WeeksOfTheYear) > 0 && r.Frequency != FrequencyYearly {
		return fmt.Errorf("weeksOfTheYear is only valid for yearly frequency, got %s", r.Frequency)
	}

	if len(r.DaysOfTheYear) > 0 && r.Frequency != FrequencyYearly {
		return fmt.Errorf("daysOfTheYear is only valid for yearly frequency, got %s", r.Frequency)
	}

	if len(r.SetPositions) > 0 {
		hasConstraint := len(r.DaysOfTheWeek) > 0 || len(r.DaysOfTheMonth) > 0 ||
			len(r.MonthsOfTheYear) > 0 || len(r.WeeksOfTheYear) > 0 || len(r.DaysOfTheYear) > 0
		if !hasConstraint {
			return fmt.Errorf("setPositions requires at least one of daysOfTheWeek, daysOfTheMonth, monthsOfTheYear, weeksOfTheYear, or daysOfTheYear")
		}
	}

	if r.End != nil && r.End.EndDate == nil && r.End.OccurrenceCount < 1 {
		return fmt.Errorf("end.occurrenceCount must be > 0, got %d", r.End.OccurrenceCount)
	}

	return nil
}

// Until sets the recurrence to end on a specific date.
func (r RecurrenceRule) Until(t time.Time) RecurrenceRule {
	r.End = &RecurrenceEnd{EndDate: &t}
	return r
}

// Count sets the recurrence to end after n occurrences.
func (r RecurrenceRule) Count(n int) RecurrenceRule {
	r.End = &RecurrenceEnd{OccurrenceCount: n}
	return r
}

// RecurrenceFrequency defines how often an event or reminder repeats.
// Values correspond to Apple's EKRecurrenceFrequency enum.
type RecurrenceFrequency int

const (
	FrequencyDaily   RecurrenceFrequency = 0 // Repeats daily.
	FrequencyWeekly  RecurrenceFrequency = 1 // Repeats weekly.
	FrequencyMonthly RecurrenceFrequency = 2 // Repeats monthly.
	FrequencyYearly  RecurrenceFrequency = 3 // Repeats yearly.
)

// String returns a human-readable representation of the recurrence frequency.
func (f RecurrenceFrequency) String() string {
	switch f {
	case FrequencyDaily:
		return "daily"
	case FrequencyWeekly:
		return "weekly"
	case FrequencyMonthly:
		return "monthly"
	case FrequencyYearly:
		return "yearly"
	default:
		return "unknown"
	}
}

// RecurrenceDayOfWeek specifies a day of the week, optionally within a
// specific week of the month/year.
type RecurrenceDayOfWeek struct {
	// DayOfTheWeek is the day (Sunday=1 through Saturday=7).
	DayOfTheWeek Weekday `json:"dayOfTheWeek"`
	// WeekNumber is 0 for every week, 1-53 for a specific week,
	// or negative (-1 to -53) for counting from the end.
	// E.g., WeekNumber=2 + DayOfTheWeek=Tuesday means "second Tuesday".
	WeekNumber int `json:"weekNumber"`
}

// Weekday represents a day of the week (EKWeekday).
// Values correspond to Apple's EKWeekday enum.
type Weekday int

const (
	Sunday    Weekday = 1 // Sunday.
	Monday    Weekday = 2 // Monday.
	Tuesday   Weekday = 3 // Tuesday.
	Wednesday Weekday = 4 // Wednesday.
	Thursday  Weekday = 5 // Thursday.
	Friday    Weekday = 6 // Friday.
	Saturday  Weekday = 7 // Saturday.
)

// String returns a human-readable representation of the weekday.
func (w Weekday) String() string {
	switch w {
	case Sunday:
		return "sunday"
	case Monday:
		return "monday"
	case Tuesday:
		return "tuesday"
	case Wednesday:
		return "wednesday"
	case Thursday:
		return "thursday"
	case Friday:
		return "friday"
	case Saturday:
		return "saturday"
	default:
		return "unknown"
	}
}

// RecurrenceEnd defines when a recurrence stops.
// Exactly one of EndDate or OccurrenceCount should be set.
type RecurrenceEnd struct {
	// EndDate stops recurrence after this date. Nil if count-based.
	EndDate *time.Time `json:"endDate,omitempty"`
	// OccurrenceCount stops after this many occurrences. 0 if date-based.
	OccurrenceCount int `json:"occurrenceCount,omitempty"`
}

// StructuredLocation represents a geographic location with optional
// coordinates and geofence radius. Corresponds to EKStructuredLocation.
type StructuredLocation struct {
	// Title is the display name of the location (e.g., "Apple Park").
	Title string `json:"title"`
	// Latitude is the geographic latitude. Zero if no coordinates are set.
	Latitude float64 `json:"latitude,omitempty"`
	// Longitude is the geographic longitude. Zero if no coordinates are set.
	Longitude float64 `json:"longitude,omitempty"`
	// Radius is the geofence radius in meters. Zero means system default.
	Radius float64 `json:"radius,omitempty"`
}

// Daily returns a [RecurrenceRule] that repeats every interval days.
func Daily(interval int) RecurrenceRule {
	return RecurrenceRule{Frequency: FrequencyDaily, Interval: interval}
}

// Weekly returns a [RecurrenceRule] that repeats every interval weeks on the specified days.
// If no days are specified, repeats on the same day of the week as the event.
func Weekly(interval int, days ...Weekday) RecurrenceRule {
	r := RecurrenceRule{Frequency: FrequencyWeekly, Interval: interval}
	for _, d := range days {
		r.DaysOfTheWeek = append(r.DaysOfTheWeek, RecurrenceDayOfWeek{DayOfTheWeek: d})
	}
	return r
}

// Monthly returns a [RecurrenceRule] that repeats every interval months on the specified
// days of the month.
func Monthly(interval int, daysOfMonth ...int) RecurrenceRule {
	r := RecurrenceRule{Frequency: FrequencyMonthly, Interval: interval}
	r.DaysOfTheMonth = append(r.DaysOfTheMonth, daysOfMonth...)
	return r
}

// Yearly returns a [RecurrenceRule] that repeats every interval years.
func Yearly(interval int) RecurrenceRule {
	return RecurrenceRule{Frequency: FrequencyYearly, Interval: interval}
}
