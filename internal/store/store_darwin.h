#ifndef STORE_DARWIN_H
#define STORE_DARWIN_H

// ek_store_get_store initializes the shared EKEventStore singleton (dispatch_once)
// and requests calendar access (TCC prompt on first use).
// Returns 1 if calendar access was granted, 0 otherwise.
int ek_store_request_calendar_access(void);

// ek_store_last_error returns the last error message, or NULL if no error.
const char* ek_store_last_error(void);

// ek_store_free frees a string allocated by the store functions.
void ek_store_free(char* ptr);

#endif
