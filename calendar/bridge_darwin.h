#ifndef BRIDGE_DARWIN_H
#define BRIDGE_DARWIN_H

// ek_cal_request_access requests calendar access and returns 1 if granted, 0 if denied.
int ek_cal_request_access(void);

// ek_cal_fetch_calendars returns a JSON array of all calendars for events.
// Caller must free the returned string with ek_cal_free.
// Returns NULL on error (check ek_cal_last_error).
char* ek_cal_fetch_calendars(void);

// ek_cal_fetch_events returns a JSON array of events within the given date range.
// start_date and end_date are ISO 8601 strings (required).
// calendar_id and search_query may be NULL to skip filtering.
// Caller must free the returned string with ek_cal_free.
char* ek_cal_fetch_events(const char* start_date, const char* end_date,
                           const char* calendar_id, const char* search_query);

// ek_cal_get_event returns a single event as JSON by its eventIdentifier.
// Caller must free the returned string with ek_cal_free.
// Returns NULL if not found.
char* ek_cal_get_event(const char* event_id);

// ek_cal_create_event creates a new event from the given JSON input.
// Returns the created event as JSON. Caller must free.
// Returns NULL on error.
char* ek_cal_create_event(const char* json_input);

// ek_cal_update_event updates an existing event.
// event_id is the eventIdentifier, json_input contains fields to update,
// span is 0 for this event only, 1 for future events.
// Returns the updated event as JSON. Caller must free.
// Returns NULL on error.
char* ek_cal_update_event(const char* event_id, const char* json_input, int span);

// ek_cal_delete_event deletes an event.
// span is 0 for this event only, 1 for future events.
// Returns "ok" on success. Caller must free.
// Returns NULL on error.
char* ek_cal_delete_event(const char* event_id, int span);

// ek_cal_free frees a string returned by the above functions.
void ek_cal_free(char* ptr);

// ek_cal_last_error returns the last error message, or NULL if no error.
const char* ek_cal_last_error(void);

#endif
