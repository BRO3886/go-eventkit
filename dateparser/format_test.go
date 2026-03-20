package dateparser

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		end    time.Time
		allDay bool
		want   string
	}{
		{"1 hour", base.Add(1 * time.Hour), false, "1h"},
		{"30 minutes", base.Add(30 * time.Minute), false, "30m"},
		{"1h 30m", base.Add(90 * time.Minute), false, "1h 30m"},
		{"all day single", base.Add(24 * time.Hour), true, "All Day"},
		{"all day multi", base.Add(72 * time.Hour), true, "3 days"},
		{"0 minutes", base, false, "0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(base, tt.end, tt.allDay)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatTimeRange(t *testing.T) {
	loc := time.UTC

	tests := []struct {
		name   string
		start  time.Time
		end    time.Time
		allDay bool
		want   string
	}{
		{
			"same day",
			time.Date(2026, 1, 15, 10, 0, 0, 0, loc),
			time.Date(2026, 1, 15, 11, 30, 0, 0, loc),
			false,
			"10:00 - 11:30",
		},
		{
			"cross day",
			time.Date(2026, 1, 15, 23, 0, 0, 0, loc),
			time.Date(2026, 1, 16, 1, 0, 0, 0, loc),
			false,
			"Jan 15 23:00 - Jan 16 01:00",
		},
		{
			"all day single",
			time.Date(2026, 1, 15, 0, 0, 0, 0, loc),
			time.Date(2026, 1, 16, 0, 0, 0, 0, loc),
			true,
			"All Day",
		},
		{
			"all day multi",
			time.Date(2026, 1, 15, 0, 0, 0, 0, loc),
			time.Date(2026, 1, 18, 0, 0, 0, 0, loc),
			true,
			"Jan 15 - Jan 18",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTimeRange(tt.start, tt.end, tt.allDay)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseAlertDuration(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"15m", 15 * time.Minute, false},
		{"1h", 1 * time.Hour, false},
		{"2h", 2 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"30min", 30 * time.Minute, false},
		{"2days", 48 * time.Hour, false},
		{"", 0, true},
		{"abc", 0, true},
		{"15", 0, true},
		{"h", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseAlertDuration(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
