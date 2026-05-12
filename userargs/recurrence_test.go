package userargs

import (
	"strings"
	"testing"

	eventkit "github.com/BRO3886/go-eventkit"
)

func TestParseRecurrence_Nil(t *testing.T) {
	rules, err := ParseRecurrence(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rules != nil {
		t.Fatalf("expected nil rules, got %v", rules)
	}
}

func TestParseRecurrence_EmptyFrequency(t *testing.T) {
	rules, err := ParseRecurrence(&RecurrenceArgs{Frequency: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rules != nil {
		t.Fatalf("expected nil rules, got %v", rules)
	}
}

func TestParseRecurrence_Daily(t *testing.T) {
	rules, err := ParseRecurrence(&RecurrenceArgs{Frequency: "daily", Interval: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	r := rules[0]
	if r.Frequency != eventkit.FrequencyDaily {
		t.Errorf("expected FrequencyDaily, got %v", r.Frequency)
	}
	if r.Interval != 2 {
		t.Errorf("expected Interval=2, got %d", r.Interval)
	}
}

func TestParseRecurrence_Weekly_ByDay(t *testing.T) {
	rules, err := ParseRecurrence(&RecurrenceArgs{
		Frequency: "weekly",
		Interval:  1,
		ByDay:     []string{"MO", "WE", "FR"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	r := rules[0]
	if r.Frequency != eventkit.FrequencyWeekly {
		t.Errorf("expected FrequencyWeekly, got %v", r.Frequency)
	}
	if len(r.DaysOfTheWeek) != 3 {
		t.Fatalf("expected 3 days, got %d", len(r.DaysOfTheWeek))
	}
	wantDays := []eventkit.Weekday{eventkit.Monday, eventkit.Wednesday, eventkit.Friday}
	for i, d := range r.DaysOfTheWeek {
		if d.DayOfTheWeek != wantDays[i] {
			t.Errorf("day[%d]: got %v, want %v", i, d.DayOfTheWeek, wantDays[i])
		}
	}
}

func TestParseRecurrence_Monthly(t *testing.T) {
	rules, err := ParseRecurrence(&RecurrenceArgs{Frequency: "monthly", Interval: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Frequency != eventkit.FrequencyMonthly {
		t.Errorf("expected FrequencyMonthly, got %v", rules[0].Frequency)
	}
}

func TestParseRecurrence_Yearly(t *testing.T) {
	rules, err := ParseRecurrence(&RecurrenceArgs{Frequency: "yearly", Interval: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Frequency != eventkit.FrequencyYearly {
		t.Errorf("expected FrequencyYearly, got %v", rules[0].Frequency)
	}
}

func TestParseRecurrence_UnknownFrequency(t *testing.T) {
	_, err := ParseRecurrence(&RecurrenceArgs{Frequency: "fortnightly"})
	if err == nil {
		t.Fatal("expected error for unknown frequency")
	}
}

func TestParseRecurrence_CountAndUntilMutuallyExclusive(t *testing.T) {
	_, err := ParseRecurrence(&RecurrenceArgs{
		Frequency: "daily",
		Count:     5,
		Until:     "2026-12-31",
	})
	if err == nil {
		t.Fatal("expected error for Count and Until both set")
	}
}

func TestParseRecurrence_Count(t *testing.T) {
	rules, err := ParseRecurrence(&RecurrenceArgs{
		Frequency: "weekly",
		Interval:  1,
		Count:     5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	r := rules[0]
	if r.End == nil {
		t.Fatal("expected End to be set")
	}
	if r.End.OccurrenceCount != 5 {
		t.Errorf("expected OccurrenceCount=5, got %d", r.End.OccurrenceCount)
	}
}

func TestParseRecurrence_Until(t *testing.T) {
	rules, err := ParseRecurrence(&RecurrenceArgs{
		Frequency: "daily",
		Interval:  1,
		Until:     "2026-12-31",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	r := rules[0]
	if r.End == nil || r.End.EndDate == nil {
		t.Fatal("expected End.EndDate to be non-nil")
	}
}

func TestParseRecurrence_UntilInvalidDate(t *testing.T) {
	_, err := ParseRecurrence(&RecurrenceArgs{
		Frequency: "daily",
		Until:     "not-a-date",
	})
	if err == nil {
		t.Fatal("expected error for invalid Until")
	}
	if !strings.Contains(err.Error(), "invalid Until") {
		t.Errorf("error should contain 'invalid Until', got: %v", err)
	}
}

func TestParseRecurrence_UnknownByDay(t *testing.T) {
	_, err := ParseRecurrence(&RecurrenceArgs{
		Frequency: "weekly",
		ByDay:     []string{"XX"},
	})
	if err == nil {
		t.Fatal("expected error for unknown weekday")
	}
	if !strings.Contains(err.Error(), "unknown weekday") {
		t.Errorf("error should contain 'unknown weekday', got: %v", err)
	}
}

func TestParseRecurrence_ZeroIntervalCoercedToOne(t *testing.T) {
	rules, err := ParseRecurrence(&RecurrenceArgs{Frequency: "daily", Interval: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rules[0].Interval != 1 {
		t.Errorf("expected Interval coerced to 1, got %d", rules[0].Interval)
	}
}

func TestParseRecurrence_NegativeIntervalErrors(t *testing.T) {
	_, err := ParseRecurrence(&RecurrenceArgs{Frequency: "daily", Interval: -7})
	if err == nil {
		t.Error("expected error for negative Interval")
	}
}

func TestParseRecurrence_NegativeCountErrors(t *testing.T) {
	_, err := ParseRecurrence(&RecurrenceArgs{Frequency: "daily", Count: -3})
	if err == nil {
		t.Error("expected error for negative Count")
	}
}

func TestParseRecurrence_ZeroIntervalDefaultsToOne(t *testing.T) {
	rules, err := ParseRecurrence(&RecurrenceArgs{Frequency: "daily", Interval: 0})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if rules[0].Interval != 1 {
		t.Errorf("Interval = %d, want 1", rules[0].Interval)
	}
}

func TestParseRecurrence_ByDayWithMonthly(t *testing.T) {
	rules, err := ParseRecurrence(&RecurrenceArgs{Frequency: "monthly", ByDay: []string{"MO", "FR"}})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if len(rules[0].DaysOfTheWeek) != 2 {
		t.Errorf("expected 2 DaysOfTheWeek, got %d", len(rules[0].DaysOfTheWeek))
	}
}

func TestParseRecurrence_ByDayWithDailyErrors(t *testing.T) {
	_, err := ParseRecurrence(&RecurrenceArgs{Frequency: "daily", ByDay: []string{"MO"}})
	if err == nil {
		t.Error("expected error: ByDay invalid for daily")
	}
}

func TestParseRecurrence_FrequencyWhitespaceTrimmed(t *testing.T) {
	rules, err := ParseRecurrence(&RecurrenceArgs{Frequency: "  Daily  "})
	if err != nil {
		t.Fatalf("expected whitespace to be trimmed, got %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}
}

func TestParseWeekday(t *testing.T) {
	tests := []struct {
		input string
		want  eventkit.Weekday
	}{
		{"MO", eventkit.Monday},
		{"mon", eventkit.Monday},
		{"Monday", eventkit.Monday},
		{"  Friday  ", eventkit.Friday},
		{"TU", eventkit.Tuesday},
		{"WE", eventkit.Wednesday},
		{"TH", eventkit.Thursday},
		{"SA", eventkit.Saturday},
		{"SU", eventkit.Sunday},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseWeekday(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseWeekday(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseWeekday_Unknown(t *testing.T) {
	_, err := ParseWeekday("garbage")
	if err == nil {
		t.Fatal("expected error for unknown weekday")
	}
}

func TestParseWeekdays(t *testing.T) {
	got, err := ParseWeekdays([]string{"MO", "wed", "Friday"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 weekdays, got %d", len(got))
	}
	want := []eventkit.Weekday{eventkit.Monday, eventkit.Wednesday, eventkit.Friday}
	for i, w := range got {
		if w != want[i] {
			t.Errorf("weekday[%d]: got %v, want %v", i, w, want[i])
		}
	}
}

func TestParseWeekdays_BadEntryStopsAtFirst(t *testing.T) {
	_, err := ParseWeekdays([]string{"MO", "BAD", "FR"})
	if err == nil {
		t.Fatal("expected error for unknown weekday in slice")
	}
}

func TestParseWeekdays_EmptySlice(t *testing.T) {
	got, err := ParseWeekdays([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}
