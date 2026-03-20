package dateparser

import (
	"testing"
	"time"
)

// Fixed reference time for deterministic tests: Wednesday, Feb 11, 2026 10:30:00 IST
func refTime(loc *time.Location) time.Time {
	return time.Date(2026, 2, 11, 10, 30, 0, 0, loc)
}

func mustLoadLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("failed to load location %q: %v", name, err)
	}
	return loc
}

func TestKeywords(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist)

	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{"today", "today", time.Date(2026, 2, 11, 0, 0, 0, 0, ist)},
		{"tomorrow", "tomorrow", time.Date(2026, 2, 12, 0, 0, 0, 0, ist)},
		{"yesterday", "yesterday", time.Date(2026, 2, 10, 0, 0, 0, 0, ist)},
		{"now", "now", now},
		{"eod", "eod", time.Date(2026, 2, 11, 17, 0, 0, 0, ist)},
		{"end of day", "end of day", time.Date(2026, 2, 11, 17, 0, 0, 0, ist)},
		{"next week", "next week", time.Date(2026, 2, 16, 0, 0, 0, 0, ist)},
		{"next month", "next month", time.Date(2026, 3, 1, 0, 0, 0, 0, ist)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateRelativeTo(tt.input, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeywords_WithDefaultHour(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist)
	opts := []Option{WithDefaultHour(9)}

	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{"today", "today", time.Date(2026, 2, 11, 9, 0, 0, 0, ist)},
		{"tomorrow", "tomorrow", time.Date(2026, 2, 12, 9, 0, 0, 0, ist)},
		{"yesterday", "yesterday", time.Date(2026, 2, 10, 9, 0, 0, 0, ist)},
		{"now unchanged", "now", now},
		{"eod unchanged", "eod", time.Date(2026, 2, 11, 17, 0, 0, 0, ist)},
		{"next week", "next week", time.Date(2026, 2, 16, 9, 0, 0, 0, ist)},
		{"next month", "next month", time.Date(2026, 3, 1, 9, 0, 0, 0, ist)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateRelativeTo(tt.input, now, opts...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEOW(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")

	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			"wednesday to friday",
			time.Date(2026, 2, 11, 10, 0, 0, 0, ist),
			time.Date(2026, 2, 13, 17, 0, 0, 0, ist),
		},
		{
			"friday returns same friday",
			time.Date(2026, 2, 13, 10, 0, 0, 0, ist),
			time.Date(2026, 2, 13, 17, 0, 0, 0, ist),
		},
		{
			"saturday to next friday",
			time.Date(2026, 2, 14, 10, 0, 0, 0, ist),
			time.Date(2026, 2, 20, 17, 0, 0, 0, ist),
		},
		{
			"sunday to next friday",
			time.Date(2026, 2, 15, 10, 0, 0, 0, ist),
			time.Date(2026, 2, 20, 17, 0, 0, 0, ist),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateRelativeTo("eow", tt.now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEOW_WithSkipToday(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")

	// On Friday with WithEOWSkipToday, should jump to next Friday
	friday := time.Date(2026, 2, 13, 10, 0, 0, 0, ist)
	got, err := ParseDateRelativeTo("eow", friday, WithEOWSkipToday())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 2, 20, 17, 0, 0, 0, ist)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// On Wednesday, WithEOWSkipToday should not change behavior
	wed := time.Date(2026, 2, 11, 10, 0, 0, 0, ist)
	got, err = ParseDateRelativeTo("eow", wed, WithEOWSkipToday())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want = time.Date(2026, 2, 13, 17, 0, 0, 0, ist)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEOW_DefaultHourDoesNotAffectEOW(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := time.Date(2026, 2, 11, 10, 0, 0, 0, ist) // Wed

	got, err := ParseDateRelativeTo("eow", now, WithDefaultHour(9))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// EOW always uses 17:00, regardless of defaultHour
	want := time.Date(2026, 2, 13, 17, 0, 0, 0, ist)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestThisWeek(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")

	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			"wednesday to sunday",
			time.Date(2026, 2, 11, 10, 0, 0, 0, ist),
			time.Date(2026, 2, 15, 23, 59, 0, 0, ist),
		},
		{
			"monday to sunday",
			time.Date(2026, 2, 9, 10, 0, 0, 0, ist),
			time.Date(2026, 2, 15, 23, 59, 0, 0, ist),
		},
		{
			"saturday to sunday",
			time.Date(2026, 2, 14, 10, 0, 0, 0, ist),
			time.Date(2026, 2, 15, 23, 59, 0, 0, ist),
		},
		{
			"sunday returns same sunday",
			time.Date(2026, 2, 15, 10, 0, 0, 0, ist),
			time.Date(2026, 2, 15, 23, 59, 0, 0, ist),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateRelativeTo("this week", tt.now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRelative(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist)

	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{"in 3 hours", "in 3 hours", now.Add(3 * time.Hour)},
		{"in 1 hour", "in 1 hour", now.Add(1 * time.Hour)},
		{"in 30 minutes", "in 30 minutes", now.Add(30 * time.Minute)},
		{"in 30 mins", "in 30 mins", now.Add(30 * time.Minute)},
		{"in 5 days", "in 5 days", now.AddDate(0, 0, 5)},
		{"in 1 day", "in 1 day", now.AddDate(0, 0, 1)},
		{"in 2 weeks", "in 2 weeks", now.AddDate(0, 0, 14)},
		{"in 1 week", "in 1 week", now.AddDate(0, 0, 7)},
		{"in 3 months", "in 3 months", now.AddDate(0, 3, 0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateRelativeTo(tt.input, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAgo(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist)

	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{"2 hours ago", "2 hours ago", now.Add(-2 * time.Hour)},
		{"1 hour ago", "1 hour ago", now.Add(-1 * time.Hour)},
		{"30 minutes ago", "30 minutes ago", now.Add(-30 * time.Minute)},
		{"5 days ago", "5 days ago", now.AddDate(0, 0, -5)},
		{"2 weeks ago", "2 weeks ago", now.AddDate(0, 0, -14)},
		{"1 month ago", "1 month ago", now.AddDate(0, -1, 0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateRelativeTo(tt.input, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNextWeekday(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist) // Wednesday

	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{"next monday", "next monday", time.Date(2026, 2, 16, 0, 0, 0, 0, ist)},
		{"next friday", "next friday", time.Date(2026, 2, 13, 0, 0, 0, 0, ist)},
		{"next wednesday (same day)", "next wednesday", time.Date(2026, 2, 18, 0, 0, 0, 0, ist)},
		{"next sunday", "next sunday", time.Date(2026, 2, 15, 0, 0, 0, 0, ist)},
		{"next sat", "next sat", time.Date(2026, 2, 14, 0, 0, 0, 0, ist)},
		{"next monday at 2pm", "next monday at 2pm", time.Date(2026, 2, 16, 14, 0, 0, 0, ist)},
		{"next friday at 3:30pm", "next friday at 3:30pm", time.Date(2026, 2, 13, 15, 30, 0, 0, ist)},
		{"next monday 10am", "next monday 10am", time.Date(2026, 2, 16, 10, 0, 0, 0, ist)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateRelativeTo(tt.input, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNextWeekday_WithDefaultHour(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist) // Wednesday

	// Bare "next monday" with WithDefaultHour(9) -> 9am
	got, err := ParseDateRelativeTo("next monday", now, WithDefaultHour(9))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 2, 16, 9, 0, 0, 0, ist)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// With explicit time, defaultHour is overridden
	got, err = ParseDateRelativeTo("next monday at 2pm", now, WithDefaultHour(9))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want = time.Date(2026, 2, 16, 14, 0, 0, 0, ist)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDateWithTime(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist)

	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{"today at 5pm", "today at 5pm", time.Date(2026, 2, 11, 17, 0, 0, 0, ist)},
		{"tomorrow at 3:30pm", "tomorrow at 3:30pm", time.Date(2026, 2, 12, 15, 30, 0, 0, ist)},
		{"today at 9am", "today at 9am", time.Date(2026, 2, 11, 9, 0, 0, 0, ist)},
		{"today at 17:00", "today at 17:00", time.Date(2026, 2, 11, 17, 0, 0, 0, ist)},
		{"today 5pm", "today 5pm", time.Date(2026, 2, 11, 17, 0, 0, 0, ist)},
		{"tomorrow 9am", "tomorrow 9am", time.Date(2026, 2, 12, 9, 0, 0, 0, ist)},
		{"yesterday 3pm", "yesterday 3pm", time.Date(2026, 2, 10, 15, 0, 0, 0, ist)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateRelativeTo(tt.input, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWeekdayWithTime(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist) // Wednesday

	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{"friday 2pm", "friday 2pm", time.Date(2026, 2, 13, 14, 0, 0, 0, ist)},
		{"monday 10:00", "monday 10:00", time.Date(2026, 2, 16, 10, 0, 0, 0, ist)},
		{"sunday 8am", "sunday 8am", time.Date(2026, 2, 15, 8, 0, 0, 0, ist)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateRelativeTo(tt.input, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMonthDay(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist)

	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{"mar 15", "mar 15", time.Date(2026, 3, 15, 0, 0, 0, 0, ist)},
		{"march 15", "march 15", time.Date(2026, 3, 15, 0, 0, 0, 0, ist)},
		{"dec 25", "dec 25", time.Date(2026, 12, 25, 0, 0, 0, 0, ist)},
		{"jan 1", "jan 1", time.Date(2026, 1, 1, 0, 0, 0, 0, ist)},
		{"mar 15 2pm", "mar 15 2pm", time.Date(2026, 3, 15, 14, 0, 0, 0, ist)},
		{"december 31 11:59pm", "december 31 11:59pm", time.Date(2026, 12, 31, 23, 59, 0, 0, ist)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateRelativeTo(tt.input, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMonthDay_WithDefaultHour(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist)

	// Bare "mar 15" with WithDefaultHour(9) -> 9am
	got, err := ParseDateRelativeTo("mar 15", now, WithDefaultHour(9))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 3, 15, 9, 0, 0, 0, ist)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// With explicit time, defaultHour is overridden
	got, err = ParseDateRelativeTo("mar 15 2pm", now, WithDefaultHour(9))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want = time.Date(2026, 3, 15, 14, 0, 0, 0, ist)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDayFirstDates(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist)

	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{"21 mar", "21 mar", time.Date(2026, 3, 21, 0, 0, 0, 0, ist)},
		{"21 March", "21 March", time.Date(2026, 3, 21, 0, 0, 0, 0, ist)},
		{"21 mar 2pm", "21 mar 2pm", time.Date(2026, 3, 21, 14, 0, 0, 0, ist)},
		{"21 march at 2pm", "21 march at 2pm", time.Date(2026, 3, 21, 14, 0, 0, 0, ist)},
		{"21 mar 2:30pm", "21 mar 2:30pm", time.Date(2026, 3, 21, 14, 30, 0, 0, ist)},
		{"15 jan", "15 jan", time.Date(2026, 1, 15, 0, 0, 0, 0, ist)},
		{"1 dec 9am", "1 dec 9am", time.Date(2026, 12, 1, 9, 0, 0, 0, ist)},
		{"21 march 2026", "21 march 2026", time.Date(2026, 3, 21, 0, 0, 0, 0, ist)},
		{"21 march 2027", "21 march 2027", time.Date(2027, 3, 21, 0, 0, 0, 0, ist)},
		{"21 mar 2026 2pm", "21 mar 2026 2pm", time.Date(2026, 3, 21, 14, 0, 0, 0, ist)},
		{"ISO bare hour 2PM", "2026-03-21 2PM", time.Date(2026, 3, 21, 14, 0, 0, 0, ist)},
		{"ISO bare hour 2pm", "2026-03-21 2pm", time.Date(2026, 3, 21, 14, 0, 0, 0, ist)},
		{"21 mar 14:00", "21 mar 14:00", time.Date(2026, 3, 21, 14, 0, 0, 0, ist)},
		{"single digit day", "1 mar", time.Date(2026, 3, 1, 0, 0, 0, 0, ist)},
		{"case insensitive 21 MAR", "21 MAR", time.Date(2026, 3, 21, 0, 0, 0, 0, ist)},
		{"case insensitive 21 Mar", "21 Mar", time.Date(2026, 3, 21, 0, 0, 0, 0, ist)},
		{"month-first with year", "mar 21 2026", time.Date(2026, 3, 21, 0, 0, 0, 0, ist)},
		{"month-first with future year", "march 15 2027", time.Date(2027, 3, 15, 0, 0, 0, 0, ist)},
		{"month-first year and time", "mar 15 2026 3pm", time.Date(2026, 3, 15, 15, 0, 0, 0, ist)},
		{"month-first with at", "mar 15 at 2pm", time.Date(2026, 3, 15, 14, 0, 0, 0, ist)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateRelativeTo(tt.input, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDayFirstInvalidDates(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist)

	inputs := []string{
		"31 feb",
		"32 mar",
		"0 jan",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := ParseDateRelativeTo(input, now)
			if err == nil {
				t.Errorf("expected error for input %q", input)
			}
		})
	}
}

func TestStandaloneWeekday(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist) // Wednesday

	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{"friday", "friday", time.Date(2026, 2, 13, 0, 0, 0, 0, ist)},
		{"monday", "monday", time.Date(2026, 2, 16, 0, 0, 0, 0, ist)},
		{"sun", "sun", time.Date(2026, 2, 15, 0, 0, 0, 0, ist)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateRelativeTo(tt.input, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStandaloneWeekday_WithDefaultHour(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist) // Wednesday

	got, err := ParseDateRelativeTo("friday", now, WithDefaultHour(9))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 2, 13, 9, 0, 0, 0, ist)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestTimeOnly(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist) // 10:30 AM

	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{"5pm", "5pm", time.Date(2026, 2, 11, 17, 0, 0, 0, ist)},
		{"9am", "9am", time.Date(2026, 2, 11, 9, 0, 0, 0, ist)},
		{"3:30pm", "3:30pm", time.Date(2026, 2, 11, 15, 30, 0, 0, ist)},
		{"17:00", "17:00", time.Date(2026, 2, 11, 17, 0, 0, 0, ist)},
		{"12am", "12am", time.Date(2026, 2, 11, 0, 0, 0, 0, ist)},
		{"12pm", "12pm", time.Date(2026, 2, 11, 12, 0, 0, 0, ist)},
		{"9:30", "9:30", time.Date(2026, 2, 11, 9, 30, 0, 0, ist)},
		{"23:59", "23:59", time.Date(2026, 2, 11, 23, 59, 0, 0, ist)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateRelativeTo(tt.input, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTimeOnly_WithSmartRollover(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist) // 10:30 AM

	// 9am is past -> rolls to tomorrow
	got, err := ParseDateRelativeTo("9am", now, WithSmartTimeRollover())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 2, 12, 9, 0, 0, 0, ist)
	if !got.Equal(want) {
		t.Errorf("9am past: got %v, want %v", got, want)
	}

	// 5pm is future -> stays today
	got, err = ParseDateRelativeTo("5pm", now, WithSmartTimeRollover())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want = time.Date(2026, 2, 11, 17, 0, 0, 0, ist)
	if !got.Equal(want) {
		t.Errorf("5pm future: got %v, want %v", got, want)
	}
}

func TestISOFormats(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist)

	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{"ISO date", "2026-03-15", time.Date(2026, 3, 15, 0, 0, 0, 0, ist)},
		{"ISO datetime", "2026-03-15 14:30", time.Date(2026, 3, 15, 14, 30, 0, 0, ist)},
		{"ISO datetime T", "2026-03-15T14:30", time.Date(2026, 3, 15, 14, 30, 0, 0, ist)},
		{"ISO datetime seconds", "2026-03-15T14:30:00", time.Date(2026, 3, 15, 14, 30, 0, 0, ist)},
		{"US format", "03/15/2026", time.Date(2026, 3, 15, 0, 0, 0, 0, ist)},
		{"English date", "Jan 15, 2026", time.Date(2026, 1, 15, 0, 0, 0, 0, ist)},
		{"English full month", "January 15, 2026", time.Date(2026, 1, 15, 0, 0, 0, 0, ist)},
		{"ISO date with 12h time", "2026-03-15 2:30PM", time.Date(2026, 3, 15, 14, 30, 0, 0, ist)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateRelativeTo(tt.input, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInvalidInputs(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist)

	inputs := []string{
		"",
		"not-a-date",
		"gibberish foo bar",
		"in many days",
		"next invalid",
		"25:00",
		"99:99",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := ParseDateRelativeTo(input, now)
			if err == nil {
				t.Errorf("expected error for input %q", input)
			}
		})
	}
}

func TestDifferentTimezones(t *testing.T) {
	timezones := []string{
		"America/New_York",
		"America/Los_Angeles",
		"Europe/London",
		"Europe/Berlin",
		"Asia/Tokyo",
		"Asia/Kolkata",
		"Australia/Sydney",
		"Pacific/Auckland",
		"UTC",
	}

	for _, tz := range timezones {
		loc := mustLoadLoc(t, tz)
		now := time.Date(2026, 6, 15, 14, 30, 0, 0, loc)

		t.Run(tz+"/today", func(t *testing.T) {
			got, err := ParseDateRelativeTo("today", now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Location() != loc {
				t.Errorf("location mismatch: got %v, want %v", got.Location(), loc)
			}
			if got.Day() != 15 {
				t.Errorf("day mismatch: got %d, want 15", got.Day())
			}
		})

		t.Run(tz+"/tomorrow", func(t *testing.T) {
			got, err := ParseDateRelativeTo("tomorrow", now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Day() != 16 {
				t.Errorf("day mismatch: got %d, want 16", got.Day())
			}
		})

		t.Run(tz+"/time_3pm", func(t *testing.T) {
			got, err := ParseDateRelativeTo("3pm", now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Hour() != 15 {
				t.Errorf("hour mismatch: got %d, want 15", got.Hour())
			}
			if got.Location() != loc {
				t.Errorf("location mismatch: got %v, want %v", got.Location(), loc)
			}
		})
	}
}

func TestDSTTransition(t *testing.T) {
	ny := mustLoadLoc(t, "America/New_York")

	// Spring forward: March 8, 2026, 2:00 AM -> 3:00 AM
	beforeDST := time.Date(2026, 3, 7, 23, 0, 0, 0, ny)

	t.Run("tomorrow across spring forward", func(t *testing.T) {
		got, err := ParseDateRelativeTo("tomorrow", beforeDST)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Day() != 8 || got.Month() != 3 {
			t.Errorf("got %v, want March 8", got)
		}
		if got.Location() != ny {
			t.Errorf("location should be America/New_York")
		}
	})

	t.Run("in 24 hours across spring forward", func(t *testing.T) {
		got, err := ParseDateRelativeTo("in 24 hours", beforeDST)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := beforeDST.Add(24 * time.Hour)
		if !got.Equal(expected) {
			t.Errorf("got %v, want %v", got, expected)
		}
	})

	// Fall back: November 1, 2026, 2:00 AM -> 1:00 AM
	beforeFallBack := time.Date(2026, 10, 31, 23, 0, 0, 0, ny)

	t.Run("tomorrow across fall back", func(t *testing.T) {
		got, err := ParseDateRelativeTo("tomorrow", beforeFallBack)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Day() != 1 || got.Month() != 11 {
			t.Errorf("got %v, want November 1", got)
		}
	})
}

func TestYearBoundary(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := time.Date(2026, 12, 31, 23, 0, 0, 0, ist)

	t.Run("tomorrow crosses year", func(t *testing.T) {
		got, err := ParseDateRelativeTo("tomorrow", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Year() != 2027 || got.Month() != 1 || got.Day() != 1 {
			t.Errorf("got %v, want 2027-01-01", got)
		}
	})

	t.Run("in 2 hours crosses year", func(t *testing.T) {
		got, err := ParseDateRelativeTo("in 2 hours", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Year() != 2027 {
			t.Errorf("got year %d, want 2027", got.Year())
		}
	})
}

func TestMonthBoundary(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")

	t.Run("feb 28 + 1 day in non-leap year", func(t *testing.T) {
		now := time.Date(2025, 2, 28, 10, 0, 0, 0, ist)
		got, err := ParseDateRelativeTo("tomorrow", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Month() != 3 || got.Day() != 1 {
			t.Errorf("got %v, want March 1", got)
		}
	})

	t.Run("feb 28 + 1 day in leap year", func(t *testing.T) {
		now := time.Date(2028, 2, 28, 10, 0, 0, 0, ist)
		got, err := ParseDateRelativeTo("tomorrow", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Month() != 2 || got.Day() != 29 {
			t.Errorf("got %v, want Feb 29", got)
		}
	})

	t.Run("in 1 month from jan 31", func(t *testing.T) {
		now := time.Date(2026, 1, 31, 10, 0, 0, 0, ist)
		got, err := ParseDateRelativeTo("in 1 month", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Month() != 3 {
			t.Errorf("got month %v, want March (Go normalizes Jan 31 + 1 month)", got.Month())
		}
	})
}

func TestHalfHourTimezone(t *testing.T) {
	india := mustLoadLoc(t, "Asia/Kolkata")
	nepal := mustLoadLoc(t, "Asia/Kathmandu")

	for _, tc := range []struct {
		name string
		loc  *time.Location
	}{
		{"India UTC+5:30", india},
		{"Nepal UTC+5:45", nepal},
	} {
		now := time.Date(2026, 6, 15, 10, 30, 0, 0, tc.loc)

		t.Run(tc.name+"/3pm", func(t *testing.T) {
			got, err := ParseDateRelativeTo("3pm", now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Hour() != 15 || got.Minute() != 0 {
				t.Errorf("got %02d:%02d, want 15:00", got.Hour(), got.Minute())
			}
			if got.Location() != tc.loc {
				t.Errorf("location mismatch: got %v, want %v", got.Location(), tc.loc)
			}
		})
	}
}

func TestRFC3339(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist)

	t.Run("RFC3339 with timezone offset", func(t *testing.T) {
		got, err := ParseDateRelativeTo("2026-03-15T14:30:00+05:30", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Hour() != 14 || got.Minute() != 30 {
			t.Errorf("got %02d:%02d, want 14:30", got.Hour(), got.Minute())
		}
	})

	t.Run("RFC3339 UTC", func(t *testing.T) {
		got, err := ParseDateRelativeTo("2026-03-15T09:00:00Z", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Hour() != 9 || got.Minute() != 0 {
			t.Errorf("got %02d:%02d, want 09:00", got.Hour(), got.Minute())
		}
	})
}

func TestCaseInsensitive(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist)

	inputs := []string{"Today", "TOMORROW", "Next Monday", "In 3 Hours", "EOD", "This Week"}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := ParseDateRelativeTo(input, now)
			if err != nil {
				t.Errorf("should be case insensitive but got error: %v", err)
			}
		})
	}
}

func TestWhitespaceHandling(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist)

	inputs := []string{"  today  ", " tomorrow ", "  in 3 hours  "}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := ParseDateRelativeTo(input, now)
			if err != nil {
				t.Errorf("should handle whitespace but got error: %v", err)
			}
		})
	}
}

func TestMidnightBoundary(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")

	midnight := time.Date(2026, 2, 11, 0, 0, 0, 0, ist)
	t.Run("today at midnight", func(t *testing.T) {
		got, err := ParseDateRelativeTo("today", midnight)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Day() != 11 {
			t.Errorf("got day %d, want 11", got.Day())
		}
	})

	lateNight := time.Date(2026, 2, 11, 23, 59, 59, 0, ist)
	t.Run("tomorrow at 23:59:59", func(t *testing.T) {
		got, err := ParseDateRelativeTo("tomorrow", lateNight)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Day() != 12 {
			t.Errorf("got day %d, want 12", got.Day())
		}
	})
}

func TestInternationalDateLine(t *testing.T) {
	auckland := mustLoadLoc(t, "Pacific/Auckland")
	samoa := mustLoadLoc(t, "Pacific/Apia")

	for _, tc := range []struct {
		name string
		loc  *time.Location
	}{
		{"Auckland", auckland},
		{"Samoa", samoa},
	} {
		now := time.Date(2026, 6, 15, 22, 0, 0, 0, tc.loc)

		t.Run(tc.name+"/tomorrow", func(t *testing.T) {
			got, err := ParseDateRelativeTo("tomorrow", now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Day() != 16 {
				t.Errorf("got day %d, want 16", got.Day())
			}
		})

		t.Run(tc.name+"/in 3 hours", func(t *testing.T) {
			got, err := ParseDateRelativeTo("in 3 hours", now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			expected := now.Add(3 * time.Hour)
			if !got.Equal(expected) {
				t.Errorf("got %v, want %v", got, expected)
			}
		})
	}
}

func TestNegativeUTCOffset(t *testing.T) {
	hawaii := mustLoadLoc(t, "Pacific/Honolulu")
	now := time.Date(2026, 6, 15, 10, 30, 0, 0, hawaii)

	t.Run("today in Hawaii", func(t *testing.T) {
		got, err := ParseDateRelativeTo("today", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Day() != 15 {
			t.Errorf("got day %d, want 15", got.Day())
		}
		if got.Location() != hawaii {
			t.Errorf("location mismatch: got %v, want %v", got.Location(), hawaii)
		}
	})

	t.Run("5pm in Hawaii", func(t *testing.T) {
		got, err := ParseDateRelativeTo("5pm", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Hour() != 17 {
			t.Errorf("hour mismatch: got %d, want 17", got.Hour())
		}
		_, offset := got.Zone()
		_, expectedOffset := now.Zone()
		if offset != expectedOffset {
			t.Errorf("offset mismatch: got %d, want %d", offset, expectedOffset)
		}
	})
}

func TestOptionsDoNotLeak(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist)

	// Call with WithDefaultHour(9)
	got1, err := ParseDateRelativeTo("today", now, WithDefaultHour(9))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got1.Hour() != 9 {
		t.Errorf("with option: got hour %d, want 9", got1.Hour())
	}

	// Call without options — should use default (0)
	got2, err := ParseDateRelativeTo("today", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got2.Hour() != 0 {
		t.Errorf("without option: got hour %d, want 0", got2.Hour())
	}
}

func TestAllOptionsCombined(t *testing.T) {
	ist := mustLoadLoc(t, "Asia/Kolkata")
	now := refTime(ist) // Wed 10:30 AM

	opts := []Option{
		WithDefaultHour(9),
		WithSmartTimeRollover(),
		WithEOWSkipToday(),
	}

	// DefaultHour: "today" at 9am
	got, err := ParseDateRelativeTo("today", now, opts...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Hour() != 9 {
		t.Errorf("defaultHour: got hour %d, want 9", got.Hour())
	}

	// SmartTimeRollover: "9am" (past) -> tomorrow
	got, err = ParseDateRelativeTo("9am", now, opts...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Day() != 12 {
		t.Errorf("smartRollover: got day %d, want 12 (tomorrow)", got.Day())
	}

	// EOWSkipToday: on Friday, should skip to next Friday
	friday := time.Date(2026, 2, 13, 10, 0, 0, 0, ist)
	got, err = ParseDateRelativeTo("eow", friday, opts...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 2, 20, 17, 0, 0, 0, ist)
	if !got.Equal(want) {
		t.Errorf("eowSkipToday: got %v, want %v", got, want)
	}
}

func TestParseDate_UsesNow(t *testing.T) {
	got, err := ParseDate("today")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	now := time.Now()
	if got.Day() != now.Day() || got.Month() != now.Month() || got.Year() != now.Year() {
		t.Errorf("ParseDate('today') should match current date, got %v", got)
	}
}
