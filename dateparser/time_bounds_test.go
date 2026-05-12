package dateparser

import (
	"testing"
	"time"
)

func TestStartOfDay(t *testing.T) {
	loc := time.UTC
	noon := time.Date(2026, 5, 8, 12, 30, 45, 0, loc)
	got := StartOfDay(noon)
	if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 {
		t.Errorf("StartOfDay = %v, want 00:00:00", got)
	}
	if got.Day() != 8 {
		t.Errorf("StartOfDay changed the day: %v", got)
	}
	// Preserves location.
	la, _ := time.LoadLocation("America/Los_Angeles")
	t2 := time.Date(2026, 5, 8, 12, 0, 0, 0, la)
	if StartOfDay(t2).Location() != la {
		t.Errorf("StartOfDay dropped location")
	}
}

func TestEndOfDayIfMidnight(t *testing.T) {
	loc := time.UTC
	midnight := time.Date(2026, 5, 8, 0, 0, 0, 0, loc)
	got := EndOfDayIfMidnight(midnight)
	if got.Hour() != 23 || got.Minute() != 59 || got.Second() != 59 {
		t.Errorf("EndOfDayIfMidnight(midnight) = %v, want 23:59:59 of same day", got)
	}
	if got.Day() != 8 {
		t.Errorf("EndOfDayIfMidnight changed the day: %v", got)
	}
	noon := time.Date(2026, 5, 8, 12, 0, 0, 0, loc)
	if !EndOfDayIfMidnight(noon).Equal(noon) {
		t.Errorf("EndOfDayIfMidnight(noon) should be unchanged")
	}
	// Even 00:00:00.000000001 is not midnight by this definition; verify.
	nearMidnight := time.Date(2026, 5, 8, 0, 0, 0, 1, loc)
	if !EndOfDayIfMidnight(nearMidnight).Equal(nearMidnight) {
		t.Errorf("EndOfDayIfMidnight with non-zero nanos should be unchanged")
	}
}
