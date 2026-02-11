# go-eventkit

Go bindings for Apple's EventKit framework. In-process, sub-200ms access to macOS Calendar events and Reminders — no AppleScript, no subprocesses.

## Features

- **Calendar events** — Full CRUD: list calendars, query events by date range, create, update, delete
- **Recurrence rules** — Read and write recurring events (daily, weekly, monthly, yearly) with full constraint support (days of week, days of month, set positions, end date/count)
- **Structured locations** — Geographic coordinates (lat/long) and geofence radius on events, beyond plain-text location strings
- **Reminders** — Full CRUD: list reminder lists, query/filter reminders, create, update, delete, complete/uncomplete
- **Pure Go API** — Idiomatic Go types, no cgo leaking to consumers
- **Fast** — Direct EventKit access via cgo + Objective-C, 100-500x faster than AppleScript/JXA
- **All accounts** — Sees iCloud, Google, Exchange, and local calendars/reminders

## Requirements

- macOS (darwin) — uses Apple's EventKit framework via cgo
- Go 1.24+
- Xcode Command Line Tools (`xcode-select --install`)

Types are importable on all platforms for development. Bridge operations return `ErrUnsupported` on non-darwin.

## Installation

```bash
go get github.com/BRO3886/go-eventkit
```

## Quick Start

### Calendar

```go
package main

import (
    "fmt"
    "log"
    "time"

    "github.com/BRO3886/go-eventkit/calendar"
)

func main() {
    client, err := calendar.New()
    if err != nil {
        log.Fatal(err) // TCC access denied
    }

    // List all calendars
    calendars, _ := client.Calendars()
    for _, c := range calendars {
        fmt.Printf("%s (%s, %s)\n", c.Title, c.Type, c.Source)
    }

    // Fetch events for the next 7 days
    now := time.Now()
    events, _ := client.Events(now, now.Add(7*24*time.Hour))
    for _, e := range events {
        fmt.Printf("%s: %s - %s\n", e.Title, e.StartDate.Format(time.Kitchen), e.EndDate.Format(time.Kitchen))
    }

    // Create an event
    event, _ := client.CreateEvent(calendar.CreateEventInput{
        Title:     "Team standup",
        StartDate: time.Date(2026, 2, 12, 10, 0, 0, 0, time.Local),
        EndDate:   time.Date(2026, 2, 12, 10, 30, 0, 0, time.Local),
        Calendar:  "Work",
        Alerts:    []calendar.Alert{{RelativeOffset: -15 * time.Minute}},
    })
    fmt.Printf("Created: %s (ID: %s)\n", event.Title, event.ID)

    // Create a recurring event with a structured location
    event, _ = client.CreateEvent(calendar.CreateEventInput{
        Title:     "Weekly sync",
        StartDate: time.Date(2026, 2, 12, 14, 0, 0, 0, time.Local),
        EndDate:   time.Date(2026, 2, 12, 15, 0, 0, 0, time.Local),
        Calendar:  "Work",
        RecurrenceRules: []calendar.RecurrenceRule{
            calendar.Weekly(1, calendar.Monday, calendar.Wednesday, calendar.Friday).
                Until(time.Date(2026, 12, 31, 0, 0, 0, 0, time.Local)),
        },
        StructuredLocation: &calendar.StructuredLocation{
            Title:     "Apple Park",
            Latitude:  37.3349,
            Longitude: -122.0090,
            Radius:    150,
        },
    })
    fmt.Printf("Created recurring: %s (rules: %d)\n", event.Title, len(event.RecurrenceRules))
}
```

### Reminders

```go
package main

import (
    "fmt"
    "log"
    "time"

    "github.com/BRO3886/go-eventkit/reminders"
)

func main() {
    client, err := reminders.New()
    if err != nil {
        log.Fatal(err)
    }

    // List all reminder lists
    lists, _ := client.Lists()
    for _, l := range lists {
        fmt.Printf("%s (%d items)\n", l.Title, l.Count)
    }

    // Get incomplete reminders from a specific list
    items, _ := client.Reminders(
        reminders.WithList("Shopping"),
        reminders.WithCompleted(false),
    )
    for _, r := range items {
        fmt.Printf("[ ] %s (due: %v)\n", r.Title, r.DueDate)
    }

    // Create a reminder
    due := time.Now().Add(24 * time.Hour)
    reminder, _ := client.CreateReminder(reminders.CreateReminderInput{
        Title:    "Buy milk",
        ListName: "Shopping",
        DueDate:  &due,
        Priority: reminders.PriorityHigh,
    })
    fmt.Printf("Created: %s (ID: %s)\n", reminder.Title, reminder.ID)

    // Complete it
    client.CompleteReminder(reminder.ID)
}
```

## API Reference

### Calendar Package

```go
import "github.com/BRO3886/go-eventkit/calendar"
```

| Method | Description |
|--------|-------------|
| `New() (*Client, error)` | Create client, request TCC access |
| `Calendars() ([]Calendar, error)` | List all calendars |
| `Events(start, end, ...ListOption) ([]Event, error)` | Query events in date range |
| `Event(id) (*Event, error)` | Get single event by ID |
| `CreateEvent(input) (*Event, error)` | Create a new event |
| `UpdateEvent(id, input, span) (*Event, error)` | Update an existing event |
| `DeleteEvent(id, span) error` | Delete an event |

**Filter options:** `WithCalendar(name)`, `WithCalendarID(id)`, `WithSearch(query)`

**Recurrence constructors:** `Daily(interval)`, `Weekly(interval, ...days)`, `Monthly(interval, ...daysOfMonth)`, `Yearly(interval)` — chain with `.Until(time)` or `.Count(n)`

### Reminders Package

```go
import "github.com/BRO3886/go-eventkit/reminders"
```

| Method | Description |
|--------|-------------|
| `New() (*Client, error)` | Create client, request TCC access |
| `Lists() ([]List, error)` | List all reminder lists |
| `Reminders(...ListOption) ([]Reminder, error)` | Query reminders with filters |
| `Reminder(id) (*Reminder, error)` | Get single reminder by ID or prefix |
| `CreateReminder(input) (*Reminder, error)` | Create a new reminder |
| `UpdateReminder(id, input) (*Reminder, error)` | Update an existing reminder |
| `DeleteReminder(id) error` | Delete a reminder |
| `CompleteReminder(id) (*Reminder, error)` | Mark as completed |
| `UncompleteReminder(id) (*Reminder, error)` | Mark as incomplete |

**Filter options:** `WithList(name)`, `WithListID(id)`, `WithCompleted(bool)`, `WithSearch(query)`, `WithDueBefore(time)`, `WithDueAfter(time)`

### Priority Values

| Constant | Value | Apple Mapping |
|----------|-------|---------------|
| `PriorityNone` | 0 | No priority |
| `PriorityHigh` | 1 | Priorities 1-4 |
| `PriorityMedium` | 5 | Priority 5 |
| `PriorityLow` | 9 | Priorities 6-9 |

## Permissions (TCC)

On first use, macOS will prompt for Calendar/Reminders access. The prompt shows the terminal app name (Terminal.app, iTerm2, etc.), not the Go binary.

Manage permissions in **System Settings > Privacy & Security > Calendars / Reminders**.

## Architecture

```
calendar/                   # Calendar event bindings
├── calendar.go             # Go types (no build constraint — importable everywhere)
├── parse.go                # JSON parsing/marshaling (platform-agnostic)
├── bridge_darwin.go        # cgo wrappers (darwin only)
├── bridge_darwin.m         # ObjC EventKit bridge
├── bridge_darwin.h         # C header
└── bridge_other.go         # !darwin stubs

reminders/                  # Reminder bindings
├── reminders.go            # Go types (no build constraint)
├── parse.go                # JSON parsing/marshaling (platform-agnostic)
├── bridge_darwin.go        # cgo wrappers (darwin only)
├── bridge_darwin.m         # ObjC EventKit bridge
├── bridge_darwin.h         # C header
└── bridge_other.go         # !darwin stubs
```

Each package maintains its own `EKEventStore` singleton via `dispatch_once`. ARC (`-fobjc-arc`) is mandatory — without it, Objective-C objects are released prematurely, causing empty results or crashes.

## Known Limitations

These are Apple EventKit limitations, not bugs:

- **Attendees are read-only** — Cannot add/remove event attendees via EventKit
- **Organizer is read-only** — Cannot set event organizer
- **Flagged property unavailable** — Reminder "flagged" state is not exposed by EventKit
- **Events require date ranges** — Cannot fetch all events unbounded
- **Reminders use async fetch** — Wrapped synchronously via `dispatch_semaphore`
- **Birthday/subscription calendars are read-only**
- **No text search on events** — Only date-range predicates (search is post-fetch filtering)
- **Recurrence: daily/weekly/monthly/yearly only** — EventKit does not support hourly or minutely frequencies
- **Recurrence is a subset of RFC 5545** — Not all iCalendar RRULE patterns are expressible via EventKit
- **Some CalDAV servers may simplify rules** — Constraint fields like `setPositions` may not survive sync

## Building & Testing

```bash
# Build (compiles ObjC via cgo)
go build ./...

# Unit tests
go test ./...

# Integration tests (requires TCC calendar/reminders access)
go run ./scripts/integration.go
go run ./scripts/integration_reminders.go

# Cross-platform stub verification
GOOS=linux CGO_ENABLED=0 go build ./...
```

## Prior Art

This library extracts the proven cgo + Objective-C bridge pattern from [rem](https://github.com/BRO3886/rem) (macOS Reminders CLI). Key improvements over rem:

- **All reminder writes via EventKit** — rem uses AppleScript for writes; go-eventkit uses EventKit directly
- **Calendar support** — rem only handles reminders
- **Library-first design** — Designed as an importable package, not a CLI

## License

MIT
