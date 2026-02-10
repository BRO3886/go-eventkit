# Concurrency & Thread-Safety Improvements — PRD

## Overview

This PRD covers three related improvements to make go-eventkit safe for concurrent use from multiple goroutines. Currently, the library works correctly in single-goroutine usage but has a subtle thread-local storage race and lacks write serialization for multi-goroutine consumers.

**Priority**: Medium — implement when building the first concurrent consumer (server, TUI, or parallel pipeline).

**Estimated scope**: ~200 lines of ObjC changes, ~100 lines of Go changes across both packages.

## Problem

### 1. Thread-Local Error Race Condition

Both `calendar/bridge_darwin.m` and `reminders/bridge_darwin.m` use `__thread` (thread-local storage) for error reporting:

```c
static __thread char* cal_last_error = NULL;
```

The Go bridge reads errors via a separate cgo call:

```go
// bridge_darwin.go
cstr := C.ek_cal_fetch_events(...)  // fails, sets error on thread T1
if cstr == nil {
    errMsg := C.ek_cal_last_error()  // may execute on thread T2!
}
```

cgo does not guarantee successive C calls execute on the same OS thread. Between `ek_cal_fetch_events` returning and `ek_cal_last_error` being called, the Go scheduler can migrate the goroutine to a different OS thread — reading a stale/empty error from T2's TLS.

**Impact**: Error messages silently lost or replaced with "unknown error". Only triggers under concurrent goroutine usage with errors.

### 2. No Write Serialization

EKEventStore write operations (`saveEvent:`, `removeEvent:`, `saveReminder:`, `removeReminder:`) are not thread-safe when called concurrently. Currently there is no serialization — concurrent writes from multiple goroutines can corrupt data.

**Impact**: Data corruption possible if consumer calls CreateEvent/UpdateEvent/DeleteEvent from multiple goroutines simultaneously.

### 3. No context.Context Support

All bridge operations are synchronous with no cancellation support. While operations are typically <200ms, large date range queries or slow iCloud sync can take seconds. There is no way for callers to set timeouts or cancel in-flight operations.

**Impact**: Callers cannot integrate with Go's standard cancellation patterns (HTTP request contexts, CLI signal handling, etc.).

## Solution

### Change 1: Inline Error Returns (Critical)

Replace `__thread` TLS error pattern with a C struct that returns both result and error atomically in a single cgo call.

**ObjC side** — new return type in both `bridge_darwin.h` files:

```c
typedef struct {
    char* result;  // JSON string or NULL on error
    char* error;   // Error message or NULL on success
} ek_result_t;

// Before:
char* ek_cal_fetch_calendars(void);
const char* ek_cal_last_error(void);

// After:
ek_result_t ek_cal_fetch_calendars(void);
// ek_cal_last_error removed entirely
```

**Go side** — each bridge function reads both fields from one cgo call:

```go
// Before:
func (c *Client) Calendars() ([]Calendar, error) {
    cstr := C.ek_cal_fetch_calendars()
    if cstr == nil {
        return nil, fmt.Errorf("calendar: %s", getLastError())  // separate cgo call!
    }
    defer C.ek_cal_free(cstr)
    return parseCalendarsJSON(C.GoString(cstr))
}

// After:
func (c *Client) Calendars() ([]Calendar, error) {
    result := C.ek_cal_fetch_calendars()
    if result.error != nil {
        defer C.free(unsafe.Pointer(result.error))
        return nil, fmt.Errorf("calendar: %s", C.GoString(result.error))
    }
    defer C.ek_cal_free(result.result)
    return parseCalendarsJSON(C.GoString(result.result))
}
```

**Affected functions** (both packages):
- `calendar`: `ek_cal_request_access`, `ek_cal_fetch_calendars`, `ek_cal_fetch_events`, `ek_cal_get_event`, `ek_cal_create_event`, `ek_cal_update_event`, `ek_cal_delete_event` (7 functions)
- `reminders`: `ek_rem_request_access`, `ek_rem_fetch_lists`, `ek_rem_fetch_reminders`, `ek_rem_get_reminder`, `ek_rem_create_reminder`, `ek_rem_update_reminder`, `ek_rem_delete_reminder`, `ek_rem_complete_reminder`, `ek_rem_uncomplete_reminder` (9 functions)

**Migration**: Remove `ek_cal_last_error` / `ek_rem_last_error` and `ek_cal_free` / `ek_rem_free` for error strings (errors are freed with `C.free`, results with `ek_cal_free`). Remove `getLastError()` helper from Go.

### Change 2: Serial Write Queue (Important)

Add a serial dispatch queue on the ObjC side for write operations. Read operations remain concurrent.

```objc
// In bridge_darwin.m (both packages)
static dispatch_queue_t get_write_queue(void) {
    static dispatch_queue_t queue;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        queue = dispatch_queue_create("com.go-eventkit.calendar.writes", DISPATCH_QUEUE_SERIAL);
    });
    return queue;
}

// Wrap write operations:
ek_result_t ek_cal_create_event(const char* json_input) {
    __block ek_result_t res;
    dispatch_sync(get_write_queue(), ^{
        res = ek_cal_create_event_impl(json_input);
    });
    return res;
}
```

**Write functions to serialize**:
- `calendar`: `create_event`, `update_event`, `delete_event`
- `reminders`: `create_reminder`, `update_reminder`, `delete_reminder`, `complete_reminder`, `uncomplete_reminder`

**Read functions left concurrent**: `fetch_calendars`, `fetch_events`, `get_event`, `fetch_lists`, `fetch_reminders`, `get_reminder`

### Change 3: Context-Aware Wrappers (Nice-to-Have)

Add `*WithContext` variants that race the cgo call against context cancellation. The underlying cgo call cannot be interrupted — this only provides early return to the caller.

```go
// New file: calendar/context.go (no build constraint — uses existing methods)

func (c *Client) EventsWithContext(ctx context.Context, start, end time.Time, opts ...ListOption) ([]Event, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    type result struct {
        events []Event
        err    error
    }
    ch := make(chan result, 1)
    go func() {
        events, err := c.Events(start, end, opts...)
        ch <- result{events, err}
    }()

    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case r := <-ch:
        return r.events, r.err
    }
}
```

**Methods to add context variants for**:
- `calendar`: `CalendarsWithContext`, `EventsWithContext`, `EventWithContext`, `CreateEventWithContext`, `UpdateEventWithContext`, `DeleteEventWithContext`
- `reminders`: `ListsWithContext`, `RemindersWithContext`, `ReminderWithContext`, `CreateReminderWithContext`, `UpdateReminderWithContext`, `DeleteReminderWithContext`, `CompleteReminderWithContext`, `UncompleteReminderWithContext`

**Alternative API**: Accept `context.Context` as first parameter on existing methods (breaking change). Recommend the `*WithContext` suffix approach to avoid breaking existing consumers.

## Implementation Order

1. **Change 1 (inline errors)** — Do first. Fixes a real bug and simplifies the codebase. Both packages independently.
2. **Change 2 (write queue)** — Do second. Builds on Change 1's refactored ObjC. Low risk.
3. **Change 3 (context wrappers)** — Do last. Pure Go additions, no ObjC changes. Can be a separate commit.

## Testing

- All existing unit tests must continue passing (JSON parsing is unaffected)
- All existing integration tests must continue passing (API contract unchanged)
- Add new unit test: verify `ek_result_t` error field is populated on failure
- Add new integration test: concurrent reads from 4 goroutines (errgroup)
- Add new integration test: concurrent writes from 2 goroutines (verify serialization)
- Add new unit test: context cancellation returns `context.Canceled`

## Non-Goals

- `runtime.LockOSThread()` — not needed for EventKit, incurs 10x context switch penalty
- NSRunLoop for `EKEventStoreChangedNotification` — too complex for a library, polling is sufficient
- Imposing `init() { runtime.LockOSThread() }` on consumers — hostile to library users

## References

- [docs/research/go-concurrency-cgo-eventkit.md](../research/go-concurrency-cgo-eventkit.md) — full research
- [Go Wiki: LockOSThread](https://go.dev/wiki/LockOSThread)
- [The Cost and Complexity of Cgo - Cockroach Labs](https://www.cockroachlabs.com/blog/the-cost-and-complexity-of-cgo/)
- [runtime.LockOSThread Performance - Go Issue #21827](https://github.com/golang/go/issues/21827)
