# Benchmarking & Performance Validation — PRD

## Overview

go-eventkit claims "sub-200ms" performance for all operations, based on informal measurements from `rem`. This PRD covers adding formal, reproducible benchmarks to validate that claim and catch performance regressions.

**Priority**: Medium — important for credibility and regression detection, but not blocking features.

**Estimated scope**: ~200 lines Go benchmark code, ~100 lines integration benchmark script.

## Problem

1. The "sub-200ms" claim in README.md and go-eventkit-prd.md is based on `rem`'s benchmarks (106-168ms), not go-eventkit's own measurements.
2. There are no `go test -bench` benchmarks in either package.
3. No baseline numbers exist for comparison when making changes (e.g., adding recurrence serialization, structured location parsing).
4. The JSON bridge format adds serialization overhead — we don't know the breakdown between EventKit time vs JSON parsing time.

## Goals

1. Validate the sub-200ms claim with real numbers on current hardware
2. Establish baseline benchmarks for all Client methods
3. Separate EventKit latency from JSON parsing overhead
4. Provide a simple way to re-run benchmarks after changes
5. Document results for the README performance section

## Non-Goals

- Micro-optimizing JSON parsing (already <1ms for hundreds of items)
- Benchmarking cgo call overhead in isolation (well-studied elsewhere)
- Continuous benchmark tracking in CI (no CI infrastructure yet)

## Design

### Layer 1: JSON Parsing Benchmarks (Unit Tests)

These run anywhere (no macOS/EventKit required) and measure the Go-side overhead.

**File**: `calendar/parse_bench_test.go`

```go
func BenchmarkParseCalendarsJSON(b *testing.B) {
    // Realistic JSON: 10 calendars with all fields populated
    jsonData := generateCalendarsJSON(10)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = parseCalendarsJSON(jsonData)
    }
}

func BenchmarkParseEventsJSON(b *testing.B) {
    // Realistic JSON: 50 events with attendees, alerts, all fields
    jsonData := generateEventsJSON(50)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = parseEventsJSON(jsonData)
    }
}

func BenchmarkParseEventsJSON_Large(b *testing.B) {
    // Stress test: 500 events
    jsonData := generateEventsJSON(500)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = parseEventsJSON(jsonData)
    }
}

func BenchmarkMarshalCreateEventInput(b *testing.B) {
    input := CreateEventInput{
        Title:     "Benchmark Event",
        StartDate: time.Now(),
        EndDate:   time.Now().Add(time.Hour),
        Location:  "Test Location",
        Notes:     "Some notes for the benchmark event",
        Calendar:  "Work",
        Alerts:    []Alert{{RelativeOffset: -15 * time.Minute}},
    }
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = marshalCreateEventInput(input)
    }
}
```

**File**: `reminders/parse_bench_test.go`

```go
func BenchmarkParseListsJSON(b *testing.B)
func BenchmarkParseRemindersJSON(b *testing.B)       // 50 reminders
func BenchmarkParseRemindersJSON_Large(b *testing.B)  // 500 reminders
func BenchmarkMarshalCreateReminderInput(b *testing.B)
```

**Scaling benchmarks** — test with varying item counts to verify linear scaling:

```go
func BenchmarkParseEventsJSON_Scaling(b *testing.B) {
    for _, count := range []int{1, 10, 50, 100, 500} {
        b.Run(fmt.Sprintf("n=%d", count), func(b *testing.B) {
            jsonData := generateEventsJSON(count)
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                _, _ = parseEventsJSON(jsonData)
            }
        })
    }
}
```

### Layer 2: End-to-End Integration Benchmarks

These require macOS with TCC calendar/reminders access and measure the full round-trip: Go → cgo → ObjC → EventKit → JSON → Go parsing.

**File**: `scripts/benchmark.go`

```go
func main() {
    // Setup
    calClient, _ := calendar.New()
    remClient, _ := reminders.New()

    benchmarks := []struct {
        name string
        fn   func() error
    }{
        // Calendar reads
        {"calendar.Calendars", func() error {
            _, err := calClient.Calendars()
            return err
        }},
        {"calendar.Events (7 days)", func() error {
            now := time.Now()
            _, err := calClient.Events(now, now.Add(7*24*time.Hour))
            return err
        }},
        {"calendar.Events (30 days)", func() error {
            now := time.Now()
            _, err := calClient.Events(now, now.Add(30*24*time.Hour))
            return err
        }},
        {"calendar.Events (365 days)", func() error {
            now := time.Now()
            _, err := calClient.Events(now, now.Add(365*24*time.Hour))
            return err
        }},
        {"calendar.Event (by ID)", func() error { /* fetch known event */ }},

        // Calendar writes (create → update → delete cycle)
        {"calendar.CreateEvent", func() error { /* create test event */ }},
        {"calendar.UpdateEvent", func() error { /* update test event */ }},
        {"calendar.DeleteEvent", func() error { /* delete test event */ }},

        // Reminder reads
        {"reminders.Lists", func() error {
            _, err := remClient.Lists()
            return err
        }},
        {"reminders.Reminders (all)", func() error {
            _, err := remClient.Reminders()
            return err
        }},
        {"reminders.Reminders (filtered)", func() error {
            _, err := remClient.Reminders(reminders.WithCompleted(false))
            return err
        }},
        {"reminders.Reminder (by ID)", func() error { /* fetch known reminder */ }},

        // Reminder writes (create → complete → uncomplete → delete cycle)
        {"reminders.CreateReminder", func() error { /* create test reminder */ }},
        {"reminders.CompleteReminder", func() error { /* complete it */ }},
        {"reminders.UncompleteReminder", func() error { /* uncomplete it */ }},
        {"reminders.DeleteReminder", func() error { /* delete it */ }},
    }

    // Run each benchmark N times, report min/median/p95/max
    const iterations = 20

    fmt.Println("go-eventkit benchmarks")
    fmt.Println("======================")
    fmt.Printf("%-35s %8s %8s %8s %8s %6s\n",
        "Operation", "Min", "Median", "P95", "Max", "Pass?")
    fmt.Println(strings.Repeat("-", 80))

    for _, bm := range benchmarks {
        durations := make([]time.Duration, 0, iterations)
        for i := 0; i < iterations; i++ {
            start := time.Now()
            err := bm.fn()
            elapsed := time.Since(start)
            if err != nil {
                fmt.Printf("%-35s ERROR: %v\n", bm.name, err)
                break
            }
            durations = append(durations, elapsed)
        }
        sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })

        min := durations[0]
        median := durations[len(durations)/2]
        p95 := durations[int(float64(len(durations))*0.95)]
        max := durations[len(durations)-1]
        pass := "OK"
        if median > 200*time.Millisecond {
            pass = "SLOW"
        }

        fmt.Printf("%-35s %8s %8s %8s %8s %6s\n",
            bm.name, min, median, p95, max, pass)
    }
}
```

**Expected output format**:
```
go-eventkit benchmarks
======================
Operation                             Min   Median      P95      Max  Pass?
--------------------------------------------------------------------------------
calendar.Calendars                   45ms     52ms     68ms     82ms    OK
calendar.Events (7 days)             89ms    105ms    142ms    168ms    OK
calendar.Events (30 days)           112ms    138ms    185ms    210ms    OK
calendar.Events (365 days)          156ms    189ms    245ms    312ms   SLOW
calendar.Event (by ID)               38ms     44ms     58ms     72ms    OK
calendar.CreateEvent                 85ms     98ms    128ms    145ms    OK
...
```

### Layer 3: Comparative Benchmark (Optional)

Compare go-eventkit against AppleScript/JXA for the same operations, to validate the "100-500x faster" claim.

```go
// AppleScript baseline for comparison
func benchAppleScript(name string) time.Duration {
    start := time.Now()
    exec.Command("osascript", "-e",
        `tell application "Calendar" to get name of every calendar`).Run()
    return time.Since(start)
}
```

## Helper: Realistic JSON Generators

The parse benchmarks need realistic JSON data. Add unexported generator functions to test files:

```go
func generateEventsJSON(count int) string {
    // Generate JSON matching the exact format bridge_darwin.m produces
    // Include: all fields populated, 2-3 attendees per event, 1-2 alerts,
    // mix of all-day and timed events, various statuses/availabilities
}

func generateCalendarsJSON(count int) string {
    // Generate JSON with mix of calendar types, sources, colors
}
```

These generators should produce JSON identical to what `bridge_darwin.m` emits — same field names, same value formats. Use the existing mock bridge test JSON as a reference.

## Performance Targets

Based on `rem` benchmarks and EventKit characteristics:

| Operation | Target (Median) | Rationale |
|---|---|---|
| Fetch calendars | <100ms | Small payload, synchronous API |
| Fetch events (7 days) | <150ms | Synchronous `eventsMatchingPredicate:` |
| Fetch events (30 days) | <200ms | Larger result set |
| Fetch events (365 days) | <300ms | May exceed 200ms — document as known |
| Fetch single event | <100ms | Single lookup by ID |
| Create event | <200ms | Write + commit + re-read |
| Update event | <200ms | Fetch + modify + commit + re-read |
| Delete event | <150ms | Fetch + remove + commit |
| Fetch lists | <100ms | Small payload |
| Fetch reminders (all) | <200ms | Async fetch wrapped with semaphore |
| Fetch reminders (filtered) | <200ms | Predicate-filtered |
| Fetch single reminder | <100ms | Single lookup |
| Create reminder | <200ms | Write + commit + re-read |
| Complete reminder | <150ms | Fetch + set bool + commit |
| Delete reminder | <150ms | Fetch + remove + commit |
| JSON parse (50 events) | <500μs | Pure Go, no I/O |
| JSON parse (500 events) | <5ms | Linear scaling |

**The "sub-200ms" claim applies to typical operations** (7-30 day ranges, moderate item counts). Large date ranges (365 days) or accounts with thousands of items may exceed 200ms — this should be documented as expected behavior rather than adjusting the claim.

## Implementation Plan

1. **Parse benchmarks** — `calendar/parse_bench_test.go` + `reminders/parse_bench_test.go` with JSON generators and scaling tests
2. **Integration benchmark script** — `scripts/benchmark.go` with all operations, statistical output
3. **Run and record** — Execute on dev machine, record baseline numbers
4. **Update README** — Replace "sub-200ms" with actual measured numbers in a performance section
5. **(Optional)** AppleScript comparison for the "100-500x" claim

## Running Benchmarks

```bash
# Parse benchmarks (runs anywhere)
go test -bench=. -benchmem ./calendar/ ./reminders/

# Integration benchmarks (requires macOS + TCC access)
go run ./scripts/benchmark.go

# With CPU profiling (for optimization)
go test -bench=BenchmarkParseEventsJSON -cpuprofile=cpu.prof ./calendar/
go tool pprof cpu.prof
```

## References

- [Go Testing: Benchmarks](https://pkg.go.dev/testing#hdr-Benchmarks)
- [rem benchmarks](https://github.com/BRO3886/rem) — 106-168ms for equivalent operations
- [docs/prd/go-eventkit-prd.md](go-eventkit-prd.md) — original performance targets
