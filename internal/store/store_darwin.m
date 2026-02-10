#import <EventKit/EventKit.h>
#import <Foundation/Foundation.h>
#include "store_darwin.h"
#include <stdlib.h>
#include <string.h>

// Thread-local error message for the store package.
static __thread char* store_last_error = NULL;

static void store_set_error(NSString* msg) {
    if (store_last_error) {
        free(store_last_error);
        store_last_error = NULL;
    }
    if (msg) {
        store_last_error = strdup([msg UTF8String]);
    }
}

const char* ek_store_last_error(void) {
    return store_last_error;
}

void ek_store_free(char* ptr) {
    if (ptr) free(ptr);
}

// Shared EKEventStore singleton — initialized once via dispatch_once.
// The store is shared across calendar and reminders packages.
static EKEventStore* shared_store = nil;
static dispatch_once_t store_once_token;
static BOOL calendar_access_granted = NO;

EKEventStore* _ek_get_shared_store(void) {
    dispatch_once(&store_once_token, ^{
        shared_store = [[EKEventStore alloc] init];
    });
    return shared_store;
}

int ek_store_request_calendar_access(void) {
    @autoreleasepool {
        store_set_error(nil);
        EKEventStore* store = _ek_get_shared_store();

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

        calendar_access_granted = granted;
        if (!granted) {
            if (accessError) {
                store_set_error([NSString stringWithFormat:@"calendar access denied: %@",
                    accessError.localizedDescription]);
            } else {
                store_set_error(@"calendar access denied");
            }
            return 0;
        }
        return 1;
    }
}
