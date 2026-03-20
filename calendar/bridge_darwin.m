#import <EventKit/EventKit.h>
#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#import <CoreLocation/CoreLocation.h>
#include "bridge_darwin.h"
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>

void ek_cal_free(char* ptr) {
    if (ptr) free(ptr);
}

// --- Shared EKEventStore singleton ---

static EKEventStore* get_store(void) {
    static EKEventStore* store = nil;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        store = [[EKEventStore alloc] init];
    });
    return store;
}

// --- Serial dispatch queue for write serialization ---

static dispatch_queue_t get_write_queue(void) {
    static dispatch_queue_t q;
    static dispatch_once_t token;
    dispatch_once(&token, ^{
        q = dispatch_queue_create("dev.sidv.eventkit.cal.writes", DISPATCH_QUEUE_SERIAL);
    });
    return q;
}

// --- Change notifications (self-pipe) ---

static int ek_watch_pipe[2] = {-1, -1};
static id ek_store_observer = nil;

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

int ek_cal_watch_read_fd(void) { return ek_watch_pipe[0]; }

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

// --- Date formatting ---

// ISO 8601 date formatter with fractional seconds, always UTC.
static NSDateFormatter* get_iso_formatter(void) {
    static NSDateFormatter* fmt = nil;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        fmt = [[NSDateFormatter alloc] init];
        [fmt setDateFormat:@"yyyy-MM-dd'T'HH:mm:ss.SSS'Z'"];
        [fmt setTimeZone:[NSTimeZone timeZoneWithName:@"UTC"]];
        [fmt setLocale:[[NSLocale alloc] initWithLocaleIdentifier:@"en_US_POSIX"]];
    });
    return fmt;
}

// Parse an ISO 8601 date string. Handles both fractional and non-fractional seconds.
static NSDate* parse_iso_date(const char* str) {
    if (!str) return nil;
    NSString* s = [NSString stringWithUTF8String:str];
    // Try our formatter first (with fractional seconds)
    NSDate* d = [get_iso_formatter() dateFromString:s];
    if (d) return d;
    // Fallback: standard ISO 8601 parser
    NSISO8601DateFormatter* iso = [[NSISO8601DateFormatter alloc] init];
    d = [iso dateFromString:s];
    if (d) return d;
    // Fallback: try without fractional seconds
    NSDateFormatter* noFrac = [[NSDateFormatter alloc] init];
    [noFrac setDateFormat:@"yyyy-MM-dd'T'HH:mm:ss'Z'"];
    [noFrac setTimeZone:[NSTimeZone timeZoneWithName:@"UTC"]];
    [noFrac setLocale:[[NSLocale alloc] initWithLocaleIdentifier:@"en_US_POSIX"]];
    return [noFrac dateFromString:s];
}

static NSString* format_date(NSDate* date) {
    if (!date) return nil;
    return [get_iso_formatter() stringFromDate:date];
}

// --- JSON serialization ---

static char* to_json(id obj) {
    NSError* error = nil;
    NSData* data = [NSJSONSerialization dataWithJSONObject:obj options:0 error:&error];
    if (!data) return NULL;
    NSString* str = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
    return strdup([str UTF8String]);
}

// --- Event to dictionary conversion ---

static NSDictionary* event_to_dict(EKEvent* e) {
    NSMutableDictionary* d = [NSMutableDictionary dictionary];

    // Use eventIdentifier — stable across recurrence edits.
    d[@"id"] = e.eventIdentifier ?: @"";
    d[@"title"] = e.title ?: @"";
    d[@"allDay"] = e.allDay ? @YES : @NO;
    d[@"location"] = e.location ?: [NSNull null];
    d[@"notes"] = e.notes ?: [NSNull null];
    d[@"url"] = e.URL ? [e.URL absoluteString] : [NSNull null];
    d[@"calendar"] = e.calendar.title ?: @"";
    d[@"calendarID"] = e.calendar.calendarIdentifier ?: @"";
    d[@"status"] = @(e.status);
    d[@"availability"] = @(e.availability);
    d[@"recurring"] = e.hasRecurrenceRules ? @YES : @NO;
    d[@"isDetached"] = e.isDetached ? @YES : @NO;

    // Occurrence date (for recurring events).
    if (e.occurrenceDate) {
        d[@"occurrenceDate"] = format_date(e.occurrenceDate);
    }

    // Recurrence rules.
    if (e.recurrenceRules && e.recurrenceRules.count > 0) {
        NSMutableArray* rules = [NSMutableArray array];
        for (EKRecurrenceRule* rule in e.recurrenceRules) {
            NSMutableDictionary* rd = [NSMutableDictionary dictionary];
            rd[@"frequency"] = @(rule.frequency);
            rd[@"interval"] = @(rule.interval);

            // Days of the week.
            if (rule.daysOfTheWeek && rule.daysOfTheWeek.count > 0) {
                NSMutableArray* days = [NSMutableArray array];
                for (EKRecurrenceDayOfWeek* dow in rule.daysOfTheWeek) {
                    [days addObject:@{
                        @"dayOfTheWeek": @(dow.dayOfTheWeek),
                        @"weekNumber": @(dow.weekNumber)
                    }];
                }
                rd[@"daysOfTheWeek"] = days;
            }

            // Days of the month.
            if (rule.daysOfTheMonth && rule.daysOfTheMonth.count > 0) {
                rd[@"daysOfTheMonth"] = rule.daysOfTheMonth;
            }

            // Months of the year.
            if (rule.monthsOfTheYear && rule.monthsOfTheYear.count > 0) {
                rd[@"monthsOfTheYear"] = rule.monthsOfTheYear;
            }

            // Weeks of the year.
            if (rule.weeksOfTheYear && rule.weeksOfTheYear.count > 0) {
                rd[@"weeksOfTheYear"] = rule.weeksOfTheYear;
            }

            // Days of the year.
            if (rule.daysOfTheYear && rule.daysOfTheYear.count > 0) {
                rd[@"daysOfTheYear"] = rule.daysOfTheYear;
            }

            // Set positions.
            if (rule.setPositions && rule.setPositions.count > 0) {
                rd[@"setPositions"] = rule.setPositions;
            }

            // Recurrence end.
            if (rule.recurrenceEnd) {
                NSMutableDictionary* endDict = [NSMutableDictionary dictionary];
                if (rule.recurrenceEnd.endDate) {
                    endDict[@"endDate"] = format_date(rule.recurrenceEnd.endDate);
                }
                if (rule.recurrenceEnd.occurrenceCount > 0) {
                    endDict[@"occurrenceCount"] = @(rule.recurrenceEnd.occurrenceCount);
                }
                rd[@"end"] = endDict;
            }

            [rules addObject:rd];
        }
        d[@"recurrenceRules"] = rules;
    } else {
        d[@"recurrenceRules"] = @[];
    }

    // Structured location.
    if (e.structuredLocation) {
        EKStructuredLocation* loc = e.structuredLocation;
        NSMutableDictionary* locDict = [NSMutableDictionary dictionary];
        locDict[@"title"] = loc.title ?: @"";
        if (loc.geoLocation) {
            locDict[@"latitude"] = @(loc.geoLocation.coordinate.latitude);
            locDict[@"longitude"] = @(loc.geoLocation.coordinate.longitude);
        }
        if (loc.radius > 0) {
            locDict[@"radius"] = @(loc.radius);
        }
        d[@"structuredLocation"] = locDict;
    }

    // Dates — always format as UTC ISO 8601.
    if (e.startDate) {
        d[@"startDate"] = format_date(e.startDate);
    }
    if (e.endDate) {
        d[@"endDate"] = format_date(e.endDate);
    }
    if (e.creationDate) {
        d[@"createdAt"] = format_date(e.creationDate);
    }
    if (e.lastModifiedDate) {
        d[@"modifiedAt"] = format_date(e.lastModifiedDate);
    }

    // Timezone — capture the event's timezone if set.
    if (e.timeZone) {
        d[@"timeZone"] = e.timeZone.name;
    } else {
        d[@"timeZone"] = [NSNull null];
    }

    // Organizer (read-only).
    if (e.organizer) {
        d[@"organizer"] = e.organizer.name ?: @"";
    } else {
        d[@"organizer"] = [NSNull null];
    }

    // Attendees (read-only).
    if (e.attendees && e.attendees.count > 0) {
        NSMutableArray* attendees = [NSMutableArray array];
        for (EKParticipant* p in e.attendees) {
            NSMutableDictionary* att = [NSMutableDictionary dictionary];
            att[@"name"] = p.name ?: @"";
            // Email: try URL first (mailto:), then name as fallback.
            if (p.URL) {
                NSString* email = [p.URL absoluteString];
                if ([email hasPrefix:@"mailto:"]) {
                    email = [email substringFromIndex:7];
                }
                att[@"email"] = email;
            } else {
                att[@"email"] = @"";
            }
            att[@"status"] = @(p.participantStatus);
            [attendees addObject:att];
        }
        d[@"attendees"] = attendees;
    } else {
        d[@"attendees"] = @[];
    }

    // Alerts.
    if (e.alarms && e.alarms.count > 0) {
        NSMutableArray* alerts = [NSMutableArray array];
        for (EKAlarm* alarm in e.alarms) {
            [alerts addObject:@{
                @"relativeOffset": @(alarm.relativeOffset)
            }];
        }
        d[@"alerts"] = alerts;
    } else {
        d[@"alerts"] = @[];
    }

    return d;
}

// --- Calendar to dictionary conversion ---

static NSDictionary* calendar_to_dict(EKCalendar* cal) {
    NSMutableDictionary* d = [NSMutableDictionary dictionary];

    d[@"id"] = cal.calendarIdentifier ?: @"";
    d[@"title"] = cal.title ?: @"";
    d[@"type"] = @(cal.type);
    d[@"source"] = cal.source.title ?: @"";
    d[@"readOnly"] = cal.allowsContentModifications ? @NO : @YES;

    // Color as hex string.
    if (cal.color) {
        NSColor* srgb = [cal.color colorUsingColorSpace:[NSColorSpace sRGBColorSpace]];
        if (srgb) {
            CGFloat r, g, b, a;
            [srgb getRed:&r green:&g blue:&b alpha:&a];
            d[@"color"] = [NSString stringWithFormat:@"#%02X%02X%02X",
                (int)(r * 255), (int)(g * 255), (int)(b * 255)];
        } else {
            d[@"color"] = @"";
        }
    } else {
        d[@"color"] = @"";
    }

    return d;
}

// --- Find calendar by name (case-insensitive) ---

static EKCalendar* find_calendar_by_name(EKEventStore* store, NSString* name) {
    NSString* lowerName = [name lowercaseString];
    for (EKCalendar* cal in [store calendarsForEntityType:EKEntityTypeEvent]) {
        if ([[cal.title lowercaseString] isEqualToString:lowerName]) {
            return cal;
        }
    }
    return nil;
}

// --- Find calendar by ID ---

static EKCalendar* find_calendar_by_id(EKEventStore* store, NSString* calId) {
    for (EKCalendar* cal in [store calendarsForEntityType:EKEntityTypeEvent]) {
        if ([cal.calendarIdentifier isEqualToString:calId]) {
            return cal;
        }
    }
    return nil;
}

// --- Available calendar names (for error messages) ---

static NSString* available_calendar_names(EKEventStore* store) {
    NSMutableArray* names = [NSMutableArray array];
    for (EKCalendar* cal in [store calendarsForEntityType:EKEntityTypeEvent]) {
        [names addObject:cal.title];
    }
    return [names componentsJoinedByString:@", "];
}

// --- Public API ---

ek_result_t ek_cal_request_access(void) {
    @autoreleasepool {
        ek_result_t res = {NULL, NULL};
        EKEventStore* store = get_store();

        __block BOOL granted = NO;
        __block NSError* accessError = nil;
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);

        if (@available(macOS 14.0, *)) {
            [store requestFullAccessToEventsWithCompletion:^(BOOL g, NSError* error) {
                granted = g;
                accessError = error;
                dispatch_semaphore_signal(sem);
            }];
        } else {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
            [store requestAccessToEntityType:EKEntityTypeEvent completion:^(BOOL g, NSError* error) {
                granted = g;
                accessError = error;
                dispatch_semaphore_signal(sem);
            }];
#pragma clang diagnostic pop
        }
        dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);

        if (!granted) {
            if (accessError) {
                res.error = strdup([[NSString stringWithFormat:@"calendar access denied: %@",
                    accessError.localizedDescription] UTF8String]);
            } else {
                res.error = strdup([@"calendar access denied" UTF8String]);
            }
            return res;
        }
        res.result = strdup("1");
        return res;
    }
}

ek_result_t ek_cal_fetch_calendars(void) {
    @autoreleasepool {
        ek_result_t res = {NULL, NULL};
        EKEventStore* store = get_store();
        NSArray<EKCalendar*>* calendars = [store calendarsForEntityType:EKEntityTypeEvent];

        NSMutableArray* result = [NSMutableArray array];
        for (EKCalendar* cal in calendars) {
            [result addObject:calendar_to_dict(cal)];
        }

        res.result = to_json(result);
        if (!res.result) res.error = strdup("JSON serialization failed");
        return res;
    }
}

ek_result_t ek_cal_fetch_events(const char* start_date, const char* end_date,
                           const char* calendar_id, const char* search_query) {
    @autoreleasepool {
        ek_result_t res = {NULL, NULL};
        EKEventStore* store = get_store();

        NSDate* start = parse_iso_date(start_date);
        NSDate* end = parse_iso_date(end_date);

        if (!start || !end) {
            res.error = strdup([@"invalid date range: start and end dates are required" UTF8String]);
            return res;
        }

        // Build calendar filter.
        NSArray<EKCalendar*>* calendars = nil;
        if (calendar_id) {
            NSString* calId = [NSString stringWithUTF8String:calendar_id];
            // Try as calendar ID first, then as calendar name.
            EKCalendar* cal = find_calendar_by_id(store, calId);
            if (!cal) {
                cal = find_calendar_by_name(store, calId);
            }
            if (cal) {
                calendars = @[cal];
            } else {
                res.error = strdup([[NSString stringWithFormat:@"calendar not found: %s (available: %@)", calendar_id, available_calendar_names(store)] UTF8String]);
                return res;
            }
        }

        // EventKit's eventsMatchingPredicate: is synchronous — no semaphore needed.
        NSPredicate* predicate = [store predicateForEventsWithStartDate:start
                                                               endDate:end
                                                             calendars:calendars];
        NSArray<EKEvent*>* events = [store eventsMatchingPredicate:predicate];

        // Apply search filter if provided.
        NSString* query = nil;
        if (search_query) {
            query = [[NSString stringWithUTF8String:search_query] lowercaseString];
        }

        NSMutableArray* result = [NSMutableArray array];
        for (EKEvent* e in events) {
            // Search filter: match against title, location, notes.
            if (query) {
                NSString* titleLower = [(e.title ?: @"") lowercaseString];
                NSString* locLower = [(e.location ?: @"") lowercaseString];
                NSString* notesLower = [(e.notes ?: @"") lowercaseString];
                if (![titleLower containsString:query] &&
                    ![locLower containsString:query] &&
                    ![notesLower containsString:query]) {
                    continue;
                }
            }
            [result addObject:event_to_dict(e)];
        }

        res.result = to_json(result);
        if (!res.result) res.error = strdup("JSON serialization failed");
        return res;
    }
}

ek_result_t ek_cal_get_event(const char* event_id) {
    @autoreleasepool {
        ek_result_t res = {NULL, NULL};
        if (!event_id) {
            res.error = strdup([@"event ID is required" UTF8String]);
            return res;
        }

        EKEventStore* store = get_store();
        NSString* eid = [NSString stringWithUTF8String:event_id];

        // Try eventWithIdentifier first (exact match).
        EKEvent* event = [store eventWithIdentifier:eid];
        if (event) {
            res.result = to_json(event_to_dict(event));
            if (!res.result) res.error = strdup("JSON serialization failed");
            return res;
        }

        // Prefix match: search events in a broad range and match by prefix.
        NSString* upperTarget = [eid uppercaseString];
        NSDate* start = [NSDate dateWithTimeIntervalSinceNow:-365 * 24 * 60 * 60]; // 1 year ago
        NSDate* end = [NSDate dateWithTimeIntervalSinceNow:365 * 24 * 60 * 60];    // 1 year from now
        NSPredicate* predicate = [store predicateForEventsWithStartDate:start
                                                               endDate:end
                                                             calendars:nil];
        NSArray<EKEvent*>* events = [store eventsMatchingPredicate:predicate];

        for (EKEvent* e in events) {
            NSString* eId = [e.eventIdentifier uppercaseString];
            if ([eId hasPrefix:upperTarget]) {
                res.result = to_json(event_to_dict(e));
                if (!res.result) res.error = strdup("JSON serialization failed");
                return res;
            }
        }

        res.error = strdup([[NSString stringWithFormat:@"event not found: %s", event_id] UTF8String]);
        return res;
    }
}

ek_result_t ek_cal_create_event(const char* json_input) {
    __block ek_result_t res = {NULL, NULL};
    dispatch_sync(get_write_queue(), ^{
        @autoreleasepool {
            if (!json_input) {
                res.error = strdup([@"JSON input is required" UTF8String]);
                return;
            }

            EKEventStore* store = get_store();

            // Parse JSON input.
            NSData* data = [NSData dataWithBytes:json_input length:strlen(json_input)];
            NSError* parseError = nil;
            NSDictionary* input = [NSJSONSerialization JSONObjectWithData:data options:0 error:&parseError];
            if (!input) {
                res.error = strdup([[NSString stringWithFormat:@"invalid JSON: %@", parseError.localizedDescription] UTF8String]);
                return;
            }

            EKEvent* event = [EKEvent eventWithEventStore:store];

            // Required fields.
            event.title = input[@"title"] ?: @"";

            NSDate* startDate = parse_iso_date([input[@"startDate"] UTF8String]);
            NSDate* endDate = parse_iso_date([input[@"endDate"] UTF8String]);
            if (!startDate || !endDate) {
                res.error = strdup([@"startDate and endDate are required" UTF8String]);
                return;
            }
            event.startDate = startDate;
            event.endDate = endDate;

            // Optional fields.
            if (input[@"allDay"] && input[@"allDay"] != [NSNull null]) {
                event.allDay = [input[@"allDay"] boolValue];
            }
            if (input[@"location"] && input[@"location"] != [NSNull null]) {
                event.location = input[@"location"];
            }
            if (input[@"notes"] && input[@"notes"] != [NSNull null]) {
                event.notes = input[@"notes"];
            }
            if (input[@"url"] && input[@"url"] != [NSNull null]) {
                event.URL = [NSURL URLWithString:input[@"url"]];
            }

            // TimeZone support.
            if (input[@"timeZone"] && input[@"timeZone"] != [NSNull null]) {
                NSTimeZone* tz = [NSTimeZone timeZoneWithName:input[@"timeZone"]];
                if (tz) {
                    event.timeZone = tz;
                }
            }

            // Calendar.
            if (input[@"calendar"] && input[@"calendar"] != [NSNull null]) {
                NSString* calName = input[@"calendar"];
                EKCalendar* cal = find_calendar_by_name(store, calName);
                if (!cal) {
                    res.error = strdup([[NSString stringWithFormat:@"calendar not found: %@ (available: %@)", calName, available_calendar_names(store)] UTF8String]);
                    return;
                }
                event.calendar = cal;
            } else {
                event.calendar = [store defaultCalendarForNewEvents];
            }

            // Alerts.
            if (input[@"alerts"] && input[@"alerts"] != [NSNull null]) {
                NSArray* alertInputs = input[@"alerts"];
                for (NSDictionary* alertInput in alertInputs) {
                    double offset = [alertInput[@"relativeOffset"] doubleValue];
                    EKAlarm* alarm = [EKAlarm alarmWithRelativeOffset:offset];
                    [event addAlarm:alarm];
                }
            }

            // Recurrence rules.
            if (input[@"recurrenceRules"] && input[@"recurrenceRules"] != [NSNull null]) {
                NSArray* ruleInputs = input[@"recurrenceRules"];
                for (NSDictionary* ruleInput in ruleInputs) {
                    EKRecurrenceFrequency freq = [ruleInput[@"frequency"] integerValue];
                    NSInteger interval = [ruleInput[@"interval"] integerValue];
                    if (interval < 1) interval = 1;

                    // Days of the week.
                    NSMutableArray<EKRecurrenceDayOfWeek*>* daysOfWeek = nil;
                    if (ruleInput[@"daysOfTheWeek"] && ruleInput[@"daysOfTheWeek"] != [NSNull null]) {
                        NSArray* dowInputs = ruleInput[@"daysOfTheWeek"];
                        daysOfWeek = [NSMutableArray arrayWithCapacity:dowInputs.count];
                        for (NSDictionary* dowInput in dowInputs) {
                            EKWeekday weekday = [dowInput[@"dayOfTheWeek"] integerValue];
                            NSInteger weekNum = [dowInput[@"weekNumber"] integerValue];
                            if (weekNum != 0) {
                                [daysOfWeek addObject:[EKRecurrenceDayOfWeek dayOfWeek:weekday weekNumber:weekNum]];
                            } else {
                                [daysOfWeek addObject:[EKRecurrenceDayOfWeek dayOfWeek:weekday]];
                            }
                        }
                    }

                    // Integer arrays.
                    NSArray<NSNumber*>* daysOfMonth = ruleInput[@"daysOfTheMonth"];
                    if (daysOfMonth == (id)[NSNull null]) daysOfMonth = nil;
                    NSArray<NSNumber*>* monthsOfYear = ruleInput[@"monthsOfTheYear"];
                    if (monthsOfYear == (id)[NSNull null]) monthsOfYear = nil;
                    NSArray<NSNumber*>* weeksOfYear = ruleInput[@"weeksOfTheYear"];
                    if (weeksOfYear == (id)[NSNull null]) weeksOfYear = nil;
                    NSArray<NSNumber*>* daysOfYear = ruleInput[@"daysOfTheYear"];
                    if (daysOfYear == (id)[NSNull null]) daysOfYear = nil;
                    NSArray<NSNumber*>* setPositions = ruleInput[@"setPositions"];
                    if (setPositions == (id)[NSNull null]) setPositions = nil;

                    // Recurrence end.
                    EKRecurrenceEnd* recEnd = nil;
                    NSDictionary* endInput = ruleInput[@"end"];
                    if (endInput && endInput != (id)[NSNull null]) {
                        if (endInput[@"endDate"] && endInput[@"endDate"] != [NSNull null]) {
                            NSDate* endDate = parse_iso_date([endInput[@"endDate"] UTF8String]);
                            if (endDate) {
                                recEnd = [EKRecurrenceEnd recurrenceEndWithEndDate:endDate];
                            }
                        } else if (endInput[@"occurrenceCount"] && [endInput[@"occurrenceCount"] integerValue] > 0) {
                            recEnd = [EKRecurrenceEnd recurrenceEndWithOccurrenceCount:[endInput[@"occurrenceCount"] integerValue]];
                        }
                    }

                    EKRecurrenceRule* rule = [[EKRecurrenceRule alloc]
                        initRecurrenceWithFrequency:freq
                                          interval:interval
                                     daysOfTheWeek:daysOfWeek
                                    daysOfTheMonth:daysOfMonth
                                   monthsOfTheYear:monthsOfYear
                                    weeksOfTheYear:weeksOfYear
                                     daysOfTheYear:daysOfYear
                                      setPositions:setPositions
                                               end:recEnd];
                    [event addRecurrenceRule:rule];
                }
            }

            // Structured location.
            if (input[@"structuredLocation"] && input[@"structuredLocation"] != [NSNull null]) {
                NSDictionary* locInput = input[@"structuredLocation"];
                NSString* locTitle = locInput[@"title"] ?: @"";
                EKStructuredLocation* loc = [EKStructuredLocation locationWithTitle:locTitle];
                NSNumber* lat = locInput[@"latitude"];
                NSNumber* lng = locInput[@"longitude"];
                if (lat && lat != (id)[NSNull null] && lng && lng != (id)[NSNull null]) {
                    loc.geoLocation = [[CLLocation alloc] initWithLatitude:[lat doubleValue]
                                                                 longitude:[lng doubleValue]];
                }
                NSNumber* radius = locInput[@"radius"];
                if (radius && radius != (id)[NSNull null]) {
                    loc.radius = [radius doubleValue];
                }
                event.structuredLocation = loc;
            }

            // Save.
            NSError* saveError = nil;
            BOOL saved = [store saveEvent:event span:EKSpanThisEvent commit:YES error:&saveError];
            if (!saved) {
                res.error = strdup([[NSString stringWithFormat:@"failed to save event: %@",
                    saveError.localizedDescription] UTF8String]);
                return;
            }

            res.result = to_json(event_to_dict(event));
            if (!res.result) res.error = strdup("JSON serialization failed");
        }
    });
    return res;
}

ek_result_t ek_cal_update_event(const char* event_id, const char* json_input, int span) {
    __block ek_result_t res = {NULL, NULL};
    dispatch_sync(get_write_queue(), ^{
        @autoreleasepool {
            if (!event_id || !json_input) {
                res.error = strdup([@"event ID and JSON input are required" UTF8String]);
                return;
            }

            EKEventStore* store = get_store();
            NSString* eid = [NSString stringWithUTF8String:event_id];

            EKEvent* event = [store eventWithIdentifier:eid];
            if (!event) {
                res.error = strdup([[NSString stringWithFormat:@"event not found: %s", event_id] UTF8String]);
                return;
            }

            // Parse JSON input.
            NSData* data = [NSData dataWithBytes:json_input length:strlen(json_input)];
            NSError* parseError = nil;
            NSDictionary* input = [NSJSONSerialization JSONObjectWithData:data options:0 error:&parseError];
            if (!input) {
                res.error = strdup([[NSString stringWithFormat:@"invalid JSON: %@", parseError.localizedDescription] UTF8String]);
                return;
            }

            // Update fields that are present in input.
            if (input[@"title"] && input[@"title"] != [NSNull null]) {
                event.title = input[@"title"];
            }
            if (input[@"startDate"] && input[@"startDate"] != [NSNull null]) {
                NSDate* d = parse_iso_date([input[@"startDate"] UTF8String]);
                if (d) event.startDate = d;
            }
            if (input[@"endDate"] && input[@"endDate"] != [NSNull null]) {
                NSDate* d = parse_iso_date([input[@"endDate"] UTF8String]);
                if (d) event.endDate = d;
            }
            if (input[@"allDay"] && input[@"allDay"] != [NSNull null]) {
                event.allDay = [input[@"allDay"] boolValue];
            }
            if (input[@"location"] != nil) {
                if (input[@"location"] == [NSNull null]) {
                    event.location = nil;
                } else {
                    event.location = input[@"location"];
                }
            }
            if (input[@"notes"] != nil) {
                if (input[@"notes"] == [NSNull null]) {
                    event.notes = nil;
                } else {
                    event.notes = input[@"notes"];
                }
            }
            if (input[@"url"] != nil) {
                if (input[@"url"] == [NSNull null]) {
                    event.URL = nil;
                } else {
                    event.URL = [NSURL URLWithString:input[@"url"]];
                }
            }

            // TimeZone support.
            if (input[@"timeZone"] != nil) {
                if (input[@"timeZone"] == [NSNull null]) {
                    event.timeZone = nil;
                } else {
                    NSTimeZone* tz = [NSTimeZone timeZoneWithName:input[@"timeZone"]];
                    if (tz) {
                        event.timeZone = tz;
                    }
                }
            }

            // Calendar (move to different calendar).
            if (input[@"calendar"] && input[@"calendar"] != [NSNull null]) {
                NSString* calName = input[@"calendar"];
                EKCalendar* cal = find_calendar_by_name(store, calName);
                if (!cal) {
                    res.error = strdup([[NSString stringWithFormat:@"calendar not found: %@ (available: %@)", calName, available_calendar_names(store)] UTF8String]);
                    return;
                }
                event.calendar = cal;
            }

            // Alerts (replace all).
            if (input[@"alerts"] != nil) {
                // Remove existing alarms.
                for (EKAlarm* alarm in [event.alarms copy]) {
                    [event removeAlarm:alarm];
                }
                if (input[@"alerts"] != [NSNull null]) {
                    NSArray* alertInputs = input[@"alerts"];
                    for (NSDictionary* alertInput in alertInputs) {
                        double offset = [alertInput[@"relativeOffset"] doubleValue];
                        EKAlarm* alarm = [EKAlarm alarmWithRelativeOffset:offset];
                        [event addAlarm:alarm];
                    }
                }
            }

            // Recurrence rules (replace all).
            if (input[@"recurrenceRules"] != nil) {
                // Remove existing rules.
                for (EKRecurrenceRule* rule in [event.recurrenceRules copy]) {
                    [event removeRecurrenceRule:rule];
                }
                if (input[@"recurrenceRules"] != [NSNull null]) {
                    NSArray* ruleInputs = input[@"recurrenceRules"];
                    for (NSDictionary* ruleInput in ruleInputs) {
                        EKRecurrenceFrequency freq = [ruleInput[@"frequency"] integerValue];
                        NSInteger interval = [ruleInput[@"interval"] integerValue];
                        if (interval < 1) interval = 1;

                        NSMutableArray<EKRecurrenceDayOfWeek*>* daysOfWeek = nil;
                        if (ruleInput[@"daysOfTheWeek"] && ruleInput[@"daysOfTheWeek"] != [NSNull null]) {
                            NSArray* dowInputs = ruleInput[@"daysOfTheWeek"];
                            daysOfWeek = [NSMutableArray arrayWithCapacity:dowInputs.count];
                            for (NSDictionary* dowInput in dowInputs) {
                                EKWeekday weekday = [dowInput[@"dayOfTheWeek"] integerValue];
                                NSInteger weekNum = [dowInput[@"weekNumber"] integerValue];
                                if (weekNum != 0) {
                                    [daysOfWeek addObject:[EKRecurrenceDayOfWeek dayOfWeek:weekday weekNumber:weekNum]];
                                } else {
                                    [daysOfWeek addObject:[EKRecurrenceDayOfWeek dayOfWeek:weekday]];
                                }
                            }
                        }

                        NSArray<NSNumber*>* daysOfMonth = ruleInput[@"daysOfTheMonth"];
                        if (daysOfMonth == (id)[NSNull null]) daysOfMonth = nil;
                        NSArray<NSNumber*>* monthsOfYear = ruleInput[@"monthsOfTheYear"];
                        if (monthsOfYear == (id)[NSNull null]) monthsOfYear = nil;
                        NSArray<NSNumber*>* weeksOfYear = ruleInput[@"weeksOfTheYear"];
                        if (weeksOfYear == (id)[NSNull null]) weeksOfYear = nil;
                        NSArray<NSNumber*>* daysOfYear = ruleInput[@"daysOfTheYear"];
                        if (daysOfYear == (id)[NSNull null]) daysOfYear = nil;
                        NSArray<NSNumber*>* setPositions = ruleInput[@"setPositions"];
                        if (setPositions == (id)[NSNull null]) setPositions = nil;

                        EKRecurrenceEnd* recEnd = nil;
                        NSDictionary* endInput = ruleInput[@"end"];
                        if (endInput && endInput != (id)[NSNull null]) {
                            if (endInput[@"endDate"] && endInput[@"endDate"] != [NSNull null]) {
                                NSDate* endDate = parse_iso_date([endInput[@"endDate"] UTF8String]);
                                if (endDate) {
                                    recEnd = [EKRecurrenceEnd recurrenceEndWithEndDate:endDate];
                                }
                            } else if (endInput[@"occurrenceCount"] && [endInput[@"occurrenceCount"] integerValue] > 0) {
                                recEnd = [EKRecurrenceEnd recurrenceEndWithOccurrenceCount:[endInput[@"occurrenceCount"] integerValue]];
                            }
                        }

                        EKRecurrenceRule* rule = [[EKRecurrenceRule alloc]
                            initRecurrenceWithFrequency:freq
                                              interval:interval
                                         daysOfTheWeek:daysOfWeek
                                        daysOfTheMonth:daysOfMonth
                                       monthsOfTheYear:monthsOfYear
                                        weeksOfTheYear:weeksOfYear
                                         daysOfTheYear:daysOfYear
                                          setPositions:setPositions
                                                   end:recEnd];
                        [event addRecurrenceRule:rule];
                    }
                }
            }

            // Structured location.
            if (input[@"structuredLocation"] != nil) {
                if (input[@"structuredLocation"] == [NSNull null]) {
                    event.structuredLocation = nil;
                } else {
                    NSDictionary* locInput = input[@"structuredLocation"];
                    NSString* locTitle = locInput[@"title"] ?: @"";
                    EKStructuredLocation* loc = [EKStructuredLocation locationWithTitle:locTitle];
                    NSNumber* lat = locInput[@"latitude"];
                    NSNumber* lng = locInput[@"longitude"];
                    if (lat && lat != (id)[NSNull null] && lng && lng != (id)[NSNull null]) {
                        loc.geoLocation = [[CLLocation alloc] initWithLatitude:[lat doubleValue]
                                                                     longitude:[lng doubleValue]];
                    }
                    NSNumber* radius = locInput[@"radius"];
                    if (radius && radius != (id)[NSNull null]) {
                        loc.radius = [radius doubleValue];
                    }
                    event.structuredLocation = loc;
                }
            }

            // Save.
            EKSpan ekSpan = (span == 1) ? EKSpanFutureEvents : EKSpanThisEvent;
            NSError* saveError = nil;
            BOOL saved = [store saveEvent:event span:ekSpan commit:YES error:&saveError];
            if (!saved) {
                res.error = strdup([[NSString stringWithFormat:@"failed to update event: %@",
                    saveError.localizedDescription] UTF8String]);
                return;
            }

            res.result = to_json(event_to_dict(event));
            if (!res.result) res.error = strdup("JSON serialization failed");
        }
    });
    return res;
}

// --- Available source names (for error messages) ---

static NSString* available_source_names(EKEventStore* store, EKEntityType entityType) {
    NSMutableArray* names = [NSMutableArray array];
    for (EKSource* source in store.sources) {
        NSSet* cals = [source calendarsForEntityType:entityType];
        if (cals.count > 0) {
            [names addObject:source.title];
        }
    }
    return [names componentsJoinedByString:@", "];
}

// --- Find source by name (case-insensitive) ---

static EKSource* find_source_by_name(EKEventStore* store, NSString* name) {
    NSString* lowerName = [name lowercaseString];
    // Multiple sources can share the same title (e.g., "iCloud" for events
    // and "iCloud" for reminders). Prefer the one that has event calendars.
    EKSource* fallback = nil;
    for (EKSource* source in store.sources) {
        if ([[source.title lowercaseString] isEqualToString:lowerName]) {
            NSSet* eventCals = [source calendarsForEntityType:EKEntityTypeEvent];
            if (eventCals.count > 0) {
                return source;
            }
            if (!fallback) {
                fallback = source;
            }
        }
    }
    return fallback;
}

// --- Parse hex color string to CGColorRef ---

static CGColorRef parse_hex_color(NSString* hex) {
    if (!hex || hex.length < 7) return NULL;
    NSString* clean = hex;
    if ([clean hasPrefix:@"#"]) {
        clean = [clean substringFromIndex:1];
    }
    if (clean.length != 6) return NULL;

    unsigned int r, g, b;
    NSScanner* scanner;

    scanner = [NSScanner scannerWithString:[clean substringWithRange:NSMakeRange(0, 2)]];
    if (![scanner scanHexInt:&r]) return NULL;
    scanner = [NSScanner scannerWithString:[clean substringWithRange:NSMakeRange(2, 2)]];
    if (![scanner scanHexInt:&g]) return NULL;
    scanner = [NSScanner scannerWithString:[clean substringWithRange:NSMakeRange(4, 2)]];
    if (![scanner scanHexInt:&b]) return NULL;

    return CGColorCreateGenericRGB(r / 255.0, g / 255.0, b / 255.0, 1.0);
}

// --- Calendar CRUD ---

ek_result_t ek_cal_create_calendar(const char* json_input) {
    __block ek_result_t res = {NULL, NULL};
    dispatch_sync(get_write_queue(), ^{
        @autoreleasepool {
            if (!json_input) {
                res.error = strdup([@"JSON input is required" UTF8String]);
                return;
            }

            EKEventStore* store = get_store();

            // Parse JSON input.
            NSData* data = [NSData dataWithBytes:json_input length:strlen(json_input)];
            NSError* parseError = nil;
            NSDictionary* input = [NSJSONSerialization JSONObjectWithData:data options:0 error:&parseError];
            if (!input) {
                res.error = strdup([[NSString stringWithFormat:@"invalid JSON: %@", parseError.localizedDescription] UTF8String]);
                return;
            }

            EKCalendar* cal = [EKCalendar calendarForEntityType:EKEntityTypeEvent eventStore:store];

            // Title (required).
            cal.title = input[@"title"] ?: @"";

            // Source (required — validated in Go layer).
            EKSource* source = find_source_by_name(store, input[@"source"]);
            if (!source) {
                res.error = strdup([[NSString stringWithFormat:@"source not found: %@ (available: %@)", input[@"source"], available_source_names(store, EKEntityTypeEvent)] UTF8String]);
                return;
            }
            cal.source = source;

            // Color.
            if (input[@"color"] && input[@"color"] != [NSNull null] && [input[@"color"] length] > 0) {
                CGColorRef color = parse_hex_color(input[@"color"]);
                if (color) {
                    cal.CGColor = color;
                    CGColorRelease(color);
                }
            }

            // Save.
            NSError* saveError = nil;
            BOOL saved = [store saveCalendar:cal commit:YES error:&saveError];
            if (!saved) {
                res.error = strdup([[NSString stringWithFormat:@"failed to save calendar: %@",
                    saveError.localizedDescription] UTF8String]);
                return;
            }

            res.result = to_json(calendar_to_dict(cal));
            if (!res.result) res.error = strdup("JSON serialization failed");
        }
    });
    return res;
}

ek_result_t ek_cal_update_calendar(const char* calendar_id, const char* json_input) {
    __block ek_result_t res = {NULL, NULL};
    dispatch_sync(get_write_queue(), ^{
        @autoreleasepool {
            if (!calendar_id || !json_input) {
                res.error = strdup([@"calendar ID and JSON input are required" UTF8String]);
                return;
            }

            EKEventStore* store = get_store();
            NSString* calId = [NSString stringWithUTF8String:calendar_id];

            EKCalendar* cal = find_calendar_by_id(store, calId);
            if (!cal) {
                res.error = strdup([[NSString stringWithFormat:@"calendar not found: %s", calendar_id] UTF8String]);
                return;
            }

            // Check immutability.
            if (cal.isImmutable) {
                res.error = strdup([[NSString stringWithFormat:@"calendar is immutable: %@", cal.title] UTF8String]);
                return;
            }

            // Parse JSON input.
            NSData* data = [NSData dataWithBytes:json_input length:strlen(json_input)];
            NSError* parseError = nil;
            NSDictionary* input = [NSJSONSerialization JSONObjectWithData:data options:0 error:&parseError];
            if (!input) {
                res.error = strdup([[NSString stringWithFormat:@"invalid JSON: %@", parseError.localizedDescription] UTF8String]);
                return;
            }

            // Update title.
            if (input[@"title"] && input[@"title"] != [NSNull null]) {
                cal.title = input[@"title"];
            }

            // Update color.
            if (input[@"color"] && input[@"color"] != [NSNull null]) {
                CGColorRef color = parse_hex_color(input[@"color"]);
                if (color) {
                    cal.CGColor = color;
                    CGColorRelease(color);
                }
            }

            // Save.
            NSError* saveError = nil;
            BOOL saved = [store saveCalendar:cal commit:YES error:&saveError];
            if (!saved) {
                res.error = strdup([[NSString stringWithFormat:@"failed to update calendar: %@",
                    saveError.localizedDescription] UTF8String]);
                return;
            }

            res.result = to_json(calendar_to_dict(cal));
            if (!res.result) res.error = strdup("JSON serialization failed");
        }
    });
    return res;
}

ek_result_t ek_cal_delete_calendar(const char* calendar_id) {
    __block ek_result_t res = {NULL, NULL};
    dispatch_sync(get_write_queue(), ^{
        @autoreleasepool {
            if (!calendar_id) {
                res.error = strdup([@"calendar ID is required" UTF8String]);
                return;
            }

            EKEventStore* store = get_store();
            NSString* calId = [NSString stringWithUTF8String:calendar_id];

            EKCalendar* cal = find_calendar_by_id(store, calId);
            if (!cal) {
                res.error = strdup([[NSString stringWithFormat:@"calendar not found: %s", calendar_id] UTF8String]);
                return;
            }

            // Check immutability.
            if (cal.isImmutable) {
                res.error = strdup([[NSString stringWithFormat:@"calendar is immutable: %@", cal.title] UTF8String]);
                return;
            }

            NSError* removeError = nil;
            BOOL removed = [store removeCalendar:cal commit:YES error:&removeError];
            if (!removed) {
                res.error = strdup([[NSString stringWithFormat:@"failed to delete calendar: %@",
                    removeError.localizedDescription] UTF8String]);
                return;
            }

            res.result = strdup("ok");
        }
    });
    return res;
}

ek_result_t ek_cal_delete_events(const char* json_ids, int span) {
    __block ek_result_t res = {NULL, NULL};
    dispatch_sync(get_write_queue(), ^{
        @autoreleasepool {
            if (!json_ids) {
                res.error = strdup([@"JSON input is required" UTF8String]);
                return;
            }

            NSData* data = [NSData dataWithBytes:json_ids length:strlen(json_ids)];
            NSError* parseError = nil;
            NSArray* ids = [NSJSONSerialization JSONObjectWithData:data options:0 error:&parseError];
            if (!ids) {
                res.error = strdup([[NSString stringWithFormat:@"invalid JSON: %@", parseError.localizedDescription] UTF8String]);
                return;
            }

            EKEventStore* store = get_store();
            EKSpan ekSpan = (span == 1) ? EKSpanFutureEvents : EKSpanThisEvent;
            NSMutableDictionary* errors = [NSMutableDictionary dictionary];

            for (NSString* eid in ids) {
                EKEvent* event = [store eventWithIdentifier:eid];
                if (!event) continue; // silently skip not found

                NSError* removeError = nil;
                BOOL removed = [store removeEvent:event span:ekSpan commit:YES error:&removeError];
                if (!removed) {
                    errors[eid] = [removeError localizedDescription] ?: @"unknown error";
                }
            }

            res.result = to_json(errors);
            if (!res.result) res.error = strdup("JSON serialization failed");
        }
    });
    return res;
}

ek_result_t ek_cal_delete_event(const char* event_id, int span) {
    __block ek_result_t res = {NULL, NULL};
    dispatch_sync(get_write_queue(), ^{
        @autoreleasepool {
            if (!event_id) {
                res.error = strdup([@"event ID is required" UTF8String]);
                return;
            }

            EKEventStore* store = get_store();
            NSString* eid = [NSString stringWithUTF8String:event_id];

            EKEvent* event = [store eventWithIdentifier:eid];
            if (!event) {
                res.error = strdup([[NSString stringWithFormat:@"event not found: %s", event_id] UTF8String]);
                return;
            }

            EKSpan ekSpan = (span == 1) ? EKSpanFutureEvents : EKSpanThisEvent;
            NSError* removeError = nil;
            BOOL removed = [store removeEvent:event span:ekSpan commit:YES error:&removeError];
            if (!removed) {
                res.error = strdup([[NSString stringWithFormat:@"failed to delete event: %@",
                    removeError.localizedDescription] UTF8String]);
                return;
            }

            res.result = strdup("ok");
        }
    });
    return res;
}
