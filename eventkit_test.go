package eventkit

import (
	"testing"
	"time"
)

// --- Recurrence frequency String() tests ---

func TestRecurrenceFrequencyString(t *testing.T) {
	tests := []struct {
		f    RecurrenceFrequency
		want string
	}{
		{FrequencyDaily, "daily"},
		{FrequencyWeekly, "weekly"},
		{FrequencyMonthly, "monthly"},
		{FrequencyYearly, "yearly"},
		{RecurrenceFrequency(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.f.String(); got != tt.want {
				t.Errorf("RecurrenceFrequency(%d).String() = %q, want %q", tt.f, got, tt.want)
			}
		})
	}
}

func TestWeekdayString(t *testing.T) {
	tests := []struct {
		w    Weekday
		want string
	}{
		{Sunday, "sunday"},
		{Monday, "monday"},
		{Tuesday, "tuesday"},
		{Wednesday, "wednesday"},
		{Thursday, "thursday"},
		{Friday, "friday"},
		{Saturday, "saturday"},
		{Weekday(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.w.String(); got != tt.want {
				t.Errorf("Weekday(%d).String() = %q, want %q", tt.w, got, tt.want)
			}
		})
	}
}

// --- Convenience constructor tests ---

func TestDailyConstructor(t *testing.T) {
	r := Daily(1)
	if r.Frequency != FrequencyDaily {
		t.Errorf("Frequency = %d, want %d", r.Frequency, FrequencyDaily)
	}
	if r.Interval != 1 {
		t.Errorf("Interval = %d, want 1", r.Interval)
	}
	if r.End != nil {
		t.Error("End should be nil")
	}
}

func TestWeeklyConstructor(t *testing.T) {
	r := Weekly(2, Monday, Friday)
	if r.Frequency != FrequencyWeekly {
		t.Errorf("Frequency = %d, want %d", r.Frequency, FrequencyWeekly)
	}
	if r.Interval != 2 {
		t.Errorf("Interval = %d, want 2", r.Interval)
	}
	if len(r.DaysOfTheWeek) != 2 {
		t.Fatalf("DaysOfTheWeek count = %d, want 2", len(r.DaysOfTheWeek))
	}
	if r.DaysOfTheWeek[0].DayOfTheWeek != Monday {
		t.Errorf("DaysOfTheWeek[0] = %d, want %d (Monday)", r.DaysOfTheWeek[0].DayOfTheWeek, Monday)
	}
	if r.DaysOfTheWeek[1].DayOfTheWeek != Friday {
		t.Errorf("DaysOfTheWeek[1] = %d, want %d (Friday)", r.DaysOfTheWeek[1].DayOfTheWeek, Friday)
	}
}

func TestWeeklyConstructorNoDays(t *testing.T) {
	r := Weekly(1)
	if r.Frequency != FrequencyWeekly {
		t.Errorf("Frequency = %d, want %d", r.Frequency, FrequencyWeekly)
	}
	if len(r.DaysOfTheWeek) != 0 {
		t.Errorf("DaysOfTheWeek should be empty, got %d", len(r.DaysOfTheWeek))
	}
}

func TestMonthlyConstructor(t *testing.T) {
	r := Monthly(1, 1, 15)
	if r.Frequency != FrequencyMonthly {
		t.Errorf("Frequency = %d, want %d", r.Frequency, FrequencyMonthly)
	}
	if r.Interval != 1 {
		t.Errorf("Interval = %d, want 1", r.Interval)
	}
	if len(r.DaysOfTheMonth) != 2 {
		t.Fatalf("DaysOfTheMonth count = %d, want 2", len(r.DaysOfTheMonth))
	}
	if r.DaysOfTheMonth[0] != 1 || r.DaysOfTheMonth[1] != 15 {
		t.Errorf("DaysOfTheMonth = %v, want [1 15]", r.DaysOfTheMonth)
	}
}

func TestYearlyConstructor(t *testing.T) {
	r := Yearly(1)
	if r.Frequency != FrequencyYearly {
		t.Errorf("Frequency = %d, want %d", r.Frequency, FrequencyYearly)
	}
	if r.Interval != 1 {
		t.Errorf("Interval = %d, want 1", r.Interval)
	}
}

func TestUntilChain(t *testing.T) {
	end := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	r := Daily(1).Until(end)
	if r.End == nil {
		t.Fatal("End should not be nil")
	}
	if r.End.EndDate == nil {
		t.Fatal("EndDate should not be nil")
	}
	if !r.End.EndDate.Equal(end) {
		t.Errorf("EndDate = %v, want %v", r.End.EndDate, end)
	}
	if r.End.OccurrenceCount != 0 {
		t.Errorf("OccurrenceCount = %d, want 0", r.End.OccurrenceCount)
	}
}

func TestCountChain(t *testing.T) {
	r := Weekly(1, Monday).Count(10)
	if r.End == nil {
		t.Fatal("End should not be nil")
	}
	if r.End.OccurrenceCount != 10 {
		t.Errorf("OccurrenceCount = %d, want 10", r.End.OccurrenceCount)
	}
	if r.End.EndDate != nil {
		t.Error("EndDate should be nil for count-based")
	}
}

// --- Validate() tests ---

func TestRecurrenceRuleValidate(t *testing.T) {
	endDate := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		rule    RecurrenceRule
		wantErr bool
	}{
		{
			name:    "valid daily rule",
			rule:    Daily(1),
			wantErr: false,
		},
		{
			name:    "valid weekly with days of week",
			rule:    Weekly(1, Monday, Friday),
			wantErr: false,
		},
		{
			name: "valid monthly with days of month",
			rule: Monthly(1, 1, 15),
			wantErr: false,
		},
		{
			name: "valid yearly with months",
			rule: RecurrenceRule{
				Frequency:       FrequencyYearly,
				Interval:        1,
				MonthsOfTheYear: []int{1, 6},
			},
			wantErr: false,
		},
		{
			name: "valid yearly with all constraint types",
			rule: RecurrenceRule{
				Frequency:       FrequencyYearly,
				Interval:        1,
				DaysOfTheWeek:   []RecurrenceDayOfWeek{{DayOfTheWeek: Monday}},
				DaysOfTheMonth:  []int{1},
				MonthsOfTheYear: []int{3},
				WeeksOfTheYear:  []int{10},
				DaysOfTheYear:   []int{100},
				SetPositions:    []int{1},
			},
			wantErr: false,
		},
		{
			name:    "interval 0",
			rule:    RecurrenceRule{Frequency: FrequencyDaily, Interval: 0},
			wantErr: true,
		},
		{
			name:    "interval -1",
			rule:    RecurrenceRule{Frequency: FrequencyDaily, Interval: -1},
			wantErr: true,
		},
		{
			name: "daysOfTheWeek on daily",
			rule: RecurrenceRule{
				Frequency:     FrequencyDaily,
				Interval:      1,
				DaysOfTheWeek: []RecurrenceDayOfWeek{{DayOfTheWeek: Monday}},
			},
			wantErr: true,
		},
		{
			name: "daysOfTheMonth on weekly",
			rule: RecurrenceRule{
				Frequency:      FrequencyWeekly,
				Interval:       1,
				DaysOfTheMonth: []int{15},
			},
			wantErr: true,
		},
		{
			name: "monthsOfTheYear on monthly",
			rule: RecurrenceRule{
				Frequency:       FrequencyMonthly,
				Interval:        1,
				MonthsOfTheYear: []int{6},
			},
			wantErr: true,
		},
		{
			name: "weeksOfTheYear on monthly",
			rule: RecurrenceRule{
				Frequency:      FrequencyMonthly,
				Interval:       1,
				WeeksOfTheYear: []int{10},
			},
			wantErr: true,
		},
		{
			name: "daysOfTheYear on weekly",
			rule: RecurrenceRule{
				Frequency:     FrequencyWeekly,
				Interval:      1,
				DaysOfTheYear: []int{100},
			},
			wantErr: true,
		},
		{
			name: "setPositions without any constraint arrays",
			rule: RecurrenceRule{
				Frequency:    FrequencyMonthly,
				Interval:     1,
				SetPositions: []int{-1},
			},
			wantErr: true,
		},
		{
			name: "end with occurrenceCount 0",
			rule: RecurrenceRule{
				Frequency: FrequencyDaily,
				Interval:  1,
				End:       &RecurrenceEnd{OccurrenceCount: 0},
			},
			wantErr: true,
		},
		{
			name: "end with valid count",
			rule: RecurrenceRule{
				Frequency: FrequencyDaily,
				Interval:  1,
				End:       &RecurrenceEnd{OccurrenceCount: 10},
			},
			wantErr: false,
		},
		{
			name: "end with endDate",
			rule: RecurrenceRule{
				Frequency: FrequencyWeekly,
				Interval:  1,
				End:       &RecurrenceEnd{EndDate: &endDate},
			},
			wantErr: false,
		},
		{
			name:    "bare daily no constraints",
			rule:    Daily(1),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// --- Recurrence rule enum values match EventKit ---

func TestRecurrenceEnumValues(t *testing.T) {
	if FrequencyDaily != 0 {
		t.Errorf("FrequencyDaily = %d, want 0", FrequencyDaily)
	}
	if FrequencyWeekly != 1 {
		t.Errorf("FrequencyWeekly = %d, want 1", FrequencyWeekly)
	}
	if FrequencyMonthly != 2 {
		t.Errorf("FrequencyMonthly = %d, want 2", FrequencyMonthly)
	}
	if FrequencyYearly != 3 {
		t.Errorf("FrequencyYearly = %d, want 3", FrequencyYearly)
	}

	// EKWeekday values
	if Sunday != 1 {
		t.Errorf("Sunday = %d, want 1", Sunday)
	}
	if Monday != 2 {
		t.Errorf("Monday = %d, want 2", Monday)
	}
	if Saturday != 7 {
		t.Errorf("Saturday = %d, want 7", Saturday)
	}
}
