#import <EventKit/EventKit.h>
#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#include "bridge_darwin.h"
#include <stdlib.h>
#include <string.h>

// Thread-local error message.
static __thread char* cal_last_error = NULL;

static void cal_set_error(NSString* msg) {
    if (cal_last_error) {
        free(cal_last_error);
        cal_last_error = NULL;
    }
    if (msg) {
        cal_last_error = strdup([msg UTF8String]);
    }
}

const char* ek_cal_last_error(void) {
    return cal_last_error;
}

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
    if (!data) {
        cal_set_error([NSString stringWithFormat:@"JSON serialization failed: %@",
            error.localizedDescription]);
        return NULL;
    }
    NSString* str = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
    return strdup([str UTF8String]);
}

// --- Event to dictionary conversion ---

static NSDictionary* event_to_dict(EKEvent* e) {
    NSMutableDictionary* d = [NSMutableDictionary dictionary];

    // Use eventIdentifier — stable across recurrence edits.
    d[@"id"] = e.eventIdentifier ?: @"";
    d[@"title"] = e.title ?: @"";
    d[@"allDay"] = @(e.allDay);
    d[@"location"] = e.location ?: [NSNull null];
    d[@"notes"] = e.notes ?: [NSNull null];
    d[@"url"] = e.URL ? [e.URL absoluteString] : [NSNull null];
    d[@"calendar"] = e.calendar.title ?: @"";
    d[@"calendarID"] = e.calendar.calendarIdentifier ?: @"";
    d[@"status"] = @(e.status);
    d[@"availability"] = @(e.availability);
    d[@"recurring"] = @(e.hasRecurrenceRules);

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
    d[@"readOnly"] = @(!cal.allowsContentModifications);

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

// --- Public API ---

int ek_cal_request_access(void) {
    @autoreleasepool {
        cal_set_error(nil);
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
                cal_set_error([NSString stringWithFormat:@"calendar access denied: %@",
                    accessError.localizedDescription]);
            } else {
                cal_set_error(@"calendar access denied");
            }
            return 0;
        }
        return 1;
    }
}

char* ek_cal_fetch_calendars(void) {
    @autoreleasepool {
        cal_set_error(nil);
        EKEventStore* store = get_store();
        NSArray<EKCalendar*>* calendars = [store calendarsForEntityType:EKEntityTypeEvent];

        NSMutableArray* result = [NSMutableArray array];
        for (EKCalendar* cal in calendars) {
            [result addObject:calendar_to_dict(cal)];
        }

        return to_json(result);
    }
}

char* ek_cal_fetch_events(const char* start_date, const char* end_date,
                           const char* calendar_id, const char* search_query) {
    @autoreleasepool {
        cal_set_error(nil);
        EKEventStore* store = get_store();

        NSDate* start = parse_iso_date(start_date);
        NSDate* end = parse_iso_date(end_date);

        if (!start || !end) {
            cal_set_error(@"invalid date range: start and end dates are required");
            return NULL;
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
                cal_set_error([NSString stringWithFormat:@"calendar not found: %s", calendar_id]);
                return NULL;
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

        return to_json(result);
    }
}

char* ek_cal_get_event(const char* event_id) {
    @autoreleasepool {
        cal_set_error(nil);
        if (!event_id) {
            cal_set_error(@"event ID is required");
            return NULL;
        }

        EKEventStore* store = get_store();
        NSString* eid = [NSString stringWithUTF8String:event_id];

        // Try eventWithIdentifier first (exact match).
        EKEvent* event = [store eventWithIdentifier:eid];
        if (event) {
            return to_json(event_to_dict(event));
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
                return to_json(event_to_dict(e));
            }
        }

        cal_set_error([NSString stringWithFormat:@"event not found: %s", event_id]);
        return NULL;
    }
}

char* ek_cal_create_event(const char* json_input) {
    @autoreleasepool {
        cal_set_error(nil);
        if (!json_input) {
            cal_set_error(@"JSON input is required");
            return NULL;
        }

        EKEventStore* store = get_store();

        // Parse JSON input.
        NSData* data = [NSData dataWithBytes:json_input length:strlen(json_input)];
        NSError* parseError = nil;
        NSDictionary* input = [NSJSONSerialization JSONObjectWithData:data options:0 error:&parseError];
        if (!input) {
            cal_set_error([NSString stringWithFormat:@"invalid JSON: %@", parseError.localizedDescription]);
            return NULL;
        }

        EKEvent* event = [EKEvent eventWithEventStore:store];

        // Required fields.
        event.title = input[@"title"] ?: @"";

        NSDate* startDate = parse_iso_date([input[@"startDate"] UTF8String]);
        NSDate* endDate = parse_iso_date([input[@"endDate"] UTF8String]);
        if (!startDate || !endDate) {
            cal_set_error(@"startDate and endDate are required");
            return NULL;
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
                cal_set_error([NSString stringWithFormat:@"calendar not found: %@", calName]);
                return NULL;
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

        // Save.
        NSError* saveError = nil;
        BOOL saved = [store saveEvent:event span:EKSpanThisEvent commit:YES error:&saveError];
        if (!saved) {
            cal_set_error([NSString stringWithFormat:@"failed to save event: %@",
                saveError.localizedDescription]);
            return NULL;
        }

        return to_json(event_to_dict(event));
    }
}

char* ek_cal_update_event(const char* event_id, const char* json_input, int span) {
    @autoreleasepool {
        cal_set_error(nil);
        if (!event_id || !json_input) {
            cal_set_error(@"event ID and JSON input are required");
            return NULL;
        }

        EKEventStore* store = get_store();
        NSString* eid = [NSString stringWithUTF8String:event_id];

        EKEvent* event = [store eventWithIdentifier:eid];
        if (!event) {
            cal_set_error([NSString stringWithFormat:@"event not found: %s", event_id]);
            return NULL;
        }

        // Parse JSON input.
        NSData* data = [NSData dataWithBytes:json_input length:strlen(json_input)];
        NSError* parseError = nil;
        NSDictionary* input = [NSJSONSerialization JSONObjectWithData:data options:0 error:&parseError];
        if (!input) {
            cal_set_error([NSString stringWithFormat:@"invalid JSON: %@", parseError.localizedDescription]);
            return NULL;
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
                cal_set_error([NSString stringWithFormat:@"calendar not found: %@", calName]);
                return NULL;
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

        // Save.
        EKSpan ekSpan = (span == 1) ? EKSpanFutureEvents : EKSpanThisEvent;
        NSError* saveError = nil;
        BOOL saved = [store saveEvent:event span:ekSpan commit:YES error:&saveError];
        if (!saved) {
            cal_set_error([NSString stringWithFormat:@"failed to update event: %@",
                saveError.localizedDescription]);
            return NULL;
        }

        return to_json(event_to_dict(event));
    }
}

char* ek_cal_delete_event(const char* event_id, int span) {
    @autoreleasepool {
        cal_set_error(nil);
        if (!event_id) {
            cal_set_error(@"event ID is required");
            return NULL;
        }

        EKEventStore* store = get_store();
        NSString* eid = [NSString stringWithUTF8String:event_id];

        EKEvent* event = [store eventWithIdentifier:eid];
        if (!event) {
            cal_set_error([NSString stringWithFormat:@"event not found: %s", event_id]);
            return NULL;
        }

        EKSpan ekSpan = (span == 1) ? EKSpanFutureEvents : EKSpanThisEvent;
        NSError* removeError = nil;
        BOOL removed = [store removeEvent:event span:ekSpan commit:YES error:&removeError];
        if (!removed) {
            cal_set_error([NSString stringWithFormat:@"failed to delete event: %@",
                removeError.localizedDescription]);
            return NULL;
        }

        return strdup("ok");
    }
}
