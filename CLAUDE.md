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
├── calendar/                    # Public: Calendar event bindings (Phase 1 — COMPLETE)
│   ├── calendar.go              # Go types: Event, Calendar, Client, options
│   ├── parse.go                 # JSON parsing/marshaling (platform-agnostic, no build tags)
│   ├── bridge_darwin.go         # cgo wrappers (//go:build darwin)
│   ├── bridge_darwin.m          # ObjC EventKit bridge for EKEvent (~624 lines)
│   ├── bridge_darwin.h          # C header
│   ├── bridge_other.go          # !darwin stubs
│   ├── calendar_test.go         # Unit tests (26+ tests)
│   └── bridge_mock_test.go      # Mock bridge tests (JSON contract)
├── reminders/                   # Public: Reminder bindings (Phase 2 — COMPLETE)
│   ├── reminders.go             # Go types: Reminder, List, Client, options
│   ├── parse.go                 # JSON parsing/marshaling (platform-agnostic, no build tags)
│   ├── bridge_darwin.go         # cgo wrappers
│   ├── bridge_darwin.m          # ObjC EventKit bridge for EKReminder (~682 lines)
│   ├── bridge_darwin.h          # C header
│   ├── bridge_other.go          # !darwin stubs
│   ├── reminders_test.go        # Unit tests (26 tests)
│   └── bridge_mock_test.go      # Mock bridge tests (JSON contract)
├── scripts/                     # Integration tests (require real EventKit)
│   ├── integration.go           # 17 calendar integration tests
│   └── integration_reminders.go # 19 reminder integration tests
├── docs/
│   ├── prd/
│   │   ├── go-eventkit-prd.md       # Full PRD with API design
│   │   └── concurrency-prd.md       # Deferred concurrency improvements (3 phases)
│   └── research/
│       ├── eventkit-framework-comprehensive.md
│       └── go-concurrency-cgo-eventkit.md
├── journals/                    # Engineering journals (4 sessions)
└── go.mod
```

## Implementation Status
- **Phase 1**: `calendar/` package — COMPLETE. Full CRUD for Calendar events. Coverage: 58.9%.
- **Phase 2**: `reminders/` package — COMPLETE. Full CRUD for Reminders (all writes via EventKit, no AppleScript). Coverage: 55.5%.
- **Phase 3**: Future frameworks (Contacts, etc.) — out of scope for now
- **Deferred**: Concurrency improvements (inline error returns, serial write queue, context wrappers) — see `docs/prd/concurrency-prd.md`

## Key Technical Decisions
- `dispatch_once` for EKEventStore singleton + TCC access request — **each package has its own singleton** (C objects can't cross cgo package boundaries)
- `dispatch_semaphore` for sync wrappers around async EventKit APIs (reminders fetch is async; calendar fetch is synchronous)
- Calendar writes via EventKit directly (`saveEvent:span:commit:`) — no AppleScript needed
- All reminder writes via EventKit (improvement over `rem` which used AppleScript for writes)
- TCC: `requestFullAccessToEventsWithCompletion:` (macOS 14+), fallback to `requestAccessToEntityType:`
- Events require date ranges for queries (can't fetch all unbounded)
- `eventIdentifier` is stable across recurrence edits (use this, not `calendarItemIdentifier`)
- Attendees/organizer are **read-only** — Apple limitation
- `isFlagged` does not exist on EKReminder — always returns false
- EventKit sees all accounts (iCloud, Google, Exchange, subscriptions, birthdays)
- `@(!boolean)` produces integer 0/1, not JSON true/false — use `@YES`/`@NO` ternary
- `parse.go` files have no build tags — all JSON parsing is fully testable without cgo

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
go run ./scripts/integration.go              # Calendar integration tests (17 tests)
go run ./scripts/integration_reminders.go    # Reminder integration tests (19 tests)
```
Test coverage ceiling is ~58-59% because cgo bridge functions (bridge_darwin.go) can't be reached by `go test`. All testable code (types, parsing, marshaling) achieves ~100% coverage.

## Downstream Consumer
- `cal` CLI (separate repo) will be the first consumer of `calendar/` package
- `rem` CLI will eventually migrate to use `reminders/` package

## Journal
Engineering journals live in `journals/` dir. See `.claude/commands/journal.md` for the journaling command.
