#import <EventKit/EventKit.h>
#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#include "bridge_darwin.h"
#include <stdlib.h>
#include <string.h>

// Thread-local error message.
static __thread char* rem_last_error = NULL;

static void rem_set_error(NSString* msg) {
    if (rem_last_error) {
        free(rem_last_error);
        rem_last_error = NULL;
    }
    if (msg) {
        rem_last_error = strdup([msg UTF8String]);
    }
}

const char* ek_rem_last_error(void) {
    return rem_last_error;
}

void ek_rem_free(char* ptr) {
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

static NSDate* parse_iso_date(const char* str) {
    if (!str) return nil;
    NSString* s = [NSString stringWithUTF8String:str];
    NSDate* d = [get_iso_formatter() dateFromString:s];
    if (d) return d;
    NSISO8601DateFormatter* iso = [[NSISO8601DateFormatter alloc] init];
    d = [iso dateFromString:s];
    if (d) return d;
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
        rem_set_error([NSString stringWithFormat:@"JSON serialization failed: %@",
            error.localizedDescription]);
        return NULL;
    }
    NSString* str = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
    return strdup([str UTF8String]);
}

// --- Synchronous reminder fetch (dispatch_semaphore for async API) ---

static NSArray<EKReminder*>* fetch_all_reminders(NSArray<EKCalendar*>* calendars) {
    EKEventStore* store = get_store();
    NSPredicate* predicate = [store predicateForRemindersInCalendars:calendars];
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);
    __block NSArray<EKReminder*>* result = @[];
    [store fetchRemindersMatchingPredicate:predicate completion:^(NSArray<EKReminder*>* reminders) {
        result = reminders ?: @[];
        dispatch_semaphore_signal(sem);
    }];
    dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);
    return result;
}

// --- Reminder to dictionary conversion ---

static NSDictionary* reminder_to_dict(EKReminder* r) {
    NSMutableDictionary* d = [NSMutableDictionary dictionary];

    d[@"id"] = r.calendarItemIdentifier ?: @"";
    d[@"title"] = r.title ?: @"";
    d[@"notes"] = r.notes ?: [NSNull null];
    d[@"list"] = r.calendar.title ?: @"";
    d[@"listID"] = r.calendar.calendarIdentifier ?: @"";
    d[@"completed"] = r.isCompleted ? @YES : @NO;
    // EventKit does NOT expose the flagged property on EKReminder.
    // r.isFlagged does not exist. Always returns NO.
    // This is a known Apple limitation (see rem journal Session 3).
    d[@"flagged"] = @NO;
    d[@"priority"] = @(r.priority);
    d[@"hasAlarms"] = r.hasAlarms ? @YES : @NO;

    // Due date from date components.
    if (r.dueDateComponents) {
        NSCalendar* cal = [NSCalendar currentCalendar];
        NSDate* dueDate = [cal dateFromComponents:r.dueDateComponents];
        if (dueDate) {
            d[@"dueDate"] = format_date(dueDate);
        }
    }

    // URL (notes may contain URL).
    if (r.URL) {
        d[@"url"] = [r.URL absoluteString];
    } else {
        d[@"url"] = [NSNull null];
    }

    // Recurrence rules.
    d[@"recurring"] = r.hasRecurrenceRules ? @YES : @NO;
    if (r.recurrenceRules && r.recurrenceRules.count > 0) {
        NSMutableArray* rules = [NSMutableArray array];
        for (EKRecurrenceRule* rule in r.recurrenceRules) {
            NSMutableDictionary* rd = [NSMutableDictionary dictionary];
            rd[@"frequency"] = @(rule.frequency);
            rd[@"interval"] = @(rule.interval);

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

            if (rule.daysOfTheMonth && rule.daysOfTheMonth.count > 0) {
                rd[@"daysOfTheMonth"] = rule.daysOfTheMonth;
            }
            if (rule.monthsOfTheYear && rule.monthsOfTheYear.count > 0) {
                rd[@"monthsOfTheYear"] = rule.monthsOfTheYear;
            }
            if (rule.weeksOfTheYear && rule.weeksOfTheYear.count > 0) {
                rd[@"weeksOfTheYear"] = rule.weeksOfTheYear;
            }
            if (rule.daysOfTheYear && rule.daysOfTheYear.count > 0) {
                rd[@"daysOfTheYear"] = rule.daysOfTheYear;
            }
            if (rule.setPositions && rule.setPositions.count > 0) {
                rd[@"setPositions"] = rule.setPositions;
            }

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

    // Alarms.
    if (r.alarms && r.alarms.count > 0) {
        NSMutableArray* alarms = [NSMutableArray array];
        for (EKAlarm* alarm in r.alarms) {
            NSMutableDictionary* a = [NSMutableDictionary dictionary];
            if (alarm.absoluteDate) {
                a[@"absoluteDate"] = format_date(alarm.absoluteDate);
                d[@"remindMeDate"] = format_date(alarm.absoluteDate);
            }
            a[@"relativeOffset"] = @(alarm.relativeOffset);
            [alarms addObject:a];
        }
        d[@"alarms"] = alarms;
    } else {
        d[@"alarms"] = @[];
    }

    // Timestamps.
    if (r.completionDate) {
        d[@"completionDate"] = format_date(r.completionDate);
    }
    if (r.creationDate) {
        d[@"createdAt"] = format_date(r.creationDate);
    }
    if (r.lastModifiedDate) {
        d[@"modifiedAt"] = format_date(r.lastModifiedDate);
    }

    return d;
}

// --- List to dictionary conversion ---

static NSDictionary* list_to_dict(EKCalendar* cal, int count) {
    NSMutableDictionary* d = [NSMutableDictionary dictionary];

    d[@"id"] = cal.calendarIdentifier ?: @"";
    d[@"title"] = cal.title ?: @"";
    d[@"source"] = cal.source.title ?: @"";
    d[@"readOnly"] = cal.allowsContentModifications ? @NO : @YES;
    d[@"count"] = @(count);

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

// --- Find list (calendar) by name (case-insensitive) ---

static EKCalendar* find_list_by_name(EKEventStore* store, NSString* name) {
    NSString* lowerName = [name lowercaseString];
    for (EKCalendar* cal in [store calendarsForEntityType:EKEntityTypeReminder]) {
        if ([[cal.title lowercaseString] isEqualToString:lowerName]) {
            return cal;
        }
    }
    return nil;
}

// --- Find reminder by ID or prefix ---

static EKReminder* find_reminder_by_id(NSString* targetId) {
    NSArray<EKReminder*>* allReminders = fetch_all_reminders(nil);
    NSString* target = [targetId uppercaseString];

    for (EKReminder* r in allReminders) {
        NSString* uuid = [r.calendarItemIdentifier uppercaseString];
        // Support full ID and prefix match.
        if ([uuid isEqualToString:target] || [uuid hasPrefix:target]) {
            return r;
        }
    }
    return nil;
}

// --- Public API ---

int ek_rem_request_access(void) {
    @autoreleasepool {
        rem_set_error(nil);
        EKEventStore* store = get_store();

        __block BOOL granted = NO;
        __block NSError* accessError = nil;
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);

        if (@available(macOS 14.0, *)) {
            [store requestFullAccessToRemindersWithCompletion:^(BOOL g, NSError* error) {
                granted = g;
                accessError = error;
                dispatch_semaphore_signal(sem);
            }];
        } else {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
            [store requestAccessToEntityType:EKEntityTypeReminder completion:^(BOOL g, NSError* error) {
                granted = g;
                accessError = error;
                dispatch_semaphore_signal(sem);
            }];
#pragma clang diagnostic pop
        }
        dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);

        if (!granted) {
            if (accessError) {
                rem_set_error([NSString stringWithFormat:@"reminders access denied: %@",
                    accessError.localizedDescription]);
            } else {
                rem_set_error(@"reminders access denied");
            }
            return 0;
        }
        return 1;
    }
}

char* ek_rem_fetch_lists(void) {
    @autoreleasepool {
        rem_set_error(nil);
        EKEventStore* store = get_store();
        NSArray<EKCalendar*>* calendars = [store calendarsForEntityType:EKEntityTypeReminder];

        // Fetch all reminders to count per-list.
        NSArray<EKReminder*>* allReminders = fetch_all_reminders(nil);
        NSMutableDictionary<NSString*, NSNumber*>* counts = [NSMutableDictionary dictionary];
        for (EKReminder* r in allReminders) {
            NSString* calId = r.calendar.calendarIdentifier;
            counts[calId] = @([counts[calId] integerValue] + 1);
        }

        NSMutableArray* result = [NSMutableArray array];
        for (EKCalendar* cal in calendars) {
            int count = [counts[cal.calendarIdentifier] intValue];
            [result addObject:list_to_dict(cal, count)];
        }

        return to_json(result);
    }
}

char* ek_rem_fetch_reminders(const char* list_name,
                              const char* completed_filter,
                              const char* search_query,
                              const char* due_before,
                              const char* due_after) {
    @autoreleasepool {
        rem_set_error(nil);
        EKEventStore* store = get_store();

        // Find calendar for list filter.
        NSArray<EKCalendar*>* cals = nil;
        if (list_name) {
            NSString* ln = [[NSString stringWithUTF8String:list_name] lowercaseString];
            NSMutableArray<EKCalendar*>* matched = [NSMutableArray array];
            for (EKCalendar* cal in [store calendarsForEntityType:EKEntityTypeReminder]) {
                if ([[cal.title lowercaseString] isEqualToString:ln]) {
                    [matched addObject:cal];
                }
            }
            if (matched.count == 0) {
                rem_set_error([NSString stringWithFormat:@"list not found: %s", list_name]);
                return NULL;
            }
            cals = matched;
        }

        NSArray<EKReminder*>* allReminders = fetch_all_reminders(cals);

        // Parse date filters.
        NSDate* dueBeforeDate = due_before ? parse_iso_date(due_before) : nil;
        NSDate* dueAfterDate = due_after ? parse_iso_date(due_after) : nil;

        // Search query.
        NSString* query = search_query ? [[NSString stringWithUTF8String:search_query] lowercaseString] : nil;

        NSMutableArray* result = [NSMutableArray array];
        for (EKReminder* r in allReminders) {
            // Completed filter.
            if (completed_filter) {
                if (strcmp(completed_filter, "true") == 0 && !r.isCompleted) continue;
                if (strcmp(completed_filter, "false") == 0 && r.isCompleted) continue;
            }

            // Search filter (title and notes).
            if (query) {
                NSString* titleLower = [(r.title ?: @"") lowercaseString];
                NSString* notesLower = [(r.notes ?: @"") lowercaseString];
                if (![titleLower containsString:query] && ![notesLower containsString:query]) {
                    continue;
                }
            }

            // Due date range filter.
            if (dueBeforeDate || dueAfterDate) {
                if (!r.dueDateComponents) continue;
                NSDate* dueDate = [[NSCalendar currentCalendar] dateFromComponents:r.dueDateComponents];
                if (!dueDate) continue;
                if (dueBeforeDate && [dueDate compare:dueBeforeDate] == NSOrderedDescending) continue;
                if (dueAfterDate && [dueDate compare:dueAfterDate] == NSOrderedAscending) continue;
            }

            [result addObject:reminder_to_dict(r)];
        }

        return to_json(result);
    }
}

char* ek_rem_get_reminder(const char* target_id) {
    @autoreleasepool {
        rem_set_error(nil);
        if (!target_id) {
            rem_set_error(@"target ID is required");
            return NULL;
        }

        EKReminder* r = find_reminder_by_id([NSString stringWithUTF8String:target_id]);
        if (!r) {
            rem_set_error([NSString stringWithFormat:@"reminder not found: %s", target_id]);
            return NULL;
        }

        return to_json(reminder_to_dict(r));
    }
}

char* ek_rem_create_reminder(const char* json_input) {
    @autoreleasepool {
        rem_set_error(nil);
        if (!json_input) {
            rem_set_error(@"JSON input is required");
            return NULL;
        }

        EKEventStore* store = get_store();

        // Parse JSON input.
        NSData* data = [NSData dataWithBytes:json_input length:strlen(json_input)];
        NSError* parseError = nil;
        NSDictionary* input = [NSJSONSerialization JSONObjectWithData:data options:0 error:&parseError];
        if (!input) {
            rem_set_error([NSString stringWithFormat:@"invalid JSON: %@", parseError.localizedDescription]);
            return NULL;
        }

        EKReminder* reminder = [EKReminder reminderWithEventStore:store];

        // Title (required).
        reminder.title = input[@"title"] ?: @"";

        // Notes.
        if (input[@"notes"] && input[@"notes"] != [NSNull null]) {
            reminder.notes = input[@"notes"];
        }

        // URL.
        if (input[@"url"] && input[@"url"] != [NSNull null]) {
            reminder.URL = [NSURL URLWithString:input[@"url"]];
        }

        // Priority.
        if (input[@"priority"] && input[@"priority"] != [NSNull null]) {
            reminder.priority = [input[@"priority"] integerValue];
        }

        // List (calendar).
        if (input[@"listName"] && input[@"listName"] != [NSNull null]) {
            NSString* listName = input[@"listName"];
            EKCalendar* cal = find_list_by_name(store, listName);
            if (!cal) {
                rem_set_error([NSString stringWithFormat:@"list not found: %@", listName]);
                return NULL;
            }
            reminder.calendar = cal;
        } else {
            reminder.calendar = [store defaultCalendarForNewReminders];
        }

        // Due date.
        if (input[@"dueDate"] && input[@"dueDate"] != [NSNull null]) {
            NSDate* dueDate = parse_iso_date([input[@"dueDate"] UTF8String]);
            if (dueDate) {
                NSCalendar* cal = [NSCalendar currentCalendar];
                NSUInteger units = NSCalendarUnitYear | NSCalendarUnitMonth | NSCalendarUnitDay |
                                   NSCalendarUnitHour | NSCalendarUnitMinute | NSCalendarUnitSecond;
                reminder.dueDateComponents = [cal components:units fromDate:dueDate];
            }
        }

        // Remind me date (alarm with absolute date).
        if (input[@"remindMeDate"] && input[@"remindMeDate"] != [NSNull null]) {
            NSDate* remindDate = parse_iso_date([input[@"remindMeDate"] UTF8String]);
            if (remindDate) {
                EKAlarm* alarm = [EKAlarm alarmWithAbsoluteDate:remindDate];
                [reminder addAlarm:alarm];
            }
        }

        // Additional alarms.
        if (input[@"alarms"] && input[@"alarms"] != [NSNull null]) {
            NSArray* alarmInputs = input[@"alarms"];
            for (NSDictionary* alarmInput in alarmInputs) {
                if (alarmInput[@"absoluteDate"] && alarmInput[@"absoluteDate"] != [NSNull null]) {
                    NSDate* absDate = parse_iso_date([alarmInput[@"absoluteDate"] UTF8String]);
                    if (absDate) {
                        [reminder addAlarm:[EKAlarm alarmWithAbsoluteDate:absDate]];
                    }
                } else if (alarmInput[@"relativeOffset"]) {
                    double offset = [alarmInput[@"relativeOffset"] doubleValue];
                    [reminder addAlarm:[EKAlarm alarmWithRelativeOffset:offset]];
                }
            }
        }

        // Recurrence rules.
        if (input[@"recurrenceRules"] && input[@"recurrenceRules"] != [NSNull null]) {
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
                [reminder addRecurrenceRule:rule];
            }
        }

        // Save via EventKit (no AppleScript!).
        NSError* saveError = nil;
        BOOL saved = [store saveReminder:reminder commit:YES error:&saveError];
        if (!saved) {
            rem_set_error([NSString stringWithFormat:@"failed to save reminder: %@",
                saveError.localizedDescription]);
            return NULL;
        }

        return to_json(reminder_to_dict(reminder));
    }
}

char* ek_rem_update_reminder(const char* reminder_id, const char* json_input) {
    @autoreleasepool {
        rem_set_error(nil);
        if (!reminder_id || !json_input) {
            rem_set_error(@"reminder ID and JSON input are required");
            return NULL;
        }

        EKEventStore* store = get_store();
        EKReminder* reminder = find_reminder_by_id([NSString stringWithUTF8String:reminder_id]);
        if (!reminder) {
            rem_set_error([NSString stringWithFormat:@"reminder not found: %s", reminder_id]);
            return NULL;
        }

        // Parse JSON input.
        NSData* data = [NSData dataWithBytes:json_input length:strlen(json_input)];
        NSError* parseError = nil;
        NSDictionary* input = [NSJSONSerialization JSONObjectWithData:data options:0 error:&parseError];
        if (!input) {
            rem_set_error([NSString stringWithFormat:@"invalid JSON: %@", parseError.localizedDescription]);
            return NULL;
        }

        // Update fields that are present in input.
        if (input[@"title"] && input[@"title"] != [NSNull null]) {
            reminder.title = input[@"title"];
        }
        if (input[@"notes"] != nil) {
            if (input[@"notes"] == [NSNull null]) {
                reminder.notes = nil;
            } else {
                reminder.notes = input[@"notes"];
            }
        }
        if (input[@"url"] != nil) {
            if (input[@"url"] == [NSNull null]) {
                reminder.URL = nil;
            } else {
                reminder.URL = [NSURL URLWithString:input[@"url"]];
            }
        }
        if (input[@"priority"] && input[@"priority"] != [NSNull null]) {
            reminder.priority = [input[@"priority"] integerValue];
        }
        if (input[@"completed"] && input[@"completed"] != [NSNull null]) {
            reminder.completed = [input[@"completed"] boolValue];
        }

        // Due date.
        if ([input objectForKey:@"dueDate"]) {
            if (input[@"dueDate"] == [NSNull null] || input[@"dueDate"] == nil) {
                reminder.dueDateComponents = nil;
            } else {
                NSDate* dueDate = parse_iso_date([input[@"dueDate"] UTF8String]);
                if (dueDate) {
                    NSCalendar* cal = [NSCalendar currentCalendar];
                    NSUInteger units = NSCalendarUnitYear | NSCalendarUnitMonth | NSCalendarUnitDay |
                                       NSCalendarUnitHour | NSCalendarUnitMinute | NSCalendarUnitSecond;
                    reminder.dueDateComponents = [cal components:units fromDate:dueDate];
                }
            }
        }

        // Remind me date (replace first alarm with absolute date).
        if (input[@"remindMeDate"] && input[@"remindMeDate"] != [NSNull null]) {
            // Remove existing alarms.
            for (EKAlarm* alarm in [reminder.alarms copy]) {
                [reminder removeAlarm:alarm];
            }
            NSDate* remindDate = parse_iso_date([input[@"remindMeDate"] UTF8String]);
            if (remindDate) {
                [reminder addAlarm:[EKAlarm alarmWithAbsoluteDate:remindDate]];
            }
        }

        // Alarms (replace all).
        if (input[@"alarms"] != nil) {
            for (EKAlarm* alarm in [reminder.alarms copy]) {
                [reminder removeAlarm:alarm];
            }
            if (input[@"alarms"] != [NSNull null]) {
                NSArray* alarmInputs = input[@"alarms"];
                for (NSDictionary* alarmInput in alarmInputs) {
                    if (alarmInput[@"absoluteDate"] && alarmInput[@"absoluteDate"] != [NSNull null]) {
                        NSDate* absDate = parse_iso_date([alarmInput[@"absoluteDate"] UTF8String]);
                        if (absDate) {
                            [reminder addAlarm:[EKAlarm alarmWithAbsoluteDate:absDate]];
                        }
                    } else if (alarmInput[@"relativeOffset"]) {
                        double offset = [alarmInput[@"relativeOffset"] doubleValue];
                        [reminder addAlarm:[EKAlarm alarmWithRelativeOffset:offset]];
                    }
                }
            }
        }

        // Recurrence rules (replace all).
        if (input[@"recurrenceRules"] != nil) {
            // Remove existing rules.
            for (EKRecurrenceRule* rule in [reminder.recurrenceRules copy]) {
                [reminder removeRecurrenceRule:rule];
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
                    [reminder addRecurrenceRule:rule];
                }
            }
        }

        // Move to different list.
        if (input[@"listName"] && input[@"listName"] != [NSNull null]) {
            NSString* listName = input[@"listName"];
            EKCalendar* cal = find_list_by_name(store, listName);
            if (!cal) {
                rem_set_error([NSString stringWithFormat:@"list not found: %@", listName]);
                return NULL;
            }
            reminder.calendar = cal;
        }

        // Save via EventKit.
        NSError* saveError = nil;
        BOOL saved = [store saveReminder:reminder commit:YES error:&saveError];
        if (!saved) {
            rem_set_error([NSString stringWithFormat:@"failed to update reminder: %@",
                saveError.localizedDescription]);
            return NULL;
        }

        return to_json(reminder_to_dict(reminder));
    }
}

char* ek_rem_delete_reminder(const char* reminder_id) {
    @autoreleasepool {
        rem_set_error(nil);
        if (!reminder_id) {
            rem_set_error(@"reminder ID is required");
            return NULL;
        }

        EKEventStore* store = get_store();
        EKReminder* reminder = find_reminder_by_id([NSString stringWithUTF8String:reminder_id]);
        if (!reminder) {
            rem_set_error([NSString stringWithFormat:@"reminder not found: %s", reminder_id]);
            return NULL;
        }

        NSError* removeError = nil;
        BOOL removed = [store removeReminder:reminder commit:YES error:&removeError];
        if (!removed) {
            rem_set_error([NSString stringWithFormat:@"failed to delete reminder: %@",
                removeError.localizedDescription]);
            return NULL;
        }

        return strdup("ok");
    }
}

// --- Find source by name (case-insensitive) ---

static EKSource* find_source_by_name(EKEventStore* store, NSString* name) {
    NSString* lowerName = [name lowercaseString];
    // Multiple sources can share the same title (e.g., "iCloud" for events
    // and "iCloud" for reminders). Prefer the one that has reminder calendars.
    EKSource* fallback = nil;
    for (EKSource* source in store.sources) {
        if ([[source.title lowercaseString] isEqualToString:lowerName]) {
            NSSet* remCals = [source calendarsForEntityType:EKEntityTypeReminder];
            if (remCals.count > 0) {
                return source;
            }
            if (!fallback) {
                fallback = source;
            }
        }
    }
    return fallback;
}

// --- Find list (calendar) by ID ---

static EKCalendar* find_list_by_id(EKEventStore* store, NSString* listId) {
    for (EKCalendar* cal in [store calendarsForEntityType:EKEntityTypeReminder]) {
        if ([cal.calendarIdentifier isEqualToString:listId]) {
            return cal;
        }
    }
    return nil;
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

// --- List CRUD ---

char* ek_rem_create_list(const char* json_input) {
    @autoreleasepool {
        rem_set_error(nil);
        if (!json_input) {
            rem_set_error(@"JSON input is required");
            return NULL;
        }

        EKEventStore* store = get_store();

        // Parse JSON input.
        NSData* data = [NSData dataWithBytes:json_input length:strlen(json_input)];
        NSError* parseError = nil;
        NSDictionary* input = [NSJSONSerialization JSONObjectWithData:data options:0 error:&parseError];
        if (!input) {
            rem_set_error([NSString stringWithFormat:@"invalid JSON: %@", parseError.localizedDescription]);
            return NULL;
        }

        EKCalendar* cal = [EKCalendar calendarForEntityType:EKEntityTypeReminder eventStore:store];

        // Title (required).
        cal.title = input[@"title"] ?: @"";

        // Source (required — validated in Go layer).
        EKSource* source = find_source_by_name(store, input[@"source"]);
        if (!source) {
            rem_set_error([NSString stringWithFormat:@"source not found: %@", input[@"source"]]);
            return NULL;
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
            rem_set_error([NSString stringWithFormat:@"failed to save list: %@",
                saveError.localizedDescription]);
            return NULL;
        }

        // Count is 0 for a newly created list.
        return to_json(list_to_dict(cal, 0));
    }
}

char* ek_rem_update_list(const char* list_id, const char* json_input) {
    @autoreleasepool {
        rem_set_error(nil);
        if (!list_id || !json_input) {
            rem_set_error(@"list ID and JSON input are required");
            return NULL;
        }

        EKEventStore* store = get_store();
        NSString* listIdStr = [NSString stringWithUTF8String:list_id];

        EKCalendar* cal = find_list_by_id(store, listIdStr);
        if (!cal) {
            rem_set_error([NSString stringWithFormat:@"list not found: %s", list_id]);
            return NULL;
        }

        // Check immutability.
        if (cal.isImmutable) {
            rem_set_error([NSString stringWithFormat:@"list is immutable: %@", cal.title]);
            return NULL;
        }

        // Parse JSON input.
        NSData* data = [NSData dataWithBytes:json_input length:strlen(json_input)];
        NSError* parseError = nil;
        NSDictionary* input = [NSJSONSerialization JSONObjectWithData:data options:0 error:&parseError];
        if (!input) {
            rem_set_error([NSString stringWithFormat:@"invalid JSON: %@", parseError.localizedDescription]);
            return NULL;
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
            rem_set_error([NSString stringWithFormat:@"failed to update list: %@",
                saveError.localizedDescription]);
            return NULL;
        }

        // Get updated reminder count.
        NSArray<EKReminder*>* reminders = fetch_all_reminders(@[cal]);
        return to_json(list_to_dict(cal, (int)reminders.count));
    }
}

char* ek_rem_delete_list(const char* list_id) {
    @autoreleasepool {
        rem_set_error(nil);
        if (!list_id) {
            rem_set_error(@"list ID is required");
            return NULL;
        }

        EKEventStore* store = get_store();
        NSString* listIdStr = [NSString stringWithUTF8String:list_id];

        EKCalendar* cal = find_list_by_id(store, listIdStr);
        if (!cal) {
            rem_set_error([NSString stringWithFormat:@"list not found: %s", list_id]);
            return NULL;
        }

        // Check immutability.
        if (cal.isImmutable) {
            rem_set_error([NSString stringWithFormat:@"list is immutable: %@", cal.title]);
            return NULL;
        }

        NSError* removeError = nil;
        BOOL removed = [store removeCalendar:cal commit:YES error:&removeError];
        if (!removed) {
            rem_set_error([NSString stringWithFormat:@"failed to delete list: %@",
                removeError.localizedDescription]);
            return NULL;
        }

        return strdup("ok");
    }
}

char* ek_rem_complete_reminder(const char* reminder_id) {
    @autoreleasepool {
        rem_set_error(nil);
        if (!reminder_id) {
            rem_set_error(@"reminder ID is required");
            return NULL;
        }

        EKEventStore* store = get_store();
        EKReminder* reminder = find_reminder_by_id([NSString stringWithUTF8String:reminder_id]);
        if (!reminder) {
            rem_set_error([NSString stringWithFormat:@"reminder not found: %s", reminder_id]);
            return NULL;
        }

        reminder.completed = YES;

        NSError* saveError = nil;
        BOOL saved = [store saveReminder:reminder commit:YES error:&saveError];
        if (!saved) {
            rem_set_error([NSString stringWithFormat:@"failed to complete reminder: %@",
                saveError.localizedDescription]);
            return NULL;
        }

        return to_json(reminder_to_dict(reminder));
    }
}

char* ek_rem_uncomplete_reminder(const char* reminder_id) {
    @autoreleasepool {
        rem_set_error(nil);
        if (!reminder_id) {
            rem_set_error(@"reminder ID is required");
            return NULL;
        }

        EKEventStore* store = get_store();
        EKReminder* reminder = find_reminder_by_id([NSString stringWithUTF8String:reminder_id]);
        if (!reminder) {
            rem_set_error([NSString stringWithFormat:@"reminder not found: %s", reminder_id]);
            return NULL;
        }

        reminder.completed = NO;

        NSError* saveError = nil;
        BOOL saved = [store saveReminder:reminder commit:YES error:&saveError];
        if (!saved) {
            rem_set_error([NSString stringWithFormat:@"failed to uncomplete reminder: %@",
                saveError.localizedDescription]);
            return NULL;
        }

        return to_json(reminder_to_dict(reminder));
    }
}
