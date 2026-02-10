# Apple EventKit Framework — Comprehensive Research

> Research compiled: February 2026
> Scope: macOS EventKit framework — all entity types, classes, capabilities, limitations, and recent API changes (macOS 13-15)

---

## Table of Contents

1. [Framework Overview](#1-framework-overview)
2. [Core Classes & Hierarchy](#2-core-classes--hierarchy)
3. [EKEventStore — The Central Hub](#3-ekeventstore--the-central-hub)
4. [EKCalendarItem — Shared Base Class](#4-ekcalendaritem--shared-base-class)
5. [EKEvent — Calendar Events](#5-ekevent--calendar-events)
6. [EKReminder — Reminders](#6-ekreminder--reminders)
7. [EKCalendar — Calendar Containers](#7-ekcalendar--calendar-containers)
8. [EKSource — Account/Source Management](#8-eksource--accountsource-management)
9. [EKRecurrenceRule — Full Recurrence Support](#9-ekrecurrencerule--full-recurrence-support)
10. [EKStructuredLocation — Geolocation Features](#10-ekstructuredlocation--geolocation-features)
11. [EKAlarm — Alerts & Location-Based Triggers](#11-ekalarm--alerts--location-based-triggers)
12. [EKParticipant — Attendee Details](#12-ekparticipant--attendee-details)
13. [EKVirtualConferenceProvider — Virtual Meetings](#13-ekvirtualconferenceprovider--virtual-meetings)
14. [Permission Model & TCC](#14-permission-model--tcc)
15. [Change Tracking & Notifications](#15-change-tracking--notifications)
16. [Batch Operations & Performance](#16-batch-operations--performance)
17. [Threading & Best Practices](#17-threading--best-practices)
18. [macOS 13 Ventura Changes](#18-macos-13-ventura-changes)
19. [macOS 14 Sonoma Changes (2023)](#19-macos-14-sonoma-changes-2023)
20. [macOS 15 Sequoia Changes (2024)](#20-macos-15-sequoia-changes-2024)
21. [Known Limitations](#21-known-limitations)
22. [Implications for go-eventkit](#22-implications-for-go-eventkit)

---

## 1. Framework Overview

EventKit is Apple's framework for accessing and manipulating calendar and reminder data on macOS and iOS. It provides a unified interface to all calendar accounts (iCloud, Google/CalDAV, Exchange, local, subscribed, birthdays) through a single API.

**Two companion frameworks:**
- **EventKit** — Data access layer (events, reminders, calendars, sources, alarms, recurrence rules)
- **EventKitUI** — View controllers for calendar UI (iOS/Mac Catalyst only; runs in a separate process since iOS 17)

**Entity Types** (EKEntityType enum):
- `EKEntityTypeEvent` (0) — Calendar events
- `EKEntityTypeReminder` (1) — Reminders

These are the only two entity types. There is no third entity type for notes, contacts, etc.

---

## 2. Core Classes & Hierarchy

```
NSObject
 └── EKObject                          # Base for all EventKit objects
      ├── EKEventStore                  # Central data access hub
      ├── EKSource                      # Account/source (iCloud, Google, etc.)
      ├── EKCalendar                    # Calendar container
      ├── EKCalendarItem (abstract)     # Base for events and reminders
      │    ├── EKEvent                  # Calendar event
      │    └── EKReminder               # Reminder/task
      ├── EKAlarm                       # Alert/notification trigger
      ├── EKRecurrenceRule              # Recurrence pattern definition
      ├── EKRecurrenceEnd               # Recurrence termination condition
      ├── EKRecurrenceDayOfWeek         # Day-of-week specifier
      ├── EKStructuredLocation          # Geolocation with geofence radius
      └── EKParticipant                 # Attendee/organizer (read-only)

EKVirtualConferenceProvider             # Extension point for video call apps
EKVirtualConferenceDescriptor           # Virtual conference details
EKVirtualConferenceURLDescriptor        # URL to join a virtual conference
EKVirtualConferenceRoomTypeDescriptor   # Room type for conference providers
```

**EKObject base class** provides:
- `hasChanges` (BOOL, read-only) — whether the object has unsaved modifications
- `isNew` (BOOL, read-only) — whether the object is newly created and not yet saved
- `refresh` — re-fetches the object's data from the store
- `reset` — resets unsaved changes
- `rollback` — discards pending changes

---

## 3. EKEventStore — The Central Hub

EKEventStore is the primary interface to calendar and reminder data. **Only one instance should exist per application** — it is heavyweight and slow to instantiate/release.

### Initialization

```objc
// Standard init
EKEventStore *store = [[EKEventStore alloc] init];

// Async init (background)
EKEventStore *store = [[EKEventStore alloc] initAsyncWithCompletionBlock:^{
    // Store is ready
}];

// Init with specific entity types
EKEventStore *store = [[EKEventStore alloc] initWithAccessToEntityTypes:EKEntityMaskEvent];

// Init with source filters (filter by specific accounts)
EKEventStore *store = [[EKEventStore alloc] initWithSourceFilters:@[filter1, filter2]];
```

### Access Request Methods

```objc
// Legacy (pre-macOS 14) — deprecated but still needed for backward compat
[store requestAccessToEntityType:EKEntityTypeEvent completion:^(BOOL granted, NSError *error) {
    // ...
}];

// macOS 14+ / iOS 17+ — Full access (read + write)
[store requestFullAccessToEventsWithCompletion:^(BOOL granted, NSError *error) {
    // ...
}];

// macOS 14+ / iOS 17+ — Write-only access (NEW)
[store requestWriteOnlyAccessToEventsWithCompletion:^(BOOL granted, NSError *error) {
    // ...
}];

// macOS 14+ / iOS 17+ — Full access to reminders
[store requestFullAccessToRemindersWithCompletion:^(BOOL granted, NSError *error) {
    // ...
}];

// Check current authorization status
EKAuthorizationStatus status = [EKEventStore authorizationStatusForEntityType:EKEntityTypeEvent];
// EKAuthorizationStatusNotDetermined, EKAuthorizationStatusRestricted,
// EKAuthorizationStatusDenied, EKAuthorizationStatusFullAccess,
// EKAuthorizationStatusWriteOnly (new in macOS 14)
```

### Event Query Methods

```objc
// Create a date-range predicate for events
NSPredicate *predicate = [store predicateForEventsWithStartDate:startDate
                                                       endDate:endDate
                                                     calendars:nil]; // nil = all calendars

// Synchronous fetch — returns all matching events (PREFERRED for Calendar events)
NSArray<EKEvent *> *events = [store eventsMatchingPredicate:predicate];

// Block-based enumeration
[store enumerateEventsMatchingPredicate:predicate usingBlock:^(EKEvent *event, BOOL *stop) {
    // Process each event; set *stop = YES to halt
}];

// Fetch single event by identifier
EKEvent *event = [store eventWithIdentifier:@"eventIdentifier"];

// Advanced predicate with time zone and prefetch hints
NSPredicate *predicate = [store predicateForEventsWithStartDate:startDate
                                                       endDate:endDate
                                                      timeZone:tz
                                                     calendars:cals
                                                  prefetchHint:hint];

// Predicate with exclusion options
NSPredicate *predicate = [store predicateForEventsWithStartDate:startDate
                                                       endDate:endDate
                                           calendarIdentifiers:calIDs
                                               prefetchHint:hint
                                           exclusionOptions:options];

// Natural language search (private API, visible in headers)
NSPredicate *predicate = [store predicateForNaturalLanguageSuggestedEventsWithSearchString:@"lunch"];
```

### Reminder Query Methods

```objc
// All reminders in specific calendars
NSPredicate *predicate = [store predicateForRemindersInCalendars:calendars];

// Incomplete reminders with due date range
NSPredicate *predicate = [store predicateForIncompleteRemindersWithDueDateStarting:startDate
                                                                            ending:endDate
                                                                         calendars:calendars];

// Completed reminders with completion date range
NSPredicate *predicate = [store predicateForCompletedRemindersWithCompletionDateStarting:startDate
                                                                                  ending:endDate
                                                                               calendars:calendars];

// Async fetch (required for reminders — no synchronous API)
id fetchIdentifier = [store fetchRemindersMatchingPredicate:predicate
                                                 completion:^(NSArray<EKReminder *> *reminders) {
    // Process reminders (called on arbitrary thread)
}];

// Cancel an in-flight fetch
[store cancelFetchRequest:fetchIdentifier];

// Single reminder by identifier
EKReminder *reminder = (EKReminder *)[store calendarItemWithIdentifier:@"calendarItemID"];
```

### Calendar & Source Methods

```objc
// All calendars for a specific entity type
NSArray<EKCalendar *> *cals = [store calendarsForEntityType:EKEntityTypeEvent];

// Single calendar by identifier
EKCalendar *cal = [store calendarWithIdentifier:@"calID"];

// Default calendars
EKCalendar *defaultEvents = store.defaultCalendarForNewEvents;
EKCalendar *defaultReminders = store.defaultCalendarForNewReminders;

// All sources
NSArray<EKSource *> *sources = store.sources;
// Also: store.delegateSources (delegate/shared sources)

// Source by identifier
EKSource *source = [store sourceWithIdentifier:@"sourceID"];
```

### Save/Remove/Commit Methods

```objc
// --- Events ---
NSError *error;

// Save event (commit:YES saves immediately, commit:NO batches)
BOOL success = [store saveEvent:event span:EKSpanThisEvent commit:YES error:&error];

// Remove event
BOOL success = [store removeEvent:event span:EKSpanFutureEvents commit:YES error:&error];

// --- Reminders ---
BOOL success = [store saveReminder:reminder commit:YES error:&error];
BOOL success = [store removeReminder:reminder commit:YES error:&error];

// --- Calendars ---
BOOL success = [store saveCalendar:calendar commit:YES error:&error];
BOOL success = [store removeCalendar:calendar commit:YES error:&error];

// --- Batch commit (when commit:NO was used above) ---
BOOL success = [store commit:&error];

// --- Reset all unsaved changes ---
[store reset];
```

### EKSpan Values

```objc
typedef NS_ENUM(NSInteger, EKSpan) {
    EKSpanThisEvent,    // Only this occurrence
    EKSpanFutureEvents  // This and all future occurrences
};
```

### Notification/Invitation Methods (from private headers)

```objc
// Respond to calendar invitations
[store respondToInvitation:event
                withStatus:EKParticipantStatusAccepted
           notifyOrganizer:YES
        placingInCalendar:calendar
                   commit:YES
                    error:&error];

// Accept invitation with date exclusions
[store acceptInvitation:event
        exceptForDates:excludedDates
       notifyOrganizer:YES
     placingInCalendar:calendar
                commit:YES
                 error:&error];

// Shared calendar invitation response
[store respondToSharedCalendarInvitation:notification
                             withStatus:status
                                 commit:YES
                                  error:&error];
```

### Properties

| Property | Type | Access | Description |
|----------|------|--------|-------------|
| `sources` | NSArray<EKSource *> | Read-only | All known sources/accounts |
| `delegateSources` | NSArray<EKSource *> | Read-only | Delegate/shared sources |
| `defaultCalendarForNewEvents` | EKCalendar | Read-write | Default calendar for new events |
| `defaultCalendarForNewReminders` | EKCalendar | Read-only | Default list for new reminders |
| `eventStoreIdentifier` | NSString | Read-only | Unique ID for this store instance |
| `calendars` | NSArray<EKCalendar *> | Read-only | All calendars (deprecated; use `calendarsForEntityType:`) |

---

## 4. EKCalendarItem — Shared Base Class

EKCalendarItem is the **abstract superclass** for both `EKEvent` and `EKReminder`. All shared properties live here.

### Properties

| Property | Type | Access | Description |
|----------|------|--------|-------------|
| `calendar` | EKCalendar | Read-write | The calendar this item belongs to |
| `calendarItemIdentifier` | NSString | Read-only | Unique ID (may change for recurring events) |
| `calendarItemExternalIdentifier` | NSString | Read-only | External/server-side ID |
| `title` | NSString | Read-write | Item title |
| `location` | NSString | Read-write | Location as string |
| `notes` | NSString | Read-write | Description/notes |
| `URL` | NSURL | Read-write | Associated URL |
| `timeZone` | NSTimeZone | Read-write | Time zone for the item |
| `creationDate` | NSDate | Read-only | When the item was created |
| `lastModifiedDate` | NSDate | Read-only | When the item was last modified |
| `hasAlarms` | BOOL | Read-only | Whether the item has any alarms |
| `hasAttendees` | BOOL | Read-only | Whether the item has any attendees |
| `hasNotes` | BOOL | Read-only | Whether the item has notes |
| `hasRecurrenceRules` | BOOL | Read-only | Whether the item has recurrence rules |
| `attendees` | NSArray<EKParticipant *> | **Read-only** | Attendees (CANNOT be set) |
| `alarms` | NSArray<EKAlarm *> | Read-only (use add/remove) | Alarms on this item |
| `recurrenceRules` | NSArray<EKRecurrenceRule *> | Read-only (use add/remove) | Recurrence rules |

### Methods

```objc
// Alarm management
- (void)addAlarm:(EKAlarm *)alarm;
- (void)removeAlarm:(EKAlarm *)alarm;

// Recurrence rule management
- (void)addRecurrenceRule:(EKRecurrenceRule *)rule;
- (void)removeRecurrenceRule:(EKRecurrenceRule *)rule;
```

**Key insight**: `attendees` is read-only at the EKCalendarItem level. There is no `addAttendee:` or `setAttendees:` method. This is an Apple-imposed limitation across the entire framework.

---

## 5. EKEvent — Calendar Events

EKEvent represents a calendar event with a start time, end time, and duration.

### Properties (in addition to EKCalendarItem)

| Property | Type | Access | Description |
|----------|------|--------|-------------|
| `eventIdentifier` | NSString | Read-only | **Stable** identifier — persists across recurrence edits |
| `startDate` | NSDate | Read-write | Event start (required for save) |
| `endDate` | NSDate | Read-write | Event end (required for save) |
| `allDay` | BOOL | Read-write | Whether this is an all-day event |
| `availability` | EKEventAvailability | Read-write | Free/busy status |
| `status` | EKEventStatus | Read-only | Confirmed/tentative/canceled |
| `organizer` | EKParticipant | **Read-only** | Event organizer |
| `isDetached` | BOOL | Read-only | Whether this occurrence was detached from recurrence |
| `occurrenceDate` | NSDate | Read-only | The original occurrence date for recurring events |
| `structuredLocation` | EKStructuredLocation | Read-write | Geolocation with coordinates |
| `birthdayContactIdentifier` | NSString | Read-only | Contact ID for birthday events |
| `birthdayPersonID` | NSInteger | Read-only | (Deprecated) Person ID for birthday events |

### Enums

```objc
typedef NS_ENUM(NSInteger, EKEventAvailability) {
    EKEventAvailabilityNotSupported = -1,
    EKEventAvailabilityBusy         = 0,
    EKEventAvailabilityFree         = 1,
    EKEventAvailabilityTentative    = 2,
    EKEventAvailabilityUnavailable  = 3
};

typedef NS_ENUM(NSInteger, EKEventStatus) {
    EKEventStatusNone      = 0,
    EKEventStatusConfirmed = 1,
    EKEventStatusTentative = 2,
    EKEventStatusCancelled = 3  // Note the British spelling
};
```

### Methods

```objc
// Factory method
+ (EKEvent *)eventWithEventStore:(EKEventStore *)eventStore;

// Compare start dates (for sorting)
- (NSComparisonResult)compareStartDateWithEvent:(EKEvent *)other;

// Refresh — re-fetch from store
- (BOOL)refresh;
```

### Identifier Notes

- **`eventIdentifier`** — Stable across recurrence edits. This is what you should store externally. Use `[store eventWithIdentifier:]` to retrieve.
- **`calendarItemIdentifier`** — Inherited from EKCalendarItem. May change if a recurring event occurrence is detached/modified. Less stable for external storage.
- **`calendarItemExternalIdentifier`** — Server-side ID (e.g., CalDAV UID). Stable across devices but may be nil for local-only events.

---

## 6. EKReminder — Reminders

EKReminder represents a task/reminder item.

### Properties (in addition to EKCalendarItem)

| Property | Type | Access | Description |
|----------|------|--------|-------------|
| `startDateComponents` | NSDateComponents | Read-write | Start date (nil = no start) |
| `dueDateComponents` | NSDateComponents | Read-write | Due date (nil = no due date) |
| `completed` | BOOL | Read-write | Completion status |
| `completionDate` | NSDate | Read-write | When marked complete |
| `priority` | NSUInteger | Read-write | 0 = none, 1 = highest, 9 = lowest |

### Date Component Semantics

- Uses `NSDateComponents` (not `NSDate`) to support floating dates (no time zone)
- A nil `timeZone` in the date components means a "floating" date
- Omitting hour/minute/second makes it an all-day reminder
- Setting `completed = YES` auto-sets `completionDate` to now
- Setting `completed = NO` auto-sets `completionDate` to nil
- Setting `completionDate` to a date auto-sets `completed = YES`
- Setting `completionDate` to nil auto-sets `completed = NO`

### Priority Values

| Value | Meaning | Apple Calendar Mapping |
|-------|---------|----------------------|
| 0 | No priority | None |
| 1 | Highest | High (!) |
| 2-4 | High range | High |
| 5 | Medium | Medium (!!) |
| 6-8 | Low range | Low |
| 9 | Lowest | Low (!!!) |

Values outside 0-9 will cause a save failure.

### Factory Method

```objc
+ (EKReminder *)reminderWithEventStore:(EKEventStore *)eventStore;
```

### Key Difference from EKEvent

Reminders use **asynchronous** fetch via `fetchRemindersMatchingPredicate:completion:`, while events have the **synchronous** `eventsMatchingPredicate:`. This is because reminders may need to query across multiple accounts that respond at different speeds.

---

## 7. EKCalendar — Calendar Containers

EKCalendar represents a calendar (e.g., "Work", "Personal", "US Holidays").

### Properties

| Property | Type | Access | Description |
|----------|------|--------|-------------|
| `calendarIdentifier` | NSString | Read-only | Unique identifier |
| `title` | NSString | Read-write | Calendar display name |
| `type` | EKCalendarType | Read-only | Calendar type |
| `source` | EKSource | Read-write | The source/account this calendar belongs to |
| `CGColor` | CGColorRef | Read-write | Display color |
| `allowedEntityTypes` | EKEntityMask | Read-only | Which entity types this calendar supports |
| `allowsContentModifications` | BOOL | Read-only | Whether items can be added/edited/deleted |
| `isSubscribed` | BOOL | Read-only | Whether this is a subscribed (read-only) calendar |
| `isImmutable` | BOOL | Read-only | Whether the calendar itself cannot be modified |
| `supportedEventAvailabilities` | EKCalendarEventAvailabilityMask | Read-only | Supported availability values |

### Calendar Types (EKCalendarType)

```objc
typedef NS_ENUM(NSInteger, EKCalendarType) {
    EKCalendarTypeLocal       = 0,  // Local-only calendar
    EKCalendarTypeCalDAV      = 1,  // CalDAV (includes iCloud, Google)
    EKCalendarTypeExchange    = 2,  // Microsoft Exchange
    EKCalendarTypeSubscription = 3, // Subscribed (read-only, .ics)
    EKCalendarTypeBirthday    = 4   // System birthdays calendar
};
```

### Creating & Saving Calendars

```objc
EKCalendar *newCal = [EKCalendar calendarForEntityType:EKEntityTypeEvent
                                            eventStore:store];
newCal.title = @"My Calendar";
newCal.source = [store localSource]; // or any EKSource
newCal.CGColor = [NSColor redColor].CGColor;

NSError *error;
[store saveCalendar:newCal commit:YES error:&error];
```

### Important Notes

- `allowsContentModifications` must be checked before attempting writes — subscribed and birthday calendars are read-only
- `supportedEventAvailabilities` returns a bitmask; `EKCalendarEventAvailabilityNone` means the calendar does not support availability
- Calendar color is a `CGColorRef`, not `NSColor` — requires Core Graphics

---

## 8. EKSource — Account/Source Management

EKSource represents a calendar account/source (e.g., iCloud, Google, Exchange, On My Mac).

### Properties

| Property | Type | Access | Description |
|----------|------|--------|-------------|
| `sourceIdentifier` | NSString | Read-only | Unique identifier for this source |
| `title` | NSString | Read-only | Display name (e.g., "iCloud", "Google") |
| `sourceType` | EKSourceType | Read-only | Type of source |
| `isDelegate` | BOOL | Read-only | Whether this is a delegate/shared source |

### Source Types (EKSourceType)

```objc
typedef NS_ENUM(NSInteger, EKSourceType) {
    EKSourceTypeLocal      = 0,  // Local/on-device
    EKSourceTypeExchange   = 1,  // Microsoft Exchange
    EKSourceTypeCalDAV     = 2,  // CalDAV (iCloud, Google, generic CalDAV)
    EKSourceTypeMobileMe   = 3,  // MobileMe (legacy, rarely seen)
    EKSourceTypeSubscribed = 4,  // Subscribed calendars
    EKSourceTypeBirthdays  = 5   // Birthday calendar source
};
```

### Methods

```objc
// Get calendars for a specific entity type from this source
NSSet<EKCalendar *> *cals = [source calendarsForEntityType:EKEntityTypeEvent];
```

### Source Management (from private headers)

The private headers reveal that EKEventStore supports saving and removing sources:

```objc
// These are in the private headers — may not be public API
- (BOOL)saveSource:(EKSource *)source commit:(BOOL)commit error:(NSError **)error;
- (BOOL)removeSource:(EKSource *)source commit:(BOOL)commit error:(NSError **)error;
```

### Practical Notes

- iCloud shows as `EKSourceTypeCalDAV` (CalDAV protocol), not a separate type
- Google calendars also show as `EKSourceTypeCalDAV`
- You cannot programmatically add a new account/source — users must configure accounts in System Settings
- `isDelegate` is useful for identifying shared/delegated calendars (common in Exchange environments)
- Sources are available as `store.sources` and `store.delegateSources`

---

## 9. EKRecurrenceRule — Full Recurrence Support

EKRecurrenceRule defines how an event or reminder repeats. It roughly corresponds to the iCalendar RRULE specification.

### Frequency Types (EKRecurrenceFrequency)

```objc
typedef NS_ENUM(NSInteger, EKRecurrenceFrequency) {
    EKRecurrenceFrequencyDaily   = 0,
    EKRecurrenceFrequencyWeekly  = 1,
    EKRecurrenceFrequencyMonthly = 2,
    EKRecurrenceFrequencyYearly  = 3
};
```

**Note**: EventKit does NOT support `SECONDLY`, `MINUTELY`, or `HOURLY` frequencies from RFC 5545.

### Constructors

```objc
// Simple recurrence (e.g., every 2 weeks)
EKRecurrenceRule *rule = [[EKRecurrenceRule alloc]
    initRecurrenceWithFrequency:EKRecurrenceFrequencyWeekly
                       interval:2
                            end:nil]; // nil = recurs forever

// Complex recurrence (e.g., every month on the 1st and 15th)
EKRecurrenceRule *rule = [[EKRecurrenceRule alloc]
    initRecurrenceWithFrequency:EKRecurrenceFrequencyMonthly
                       interval:1
                  daysOfTheWeek:nil
                 daysOfTheMonth:@[@1, @15]
                monthsOfTheYear:nil
                 weeksOfTheYear:nil
                  daysOfTheYear:nil
                   setPositions:nil
                            end:recurrenceEnd];
```

### Properties

| Property | Type | Access | Description |
|----------|------|--------|-------------|
| `frequency` | EKRecurrenceFrequency | Read-only | Daily, weekly, monthly, yearly |
| `interval` | NSInteger | Read-only | Interval between occurrences (e.g., 2 = every other) |
| `recurrenceEnd` | EKRecurrenceEnd | Read-write | When recurrence stops (or nil = forever) |
| `firstDayOfTheWeek` | NSInteger | Read-only | 0 = default (locale), 1 = Sunday, 2 = Monday, ... 7 = Saturday |
| `daysOfTheWeek` | NSArray<EKRecurrenceDayOfWeek *> | Read-only | Which days of the week |
| `daysOfTheMonth` | NSArray<NSNumber *> | Read-only | Days of month (-31 to 31, excl. 0) |
| `daysOfTheYear` | NSArray<NSNumber *> | Read-only | Days of year (-366 to 366, excl. 0) |
| `weeksOfTheYear` | NSArray<NSNumber *> | Read-only | Weeks of year (-53 to 53, excl. 0) |
| `monthsOfTheYear` | NSArray<NSNumber *> | Read-only | Months (1-12) |
| `setPositions` | NSArray<NSNumber *> | Read-only | Filters occurrences within a period |
| `calendarIdentifier` | NSString | Read-only | Calendar system identifier |

### EKRecurrenceEnd

```objc
// End after a specific date
EKRecurrenceEnd *end = [EKRecurrenceEnd recurrenceEndWithEndDate:endDate];

// End after N occurrences
EKRecurrenceEnd *end = [EKRecurrenceEnd recurrenceEndWithOccurrenceCount:10];
```

| Property | Type | Description |
|----------|------|-------------|
| `endDate` | NSDate | The end date (nil if count-based) |
| `occurrenceCount` | NSUInteger | Number of occurrences (0 if date-based) |

### EKRecurrenceDayOfWeek

```objc
// Simple: every Monday
EKRecurrenceDayOfWeek *monday = [EKRecurrenceDayOfWeek dayOfWeek:EKWeekdayMonday];

// With week number: second Tuesday of the month
EKRecurrenceDayOfWeek *secondTuesday = [EKRecurrenceDayOfWeek dayOfWeek:EKWeekdayTuesday
                                                             weekNumber:2];

// Negative: last Friday of the month
EKRecurrenceDayOfWeek *lastFriday = [EKRecurrenceDayOfWeek dayOfWeek:EKWeekdayFriday
                                                          weekNumber:-1];
```

| Property | Type | Description |
|----------|------|-------------|
| `dayOfTheWeek` | EKWeekday | Sun(1) through Sat(7) |
| `weekNumber` | NSInteger | 0 = every week; 1-53 = specific week; negative = from end |

### Weekday Constants (EKWeekday)

```objc
EKWeekdaySunday    = 1
EKWeekdayMonday    = 2
EKWeekdayTuesday   = 3
EKWeekdayWednesday = 4
EKWeekdayThursday  = 5
EKWeekdayFriday    = 6
EKWeekdaySaturday  = 7
```

### Common Recurrence Patterns — ObjC Examples

```objc
// Every day
[[EKRecurrenceRule alloc] initRecurrenceWithFrequency:EKRecurrenceFrequencyDaily
                                             interval:1 end:nil];

// Every weekday (Mon-Fri)
EKRecurrenceDayOfWeek *mon = [EKRecurrenceDayOfWeek dayOfWeek:EKWeekdayMonday];
EKRecurrenceDayOfWeek *tue = [EKRecurrenceDayOfWeek dayOfWeek:EKWeekdayTuesday];
EKRecurrenceDayOfWeek *wed = [EKRecurrenceDayOfWeek dayOfWeek:EKWeekdayWednesday];
EKRecurrenceDayOfWeek *thu = [EKRecurrenceDayOfWeek dayOfWeek:EKWeekdayThursday];
EKRecurrenceDayOfWeek *fri = [EKRecurrenceDayOfWeek dayOfWeek:EKWeekdayFriday];
[[EKRecurrenceRule alloc] initRecurrenceWithFrequency:EKRecurrenceFrequencyWeekly
                                             interval:1
                                        daysOfTheWeek:@[mon, tue, wed, thu, fri]
                                       daysOfTheMonth:nil
                                      monthsOfTheYear:nil
                                       weeksOfTheYear:nil
                                        daysOfTheYear:nil
                                         setPositions:nil
                                                  end:nil];

// First Monday of every month
EKRecurrenceDayOfWeek *firstMonday = [EKRecurrenceDayOfWeek dayOfWeek:EKWeekdayMonday
                                                           weekNumber:1];
[[EKRecurrenceRule alloc] initRecurrenceWithFrequency:EKRecurrenceFrequencyMonthly
                                             interval:1
                                        daysOfTheWeek:@[firstMonday]
                                       daysOfTheMonth:nil
                                      monthsOfTheYear:nil
                                       weeksOfTheYear:nil
                                        daysOfTheYear:nil
                                         setPositions:nil
                                                  end:nil];

// Every year on March 15, for 5 occurrences
EKRecurrenceEnd *end = [EKRecurrenceEnd recurrenceEndWithOccurrenceCount:5];
[[EKRecurrenceRule alloc] initRecurrenceWithFrequency:EKRecurrenceFrequencyYearly
                                             interval:1
                                        daysOfTheWeek:nil
                                       daysOfTheMonth:@[@15]
                                      monthsOfTheYear:@[@3]
                                       weeksOfTheYear:nil
                                        daysOfTheYear:nil
                                         setPositions:nil
                                                  end:end];

// Last weekday of every month (using setPositions)
// Set positions filter the set generated by the other parameters
[[EKRecurrenceRule alloc] initRecurrenceWithFrequency:EKRecurrenceFrequencyMonthly
                                             interval:1
                                        daysOfTheWeek:@[mon, tue, wed, thu, fri]
                                       daysOfTheMonth:nil
                                      monthsOfTheYear:nil
                                       weeksOfTheYear:nil
                                        daysOfTheYear:nil
                                         setPositions:@[@(-1)]  // last one
                                                  end:nil];
```

### Limitations

- No `SECONDLY`, `MINUTELY`, or `HOURLY` frequency
- No `BYSETPOS` with daily frequency
- `setPositions` range is -366 to 366 (excluding 0)
- Cannot express all RFC 5545 RRULE patterns — EventKit is a subset
- Third-party libraries like [RWMRecurrenceRule](https://github.com/rmaddy/RWMRecurrenceRule) bridge iCalendar RRULE strings to EKRecurrenceRule

---

## 10. EKStructuredLocation — Geolocation Features

EKStructuredLocation represents a geographic location with optional geofencing capabilities. It is used by both events (as a structured location property) and alarms (for location-based triggers).

### Properties

| Property | Type | Access | Description |
|----------|------|--------|-------------|
| `title` | NSString | Read-write | Location display name |
| `geoLocation` | CLLocation | Read-write | Geographic coordinates (lat/long + altitude, etc.) |
| `radius` | double | Read-write | Geofence radius in meters (0 = use system default) |

### Factory Methods

```objc
// Create from a title string
EKStructuredLocation *loc = [EKStructuredLocation locationWithTitle:@"Apple Park"];

// Create from a MapKit map item
EKStructuredLocation *loc = [EKStructuredLocation locationWithMapItem:mapItem];
```

### Usage with Events

```objc
EKEvent *event = [EKEvent eventWithEventStore:store];
event.title = @"Team Meeting";

// Set structured location with coordinates
EKStructuredLocation *location = [EKStructuredLocation locationWithTitle:@"Apple Park"];
location.geoLocation = [[CLLocation alloc] initWithLatitude:37.3349 longitude:-122.0090];
event.structuredLocation = location;

// The plain .location string is separate from structuredLocation
event.location = @"Apple Park, 1 Apple Park Way, Cupertino, CA";
```

### Usage with Alarms (Location-Based Triggers)

```objc
EKAlarm *alarm = [[EKAlarm alloc] init];

EKStructuredLocation *loc = [EKStructuredLocation locationWithTitle:@"Office"];
loc.geoLocation = [[CLLocation alloc] initWithLatitude:37.3349 longitude:-122.0090];
loc.radius = 200.0; // 200 meter radius

alarm.structuredLocation = loc;
alarm.proximity = EKAlarmProximityEnter; // Trigger when entering the area
// or EKAlarmProximityLeave for when leaving

[event addAlarm:alarm];
```

### Automatic Geocoding

The private EKEventStore header reveals a property `automaticLocationGeocodingAllowed` — suggesting EventKit can automatically geocode location strings to coordinates.

---

## 11. EKAlarm — Alerts & Location-Based Triggers

EKAlarm defines when and how the user is notified about an event or reminder.

### Properties

| Property | Type | Access | Description |
|----------|------|--------|-------------|
| `absoluteDate` | NSDate | Read-write | Fire at this exact date/time |
| `relativeOffset` | NSTimeInterval | Read-write | Seconds relative to event start (negative = before) |
| `structuredLocation` | EKStructuredLocation | Read-write | Location for geofence-based alarms |
| `proximity` | EKAlarmProximity | Read-write | Enter/leave geofence trigger |
| `type` | EKAlarmType | Read-only | Type of alarm (display, audio, etc.) |

### Proximity Values

```objc
typedef NS_ENUM(NSInteger, EKAlarmProximity) {
    EKAlarmProximityNone  = 0,  // Not a location-based alarm
    EKAlarmProximityEnter = 1,  // Trigger when entering the area
    EKAlarmProximityLeave = 2   // Trigger when leaving the area
};
```

### Alarm Types

```objc
typedef NS_ENUM(NSInteger, EKAlarmType) {
    EKAlarmTypeDisplay    = 0,  // Display notification
    EKAlarmTypeAudio      = 1,  // Play sound
    EKAlarmTypeProcedure  = 2,  // Run procedure (deprecated/unused)
    EKAlarmTypeEmail      = 3   // Send email (may not work on all accounts)
};
```

### Factory Methods

```objc
// Alarm at absolute date
EKAlarm *alarm = [EKAlarm alarmWithAbsoluteDate:fireDate];

// Alarm relative to event start (negative = before)
EKAlarm *alarm = [EKAlarm alarmWithRelativeOffset:-900]; // 15 minutes before (-15 * 60)

// Common relative offsets
// -300    = 5 minutes before
// -900    = 15 minutes before
// -1800   = 30 minutes before
// -3600   = 1 hour before
// -86400  = 1 day before
```

### Location-Based Alarm Example

```objc
EKAlarm *alarm = [[EKAlarm alloc] init];

EKStructuredLocation *loc = [EKStructuredLocation locationWithTitle:@"Home"];
loc.geoLocation = [[CLLocation alloc] initWithLatitude:37.33 longitude:-122.01];
loc.radius = 100.0;

alarm.structuredLocation = loc;
alarm.proximity = EKAlarmProximityLeave; // Alert when leaving home

[reminder addAlarm:alarm]; // Commonly used with reminders
```

**Note**: Setting `absoluteDate` clears `relativeOffset` and vice versa. Setting `structuredLocation` and `proximity` creates a location-based alarm independent of time-based triggers.

---

## 12. EKParticipant — Attendee Details

EKParticipant represents an attendee or organizer of a calendar event. This is a **read-only** class — you cannot create, modify, or add participants programmatically.

### Properties

| Property | Type | Access | Description |
|----------|------|--------|-------------|
| `name` | NSString | Read-only | Display name |
| `URL` | NSURL | Read-only | URL representing this participant (typically mailto:) |
| `participantStatus` | EKParticipantStatus | Read-only | Response status |
| `participantRole` | EKParticipantRole | Read-only | Role in the event |
| `participantType` | EKParticipantType | Read-only | Type of participant |
| `isCurrentUser` | BOOL | Read-only | Whether this is the current device owner |

### Participant Status (EKParticipantStatus)

```objc
typedef NS_ENUM(NSInteger, EKParticipantStatus) {
    EKParticipantStatusUnknown    = 0,
    EKParticipantStatusPending    = 1,  // Not yet responded
    EKParticipantStatusAccepted   = 2,
    EKParticipantStatusDeclined   = 3,
    EKParticipantStatusTentative  = 4,
    EKParticipantStatusDelegated  = 5,
    EKParticipantStatusCompleted  = 6,
    EKParticipantStatusInProcess  = 7
};
```

### Participant Role (EKParticipantRole)

```objc
typedef NS_ENUM(NSInteger, EKParticipantRole) {
    EKParticipantRoleUnknown       = 0,
    EKParticipantRoleRequired      = 1,  // Required attendee
    EKParticipantRoleOptional      = 2,  // Optional attendee
    EKParticipantRoleChair         = 3,  // Meeting chair
    EKParticipantRoleNonParticipant = 4  // Non-participant (e.g., FYI)
};
```

### Participant Type (EKParticipantType)

```objc
typedef NS_ENUM(NSInteger, EKParticipantType) {
    EKParticipantTypeUnknown  = 0,
    EKParticipantTypePerson   = 1,  // Individual
    EKParticipantTypeRoom     = 2,  // Conference room
    EKParticipantTypeResource = 3,  // Equipment/resource
    EKParticipantTypeGroup    = 4   // Group/distribution list
};
```

### Methods

```objc
// Get the corresponding AddressBook person (deprecated — use Contacts framework)
- (ABPerson *)ABPersonInAddressBook:(ABAddressBook *)addressBook;

// Modern alternative: use contactPredicate
@property (nonatomic, readonly) NSPredicate *contactPredicate; // iOS 9+ / macOS 10.11+
```

### Critical Limitation

**You CANNOT add attendees to events via EventKit.** This is a long-standing Apple restriction (radar://15504551, filed circa 2013, still unfixed as of 2026). The `attendees` property on EKCalendarItem is read-only. The `organizer` property on EKEvent is also read-only.

To create events with attendees, the only options are:
1. Use EventKitUI's `EKEventEditViewController` (iOS/Mac Catalyst only)
2. Create the event on the server side via CalDAV/Exchange protocol directly
3. Use Siri Event Suggestions (limited, indirect)

---

## 13. EKVirtualConferenceProvider — Virtual Meetings

Added in macOS 12 / iOS 15, this is an extension point for video conferencing apps (Zoom, Teams, etc.) to integrate with Calendar.

### Architecture

Your app ships an **App Extension** that subclasses `EKVirtualConferenceProvider`. Calendar.app discovers it and offers your conference rooms in the location picker.

### Key Classes

```objc
// The provider you subclass
@interface EKVirtualConferenceProvider : NSObject

// Override to return available room types
- (void)fetchAvailableRoomTypesWithCompletionHandler:
    (void(^)(NSArray<EKVirtualConferenceRoomTypeDescriptor *> *roomTypes, NSError *error))completionHandler;

// Override to create a conference for a selected room type
- (void)fetchVirtualConferenceForIdentifier:(EKVirtualConferenceRoomTypeIdentifier)identifier
                          completionHandler:
    (void(^)(EKVirtualConferenceDescriptor *descriptor, NSError *error))completionHandler;
@end

// Room type descriptor
@interface EKVirtualConferenceRoomTypeDescriptor : NSObject
- (instancetype)initWithTitle:(NSString *)title
                   identifier:(EKVirtualConferenceRoomTypeIdentifier)identifier;
@end

// Conference descriptor (returned when a room type is selected)
@interface EKVirtualConferenceDescriptor : NSObject
- (instancetype)initWithTitle:(NSString *)title
              urlDescriptors:(NSArray<EKVirtualConferenceURLDescriptor *> *)urlDescriptors
           conferenceDetails:(NSString *)conferenceDetails;
@end

// URL descriptor for joining
@interface EKVirtualConferenceURLDescriptor : NSObject
- (instancetype)initWithTitle:(NSString *)title URL:(NSURL *)URL;
@end
```

### Relevance for go-eventkit

This is an **App Extension** API, not a library API. It is not relevant for go-eventkit's bridge pattern. However, events created via virtual conference providers will have their `URL` and `structuredLocation` populated, which go-eventkit would read.

---

## 14. Permission Model & TCC

### Evolution of Calendar Permissions

| OS Version | Permission Model |
|------------|-----------------|
| macOS < 13 | Single prompt via `requestAccessToEntityType:` |
| macOS 13 (Ventura) | Sandbox changes; non-sandboxed apps may lose access |
| macOS 14 (Sonoma) | Three-tier model: None / Write-Only / Full Access |
| macOS 15 (Sequoia) | Same three-tier model; no significant changes to EventKit permissions |

### macOS 14+ Permission APIs

```objc
// Check authorization status
EKAuthorizationStatus status = [EKEventStore authorizationStatusForEntityType:EKEntityTypeEvent];

// Possible values:
// EKAuthorizationStatusNotDetermined  — never asked
// EKAuthorizationStatusRestricted     — system restriction (MDM, parental controls)
// EKAuthorizationStatusDenied         — user denied
// EKAuthorizationStatusFullAccess     — full read+write (new in macOS 14)
// EKAuthorizationStatusWriteOnly      — write-only (new in macOS 14)
// EKAuthorizationStatusAuthorized     — deprecated; equivalent to FullAccess

// Request full access (read + write)
[store requestFullAccessToEventsWithCompletion:^(BOOL granted, NSError *error) { ... }];
[store requestFullAccessToRemindersWithCompletion:^(BOOL granted, NSError *error) { ... }];

// Request write-only access (new in macOS 14)
[store requestWriteOnlyAccessToEventsWithCompletion:^(BOOL granted, NSError *error) { ... }];
// Note: There is NO requestWriteOnlyAccessToReminders — reminders always need full access
```

### Info.plist Keys

```xml
<!-- macOS 14+ -->
<key>NSCalendarsFullAccessUsageDescription</key>
<string>This app needs to read and write your calendar events.</string>

<key>NSCalendarsWriteOnlyAccessUsageDescription</key>
<string>This app needs to create calendar events.</string>

<key>NSRemindersFullAccessUsageDescription</key>
<string>This app needs to read and manage your reminders.</string>

<!-- Legacy (pre-macOS 14, still needed for backward compat) -->
<key>NSCalendarsUsageDescription</key>
<string>This app needs calendar access.</string>

<key>NSRemindersUsageDescription</key>
<string>This app needs reminders access.</string>
```

### Write-Only Access Limitations

When granted write-only access:
- Can create new events via `saveEvent:span:commit:`
- Can access `defaultCalendarForNewEvents` (to set event's calendar)
- **Cannot** read existing events (including those the app created)
- **Cannot** read the calendar list
- **Cannot** create new calendars
- **Cannot** read/modify/delete existing events

### Backward Compatible Permission Request (ObjC)

```objc
- (void)requestCalendarAccessWithCompletion:(void(^)(BOOL granted))completion {
    EKEventStore *store = [self sharedStore];

    if (@available(macOS 14.0, *)) {
        [store requestFullAccessToEventsWithCompletion:^(BOOL granted, NSError *error) {
            completion(granted);
        }];
    } else {
        [store requestAccessToEntityType:EKEntityTypeEvent
                              completion:^(BOOL granted, NSError *error) {
            completion(granted);
        }];
    }
}
```

### TCC for CLI Tools

- CLI tools (like go-eventkit consumers) trigger a TCC prompt on first access
- The prompt shows the terminal app name (Terminal.app, iTerm2, etc.) — not the CLI tool itself
- Once granted, the permission applies to all CLI tools run from that terminal
- Permission can be revoked in System Settings > Privacy & Security > Calendars/Reminders
- Hardened runtime or notarization is NOT required for TCC prompts in development

---

## 15. Change Tracking & Notifications

### EKEventStoreChangedNotification

The primary mechanism for detecting external changes to calendar data.

```objc
// Register for changes
[[NSNotificationCenter defaultCenter] addObserver:self
                                         selector:@selector(storeChanged:)
                                             name:EKEventStoreChangedNotification
                                           object:store];

- (void)storeChanged:(NSNotification *)notification {
    // IMPORTANT: This notification has NO payload about what changed.
    // You must re-fetch all relevant data.
    // All previously fetched EKEvent/EKReminder instances should be
    // considered INVALID after receiving this notification.

    // Refresh individual objects:
    BOOL stillValid = [event refresh];
    if (!stillValid) {
        // Event was deleted or significantly changed — discard and re-fetch
    }

    // Or re-fetch everything
    NSArray *events = [store eventsMatchingPredicate:predicate];
}

// Deregister when done
[[NSNotificationCenter defaultCenter] removeObserver:self
                                                name:EKEventStoreChangedNotification
                                              object:store];
```

### Key Behaviors

1. **No change details** — The notification does not tell you what changed. A complete reload is the only reliable approach.
2. **All objects invalidated** — After receiving the notification, all previously fetched EKEvent/EKReminder objects should be considered stale.
3. **Refresh individual objects** — Call `[event refresh]` which returns `YES` if the object is still valid, `NO` if it was deleted or is otherwise invalid.
4. **Posted on the thread that made the change** — If another process changes the calendar, the notification may arrive on any thread.
5. **Coalesced** — Multiple rapid changes may result in a single notification.
6. **Cross-process** — Changes from Calendar.app, other apps, or sync operations all trigger this notification.

### EKObject Refresh Pattern

```objc
// Check if a specific object is still valid
if ([calendarItem refresh]) {
    // Object is still valid, properties have been updated
    NSLog(@"Updated title: %@", calendarItem.title);
} else {
    // Object was deleted or is invalid
    // Must re-fetch if needed
}
```

### Change Listener (from private headers)

The private headers show `EKChangeListener` as a property on EKEventStore, and a `onlyNotifyForAccountedChanges` property — suggesting internal fine-grained change tracking exists but is not public.

### For go-eventkit

In a cgo context, change notifications require running an `NSRunLoop` or `CFRunLoop` to receive `NSNotification` dispatches. This is complex but possible. The simplest approach for a library is to expose a polling-based `Refresh()` method rather than real-time notifications.

---

## 16. Batch Operations & Performance

### Commit Batching

The `commit:` parameter on save/remove methods controls batching:

```objc
// Individual commits (simple but slower for bulk operations)
[store saveEvent:event1 span:EKSpanThisEvent commit:YES error:&error];
[store saveEvent:event2 span:EKSpanThisEvent commit:YES error:&error];

// Batched commits (faster for bulk operations)
[store saveEvent:event1 span:EKSpanThisEvent commit:NO error:&error];
[store saveEvent:event2 span:EKSpanThisEvent commit:NO error:&error];
[store saveEvent:event3 span:EKSpanThisEvent commit:NO error:&error];
// Single commit for all changes
BOOL success = [store commit:&error];

// If something goes wrong before commit, you can roll back
[store reset]; // Discards all uncommitted changes
```

### Performance Patterns

1. **Predicate-based fetching** — Always use predicates with date ranges. Never fetch all events unbounded.
2. **Calendar filtering** — Pass specific calendars to predicates to reduce the search space.
3. **Prefetch hints** — Private API shows `prefetchHint` parameters on predicates for optimization.
4. **Background fetch queue** — EKEventStore has a `backgroundFetchQueue` property (private) for async operations.
5. **Copy for background** — `[store copyForBackgroundUpdate]` creates a store copy safe for background threads.

### Editing Contexts (from private headers)

```objc
// Open an editing context (transactional edit)
id context = [store openEditingContextWithObject:event];
// or for multiple objects:
id context = [store openEditingContextWithObjects:@[event1, event2]];

// Make changes to the objects...

// Commit changes from the editing context
[store closeEditingContextAndCommitChanges:context];

// Or discard changes
[store closeEditingContextWithoutCommittingChanges:context];

// Or commit without closing (continue editing)
[store commitChangesFromEditingContextWithoutClosing:context];
```

These editing context APIs appear to be private/internal but suggest Apple supports transactional editing patterns.

### Performance Guidelines

| Guideline | Rationale |
|-----------|-----------|
| One EKEventStore per app | Heavyweight; slow to create/destroy |
| Batch with `commit:NO` + `commit:` | Reduces database writes for bulk operations |
| Use date-range predicates | Unbounded queries are slow and memory-intensive |
| Filter by calendar when possible | Reduces search space |
| Refresh objects before re-use | Cached objects may be stale |
| Use `enumerateEventsMatchingPredicate:usingBlock:` for large result sets | More memory-efficient than `eventsMatchingPredicate:` |

---

## 17. Threading & Best Practices

### Thread Safety

1. **EKEventStore** is NOT thread-safe — all operations on a single store should be serialized.
2. **Completion handlers** for async methods may be called on arbitrary threads (not necessarily the main thread).
3. **For background work**, use `[store copyForBackgroundUpdate]` to create a thread-safe copy.
4. **Notification callbacks** may arrive on any thread.

### Singleton Pattern (recommended for cgo)

```objc
// dispatch_once ensures thread-safe singleton initialization
static EKEventStore *sharedStore = nil;
static dispatch_once_t onceToken;

EKEventStore* GetSharedEventStore(void) {
    dispatch_once(&onceToken, ^{
        sharedStore = [[EKEventStore alloc] init];
    });
    return sharedStore;
}
```

### Synchronous Wrappers for Async APIs

```objc
// Wrap async reminder fetch with dispatch_semaphore for synchronous Go callers
char* FetchReminders(const char* calendarID) {
    EKEventStore *store = GetSharedEventStore();
    dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);

    __block NSArray<EKReminder *> *results = nil;

    NSPredicate *predicate = [store predicateForRemindersInCalendars:nil];
    [store fetchRemindersMatchingPredicate:predicate completion:^(NSArray *reminders) {
        results = reminders;
        dispatch_semaphore_signal(semaphore);
    }];

    dispatch_semaphore_wait(semaphore, DISPATCH_TIME_FOREVER);

    // Serialize to JSON and return...
}
```

### ARC Requirements

```objc
// CRITICAL: ARC must be enabled
// #cgo CFLAGS: -x objective-c -fobjc-arc

// Without ARC, objects in __block variables and completion handlers
// can be released prematurely, causing:
// - Empty result sets
// - SIGSEGV crashes
// - Intermittent data corruption
```

### Date Handling

```objc
// Use NSCalendar and NSDateComponents for date math
// NEVER use raw NSTimeInterval arithmetic — DST transitions cause bugs

NSCalendar *calendar = [NSCalendar currentCalendar];
NSDateComponents *components = [[NSDateComponents alloc] init];
components.hour = 2;
NSDate *twoHoursLater = [calendar dateByAddingComponents:components toDate:startDate options:0];
```

---

## 18. macOS 13 Ventura Changes

### Key Changes (TN3130)

1. **Sandboxing impact** — Non-sandboxed apps may need additional entitlements for full EventKit access.
2. **TCC enforcement tightened** — Calendar/Reminder access requires explicit user approval even for non-sandboxed apps.
3. **No new EventKit APIs** — The framework API itself was unchanged; the changes were primarily about access control enforcement.

---

## 19. macOS 14 Sonoma Changes (2023)

### Major Changes (TN3153, WWDC23)

This was the **biggest EventKit API change in years**, introducing the three-tier permission model.

#### New Permission APIs

```objc
// NEW: Granular access requests
- (void)requestFullAccessToEventsWithCompletion:(EKEventStoreRequestAccessCompletionHandler)completion;
- (void)requestWriteOnlyAccessToEventsWithCompletion:(EKEventStoreRequestAccessCompletionHandler)completion;
- (void)requestFullAccessToRemindersWithCompletion:(EKEventStoreRequestAccessCompletionHandler)completion;

// NEW: Authorization status values
EKAuthorizationStatusFullAccess  // Replaces EKAuthorizationStatusAuthorized
EKAuthorizationStatusWriteOnly   // NEW — write but cannot read
```

#### Deprecated APIs

```objc
// DEPRECATED in macOS 14:
- (void)requestAccessToEntityType:(EKEntityType)entityType
                       completion:(EKEventStoreRequestAccessCompletionHandler)completion;

// DEPRECATED status value:
EKAuthorizationStatusAuthorized  // Use EKAuthorizationStatusFullAccess instead
```

#### New Info.plist Keys

- `NSCalendarsFullAccessUsageDescription`
- `NSCalendarsWriteOnlyAccessUsageDescription`
- `NSRemindersFullAccessUsageDescription`

#### EventKitUI Changes (iOS 17, relevant background)

- EventKitUI view controllers now run in a **separate process** (out-of-process)
- This means EventKitUI can present calendar UI **without any permission prompt**
- The user picks/edits in the system process; only the result is returned to the app
- macOS desktop apps don't have EventKitUI, but this informs the direction of the framework

#### Virtual Conference Enhancements

- `EKVirtualConferenceProvider` gained better support for Universal Links in URL descriptors

---

## 20. macOS 15 Sequoia Changes (2024)

Based on available release notes and documentation:

### EventKit Changes

- **No major new EventKit APIs** were introduced in macOS 15 Sequoia specifically
- The three-tier permission model from macOS 14 continues unchanged
- Bug fixes and stability improvements to EventKit sync
- Some reports of EventKit-related issues in early macOS 15 betas that were resolved in subsequent updates

### Broader Context

- macOS 15 focused more on Apple Intelligence, iPhone Mirroring, and Safari improvements
- Calendar.app received UI updates but the underlying EventKit framework API was stable
- The framework version number incremented but the public API surface remained essentially the same

### What to Watch For

- Apple may introduce further granularity in permissions in future releases
- The trend is toward less access for apps (write-only as default, full access requiring justification)
- EventKitUI's out-of-process model from iOS may eventually come to macOS

---

## 21. Known Limitations

### Hard Limitations (Apple-imposed, cannot be worked around)

| Limitation | Details |
|------------|---------|
| **Attendees are read-only** | Cannot add, remove, or modify attendees/participants via EventKit. `attendees` property on EKCalendarItem is read-only. No `addAttendee:` method exists. Apple radar filed in 2013, still unfixed. |
| **Organizer is read-only** | `organizer` property on EKEvent is read-only. Cannot set the organizer of an event. |
| **No hourly/minutely/secondly recurrence** | EKRecurrenceFrequency only supports daily, weekly, monthly, yearly. |
| **Cannot create accounts/sources** | EKSource is read-only from the API perspective. Users must configure accounts in System Settings. |
| **Birthday calendar is read-only** | Birthday events come from Contacts and cannot be modified via EventKit. |
| **Subscribed calendars are read-only** | .ics subscription calendars cannot be modified. |
| **No event search by content** | Public API only supports date-range predicates for events. There is no text search predicate (private API has `predicateForEventsWithTitle:location:notes:` but it is not public). |
| **Reminders require async fetch** | Unlike events, reminders cannot be fetched synchronously. Must use `fetchRemindersMatchingPredicate:completion:`. |
| **No partial/incremental sync** | `EKEventStoreChangedNotification` does not indicate what changed. Full re-fetch is required. |
| **Flagged property not in EventKit** | Reminders' "flagged" state (shown in Apple Reminders app) is NOT available via EventKit. |
| **No rich text in notes** | Notes field is plain text only. |
| **No attachment support** | EventKit does not support file attachments on events or reminders. |
| **Travel time not exposed** | Calendar.app's travel time feature is not accessible via EventKit. |

### Soft Limitations (can be worked around)

| Limitation | Workaround |
|------------|-----------|
| Write-only cannot read own events | Request full access instead of write-only |
| CLI tools show terminal app in TCC | Expected behavior; document for users |
| No notification delivery in library | Use polling with `refresh` method |
| Date-range required for event queries | Use sensible defaults (e.g., past year to next year) |
| No RFC 5545 RRULE string support | Parse RRULE and construct EKRecurrenceRule manually |
| All-day event timezone handling | Use the event's `timeZone` property and Foundation's Calendar for date math |

### Data Quirks

- `eventIdentifier` is stable across recurrence edits; `calendarItemIdentifier` is NOT
- `calendarItemExternalIdentifier` may be nil for local-only events
- `occurrenceDate` is the original date for a recurring event occurrence, even if it has been moved
- `isDetached` is YES when a single occurrence of a recurring event has been modified independently
- Some Exchange servers return different data than CalDAV for the same logical properties
- Google Calendar via CalDAV may have sync delays compared to native Calendar.app

---

## 22. Implications for go-eventkit

### Phase 1 (Calendar) — Enhancement Opportunities

Based on this research, the current PRD covers the core well. Additional capabilities to consider:

1. **Recurrence Rules** — Full `EKRecurrenceRule` support would be valuable. Map to a Go `RecurrenceRule` struct with frequency, interval, days-of-week, end condition.

2. **Structured Location** — Expose `EKStructuredLocation` with lat/long/radius alongside the plain string `Location`. Useful for map integrations.

3. **Calendar CRUD** — `CreateCalendar`, `DeleteCalendar` operations on the Client. Simple to implement.

4. **Source listing** — `Sources()` method to list all accounts/sources. Helps users understand their calendar landscape.

5. **Change notification** — Even a simple `HasChanges()` polling method would be useful for long-running consumers.

6. **Batch operations** — Expose `commit:NO` pattern as a `Batch` method for bulk writes.

### Phase 2 (Reminders) — Key Differences from Calendar

1. Async fetch is mandatory — need `dispatch_semaphore` pattern (already proven in `rem`)
2. `NSDateComponents` for dates instead of `NSDate`
3. Priority is 0-9 integer (not an enum)
4. Completion state is bidirectionally linked with completion date
5. No `reminderIdentifier` — must use `calendarItemIdentifier`

### Permission Strategy

For go-eventkit as a library:
- Default to requesting **full access** (the library is designed for full CRUD)
- Support macOS 14+ `requestFullAccessToEventsWithCompletion:` with fallback to `requestAccessToEntityType:`
- Return specific `ErrWriteOnly` or `ErrAccessDenied` sentinel errors
- Document that CLI consumers will see the terminal app name in TCC prompts

### Threading Strategy

- `dispatch_once` singleton for EKEventStore (proven in `rem`)
- All EventKit operations serialized through the singleton
- `dispatch_semaphore` for wrapping async APIs (reminders)
- No need for background fetch queues — synchronous is fine for CLI tools
- ARC is non-negotiable

---

## Sources

- [EventKit Framework - Apple Developer Documentation](https://developer.apple.com/documentation/eventkit)
- [TN3153: Adopting API changes for EventKit in iOS 17, macOS 14, and watchOS 10](https://developer.apple.com/documentation/technotes/tn3153-adopting-api-changes-for-eventkit-in-ios-macos-and-watchos)
- [TN3130: Changes to EventKit in macOS Ventura 13](https://developer.apple.com/documentation/technotes/tn3130-changes-to-eventkit-in-macos13-ventura)
- [Discover Calendar and EventKit - WWDC23](https://developer.apple.com/videos/play/wwdc2023/10052/)
- [EKEventStore - Apple Developer Documentation](https://developer.apple.com/documentation/eventkit/ekeventstore)
- [EKRecurrenceRule - Apple Developer Documentation](https://developer.apple.com/documentation/eventkit/ekrecurrencerule)
- [EKParticipant - Apple Developer Documentation](https://developer.apple.com/documentation/eventkit/ekparticipant)
- [Updating with notifications - Apple Developer Documentation](https://developer.apple.com/documentation/eventkit/updating-with-notifications)
- [EKEventStore.h private headers (macOS)](https://github.com/w0lfschild/macOS_headers/blob/master/macOS/Frameworks/EventKit/727.1/EKEventStore.h)
- [EKParticipant.h headers](https://github.com/phracker/MacOSX-SDKs/blob/master/MacOSX10.9.sdk/System/Library/Frameworks/EventKit.framework/Versions/A/Headers/EKParticipant.h)
- [EKRecurrenceRule - Microsoft Learn (Xamarin reference)](https://learn.microsoft.com/en-us/dotnet/api/eventkit.ekrecurrencerule?view=xamarin-ios-sdk-12)
- [EKStructuredLocation - Microsoft Learn (Xamarin reference)](https://learn.microsoft.com/en-us/dotnet/api/eventkit.ekstructuredlocation?view=xamarin-ios-sdk-12)
- [EKCalendarItem - Microsoft Learn](https://learn.microsoft.com/en-us/dotnet/api/eventkit.ekcalendaritem?view=net-ios-26.0-10.0)
- [EKSource - Microsoft Learn (Xamarin reference)](https://learn.microsoft.com/en-us/dotnet/api/eventkit.eksource?view=xamarin-ios-sdk-12)
- [How to monitor system calendar for changes with EventKit](https://nemecek.be/blog/63/how-to-monitor-system-calendar-for-changes-with-eventkit)
- [Radar: Should be possible to add attendees to events with EventKit](https://openradar.appspot.com/15504551)
- [RWMRecurrenceRule - iCalendar RRULE support for EventKit](https://github.com/rmaddy/RWMRecurrenceRule)
- [Accessing Calendar using EventKit and EventKitUI](https://developer.apple.com/documentation/EventKit/accessing-calendar-using-eventkit-and-eventkitui)
- [macOS Sequoia 15 Release Notes](https://developer.apple.com/documentation/macos-release-notes/macos-15-release-notes)
