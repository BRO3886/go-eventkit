#ifndef BRIDGE_DARWIN_H
#define BRIDGE_DARWIN_H

typedef struct {
    char* result;
    char* error;
} ek_result_t;

// ek_cal_request_access requests calendar access.
// On success: result is "1". On failure: error is set.
ek_result_t ek_cal_request_access(void);

// ek_cal_fetch_calendars returns a JSON array of all calendars for events.
// Caller must free result with ek_cal_free.
ek_result_t ek_cal_fetch_calendars(void);

// ek_cal_fetch_events returns a JSON array of events within the given date range.
// start_date and end_date are ISO 8601 strings (required).
// calendar_id and search_query may be NULL to skip filtering.
// Caller must free result with ek_cal_free.
ek_result_t ek_cal_fetch_events(const char* start_date, const char* end_date,
                           const char* calendar_id, const char* search_query);

// ek_cal_get_event returns a single event as JSON by its eventIdentifier.
// Caller must free result with ek_cal_free.
ek_result_t ek_cal_get_event(const char* event_id);

// ek_cal_create_event creates a new event from the given JSON input.
// Returns the created event as JSON. Caller must free result.
ek_result_t ek_cal_create_event(const char* json_input);

// ek_cal_update_event updates an existing event.
// event_id is the eventIdentifier, json_input contains fields to update,
// span is 0 for this event only, 1 for future events.
// Returns the updated event as JSON. Caller must free result.
ek_result_t ek_cal_update_event(const char* event_id, const char* json_input, int span);

// ek_cal_delete_event deletes an event.
// span is 0 for this event only, 1 for future events.
// Returns "ok" on success. Caller must free result.
ek_result_t ek_cal_delete_event(const char* event_id, int span);

// ek_cal_create_calendar creates a new calendar from the given JSON input.
// Returns the created calendar as JSON. Caller must free result.
ek_result_t ek_cal_create_calendar(const char* json_input);

// ek_cal_update_calendar updates an existing calendar.
// calendar_id is the calendarIdentifier, json_input contains fields to update.
// Returns the updated calendar as JSON. Caller must free result.
ek_result_t ek_cal_update_calendar(const char* calendar_id, const char* json_input);

// ek_cal_delete_calendar deletes a calendar.
// Returns "ok" on success. Caller must free result.
ek_result_t ek_cal_delete_calendar(const char* calendar_id);

// ek_cal_delete_events deletes multiple events in a single bridge call.
// json_ids is a JSON array of event identifiers.
// span is 0 for this event only, 1 for future events.
// Returns a JSON object mapping failed event IDs to error messages.
// Caller must free result with ek_cal_free.
ek_result_t ek_cal_delete_events(const char* json_ids, int span);

// ek_cal_free frees a string returned by the above functions.
void ek_cal_free(char* ptr);

// ek_cal_watch_start registers EKEventStoreChangedNotification observer and
// creates a pipe. Returns 1 on success, 0 on failure.
int ek_cal_watch_start(void);

// ek_cal_watch_read_fd returns the read end of the notification pipe, or -1.
int ek_cal_watch_read_fd(void);

// ek_cal_watch_stop removes the observer and closes the pipe.
void ek_cal_watch_stop(void);

#endif
