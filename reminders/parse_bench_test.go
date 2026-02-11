package reminders

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/BRO3886/go-eventkit"
)

// --- JSON generators ---

func stringPtr(s string) *string { return &s }

func generateListsJSON(count int) string {
	colors := []string{"#FF0000", "#00FF00", "#0000FF", "#FFFF00", "#FF00FF"}
	sources := []string{"iCloud", "Exchange", "On My Mac", "Gmail"}

	raw := make([]rawList, count)
	for i := range raw {
		raw[i] = rawList{
			ID:       fmt.Sprintf("list-%04d", i),
			Title:    fmt.Sprintf("List %d", i),
			Color:    colors[i%len(colors)],
			Source:   sources[i%len(sources)],
			Count:    10 + i*3,
			ReadOnly: i%4 == 0,
		}
	}
	data, _ := json.Marshal(raw)
	return string(data)
}

func generateRemindersJSON(count int) string {
	baseTime := time.Date(2026, 2, 11, 9, 0, 0, 0, time.UTC)
	dateStr := func(t time.Time) *string {
		s := t.UTC().Format("2006-01-02T15:04:05.000Z")
		return &s
	}

	raw := make([]rawReminder, count)
	for i := range raw {
		r := rawReminder{
			ID:        fmt.Sprintf("rem-%04d", i),
			Title:     fmt.Sprintf("Reminder %d", i),
			List:      fmt.Sprintf("List %d", i%5),
			ListID:    fmt.Sprintf("list-%04d", i%5),
			Priority:  []int{0, 1, 5, 9}[i%4],
			Completed: i%3 == 0,
			Flagged:   false,
			Recurring: i%4 == 0,
			HasAlarms: i%5 == 0,
			Alarms:    []rawAlarm{},
			CreatedAt: dateStr(baseTime.Add(-48 * time.Hour)),
			ModifiedAt: dateStr(baseTime.Add(-1 * time.Hour)),
		}

		// Every 2nd has notes.
		if i%2 == 0 {
			r.Notes = stringPtr(fmt.Sprintf("Notes for reminder %d with details", i))
		}

		// Every 2nd has dueDate.
		if i%2 == 0 {
			r.DueDate = dateStr(baseTime.Add(time.Duration(i) * 24 * time.Hour))
		}

		// Every 3rd completed gets a completionDate.
		if i%3 == 0 {
			r.CompletionDate = dateStr(baseTime.Add(-time.Duration(i) * time.Hour))
		}

		// Every 7th has a URL.
		if i%7 == 0 {
			r.URL = stringPtr("https://example.com/task")
		}

		// Every 5th has alarms.
		if i%5 == 0 {
			r.HasAlarms = true
			r.Alarms = []rawAlarm{
				{RelativeOffset: -900},
			}
			if i%10 == 0 {
				absDate := baseTime.Add(time.Duration(i) * time.Hour).UTC().Format("2006-01-02T15:04:05.000Z")
				r.Alarms = append(r.Alarms, rawAlarm{AbsoluteDate: &absDate, RelativeOffset: 0})
			}
		}

		// Every 4th has recurrence rules.
		if i%4 == 0 {
			rule := rawRecurrenceRule{
				Frequency: i % 4,
				Interval:  1 + i%3,
			}
			if i%8 == 0 {
				rule.DaysOfTheWeek = []rawRecurrenceDayOfWeek{
					{DayOfTheWeek: 2, WeekNumber: 0},
					{DayOfTheWeek: 6, WeekNumber: 0},
				}
			}
			if i%12 == 0 {
				endDateStr := baseTime.Add(180 * 24 * time.Hour).UTC().Format("2006-01-02T15:04:05.000Z")
				rule.End = &rawRecurrenceEnd{EndDate: &endDateStr}
			} else if i%8 == 0 {
				rule.End = &rawRecurrenceEnd{OccurrenceCount: 10}
			}
			r.RecurrenceRules = []rawRecurrenceRule{rule}
		}

		raw[i] = r
	}
	data, _ := json.Marshal(raw)
	return string(data)
}

// --- Benchmarks ---

func BenchmarkParseListsJSON(b *testing.B) {
	jsonStr := generateListsJSON(10)
	b.ResetTimer()
	for b.Loop() {
		if _, err := parseListsJSON(jsonStr); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseRemindersJSON(b *testing.B) {
	jsonStr := generateRemindersJSON(50)
	b.ResetTimer()
	for b.Loop() {
		if _, err := parseRemindersJSON(jsonStr); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseRemindersJSON_Large(b *testing.B) {
	jsonStr := generateRemindersJSON(500)
	b.ResetTimer()
	for b.Loop() {
		if _, err := parseRemindersJSON(jsonStr); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarshalCreateReminderInput(b *testing.B) {
	due := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	remind := time.Date(2026, 3, 15, 9, 30, 0, 0, time.UTC)
	alarm := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)

	input := CreateReminderInput{
		Title:        "Benchmark Reminder",
		Notes:        "Detailed notes for the benchmark reminder task",
		ListName:     "Work",
		DueDate:      &due,
		RemindMeDate: &remind,
		Priority:     PriorityHigh,
		URL:          "https://example.com/task",
		Alarms: []Alarm{
			{AbsoluteDate: &alarm},
			{RelativeOffset: -15 * time.Minute},
		},
		RecurrenceRules: []eventkit.RecurrenceRule{
			eventkit.Weekly(1, eventkit.Monday, eventkit.Friday).Count(20),
		},
	}

	b.ResetTimer()
	for b.Loop() {
		if _, err := marshalCreateInput(input); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseRemindersJSON_Scaling(b *testing.B) {
	for _, n := range []int{1, 10, 50, 100, 500} {
		jsonStr := generateRemindersJSON(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for b.Loop() {
				if _, err := parseRemindersJSON(jsonStr); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkParseListsJSON_Scaling(b *testing.B) {
	for _, n := range []int{1, 10, 50, 100, 500} {
		jsonStr := generateListsJSON(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for b.Loop() {
				if _, err := parseListsJSON(jsonStr); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
