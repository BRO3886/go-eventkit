# Go Concurrency with cgo and Apple EventKit

Research compiled: 2026-02-11

## 1. cgo Threading Model

### How cgo Interacts with Go Goroutines

- **Goroutine pinned to OS thread** for the duration of the C call
- **Go scheduler cannot preempt** while in C code
- **Blocking cgo call occupies entire OS thread** — runtime spawns new threads for other goroutines
- **`GOMAXPROCS` limits Go code threads, not cgo threads**
- **Successive cgo calls may land on different OS threads** (unless `runtime.LockOSThread()`)

### Thread-Local Storage Hazard

The `__thread` (TLS) error pattern used in the bridges is a race condition:

1. Goroutine A calls `ek_cal_fetch_events()` which fails and sets `cal_last_error` on thread T1
2. Between the cgo return and the next cgo call, goroutine A may be rescheduled to thread T2
3. Goroutine A calls `ek_cal_last_error()` on thread T2, reading stale/empty error

**Recommended fix**: Return errors inline (struct with result + error) rather than separate TLS error reads.

## 2. dispatch_queue vs Go Goroutines

### Recommendations

- **Do NOT use GCD dispatch_queues to schedule work from Go** — creates two competing schedulers
- **Use dispatch_semaphore only for waiting on async completion handlers** (reminders fetch)
- **Never dispatch_sync onto main queue from cgo call** — risks deadlock
- **Use serial dispatch queue on ObjC side** for write serialization

## 3. Thread-Safety of EKEventStore

| Operation | Thread-Safe? | Concurrent OK? |
|-----------|-------------|----------------|
| `eventsMatchingPredicate:` | Yes (read) | Yes |
| `calendarsForEntityType:` | Yes (read) | Yes |
| `eventWithIdentifier:` | Yes (read) | Yes |
| `saveEvent:span:commit:` | No (write) | Must serialize |
| `removeEvent:span:commit:` | No (write) | Must serialize |
| `fetchRemindersMatchingPredicate:` | Yes (async read) | Yes |
| `saveReminder:commit:` | No (write) | Must serialize |

## 4. Concurrent Fetch Patterns

### Parallel Reads from Go

```go
func (c *Client) FetchAll(ctx context.Context, start, end time.Time) (*FetchResult, error) {
    g, ctx := errgroup.WithContext(ctx)
    var calendars []Calendar
    var events []Event

    g.Go(func() error {
        var err error
        calendars, err = c.Calendars()
        return err
    })
    g.Go(func() error {
        var err error
        events, err = c.Events(start, end)
        return err
    })

    if err := g.Wait(); err != nil {
        return nil, err
    }
    return &FetchResult{Calendars: calendars, Events: events}, nil
}
```

### Limit Concurrent cgo Calls

Use a semaphore channel to cap at 4-8 concurrent cgo calls to prevent thread exhaustion.

## 5. Context Cancellation with cgo

### Pattern: Goroutine Wrapper with Context Race

```go
func (c *Client) EventsWithContext(ctx context.Context, start, end time.Time) ([]Event, error) {
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
        events, err := c.Events(start, end)
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

**Caveats**: The cgo call continues running even after context cancellation. For short operations (< 200ms), context cancellation is mainly for timeout propagation.

## 6. runtime.LockOSThread()

**NOT needed for EventKit operations:**
- EKEventStore does not require main thread
- Singleton is process-global, not thread-local
- EventKit is not a UI framework

**Do NOT impose `init() { runtime.LockOSThread() }` on library consumers.**

## 7. NSDateFormatter Thread Safety

`NSDateFormatter` is thread-safe on macOS 10.9+ (64-bit). The `dispatch_once` formatter pattern is fine.

## 8. Best Practices for go-eventkit

1. **Eliminate `__thread` error storage** — return errors inline with results
2. **Keep `dispatch_once` singleton** — correct and proven
3. **Serialize writes on ObjC side** with serial dispatch queue
4. **Allow concurrent reads** — EventKit reads are thread-safe
5. **Limit concurrent cgo calls** to 4-8 with semaphore channel
6. **Do NOT use `runtime.LockOSThread()`** for EventKit
7. **Context support via goroutine wrapper** race pattern
8. **Batch writes** with `commit:NO` + final `commit:` for bulk operations

## Sources

- [Go Wiki: LockOSThread](https://go.dev/wiki/LockOSThread)
- [The Cost and Complexity of Cgo - Cockroach Labs](https://www.cockroachlabs.com/blog/the-cost-and-complexity-of-cgo/)
- [runtime.LockOSThread Performance Penalty - Go Issue #21827](https://github.com/golang/go/issues/21827)
- [Apple Thread Safety Summary](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/Multithreading/ThreadSafetySummary/ThreadSafetySummary.html)
- [EKEventStore Documentation](https://developer.apple.com/documentation/eventkit/ekeventstore)
- [NSDateFormatter Thread Safety](https://medium.com/@rosescugeorge/as-far-as-i-know-as-of-ios-7-dateformatter-is-indeed-thread-safe-25f38ee18d4d)
