# go-eventkit — Product Requirements Document

## Overview

**go-eventkit** is a Go library providing native macOS EventKit bindings via cgo + Objective-C. It exposes idiomatic Go types and a public client API for Calendar events and Reminders, backed by Apple's EventKit framework for in-process, sub-200ms access.

**Repository**: `github.com/BRO3886/go-eventkit`

## Problem

There is no Go library for accessing macOS Calendar and Reminders natively. Developers resort to:

1. **AppleScript/JXA via `osascript`** — 8-60s per query. Apple Events are serialized; bulk reads are O(n) IPC calls. Unusable for any real-time or interactive tool.
2. **Shelling out to Swift/Python scripts** — subprocess overhead, two-binary distribution, fragile path resolution.
3. **Building custom cgo bridges per-project** — the approach used in [rem](https://github.com/BRO3886/rem). Works, but the EventKit + cgo + ObjC + ARC + TCC boilerplate is non-trivial and shouldn't be re-implemented by every consumer.

**go-eventkit** extracts the proven cgo + Objective-C EventKit bridge pattern from `rem` into a standalone, reusable Go package.

## Goals

1. **Provide idiomatic Go bindings for EventKit** — pure Go types, no cgo leaking into the public API
2. **Start with Calendar (events)** — primary use case for the new `cal` CLI
3. **Bring in Reminders second** — extract and generalize from `rem`'s existing bridge
4. **Sub-200ms reads** — in-process EventKit, no subprocess, no Apple Events
5. **Full CRUD** — reads and writes via EventKit (no AppleScript dependency)
6. **Single `go get`** — cgo compiles the ObjC automatically, no separate build steps
7. **Clean cross-platform story** — `!darwin` stubs with clear error messages

## Non-Goals

- GUI or TUI components
- Support for non-EventKit frameworks (Contacts, Photos, etc.) in v1 — possible future extension
- iOS support (macOS only — EventKit API surface differs)
- Replacing Apple's EventKit framework — this is a thin, opinionated bridge

## Target Users

1. **CLI tool authors** — building tools like `cal`, `rem`, or combined productivity CLIs
2. **MCP server builders** — exposing calendar/reminders to AI agents via Model Context Protocol
3. **Automation developers** — Go scripts/services that read/write macOS calendar data
4. **AI agent tool builders** — LLM function-calling tools that need local system access

## Architecture

### Package Structure

```
github.com/BRO3886/go-eventkit
├── calendar/                    # Public: Calendar event bindings
│   ├── calendar.go              # Go types: Event, Calendar, Client, options
│   ├── bridge_darwin.go         # cgo wrappers (//go:build darwin)
│   ├── bridge_darwin.m          # ObjC EventKit bridge for EKEvent
│   ├── bridge_darwin.h          # C header
│   └── bridge_other.go          # !darwin stubs
├── reminders/                   # Public: Reminder bindings (phase 2)
│   ├── reminders.go             # Go types: Reminder, List, Client, options
│   ├── bridge_darwin.go         # cgo wrappers
│   ├── bridge_darwin.m          # ObjC EventKit bridge for EKReminder
│   ├── bridge_darwin.h          # C header
│   └── bridge_other.go          # !darwin stubs
├── internal/
│   └── store/                   # Shared EKEventStore singleton
│       ├── store_darwin.m       # dispatch_once init, TCC permission requests
│       ├── store_darwin.h       # C header
│       └── store_darwin.go      # Go accessor (//go:build darwin)
├── docs/
│   └── prd/
├── go.mod
├── LICENSE
└── README.md
```

### Design Principles

1. **cgo stays internal** — public API is pure Go types. The cgo boundary is in `bridge_darwin.go` files, never exposed to consumers.

2. **JSON as the bridge format** — ObjC returns JSON strings via `char*`. Go parses them into typed structs. This keeps the C interface minimal (a few functions returning strings) and leverages Go's excellent JSON support. Proven pattern from `rem` — JSON serialization cost is <1ms for hundreds of items.

3. **Shared EKEventStore singleton** — `internal/store/` manages a single `EKEventStore` instance via `dispatch_once`. Both `calendar/` and `reminders/` packages use it. TCC permissions are requested per entity type on first access.

4. **ARC is mandatory** — `#cgo CFLAGS: -x objective-c -fobjc-arc`. Without ARC, ObjC objects in completion handlers and `__block` variables get released prematurely, causing silent empty results or SIGSEGV. This was a critical lesson from `rem`.

5. **Synchronous Go API, async ObjC internals** — EventKit's async APIs are wrapped with `dispatch_semaphore` in ObjC. Go callers get blocking function calls. For Calendar events specifically, the synchronous `eventsMatchingPredicate:` is available and preferred.

6. **One sub-package per entity type** — `calendar/` and `reminders/` are independently importable. A project that only needs calendar doesn't pull in reminders code.

## Phase 1: `calendar/` Package

### Public Types

```go
package calendar

// Client provides access to macOS Calendar via EventKit.
type Client struct{}

// New creates a new Calendar client.
// Requests full calendar access on first use (TCC prompt).
// Panics on non-darwin platforms.
func New() *Client

// Calendar represents a calendar (e.g., "Work", "Personal", "Holidays").
type Calendar struct {
    ID       string
    Title    string
    Type     CalendarType   // Local, CalDAV, Exchange, Birthday, Subscription
    Color    string         // Hex color code
    Source   string         // Account name (e.g., "iCloud", "Google")
    ReadOnly bool
}

// Event represents a calendar event.
type Event struct {
    ID          string
    Title       string
    StartDate   time.Time
    EndDate     time.Time
    AllDay      bool
    Location    string
    Notes       string
    URL         string
    Calendar    string      // Calendar title
    CalendarID  string      // Calendar identifier
    Status      EventStatus // Confirmed, Tentative, Canceled
    Availability Availability // Busy, Free, Tentative, Unavailable
    Organizer   string      // Display name (read-only)
    Attendees   []Attendee  // Read-only
    Recurring   bool        // Whether this is a recurring event
    Alerts      []Alert     // Reminder alerts
    CreatedAt   time.Time
    ModifiedAt  time.Time
}

type Attendee struct {
    Name   string
    Email  string
    Status ParticipantStatus // Accepted, Declined, Tentative, Pending
}

type Alert struct {
    RelativeOffset time.Duration // Negative = before event (e.g., -15m)
}

type EventStatus int
const (
    StatusConfirmed EventStatus = iota
    StatusTentative
    StatusCanceled
)

type Availability int
const (
    AvailabilityBusy Availability = iota
    AvailabilityFree
    AvailabilityTentative
    AvailabilityUnavailable
)

type CalendarType int
const (
    CalendarTypeLocal CalendarType = iota
    CalendarTypeCalDAV
    CalendarTypeExchange
    CalendarTypeBirthday
    CalendarTypeSubscription
)
```

### Client Methods

```go
// --- Reads ---

// Calendars returns all calendars for events.
func (c *Client) Calendars() ([]Calendar, error)

// Events returns events within the given time range.
// Options can filter by calendar, search query, etc.
func (c *Client) Events(start, end time.Time, opts ...ListOption) ([]Event, error)

// Event returns a single event by ID (full ID or prefix).
func (c *Client) Event(id string) (*Event, error)

// --- Writes ---

// CreateEvent creates a new calendar event.
func (c *Client) CreateEvent(input CreateEventInput) (*Event, error)

// UpdateEvent updates an existing event.
// Span controls whether to update just this occurrence or future occurrences too.
func (c *Client) UpdateEvent(id string, input UpdateEventInput, span Span) (*Event, error)

// DeleteEvent removes an event.
// Span controls whether to delete just this occurrence or future occurrences too.
func (c *Client) DeleteEvent(id string, span Span) error

// --- Options ---

type ListOption func(*listOptions)

func WithCalendar(name string) ListOption    // Filter by calendar name
func WithSearch(query string) ListOption      // Full-text search
func WithCalendarID(id string) ListOption     // Filter by calendar ID

type Span int
const (
    ThisEvent    Span = iota // Only this occurrence
    FutureEvents             // This and all future occurrences
)
```

### Write Input Types

```go
type CreateEventInput struct {
    Title       string
    StartDate   time.Time
    EndDate     time.Time
    AllDay      bool
    Location    string        // Optional
    Notes       string        // Optional
    URL         string        // Optional
    Calendar    string        // Calendar name (optional, uses default if empty)
    Alerts      []Alert       // Optional
}

type UpdateEventInput struct {
    Title       *string       // nil = don't change
    StartDate   *time.Time
    EndDate     *time.Time
    AllDay      *bool
    Location    *string
    Notes       *string
    URL         *string
    Calendar    *string       // Move to different calendar
    Alerts      *[]Alert
}
```

### ObjC Bridge Functions (internal)

```c
// bridge_darwin.h
char* ek_cal_fetch_calendars(void);
char* ek_cal_fetch_events(const char* start_date, const char* end_date,
                           const char* calendar_id, const char* search_query);
char* ek_cal_get_event(const char* event_id);
char* ek_cal_create_event(const char* json_input);
char* ek_cal_update_event(const char* event_id, const char* json_input, int span);
char* ek_cal_delete_event(const char* event_id, int span);
void  ek_free(char* str);
char* ek_last_error(void);
```

### EventKit Mapping

| Go field | EventKit property | Notes |
|----------|-------------------|-------|
| `ID` | `eventIdentifier` | Stable across recurrence edits (unlike `calendarItemIdentifier`) |
| `Title` | `title` | Read-write |
| `StartDate` | `startDate` | NSDate → time.Time (UTC → local) |
| `EndDate` | `endDate` | NSDate → time.Time |
| `AllDay` | `allDay` | BOOL |
| `Location` | `location` | String, read-write |
| `Notes` | `notes` | String, read-write |
| `URL` | `URL` | NSURL, read-write (unlike Reminders which lack this) |
| `Calendar` | `calendar.title` | Read; set via `calendar` property on write |
| `Status` | `status` | EKEventStatus enum |
| `Availability` | `availability` | EKEventAvailability enum |
| `Organizer` | `organizer.name` | Read-only EKParticipant |
| `Attendees` | `attendees` | Read-only array of EKParticipant |
| `Recurring` | Derived from `hasRecurrenceRules` | Read-only boolean |
| `Alerts` | `alarms` | Array of EKAlarm, read-write |

## Phase 2: `reminders/` Package

Extract the existing ObjC bridge from `rem` (`internal/eventkit/`) into `go-eventkit/reminders/`. Key changes from current `rem` implementation:

1. **Writes via EventKit** — replace AppleScript writes with `saveReminder:commit:error:` and `removeReminder:commit:error:`. Eliminates the AppleScript dependency entirely.
2. **Public types** — `Reminder`, `List`, `Priority`, `Client` matching the pattern from `calendar/`.
3. **Flagged property** — still not available in EventKit. Document the limitation. Consider a JXA fallback utility or accept the gap.

### Migration path for `rem`

Once `go-eventkit/reminders` is stable:
1. `rem` replaces `internal/eventkit/` and `internal/applescript/` with `go-eventkit/reminders`
2. `rem`'s `pkg/client/` becomes a thin wrapper or is deprecated in favor of direct `go-eventkit/reminders` usage
3. `rem` shrinks to just CLI commands + UI formatting

## Phase 3: Future Extensions (Out of Scope for v1)

Potential additional packages following the same pattern:
- `contacts/` — AddressBook/Contacts framework
- `photos/` — PhotoKit framework
- `notes/` — Notes (no public framework — would need AppleScript/SQLite)

These are not planned but the architecture supports them.

## Technical Requirements

### Build Requirements
- macOS 10.13+ (EventKit availability for full calendar access)
- Xcode Command Line Tools (clang + framework headers)
- Go 1.21+ with cgo enabled
- `CGO_ENABLED=1` (default on macOS)

### cgo Directives (per bridge file)
```go
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework EventKit -framework Foundation
```

### TCC Permissions
- Calendar access: `requestFullAccessToEventsWithCompletion:` (macOS 14+) with fallback to `requestAccessToEntityType:EKEntityTypeEvent`
- Reminders access: `requestFullAccessToRemindersWithCompletion:` (macOS 14+) with fallback to `requestAccessToEntityType:EKEntityTypeReminder`
- Each entity type is a separate TCC prompt
- Permissions requested lazily on first use of each package

### Cross-Platform
- `//go:build darwin` on all cgo files
- `//go:build !darwin` stubs returning `ErrUnsupported` for every public function
- Types (structs, enums, constants) in platform-agnostic files — importable everywhere

### Error Handling
- All public methods return `(result, error)` — no panics except `New()` on non-darwin
- EventKit errors wrapped with context: `fmt.Errorf("calendar: failed to fetch events: %w", err)`
- TCC denial returns a specific `ErrAccessDenied` sentinel error
- Non-darwin calls return `ErrUnsupported`

### Testing Strategy
- **Unit tests** for Go-side JSON parsing, type conversions, option builders — run on any platform
- **Integration tests** (build-tagged `darwin`) that hit real EventKit — require TCC access, run on macOS only
- **Cross-platform build test** — `GOOS=linux CGO_ENABLED=0 go build ./...` in CI

## Performance Targets

Based on `rem` benchmarks with the same EventKit bridge pattern:

| Operation | Target |
|-----------|--------|
| Fetch all calendars | <150ms |
| Fetch events (1 month range) | <200ms |
| Fetch single event by ID | <150ms |
| Create event | <200ms |
| Update event | <200ms |
| Delete event | <150ms |

These are conservative — `rem` achieves 106-168ms for equivalent operations.

## Success Criteria

1. `go get github.com/BRO3886/go-eventkit/calendar` works with no external dependencies beyond Xcode CLT
2. All CRUD operations work against real macOS Calendar
3. Sees all calendar accounts (iCloud, Google, Exchange, local) — not just local like AppleScript
4. Sub-200ms for all operations
5. `cal` CLI built on top ships as a single binary
6. Clean `go doc` output — well-documented public API

## Open Questions

1. **Event recurrence** — should `CreateEventInput` support recurrence rules in v1, or defer to a later version? Recurrence in EventKit is complex (`EKRecurrenceRule` with frequency, interval, days of week, end conditions).
2. **Structured locations** — EventKit supports `EKStructuredLocation` with coordinates and geofence radius. Worth exposing in v1, or just the string `location`?
3. **Multiple alerts** — Events can have multiple alarms. Support `[]Alert` from day one, or start with a single alert?
4. **Context variants** — should methods have `Context`-aware versions (e.g., `EventsContext(ctx, start, end)`) for cancellation? The ObjC semaphore blocks the goroutine, so context cancellation would require a separate goroutine + select.
5. **Module name** — `github.com/BRO3886/go-eventkit` confirmed? Or a GitHub org?
