# Future Capabilities — PRD (Deferred)

## Overview

This PRD documents EventKit capabilities that are **not planned for immediate implementation** but are worth tracking for future releases. These are lower priority than recurrence rules and structured locations, and should be picked up based on consumer demand.

---

## 1. Calendar CRUD (Create / Delete Calendars)

**What**: Allow creating and deleting calendar containers (not just events within them).

**EventKit API**:
```objc
EKCalendar *cal = [EKCalendar calendarForEntityType:EKEntityTypeEvent eventStore:store];
cal.title = @"My Calendar";
cal.source = someSource;
cal.CGColor = [NSColor redColor].CGColor;
[store saveCalendar:cal commit:YES error:&error];
[store removeCalendar:cal commit:YES error:&error];
```

**Go API**:
```go
type CreateCalendarInput struct {
    Title    string
    Source   string // Source name (e.g., "iCloud")
    Color    string // Hex color
}

func (c *Client) CreateCalendar(input CreateCalendarInput) (*Calendar, error)
func (c *Client) DeleteCalendar(id string) error
```

**Complexity**: Low (~50 lines ObjC, ~30 lines Go).

**When to build**: When a consumer needs to programmatically manage calendars (e.g., a sync tool that creates project-specific calendars).

---

## 2. Source Listing

**What**: List all configured accounts/sources (iCloud, Google, Exchange, Local, etc.).

**EventKit API**:
```objc
NSArray<EKSource *> *sources = store.sources;
// Each source has: sourceIdentifier, title, sourceType
```

**Go API**:
```go
type Source struct {
    ID    string     `json:"id"`
    Title string     `json:"title"`
    Type  SourceType `json:"type"` // Local, Exchange, CalDAV, Subscribed, Birthdays
}

func (c *Client) Sources() ([]Source, error)
```

**Complexity**: Low (~30 lines ObjC, ~40 lines Go).

**When to build**: When a consumer needs to enumerate accounts or assign calendars to specific sources during creation.

---

## 3. Default Calendar Access

**What**: Expose the system default calendars for new events and reminders.

**EventKit API**:
```objc
EKCalendar *defaultEvents = store.defaultCalendarForNewEvents;
EKCalendar *defaultReminders = store.defaultCalendarForNewReminders;
```

**Go API**:
```go
// calendar package
func (c *Client) DefaultCalendar() (*Calendar, error)

// reminders package
func (c *Client) DefaultList() (*List, error)
```

**Complexity**: Very low (~10 lines each).

**When to build**: Useful for any consumer that creates items without specifying a calendar. Currently the ObjC bridge falls back to the default internally, but consumers can't query it.

---

## 4. Richer Attendee Information

**What**: Expose `ParticipantRole` and `ParticipantType` on attendees (currently only name, email, status).

**EventKit API**:
```objc
participant.participantRole   // Required, Optional, Chair, NonParticipant
participant.participantType   // Person, Room, Resource, Group
participant.isCurrentUser     // Whether this is the device owner
```

**Go API changes**:
```go
type Attendee struct {
    Name          string            `json:"name"`
    Email         string            `json:"email"`
    Status        ParticipantStatus `json:"status"`
    Role          ParticipantRole   `json:"role"`          // NEW
    Type          ParticipantType   `json:"type"`          // NEW
    IsCurrentUser bool              `json:"isCurrentUser"` // NEW
}
```

**Complexity**: Low (~20 lines ObjC, ~30 lines Go + enums).

**When to build**: When a consumer needs to distinguish required vs optional attendees, or identify conference rooms vs people.

---

## 5. Event Detachment & External IDs

**What**: Expose additional event identifiers and recurrence detachment info.

**Fields to add to Event**:
```go
type Event struct {
    // ... existing fields ...

    // ExternalID is the server-side identifier (CalDAV UID, Exchange ID).
    // May be empty for local-only events.
    ExternalID string `json:"externalID,omitempty"`
    // IsDetached is true when this occurrence of a recurring event was
    // modified independently from the series.
    IsDetached bool `json:"isDetached"`
    // OccurrenceDate is the original date for this occurrence of a recurring
    // event, even if it was moved to a different date.
    OccurrenceDate *time.Time `json:"occurrenceDate,omitempty"`
}
```

**Complexity**: Very low (~10 lines ObjC to serialize these fields).

**Note**: `IsDetached` and `OccurrenceDate` are included in the recurrence-location PRD since they're closely related to recurrence support. `ExternalID` (`calendarItemExternalIdentifier`) is separate.

**When to build**: When a consumer needs to correlate events with server-side IDs or detect detached occurrences.

---

## 6. Location-Based Alarms

**What**: Alarms that trigger based on entering/leaving a geographic area (geofence), not just time offsets.

**EventKit API**:
```objc
EKAlarm *alarm = [[EKAlarm alloc] init];
alarm.structuredLocation = loc;       // EKStructuredLocation with radius
alarm.proximity = EKAlarmProximityEnter; // or EKAlarmProximityLeave
```

**Go API changes**:
```go
type Alarm struct {
    // Time-based (existing)
    AbsoluteDate   *time.Time    `json:"absoluteDate,omitempty"`
    RelativeOffset time.Duration `json:"relativeOffset,omitempty"`

    // Location-based (NEW)
    Location  *StructuredLocation `json:"location,omitempty"`
    Proximity AlarmProximity      `json:"proximity,omitempty"`
}

type AlarmProximity int

const (
    ProximityNone  AlarmProximity = 0
    ProximityEnter AlarmProximity = 1
    ProximityLeave AlarmProximity = 2
)
```

**Complexity**: Medium — depends on structured location support being implemented first.

**When to build**: When a consumer needs "remind me when I arrive at..." functionality. Primarily useful for reminders, but events support it too.

---

## 7. Batch Operations

**What**: Expose EventKit's `commit:NO` pattern for bulk writes, committing all changes in a single database transaction.

**EventKit API**:
```objc
[store saveEvent:event1 span:EKSpanThisEvent commit:NO error:&error];
[store saveEvent:event2 span:EKSpanThisEvent commit:NO error:&error];
[store saveEvent:event3 span:EKSpanThisEvent commit:NO error:&error];
[store commit:&error]; // Single commit for all
[store reset];         // Or rollback
```

**Go API**:
```go
type Batch struct { /* internal state */ }

func (c *Client) BeginBatch() *Batch
func (b *Batch) CreateEvent(input CreateEventInput) error
func (b *Batch) UpdateEvent(id string, input UpdateEventInput, span Span) error
func (b *Batch) DeleteEvent(id string, span Span) error
func (b *Batch) Commit() error
func (b *Batch) Rollback()
```

**Complexity**: Medium (~100 lines ObjC, ~80 lines Go). Needs careful error handling — partial failures within a batch.

**When to build**: When a consumer performs bulk operations (calendar migration, import from .ics, etc.).

---

## 8. Change Notifications / Polling

**What**: Detect external changes to calendar/reminder data (from Calendar.app, sync, other apps).

**EventKit API**:
```objc
// Notification-based (requires NSRunLoop)
[[NSNotificationCenter defaultCenter] addObserver:self
                                         selector:@selector(storeChanged:)
                                             name:EKEventStoreChangedNotification
                                           object:store];

// Object-level refresh
BOOL stillValid = [event refresh];
```

**Go API** (polling approach — avoids NSRunLoop complexity):
```go
// Refresh re-fetches an event from the store.
// Returns the updated event, or ErrNotFound if it was deleted.
func (c *Client) RefreshEvent(id string) (*Event, error)

// HasChanges returns true if any calendar data has changed since the
// last call. Requires periodic polling.
func (c *Client) HasChanges() bool
```

**Complexity**: High for notification-based (needs NSRunLoop or CFRunLoop from cgo). Low for polling-based `RefreshEvent` (just re-fetch by ID).

**When to build**: When a long-running consumer (TUI, server, watcher) needs to react to external changes. The concurrency PRD's `context.Context` support is a prerequisite.

---

## 9. Write-Only Access Mode

**What**: Support the macOS 14+ write-only calendar access tier.

**EventKit API**:
```objc
[store requestWriteOnlyAccessToEventsWithCompletion:^(BOOL granted, NSError *error) { ... }];
```

**Limitations**: Can only create events into the default calendar. Cannot read events, list calendars, or delete.

**Go API**:
```go
// NewWriteOnly creates a client with write-only access.
// Can only call CreateEvent (uses default calendar). All read methods return ErrWriteOnly.
func NewWriteOnly() (*Client, error)
```

**Complexity**: Low (~20 lines).

**When to build**: When a consumer only needs to create events (e.g., an AI agent adding events to a user's calendar without reading existing ones). Provides a more privacy-friendly access level.

---

## 10. Reminder Recurrence

**What**: EKReminder inherits recurrence rule support from EKCalendarItem. Repeating reminders (e.g., "take medication daily") use the same `EKRecurrenceRule` system as events.

**Go API**: Same `RecurrenceRule` types from the recurrence-location PRD, added to `Reminder` struct and `CreateReminderInput`/`UpdateReminderInput`.

**Complexity**: Low if event recurrence is already implemented (reuse the same types and ObjC serialization pattern).

**When to build**: After event recurrence is shipped. Can share the Go types; ObjC serialization is nearly identical.

---

## Priority Matrix

| Capability | Complexity | Consumer Value | Prerequisite |
|---|---|---|---|
| Calendar CRUD | Low | Medium | None |
| Source listing | Low | Low | None |
| Default calendar access | Very low | Medium | None |
| Richer attendee info | Low | Low | None |
| External IDs | Very low | Low | None |
| Location-based alarms | Medium | Medium | Structured locations |
| Batch operations | Medium | Medium | None |
| Change notifications | High | High | Concurrency PRD |
| Write-only access | Low | Low | None |
| Reminder recurrence | Low | Medium | Event recurrence |

## Recommendation

Pick up in this order based on effort-to-value ratio:
1. **Default calendar access** — trivial, immediately useful
2. **Calendar CRUD** — low effort, enables calendar management tools
3. **Source listing** — low effort, completes the metadata story
4. **Richer attendee info** — low effort, enriches existing data
5. **Reminder recurrence** — low effort after event recurrence ships
6. **Batch operations** — medium effort, needed for import/migration
7. **Location-based alarms** — after structured locations ship
8. **Change notifications** — high effort, defer until long-running consumer exists
9. **Write-only access** — niche use case
