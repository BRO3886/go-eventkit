package calendar

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/BRO3886/go-eventkit"
)

// --- JSON generators ---

func stringPtr(s string) *string { return &s }
func float64Ptr(f float64) *float64 { return &f }

func generateCalendarsJSON(count int) string {
	types := []int{0, 1, 2, 3, 4}
	colors := []string{"#FF0000", "#00FF00", "#0000FF", "#FFFF00", "#FF00FF", "#AABBCC"}
	sources := []string{"iCloud", "Gmail", "Exchange", "On My Mac", "Subscriptions"}

	raw := make([]rawCalendar, count)
	for i := range raw {
		raw[i] = rawCalendar{
			ID:       fmt.Sprintf("cal-%04d", i),
			Title:    fmt.Sprintf("Calendar %d", i),
			Type:     types[i%len(types)],
			Color:    colors[i%len(colors)],
			Source:   sources[i%len(sources)],
			ReadOnly: i%3 == 0,
		}
	}
	data, _ := json.Marshal(raw)
	return string(data)
}

func generateEventsJSON(count int) string {
	baseTime := time.Date(2026, 2, 11, 9, 0, 0, 0, time.UTC)
	dateStr := func(t time.Time) *string {
		s := t.UTC().Format("2006-01-02T15:04:05.000Z")
		return &s
	}

	raw := make([]rawEvent, count)
	for i := range raw {
		start := baseTime.Add(time.Duration(i) * time.Hour)
		end := start.Add(30 * time.Minute)
		created := baseTime.Add(-24 * time.Hour)
		modified := baseTime.Add(-1 * time.Hour)

		e := rawEvent{
			ID:           fmt.Sprintf("evt-%04d", i),
			Title:        fmt.Sprintf("Event %d", i),
			StartDate:    dateStr(start),
			EndDate:      dateStr(end),
			AllDay:       i%5 == 0,
			Calendar:     fmt.Sprintf("Calendar %d", i%5),
			CalendarID:   fmt.Sprintf("cal-%04d", i%5),
			Status:       i % 4,
			Availability: i % 4,
			Recurring:    i%3 == 0,
			IsDetached:   false,
			Attendees:    []rawAttendee{},
			Alerts:       []rawAlert{},
			CreatedAt:    dateStr(created),
			ModifiedAt:   dateStr(modified),
		}

		if i%2 == 0 {
			e.Location = stringPtr(fmt.Sprintf("Room %d", i))
		}
		if i%3 == 0 {
			e.Notes = stringPtr(fmt.Sprintf("Notes for event %d with some detail", i))
		}
		if i%7 == 0 {
			e.URL = stringPtr("https://meet.example.com/room")
		}
		if i%4 == 0 {
			e.Organizer = stringPtr("Organizer Name")
			e.TimeZone = stringPtr("America/New_York")
		}

		// Attendees: 2-3 per event.
		numAttendees := 2 + i%2
		for j := 0; j < numAttendees; j++ {
			e.Attendees = append(e.Attendees, rawAttendee{
				Name:   fmt.Sprintf("Person %d", j),
				Email:  fmt.Sprintf("person%d@example.com", j),
				Status: j % 5,
			})
		}

		// Alerts: 1-2 per event.
		e.Alerts = append(e.Alerts, rawAlert{RelativeOffset: -900})
		if i%2 == 0 {
			e.Alerts = append(e.Alerts, rawAlert{RelativeOffset: -3600})
		}

		// Recurrence rules: every 3rd event.
		if i%3 == 0 {
			rule := rawRecurrenceRule{
				Frequency: i % 4,
				Interval:  1 + i%3,
			}
			if i%6 == 0 {
				rule.DaysOfTheWeek = []rawRecurrenceDayOfWeek{
					{DayOfTheWeek: 2, WeekNumber: 0},
					{DayOfTheWeek: 4, WeekNumber: 0},
				}
			}
			if i%9 == 0 {
				endDateStr := baseTime.Add(365 * 24 * time.Hour).UTC().Format("2006-01-02T15:04:05.000Z")
				rule.End = &rawRecurrenceEnd{EndDate: &endDateStr}
			}
			e.RecurrenceRules = []rawRecurrenceRule{rule}
		}

		// Structured location: every 4th event.
		if i%4 == 0 {
			e.StructuredLocation = &rawStructuredLocation{
				Title:     "Apple Park",
				Latitude:  float64Ptr(37.3349),
				Longitude: float64Ptr(-122.009),
				Radius:    float64Ptr(150.0),
			}
		}

		raw[i] = e
	}
	data, _ := json.Marshal(raw)
	return string(data)
}

// --- Benchmarks ---

func BenchmarkParseCalendarsJSON(b *testing.B) {
	jsonStr := generateCalendarsJSON(10)
	b.ResetTimer()
	for b.Loop() {
		if _, err := parseCalendarsJSON(jsonStr); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseEventsJSON(b *testing.B) {
	jsonStr := generateEventsJSON(50)
	b.ResetTimer()
	for b.Loop() {
		if _, err := parseEventsJSON(jsonStr); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseEventsJSON_Large(b *testing.B) {
	jsonStr := generateEventsJSON(500)
	b.ResetTimer()
	for b.Loop() {
		if _, err := parseEventsJSON(jsonStr); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarshalCreateEventInput(b *testing.B) {
	endDate := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	input := CreateEventInput{
		Title:     "Benchmark Meeting",
		StartDate: time.Date(2026, 2, 12, 10, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 2, 12, 11, 0, 0, 0, time.UTC),
		AllDay:    false,
		Location:  "Conference Room A",
		Notes:     "Quarterly planning session with the entire team",
		URL:       "https://meet.example.com/quarterly",
		Calendar:  "Work",
		TimeZone:  "America/New_York",
		Alerts: []Alert{
			{RelativeOffset: -15 * time.Minute},
			{RelativeOffset: -1 * time.Hour},
		},
		RecurrenceRules: []eventkit.RecurrenceRule{
			eventkit.Weekly(1, eventkit.Monday, eventkit.Wednesday, eventkit.Friday).Until(endDate),
		},
		StructuredLocation: &eventkit.StructuredLocation{
			Title:     "Apple Park",
			Latitude:  37.3349,
			Longitude: -122.009,
			Radius:    150.0,
		},
	}

	b.ResetTimer()
	for b.Loop() {
		if _, err := marshalCreateInput(input); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseEventsJSON_Scaling(b *testing.B) {
	for _, n := range []int{1, 10, 50, 100, 500} {
		jsonStr := generateEventsJSON(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for b.Loop() {
				if _, err := parseEventsJSON(jsonStr); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkParseCalendarsJSON_Scaling(b *testing.B) {
	for _, n := range []int{1, 10, 50, 100, 500} {
		jsonStr := generateCalendarsJSON(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for b.Loop() {
				if _, err := parseCalendarsJSON(jsonStr); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
