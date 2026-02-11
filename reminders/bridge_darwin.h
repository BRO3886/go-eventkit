#ifndef REMINDERS_BRIDGE_DARWIN_H
#define REMINDERS_BRIDGE_DARWIN_H

// ek_rem_request_access initializes the shared EKEventStore and requests
// reminder access (TCC prompt on first use).
// Returns 1 if access was granted, 0 otherwise.
int ek_rem_request_access(void);

// ek_rem_fetch_lists returns a JSON array of all reminder lists.
// Caller must free the returned string with ek_rem_free.
char* ek_rem_fetch_lists(void);

// ek_rem_fetch_reminders returns a JSON array of reminders matching the given filters.
// All filter parameters may be NULL to skip that filter.
// completed_filter: "true" = completed only, "false" = incomplete only, NULL = all.
// Caller must free the returned string with ek_rem_free.
char* ek_rem_fetch_reminders(const char* list_name,
                              const char* completed_filter,
                              const char* search_query,
                              const char* due_before,
                              const char* due_after);

// ek_rem_get_reminder returns a single reminder as JSON by ID or ID prefix.
// Caller must free the returned string with ek_rem_free.
// Returns NULL if not found.
char* ek_rem_get_reminder(const char* target_id);

// ek_rem_create_reminder creates a new reminder from JSON input.
// Returns the created reminder as JSON, or NULL on error.
// Caller must free the returned string with ek_rem_free.
char* ek_rem_create_reminder(const char* json_input);

// ek_rem_update_reminder updates an existing reminder.
// Only fields present in json_input are updated.
// Returns the updated reminder as JSON, or NULL on error.
char* ek_rem_update_reminder(const char* reminder_id, const char* json_input);

// ek_rem_delete_reminder deletes a reminder by ID.
// Returns "ok" on success, NULL on error.
char* ek_rem_delete_reminder(const char* reminder_id);

// ek_rem_complete_reminder marks a reminder as completed.
// Returns the updated reminder as JSON, or NULL on error.
char* ek_rem_complete_reminder(const char* reminder_id);

// ek_rem_uncomplete_reminder marks a reminder as incomplete.
// Returns the updated reminder as JSON, or NULL on error.
char* ek_rem_uncomplete_reminder(const char* reminder_id);

// ek_rem_create_list creates a new reminder list from the given JSON input.
// Returns the created list as JSON. Caller must free.
// Returns NULL on error.
char* ek_rem_create_list(const char* json_input);

// ek_rem_update_list updates an existing reminder list.
// list_id is the calendarIdentifier, json_input contains fields to update.
// Returns the updated list as JSON. Caller must free.
// Returns NULL on error (including immutable lists).
char* ek_rem_update_list(const char* list_id, const char* json_input);

// ek_rem_delete_list deletes a reminder list.
// Returns "ok" on success. Caller must free.
// Returns NULL on error (including immutable lists).
char* ek_rem_delete_list(const char* list_id);

// ek_rem_free frees a string returned by the above functions.
void ek_rem_free(char* ptr);

// ek_rem_last_error returns the last error message, or NULL if no error.
const char* ek_rem_last_error(void);

#endif
