# Change Notifications — PRD

## Status

Planned

## Overview

Add `WatchChanges(ctx context.Context) (<-chan struct{}, error)` to both `calendar.Client` and `reminders.Client`. The method returns a Go channel that receives a signal whenever the EventKit database changes — from this process's own writes, iCloud sync, Calendar.app edits, other apps, or Exchange push. Consumers re-fetch after each signal; the channel carries no diff payload.

**One-line summary**: Subscribe to `EKEventStoreChangedNotification` via a self-pipe trick, delivering signals on a Go channel without requiring NSRunLoop or `//export`.

---

## Background

### Current state

go-eventkit has no mechanism to detect external changes. Consumers that need to stay current must poll by calling `Events` or `Reminders` on a timer. This is wasteful for long-running consumers (TUI dashboards, server-side caches, reactive CLI tools) that may sit idle for long stretches.

### Why change notifications are better than polling

- No unnecessary EventKit calls when nothing has changed.
- Immediately responsive to changes — the notification fires within milliseconds of a write, whether from this process or an external source (iCloud sync typically delivers within 1-2 seconds of propagation).
- Eliminates the awkward choice of polling interval: too short wastes CPU, too long misses changes.

### Prior deferral reason and why it no longer applies

`future-capabilities-prd.md` (item 8) and `concurrency-prd.md` both deferred this feature, citing:

> **Complexity: High** — notification-based approach needs NSRunLoop or CFRunLoop from cgo.

This was accurate for the selector-based `addObserver:selector:object:name:` API, which requires the observer's thread to run a run loop to receive notifications. However, the block-based variant `addObserverForName:object:queue:usingBlock:` with `queue:nil` delivers synchronously on the posting thread — no run loop required.

Combined with the **self-pipe trick** (the block writes one byte to a Unix pipe; a Go goroutine reads the other end), the implementation requires no run loop, no `//export`, no `runtime.LockOSThread()`, and no NSRunLoop thread management. The complexity is now **Low**: approximately 50 lines ObjC and 30 lines Go per package.

---

## Goals

1. Add `WatchChanges(ctx context.Context) (<-chan struct{}, error)` to `calendar.Client`.
2. Add `WatchChanges(ctx context.Context) (<-chan struct{}, error)` to `reminders.Client`.
3. Implement using the self-pipe trick — safe across cgo/ObjC thread boundaries.
4. Channel is closed when `ctx` is cancelled or the pipe fails.
5. Add non-darwin stubs (`bridge_other.go`) returning `ErrUnsupported`.
6. Keep the public API pure Go — no cgo types visible to consumers.

---

## Non-Goals

- **Granular diff of what changed**: Apple's recommendation is to re-fetch everything on any notification. The `userInfo` dict contains private keys (`EKEventStoreChangeTypeUserInfoKey`) whose values are undocumented and change across OS versions. The channel carries `struct{}` only.
- **Main thread enforcement**: Not required. `queue:nil` delivers on the posting thread; the self-pipe never calls back into Go from that thread.
- **Windows/Linux support**: EventKit is macOS-only. Non-darwin stub returns `ErrUnsupported`.
- **Multiple simultaneous watchers per package**: One watcher at a time per package. A second call to `WatchChanges` while the first is active returns an error. This can be relaxed in a future PRD via fan-out if consumers demand it.
- **Cross-package unified watcher**: A single channel that coalesces calendar and reminder changes is out of scope. Each package's `EKEventStore` is independent; a unified watcher would require a separate coordinator package.
- **Debouncing**: Callers are responsible for debouncing if desired. Rapid successive changes (e.g., batch imports) will produce multiple signals. The buffered channel (capacity 16) provides natural coalescing under burst.

---

## API Design

### calendar package

```go
// WatchChanges returns a channel that receives a value whenever the
// EventKit calendar database changes. Changes include writes by this
// process (CreateEvent, UpdateEvent, DeleteEvent, CreateCalendar, etc.),
// iCloud sync, Calendar.app edits, Exchange push, and changes by other
// apps with calendar access.
//
// The channel is closed when ctx is cancelled or an internal read error
// occurs. After ctx cancellation, any pending signals already in the
// channel buffer are still readable.
//
// The channel carries no information about what specifically changed.
// Callers should re-fetch the data they care about after each signal.
// The channel is buffered (capacity 16); if the consumer falls behind,
// excess signals are dropped rather than blocking — this is safe because
// callers re-fetch anyway.
//
// Only one watcher may be active per process. A second call to
// WatchChanges while the first is active returns an error.
//
// Returns [ErrUnsupported] on non-darwin platforms.
func (c *Client) WatchChanges(ctx context.Context) (<-chan struct{}, error)
```

### reminders package

```go
// WatchChanges returns a channel that receives a value whenever the
// EventKit reminders database changes. Changes include writes by this
// process (CreateReminder, UpdateReminder, DeleteReminder, CreateList,
// etc.), iCloud sync, Reminders.app edits, and changes by other apps
// with reminders access.
//
// The channel is closed when ctx is cancelled or an internal read error
// occurs. After ctx cancellation, any pending signals already in the
// channel buffer are still readable.
//
// The channel carries no information about what specifically changed.
// Callers should re-fetch the data they care about after each signal.
// The channel is buffered (capacity 16); if the consumer falls behind,
// excess signals are dropped rather than blocking.
//
// Only one watcher may be active per process. A second call to
// WatchChanges while the first is active returns an error.
//
// Returns [ErrUnsupported] on non-darwin platforms.
func (c *Client) WatchChanges(ctx context.Context) (<-chan struct{}, error)
```

### Behaviour specification

| Property | Value |
|---|---|
| Channel type | `<-chan struct{}` |
| Buffer capacity | 16 |
| On ctx cancellation | Channel is closed; goroutine exits; `ek_cal_watch_stop` / `ek_rem_watch_stop` called |
| On pipe read error | Channel is closed; goroutine exits; cleanup called |
| Burst coalescing | Extra signals dropped via `select { case ch <- struct{}{}: default: }` |
| Multiple callers | Second call returns `errors.New("calendar: watcher already active")` |
| Non-darwin | Returns `nil, ErrUnsupported` |
| Blocking behaviour | Never blocks the caller; goroutine owns the pipe read loop |

### Consumer pattern

```go
client, err := calendar.New()
if err != nil {
    log.Fatal(err)
}

ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
defer cancel()

changes, err := client.WatchChanges(ctx)
if err != nil {
    log.Fatal(err)
}

for range changes {
    events, err := client.Events(start, end)
    if err != nil {
        log.Println("fetch error:", err)
        continue
    }
    render(events) // re-render TUI / refresh cache
}
// channel closed: ctx cancelled or pipe error
```

---

## Implementation Plan

### Step 1 — ObjC additions to `calendar/bridge_darwin.m`

Add three new C functions after the existing `get_store()` definition. Requires adding `#include <unistd.h>` and `#include <fcntl.h>` at the top of the file:

```objc
static int ek_watch_pipe[2] = {-1, -1};
static id ek_store_observer = nil;

// ek_cal_watch_start creates the pipe and registers the NSNotificationCenter
// observer. Returns 1 on success, 0 on pipe creation failure, 1 if already
// started (idempotent for internal use — the Go mutex prevents double-call).
int ek_cal_watch_start(void) {
    if (ek_watch_pipe[0] != -1) return 1;
    if (pipe(ek_watch_pipe) != 0) return 0;
    fcntl(ek_watch_pipe[0], F_SETFD, FD_CLOEXEC);
    fcntl(ek_watch_pipe[1], F_SETFD, FD_CLOEXEC);

    EKEventStore *store = get_store();
    ek_store_observer = [[NSNotificationCenter defaultCenter]
        addObserverForName:EKEventStoreChangedNotification
                    object:store
                     queue:nil
                usingBlock:^(NSNotification *note) {
                    char b = 1;
                    write(ek_watch_pipe[1], &b, 1);
                }];
    return 1;
}

// ek_cal_watch_read_fd returns the read end of the notification pipe.
// Returns -1 if the watcher has not been started.
int ek_cal_watch_read_fd(void) { return ek_watch_pipe[0]; }

// ek_cal_watch_stop removes the observer and closes both pipe ends.
void ek_cal_watch_stop(void) {
    if (ek_store_observer) {
        [[NSNotificationCenter defaultCenter] removeObserver:ek_store_observer];
        ek_store_observer = nil;
    }
    if (ek_watch_pipe[0] != -1) {
        close(ek_watch_pipe[0]);
        close(ek_watch_pipe[1]);
        ek_watch_pipe[0] = ek_watch_pipe[1] = -1;
    }
}
```

### Step 2 — Header declarations in `calendar/bridge_darwin.h`

Add before `#endif`:

```c
// ek_cal_watch_start registers EKEventStoreChangedNotification observer and
// creates a pipe. Returns 1 on success, 0 on failure.
int ek_cal_watch_start(void);

// ek_cal_watch_read_fd returns the read end of the notification pipe, or -1.
int ek_cal_watch_read_fd(void);

// ek_cal_watch_stop removes the observer and closes the pipe.
void ek_cal_watch_stop(void);
```

### Step 3 — Go watcher in `calendar/bridge_darwin.go`

Add after the existing bridge function wrappers. New imports needed: `"context"`, `"errors"`, `"os"`, `"sync"`.

```go
var calWatchMu sync.Mutex
var calWatchActive bool

func (c *Client) WatchChanges(ctx context.Context) (<-chan struct{}, error) {
    calWatchMu.Lock()
    if calWatchActive {
        calWatchMu.Unlock()
        return nil, errors.New("calendar: watcher already active")
    }
    if C.ek_cal_watch_start() == 0 {
        calWatchMu.Unlock()
        return nil, errors.New("calendar: failed to start watcher")
    }
    calWatchActive = true
    calWatchMu.Unlock()

    fd := int(C.ek_cal_watch_read_fd())
    f := os.NewFile(uintptr(fd), "ek-cal-watch-pipe")

    ch := make(chan struct{}, 16)
    go func() {
        defer func() {
            C.ek_cal_watch_stop()
            calWatchMu.Lock()
            calWatchActive = false
            calWatchMu.Unlock()
            close(ch)
        }()
        buf := make([]byte, 64)
        for {
            select {
            case <-ctx.Done():
                return
            default:
            }
            n, err := f.Read(buf)
            if err != nil || n == 0 {
                return
            }
            for i := 0; i < n; i++ {
                select {
                case ch <- struct{}{}:
                default: // drop: consumer will re-fetch anyway
                }
            }
        }
    }()
    return ch, nil
}
```

### Step 4 — Non-darwin stub in `calendar/bridge_other.go`

```go
func (c *Client) WatchChanges(ctx context.Context) (<-chan struct{}, error) {
    return nil, ErrUnsupported
}
```

### Step 5 — Mirror for `reminders` package

Repeat Steps 1–4 for `reminders/` with the following substitutions:

| calendar | reminders |
|---|---|
| `ek_watch_pipe` | `ek_rem_watch_pipe` |
| `ek_store_observer` | `ek_rem_store_observer` |
| `ek_cal_watch_start` | `ek_rem_watch_start` |
| `ek_cal_watch_read_fd` | `ek_rem_watch_read_fd` |
| `ek_cal_watch_stop` | `ek_rem_watch_stop` |
| `calWatchMu` | `remWatchMu` |
| `calWatchActive` | `remWatchActive` |
| error prefix `"calendar:"` | `"reminders:"` |
| pipe file name `"ek-cal-watch-pipe"` | `"ek-rem-watch-pipe"` |

`EKEventStoreChangedNotification` is the same name in both — it fires for the respective `EKEventStore` instance.

### Implementation order

1. `calendar/bridge_darwin.h` — add three declarations
2. `calendar/bridge_darwin.m` — add three C functions + includes
3. `calendar/bridge_darwin.go` — add `WatchChanges` method and mutex vars
4. `calendar/bridge_other.go` — add `WatchChanges` stub
5. `go build ./calendar/` — verify
6. Repeat steps 1–4 for `reminders/`
7. `go build ./...` — verify full build including cross-platform stubs
8. Write tests (see Testing Strategy)

---

## Testing Strategy

### Unit tests — internal helper injection

Extract the pipe-reading loop into an unexported `watchChangesFromFile(ctx context.Context, f *os.File) <-chan struct{}` helper in a non-cgo file (no build tag). This decouples the channel logic from the ObjC layer and makes it fully testable with `os.Pipe()`:

```go
// calendar/watch_test.go (no build tag — pure Go, uses os.Pipe)

func TestWatchChanges_SignalOnWrite(t *testing.T) {
    r, w, _ := os.Pipe()
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    ch := watchChangesFromFile(ctx, r)

    w.Write([]byte{1})
    select {
    case _, ok := <-ch:
        if !ok {
            t.Fatal("channel closed unexpectedly")
        }
    case <-time.After(time.Second):
        t.Fatal("timeout waiting for signal")
    }
}

func TestWatchChanges_CtxCancel(t *testing.T) {
    r, _, _ := os.Pipe()
    ctx, cancel := context.WithCancel(context.Background())
    ch := watchChangesFromFile(ctx, r)
    cancel()
    _, ok := <-ch
    if ok {
        t.Fatal("channel should be closed after cancel")
    }
}

func TestWatchChanges_PipeClose(t *testing.T) {
    r, w, _ := os.Pipe()
    ch := watchChangesFromFile(context.Background(), r)
    w.Close()
    _, ok := <-ch
    if ok {
        t.Fatal("channel should be closed after pipe EOF")
    }
}

func TestWatchChanges_Coalescing(t *testing.T) {
    r, w, _ := os.Pipe()
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    ch := watchChangesFromFile(ctx, r)
    w.Write(bytes.Repeat([]byte{1}, 100))
    time.Sleep(50 * time.Millisecond)

    count := 0
    for {
        select {
        case _, ok := <-ch:
            if !ok {
                goto done
            }
            count++
        default:
            goto done
        }
    }
done:
    if count > 16 {
        t.Fatalf("expected at most 16 coalesced signals, got %d", count)
    }
}
```

### Cross-platform build test

```bash
GOOS=linux CGO_ENABLED=0 go build ./...
```

Verifies non-darwin stubs compile and `WatchChanges` is accessible on all platforms.

### Integration tests (`scripts/integration.go` additions, tests 32–35)

```
[32] WatchChanges: create event while watching → channel receives signal within 5s
[33] WatchChanges: ctx cancel → channel closes
[34] WatchChanges: double call → second returns error
[35] WatchChanges: stop and restart → second WatchChanges succeeds after first ctx cancelled
```

Mirror the same 4 tests in `scripts/integration_reminders.go` (tests 31–34).

### Coverage impact

`WatchChanges` in `bridge_darwin.go` is unreachable by `go test` (same cgo boundary constraint as all other bridge functions). The `watchChangesFromFile` helper is fully testable. Overall coverage ceiling remains approximately 55-57%.

---

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Calling Go from non-Go thread | N/A | SIGSEGV / runtime corruption | Avoided entirely — the block only calls `write(2)`, a pure C syscall |
| NSRunLoop requirement | N/A | Notifications never fire | Non-issue — `addObserverForName:object:queue:usingBlock:` with `queue:nil` delivers on the posting thread |
| Goroutine leak | Low | Memory growth in long-running processes | `ctx.Done()` cancellation always terminates the goroutine; `defer close(ch)` + `defer ek_cal_watch_stop()` ensure cleanup |
| Multiple concurrent callers | Low | Duplicate pipe / double observer | `calWatchMu` mutex serializes calls; second call returns error while first is active |
| Missed notifications during startup | Low | First change after `WatchChanges` not seen | Observer registered before `WatchChanges` returns; no window where changes are missed |
| Pipe buffer exhaustion (kernel) | Very low | `write` blocks in ObjC block | Kernel pipe buffer ~64 KB = 65,536 single-byte notifications. The Go goroutine drains continuously; exhaustion is not possible in practice |
| EKEventStore thread safety | Low | Data corruption | No EventKit calls in the notification hot path — only `write(2)` |
| `os.NewFile` fd lifecycle | Low | Double-close | `os.NewFile` does not take ownership. Do not call `f.Close()` — let `ek_cal_watch_stop` own the fd lifecycle via `close(2)` |
| FD inheritance by child processes | Low | fd leak | Both pipe ends set `FD_CLOEXEC` — not inherited by subprocesses |

---

## Open Questions

1. **Should `watchChangesFromFile` be exported?** Keep unexported — consumers don't need to inject pipes. Exported only if a future test framework demands it.

2. **Should "already active" be a sentinel error var?** Could define `ErrWatcherActive`. Add if a second caller use case (fan-out) emerges; plain `errors.New` is sufficient for now.

3. **Should both packages ship in the same commit?** Yes — the API surface is symmetric. Shipping only one creates an asymmetric library.

4. **Unified cross-package watcher?** Out of scope. A hypothetical `eventkit.WatchAll(ctx)` fan-out is deferred until a consumer demonstrates the need.

5. **Interaction with concurrency PRD?** The three new watcher functions do not use the `__thread` error pattern, so there is no conflict with the concurrency PRD's `ek_result_t` refactor. The two can be implemented in either order independently.

6. **What if the consumer never reads from the channel?** Signals beyond buffer capacity (16) are silently dropped via the non-blocking select. The goroutine never blocks. This is intentional and documented in the godoc.

---

## References

- `EKEventStoreChangedNotification` — Apple Developer Documentation
- `addObserverForName:object:queue:usingBlock:` — NSNotificationCenter
- golang/go#3068 — calling `//export` Go functions from threads not created by Go's runtime
- `docs/prd/concurrency-prd.md` — write serialization and context support (no dependency, independent work)
- `docs/prd/future-capabilities-prd.md` — item 8, original deferral reasoning
- `calendar/bridge_darwin.go`, `reminders/bridge_darwin.go` — existing cgo bridge patterns
