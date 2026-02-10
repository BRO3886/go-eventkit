//go:build darwin

// Package store manages the shared EKEventStore singleton used by calendar and reminders packages.
package store

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework EventKit -framework Foundation
#include "store_darwin.h"
*/
import "C"
import "fmt"

// RequestCalendarAccess initializes the shared EKEventStore and requests
// calendar access from the user (TCC prompt on first call).
// Returns nil if access was granted, or an error if denied.
func RequestCalendarAccess() error {
	granted := C.ek_store_request_calendar_access()
	if granted == 0 {
		cerr := C.ek_store_last_error()
		if cerr != nil {
			return fmt.Errorf("%s", C.GoString(cerr))
		}
		return fmt.Errorf("calendar access denied")
	}
	return nil
}
