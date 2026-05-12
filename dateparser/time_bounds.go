package dateparser

import "time"

// StartOfDay returns t at 00:00:00 in t's location.
func StartOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// EndOfDayIfMidnight bumps a midnight time to 23:59:59 (same date, same
// location). Times with explicit hours are returned unchanged.
//
// This makes "from=Feb 12, to=Feb 12" a same-day inclusive range rather
// than a zero-duration range. Useful for CLI/MCP date-range args where
// the user means "the whole day."
func EndOfDayIfMidnight(t time.Time) time.Time {
	if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0 {
		return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
	}
	return t
}
