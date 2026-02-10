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
├── docs/prd/go-eventkit-prd.md  # Full PRD with API design
├── journals/                    # Engineering journals
└── go.mod
```

## Phased Implementation
- **Phase 1**: `calendar/` package — full CRUD for Calendar events
- **Phase 2**: `reminders/` package — extract + improve from rem's bridge
- **Phase 3**: Future frameworks (Contacts, etc.) — out of scope for now

## Key Technical Decisions (from rem experience)
- `dispatch_once` for EKEventStore singleton + TCC access request
- `dispatch_semaphore` for sync wrappers around async EventKit APIs
- Calendar events have a **synchronous** fetch API (`eventsMatchingPredicate:`) — simpler than reminders
- Calendar writes via EventKit directly (`saveEvent:span:commit:`) — no AppleScript needed
- TCC: `requestFullAccessToEventsWithCompletion:` (macOS 14+), fallback to `requestAccessToEntityType:EKEntityTypeEvent`
- Events require date ranges for queries (can't fetch all unbounded)
- `eventIdentifier` is stable across recurrence edits (use this, not `calendarItemIdentifier`)
- Attendees/organizer are **read-only** — Apple limitation
- EventKit sees all accounts (iCloud, Google, Exchange) — more complete than AppleScript

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
```

## Downstream Consumer
- `cal` CLI (separate repo) will be the first consumer of `calendar/` package
- `rem` CLI will eventually migrate to use `reminders/` package

## Journal
Engineering journals live in `journals/` dir. See `.claude/commands/journal.md` for the journaling command.
