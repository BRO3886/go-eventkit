# Recurrence Rules & Structured Locations — PRD

## Overview

This PRD covers two high-value feature additions to go-eventkit: **recurrence rule support** (read + write) and **structured location support** (read + write). These are the two most significant gaps between what EventKit exposes and what go-eventkit currently surfaces.

**Priority**: High — these are the most requested capabilities for any calendar library.

**Estimated scope**: ~300 lines ObjC, ~250 lines Go, ~400 lines tests (per feature).

## Problem

### 1. Recurrence Rules

go-eventkit exposes `Event.Recurring` as a boolean but provides no way to:
- Read **what** the recurrence pattern is (daily, weekly on Mon/Wed, monthly on 15th, etc.)
- Create recurring events
- Modify recurrence rules on existing events

Users who need recurring events must fall back to direct EventKit/AppleScript access, defeating the purpose of the library.

### 2. Structured Locations

go-eventkit exposes `Event.Location` as a plain string but EventKit also provides `EKStructuredLocation` with:
- Geographic coordinates (latitude/longitude)
- Geofence radius
- Map integration metadata

This is valuable for CLI tools that want to open locations in Maps, calculate distances, or create location-based reminders.

---

## Feature 1: Recurrence Rules

### Go Types

```go
// RecurrenceRule defines how an event or reminder repeats.
// Corresponds to EKRecurrenceRule (subset of iCalendar RRULE).
type RecurrenceRule struct {
    // Frequency is how often the event repeats (daily, weekly, monthly, yearly).
    Frequency RecurrenceFrequency `json:"frequency"`
    // Interval is the number of frequency units between occurrences.
    // E.g., Frequency=Weekly + Interval=2 means every 2 weeks.
    Interval int `json:"interval"`
    // DaysOfTheWeek specifies which days of the week the event occurs on.
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

// RecurrenceFrequency defines how often an event repeats.
type RecurrenceFrequency int

const (
    FrequencyDaily   RecurrenceFrequency = 0
    FrequencyWeekly  RecurrenceFrequency = 1
    FrequencyMonthly RecurrenceFrequency = 2
    FrequencyYearly  RecurrenceFrequency = 3
)

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
type Weekday int

const (
    Sunday    Weekday = 1
    Monday    Weekday = 2
    Tuesday   Weekday = 3
    Wednesday Weekday = 4
    Thursday  Weekday = 5
    Friday    Weekday = 6
    Saturday  Weekday = 7
)

// RecurrenceEnd defines when a recurrence stops.
// Exactly one of EndDate or OccurrenceCount should be set.
type RecurrenceEnd struct {
    // EndDate stops recurrence after this date. Nil if count-based.
    EndDate *time.Time `json:"endDate,omitempty"`
    // OccurrenceCount stops after this many occurrences. 0 if date-based.
    OccurrenceCount int `json:"occurrenceCount,omitempty"`
}
```

### Changes to Event Type

```go
type Event struct {
    // ... existing fields ...

    // Recurring is true if this event has recurrence rules.
    Recurring bool `json:"recurring"`
    // RecurrenceRules contains the recurrence patterns for this event.
    // Empty if the event is not recurring.
    RecurrenceRules []RecurrenceRule `json:"recurrenceRules,omitempty"`
    // IsDetached is true when this occurrence of a recurring event was
    // modified independently from the series.
    IsDetached bool `json:"isDetached"`
    // OccurrenceDate is the original date for this occurrence of a recurring
    // event, even if the occurrence has been moved to a different date.
    OccurrenceDate *time.Time `json:"occurrenceDate,omitempty"`
}
```

### Changes to CreateEventInput / UpdateEventInput

```go
type CreateEventInput struct {
    // ... existing fields ...

    // RecurrenceRules sets the recurrence pattern(s) for the event.
    // Most events have zero or one rule. Multiple rules are supported by
    // EventKit but uncommon.
    RecurrenceRules []RecurrenceRule `json:"recurrenceRules,omitempty"`
}

type UpdateEventInput struct {
    // ... existing fields ...

    // RecurrenceRules replaces all existing recurrence rules.
    // Pass an empty slice to make the event non-recurring.
    // Pass nil to leave recurrence unchanged.
    RecurrenceRules *[]RecurrenceRule `json:"recurrenceRules,omitempty"`
}
```

### ObjC Bridge Changes

**Reading** — In the event serialization block of `bridge_darwin.m`, serialize `recurrenceRules` array:

```objc
// For each EKRecurrenceRule, serialize:
// frequency, interval, daysOfTheWeek (with weekNumber), daysOfTheMonth,
// monthsOfTheYear, weeksOfTheYear, daysOfTheYear, setPositions,
// recurrenceEnd (endDate or occurrenceCount)
```

**Writing** — In `ek_cal_create_event` and `ek_cal_update_event`, parse the `recurrenceRules` JSON and construct `EKRecurrenceRule` objects:

```objc
// Use the complex initializer:
[[EKRecurrenceRule alloc] initRecurrenceWithFrequency:freq
                                             interval:interval
                                        daysOfTheWeek:daysOfWeek
                                       daysOfTheMonth:daysOfMonth
                                      monthsOfTheYear:months
                                       weeksOfTheYear:weeks
                                        daysOfTheYear:daysOfYear
                                         setPositions:positions
                                                  end:recurrenceEnd];
```

### Convenience Constructors (Go side)

```go
// Common recurrence patterns for CreateEventInput.RecurrenceRules:

// Daily returns a rule that repeats every N days.
func Daily(interval int) RecurrenceRule

// Weekly returns a rule that repeats every N weeks on the specified days.
// If no days are specified, repeats on the same day of the week as the event.
func Weekly(interval int, days ...Weekday) RecurrenceRule

// Monthly returns a rule that repeats every N months on the specified days of the month.
func Monthly(interval int, daysOfMonth ...int) RecurrenceRule

// Yearly returns a rule that repeats every N years.
func Yearly(interval int) RecurrenceRule

// Methods on RecurrenceRule for chaining:

// Until sets the recurrence to end on a specific date.
func (r RecurrenceRule) Until(t time.Time) RecurrenceRule

// Count sets the recurrence to end after N occurrences.
func (r RecurrenceRule) Count(n int) RecurrenceRule
```

**Usage example:**

```go
event, _ := client.CreateEvent(calendar.CreateEventInput{
    Title:     "Team standup",
    StartDate: time.Date(2026, 2, 12, 10, 0, 0, 0, time.Local),
    EndDate:   time.Date(2026, 2, 12, 10, 30, 0, 0, time.Local),
    RecurrenceRules: []calendar.RecurrenceRule{
        calendar.Weekly(1, calendar.Monday, calendar.Wednesday, calendar.Friday).
            Until(time.Date(2026, 12, 31, 0, 0, 0, 0, time.Local)),
    },
})
```

---

## Feature 2: Structured Locations

### Go Types

```go
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
```

### Changes to Event Type

```go
type Event struct {
    // ... existing fields ...

    // Location is the event's location as a plain string.
    Location string `json:"location"`
    // StructuredLocation contains geographic coordinates and geofence data.
    // Nil if no structured location is set. The Title field may differ from Location.
    StructuredLocation *StructuredLocation `json:"structuredLocation,omitempty"`
}
```

### Changes to CreateEventInput / UpdateEventInput

```go
type CreateEventInput struct {
    // ... existing fields ...

    // StructuredLocation sets geographic coordinates for the event.
    // If set, this takes precedence over Location for map integrations.
    // The plain Location string is set independently.
    StructuredLocation *StructuredLocation `json:"structuredLocation,omitempty"`
}

type UpdateEventInput struct {
    // ... existing fields ...

    // StructuredLocation updates the geographic location. Set to non-nil to
    // update, leave nil to keep unchanged.
    StructuredLocation *StructuredLocation `json:"structuredLocation,omitempty"`
}
```

### ObjC Bridge Changes

**Reading** — Serialize `event.structuredLocation` when non-nil:

```objc
if (event.structuredLocation) {
    EKStructuredLocation *loc = event.structuredLocation;
    NSMutableDictionary *locDict = [NSMutableDictionary dictionary];
    locDict[@"title"] = loc.title ?: @"";
    if (loc.geoLocation) {
        locDict[@"latitude"] = @(loc.geoLocation.coordinate.latitude);
        locDict[@"longitude"] = @(loc.geoLocation.coordinate.longitude);
    }
    if (loc.radius > 0) {
        locDict[@"radius"] = @(loc.radius);
    }
    eventDict[@"structuredLocation"] = locDict;
}
```

**Writing** — Parse `structuredLocation` from JSON and set on event:

```objc
NSDictionary *locJSON = input[@"structuredLocation"];
if (locJSON) {
    EKStructuredLocation *loc = [EKStructuredLocation locationWithTitle:locJSON[@"title"]];
    NSNumber *lat = locJSON[@"latitude"];
    NSNumber *lng = locJSON[@"longitude"];
    if (lat && lng) {
        loc.geoLocation = [[CLLocation alloc] initWithLatitude:[lat doubleValue]
                                                     longitude:[lng doubleValue]];
    }
    NSNumber *radius = locJSON[@"radius"];
    if (radius) {
        loc.radius = [radius doubleValue];
    }
    event.structuredLocation = loc;
}
```

**Note**: Reading coordinates requires `#import <CoreLocation/CoreLocation.h>` and adding `-framework CoreLocation` to LDFLAGS.

---

## Implementation Order

1. **Recurrence rules — read** (add `RecurrenceRules`, `IsDetached`, `OccurrenceDate` to Event serialization)
2. **Structured locations — read** (add `StructuredLocation` to Event serialization)
3. **Recurrence rules — write** (parse rules in create/update, add convenience constructors)
4. **Structured locations — write** (parse location in create/update)
5. **Recurrence rules for reminders** (EKReminder also supports recurrence — add if needed)

## Testing

### Unit Tests (parse.go)
- Parse event JSON with recurrence rules (daily, weekly with days, monthly, yearly with end conditions)
- Parse event JSON with structured location (with/without coordinates)
- Parse event JSON with `isDetached=true` and `occurrenceDate`
- Round-trip: Go struct → JSON → Go struct for all new types
- Convenience constructors: `Daily(1)`, `Weekly(2, Monday, Friday).Until(...)`, etc.

### Mock Bridge Tests
- Full JSON contract tests with recurrence and location fields
- Edge cases: empty recurrence array, nil structured location, zero coordinates

### Integration Tests
- Create event with daily recurrence → read back → verify rule
- Create event with "every 2 weeks on Mon/Wed/Fri for 10 occurrences" → verify
- Create event with structured location (lat/long) → read back → verify coordinates
- Update event: add recurrence rule to non-recurring event
- Update event: remove recurrence (set empty slice)
- Delete single occurrence of recurring event (SpanThisEvent)
- Delete all future occurrences (SpanFutureEvents)

## Known Limitations

- EventKit only supports Daily/Weekly/Monthly/Yearly — no hourly/minutely
- `setPositions` range is -366 to 366 (excluding 0)
- Not all RFC 5545 RRULE patterns are expressible via EventKit
- Structured location requires CoreLocation framework (adds ~negligible binary size)
- Some CalDAV servers may not preserve all recurrence rule fields on sync

## References

- [EKRecurrenceRule — Apple Developer Docs](https://developer.apple.com/documentation/eventkit/ekrecurrencerule)
- [EKStructuredLocation — Apple Developer Docs](https://developer.apple.com/documentation/eventkit/ekstructuredlocation)
- [docs/research/eventkit-framework-comprehensive.md](../research/eventkit-framework-comprehensive.md) — Sections 9, 10
