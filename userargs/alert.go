package userargs

import (
	"fmt"
	"strings"
	"time"

	"github.com/BRO3886/go-eventkit/dateparser"
)

// MaxAlertOffset bounds how far before an event an alert may fire. EventKit
// itself permits larger values but anything beyond a year is almost always
// a parsing bug (e.g., dateparser.ParseAlertDuration overflowing on
// pathological inputs like "99999999999999999d").
const MaxAlertOffset = 365 * 24 * time.Hour

// ParseAlertOffsets converts strings like "15m", "1h", "1d" into non-negative
// durations representing the offset *before* the event. Callers wrap each
// result into the native alert/alarm type:
//
//	for _, d := range offsets {
//	    alerts = append(alerts, calendar.Alert{RelativeOffset: -d})
//	}
//
// A zero offset is valid and means "alert at the time of the event" — every
// calendar UI offers it, so it is passed through rather than rejected.
//
// Returns an error if any input fails dateparser.ParseAlertDuration, is
// negative, or exceeds MaxAlertOffset. The negation is intentionally
// *not* applied here — callers may want positive offsets for after-event
// alerts in some contexts.
func ParseAlertOffsets(in []string) ([]time.Duration, error) {
	out := make([]time.Duration, 0, len(in))
	for _, s := range in {
		d, err := dateparser.ParseAlertDuration(strings.TrimSpace(s))
		if err != nil {
			return nil, fmt.Errorf("invalid alert offset %q: %w", s, err)
		}
		if d < 0 {
			return nil, fmt.Errorf("alert offset %q must not be negative", s)
		}
		if d > MaxAlertOffset {
			return nil, fmt.Errorf("alert offset %q exceeds one year before event", s)
		}
		out = append(out, d)
	}
	return out, nil
}
