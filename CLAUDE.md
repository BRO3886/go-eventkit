# go-eventkit — Go bindings for Apple's EventKit framework

## What is this?
A Go library providing native macOS EventKit bindings via cgo + Objective-C. Exposes idiomatic Go types and a public client API for Calendar events and Reminders. In-process, sub-200ms access — no AppleScript, no subprocesses.

**Repository**: `github.com/BRO3886/go-eventkit`

## Non-Negotiables
- **Conventional Commits**: ALL commits MUST follow [Conventional Commits](https://www.conventionalcommits.org/). Format: `type(scope): description`. Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `build`, `ci`, `perf`. No exceptions.
- **ARC is mandatory**: `#cgo CFLAGS: -x objective-c -fobjc-arc` — without ARC, ObjC objects get released prematurely and EventKit returns empty results or SIGSEGV. This is critical.
- **cgo stays internal**: Public API is pure Go types. No cgo leaking to consumers.
- **JSON bridge format**: ObjC returns JSON via `char*`, Go parses into typed structs. Keeps C interface minimal.

## Architecture
```
go-eventkit/
├── eventkit.go                  # Shared types: RecurrenceRule, StructuredLocation, Weekday, etc.
├── eventkit_test.go             # Tests for shared types and convenience constructors
├── calendar/                    # Public: Calendar event bindings (Phase 1 — COMPLETE)
│   ├── calendar.go              # Go types: Event, Calendar, Client, options, calendar CRUD inputs
│   ├── parse.go                 # JSON parsing/marshaling (platform-agnostic, no build tags)
│   ├── bridge_darwin.go         # cgo wrappers (//go:build darwin) — includes WatchChanges
│   ├── bridge_darwin.m          # ObjC EventKit bridge for EKEvent + watch pipe
│   ├── bridge_darwin.h          # C header
│   ├── bridge_other.go          # !darwin stubs
│   ├── watch.go                 # watchChangesFromFile helper (no build tag, unit-testable)
│   ├── watch_test.go            # 4 unit tests via os.Pipe injection
│   ├── calendar_test.go         # Unit tests
│   └── bridge_mock_test.go      # Mock bridge tests (JSON contract)
├── reminders/                   # Public: Reminder bindings (Phase 2 — COMPLETE)
│   ├── reminders.go             # Go types: Reminder, List, Client, options, list CRUD inputs
│   ├── parse.go                 # JSON parsing/marshaling (platform-agnostic, no build tags)
│   ├── bridge_darwin.go         # cgo wrappers — includes WatchChanges
│   ├── bridge_darwin.m          # ObjC EventKit bridge for EKReminder + watch pipe
│   ├── bridge_darwin.h          # C header
│   ├── bridge_other.go          # !darwin stubs
│   ├── watch.go                 # watchChangesFromFile helper (no build tag, unit-testable)
│   ├── watch_test.go            # 4 unit tests via os.Pipe injection
│   ├── reminders_test.go        # Unit tests
│   └── bridge_mock_test.go      # Mock bridge tests (JSON contract)
├── dateparser/                  # Public: Natural language date parsing (shared by cal + rem CLIs)
│   ├── dateparser.go            # ParseDate, ParseDateRelativeTo, options, all parse functions
│   ├── format.go                # FormatDuration, FormatTimeRange, ParseAlertDuration
│   ├── dateparser_test.go       # 35 test functions (keywords, relative, weekday, tz, DST, etc.)
│   └── format_test.go           # Format/duration test suite
├── scripts/                     # Integration tests (require real EventKit)
│   ├── integration.go           # 35 calendar integration tests (incl. WatchChanges tests 32-35)
│   ├── integration_reminders.go # 34 reminder integration tests (incl. WatchChanges tests 31-34)
│   └── watch-demo/              # Demo: producer + consumer across two processes
│       ├── consumer/main.go     # Watches calendar changes, diffs and prints what changed
│       └── producer/main.go     # Creates, updates, deletes an event with pauses
├── docs/
│   ├── prd/
│   │   ├── go-eventkit-prd.md              # Full PRD with API design
│   │   ├── concurrency-prd.md              # Deferred concurrency improvements (3 phases)
│   │   ├── recurrence-location-prd.md      # Recurrence rules & structured locations (DONE)
│   │   ├── change-notifications-prd.md     # WatchChanges API (DONE — v0.3.0)
│   │   ├── benchmarking-prd.md             # Performance benchmarking (planned)
│   │   └── future-capabilities-prd.md      # Deferred capabilities (10 items)
│   └── research/
│       ├── eventkit-framework-comprehensive.md
│       └── go-concurrency-cgo-eventkit.md
├── journals/                    # Engineering journals (10 sessions)
└── go.mod
```

## Implementation Status
- **Root package** (`eventkit.go`): Shared types — RecurrenceRule, StructuredLocation, Weekday, convenience constructors. Coverage: 100%.
- **Phase 1**: `calendar/` package — COMPLETE. Full event CRUD + calendar container CRUD + recurrence rules + structured locations. Coverage: 56.7%.
- **Phase 2**: `reminders/` package — COMPLETE. Full reminder CRUD + list container CRUD + recurrence rules. Coverage: 54.9%.
- **dateparser**: `dateparser/` package — COMPLETE. Shared natural language date parser for cal + rem CLIs. 3 options (`WithDefaultHour`, `WithSmartTimeRollover`, `WithEOWSkipToday`). 35 test functions.
- **Phase 3**: `WatchChanges` — COMPLETE (v0.3.0). Change notifications via self-pipe + EKEventStoreChangedNotification. 4 unit tests per package. See `docs/prd/change-notifications-prd.md`.
- **Concurrency safety** (v0.4.0): Inline error returns (`ek_result_t`), serial write queue (`dispatch_sync`), non-blocking WatchChanges pipe reads. `RecurrenceRule.Validate()` catches invalid constraints. Batch delete: `DeleteEvents(ids, span)`, `DeleteReminders(ids)`. Better "not found" errors include available names.
- **URL attachments** (v0.5.0): Reminder URLs write to the real Reminders.app URL field via ReminderKit private API introspection.
- **Suppress default alarms** (v0.6.0): `CreateEventInput.SuppressDefaultAlarms` bool opts out of calendar-inherited default alarms at save time. Bridge clears `event.alarms` before adding user-supplied alerts.
- **Deferred**: Future frameworks (Contacts, etc.) — out of scope for now
- **Deferred**: Performance benchmarking — see `docs/prd/benchmarking-prd.md`
- **Deferred**: 10 future capabilities — see `docs/prd/future-capabilities-prd.md`

## Key Technical Decisions
- `dispatch_once` for EKEventStore singleton + TCC access request — **each package has its own singleton** (C objects can't cross cgo package boundaries)
- **Serial write queue**: All write operations (`saveEvent`, `removeEvent`, `saveCalendar`, `removeCalendar`, `saveReminder`, `removeReminder`) are wrapped in `dispatch_sync` on a per-package serial queue (`dev.sidv.eventkit.cal.writes` / `dev.sidv.eventkit.rem.writes`). Reads (`eventsMatchingPredicate`, `calendarsForEntityType`, `fetchRemindersMatchingPredicate`) stay concurrent. This prevents EventKit database corruption from concurrent goroutine writes.
- **Inline error returns (`ek_result_t`)**: All bridge functions return a struct with `result` + `error` fields. No `__thread` TLS — safe under Go's M:N scheduler.
- `dispatch_semaphore` for sync wrappers around async EventKit APIs (reminders fetch is async; calendar fetch is synchronous)
- Calendar writes via EventKit directly (`saveEvent:span:commit:`) — no AppleScript needed
- Calendar/list container CRUD via `saveCalendar:commit:` / `removeCalendar:commit:` — color via `CGColorCreateGenericRGB()`, source is **required** (no default fallback), immutability check via `cal.isImmutable`
- `find_source_by_name` is entity-aware — prefers sources with matching entity type calendars (macOS has duplicate "iCloud" EKSource objects for events vs reminders)
- All reminder writes via EventKit (improvement over `rem` which used AppleScript for writes)
- TCC: `requestFullAccessToEventsWithCompletion:` (macOS 14+), fallback to `requestAccessToEntityType:`
- Events require date ranges for queries (can't fetch all unbounded)
- `eventIdentifier` is stable across recurrence edits (use this, not `calendarItemIdentifier`)
- Attendees/organizer are **read-only** — Apple limitation
- `isFlagged` does not exist on EKReminder — always returns false
- EventKit sees all accounts (iCloud, Google, Exchange, subscriptions, birthdays)
- `@(!boolean)` produces integer 0/1, not JSON true/false — use `@YES`/`@NO` ternary
- `parse.go` files have no build tags — all JSON parsing is fully testable without cgo
- Shared types live in root `eventkit` package — both `calendar/` and `reminders/` import from root, no cross-dependency
- Raw bridge types (`rawRecurrenceRule` etc.) duplicated per package — they're unexported and can't be shared across cgo boundaries
- EKRecurrenceRule: use complex initializer (all constraint arrays accept nil) — one code path for all rule types
- `*float64` for raw location coordinates to distinguish "not set" from zero (Null Island at 0,0 is valid)
- EKReminder inherits recurrence from EKCalendarItem — same ObjC bridge pattern as calendar
- **WatchChanges self-pipe**: ObjC block observer writes 1 byte to a pipe on `EKEventStoreChangedNotification`; Go goroutine reads the read end. `queue:nil` delivers on the posting thread (no NSRunLoop needed for intra-process). Package-level mutex enforces one watcher per package.
- **Cross-process notifications require main thread CFRunLoop**: `EKEventStoreChangedNotification` from other processes only fires on the OS main thread's run loop. Consumer binaries must call `runtime.LockOSThread()` in `main()` and pump `CFRunLoopRunInMode` from the main goroutine. See `scripts/watch-demo/consumer/main.go`.
- **`watchChangesFromFile` helper** extracted to `watch.go` (no build tag) — makes the channel logic unit-testable without cgo via `os.Pipe()`
- **Do not call `f.Close()`** on the pipe file — `ek_cal/rem_watch_stop` owns the fd lifecycle via `close(2)`

## Prior Art
- This package extracts the proven cgo + ObjC pattern from [rem](https://github.com/BRO3886/rem) (macOS Reminders CLI)
- `rem` achieved <200ms reads with this exact approach (100-500x faster than JXA/AppleScript)
- No competing Go EventKit package exists (verified Feb 2026)
- `progrium/darwinkit` (5.4k stars) covers 33 Apple frameworks but NOT EventKit

## Build & Test
```bash
go build ./...              # Compiles ObjC via cgo automatically
go test ./...               # Unit tests (JSON parsing, types)
GOOS=linux CGO_ENABLED=0 go build ./...  # Verify cross-platform stubs
go run ./scripts/integration.go              # Calendar integration tests (35 tests)
go run ./scripts/integration_reminders.go    # Reminder integration tests (34 tests)
# Watch demo (two terminals):
go run ./scripts/watch-demo/consumer         # Terminal 1: watches for changes
go run ./scripts/watch-demo/producer         # Terminal 2: creates/updates/deletes event
```
Test coverage ceiling is ~55-57% because cgo bridge functions (bridge_darwin.go) can't be reached by `go test`. All testable code (types, parsing, marshaling) achieves ~100% coverage.

## Downstream Consumers
- `cal` CLI (separate repo) — consumes `calendar/` and `dateparser/` packages
- `rem` CLI — will migrate to consume `reminders/` and `dateparser/` packages
- **dateparser consumer migration**:
  - cal: `dateparser.ParseDate(input)` — no options, defaults match cal behavior
  - rem: `dateparser.ParseDate(input, dateparser.WithDefaultHour(9), dateparser.WithSmartTimeRollover(), dateparser.WithEOWSkipToday())`

## Journal
Engineering journals live in `journals/` dir. See `.claude/commands/journal.md` for the journaling command.
