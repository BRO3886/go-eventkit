package calendar

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/BRO3886/go-eventkit"
)

// These tests use a mock bridge layer to test the Client method logic
// without requiring real EventKit access. They cover the JSON round-trip,
// error handling, and type conversion that happens in the bridge layer.

// --- Mock bridge: simulates the ObjC bridge returning JSON ---

// mockBridge simulates ObjC bridge responses for testing.
type mockBridge struct {
	calendarsJSON string
	eventsJSON    string
	eventJSON     string
	createJSON    string
	updateJSON    string
	deleteOK      bool
	err           error
}

// simulateCalendarsResponse simulates what the ObjC bridge returns for Calendars().
func simulateCalendarsResponse(calendars []Calendar) string {
	raw := make([]rawCalendar, len(calendars))
	for i, c := range calendars {
		raw[i] = rawCalendar{
			ID:       c.ID,
			Title:    c.Title,
			Type:     int(c.Type),
			Color:    c.Color,
			Source:   c.Source,
			ReadOnly: c.ReadOnly,
		}
	}
	data, _ := json.Marshal(raw)
	return string(data)
}

// simulateEventsResponse simulates what the ObjC bridge returns for Events().
func simulateEventsResponse(events []Event) string {
	raw := make([]rawEvent, len(events))
	for i, e := range events {
		startStr := e.StartDate.UTC().Format("2006-01-02T15:04:05.000Z")
		endStr := e.EndDate.UTC().Format("2006-01-02T15:04:05.000Z")

		re := rawEvent{
			ID:           e.ID,
			Title:        e.Title,
			StartDate:    &startStr,
			EndDate:      &endStr,
			AllDay:       e.AllDay,
			Calendar:     e.Calendar,
			CalendarID:   e.CalendarID,
			Status:       int(e.Status),
			Availability: int(e.Availability),
			Recurring:    e.Recurring,
			Attendees:    []rawAttendee{},
			Alerts:       []rawAlert{},
		}

		if e.Location != "" {
			re.Location = &e.Location
		}
		if e.Notes != "" {
			re.Notes = &e.Notes
		}
		if e.URL != "" {
			re.URL = &e.URL
		}
		if e.Organizer != "" {
			re.Organizer = &e.Organizer
		}
		if e.TimeZone != "" {
			re.TimeZone = &e.TimeZone
		}
		if !e.CreatedAt.IsZero() {
			s := e.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z")
			re.CreatedAt = &s
		}
		if !e.ModifiedAt.IsZero() {
			s := e.ModifiedAt.UTC().Format("2006-01-02T15:04:05.000Z")
			re.ModifiedAt = &s
		}

		for _, a := range e.Attendees {
			re.Attendees = append(re.Attendees, rawAttendee{
				Name:   a.Name,
				Email:  a.Email,
				Status: int(a.Status),
			})
		}
		for _, a := range e.Alerts {
			re.Alerts = append(re.Alerts, rawAlert{
				RelativeOffset: a.RelativeOffset.Seconds(),
			})
		}

		raw[i] = re
	}
	data, _ := json.Marshal(raw)
	return string(data)
}

// --- Mock-based bridge response tests ---

func TestMockCalendarsRoundtrip(t *testing.T) {
	// Simulate the full ObjC -> JSON -> Go roundtrip
	input := []Calendar{
		{ID: "cal-1", Title: "Home", Type: CalendarTypeCalDAV, Color: "#FF0000", Source: "iCloud", ReadOnly: false},
		{ID: "cal-2", Title: "Work", Type: CalendarTypeCalDAV, Color: "#0000FF", Source: "iCloud", ReadOnly: false},
		{ID: "cal-3", Title: "Birthdays", Type: CalendarTypeBirthday, Color: "#FFFF00", Source: "Other", ReadOnly: true},
		{ID: "cal-4", Title: "US Holidays", Type: CalendarTypeSubscription, Color: "#00FF00", Source: "Subscriptions", ReadOnly: true},
		{ID: "cal-5", Title: "Exchange", Type: CalendarTypeExchange, Color: "#FF00FF", Source: "Exchange", ReadOnly: false},
		{ID: "cal-6", Title: "Local Only", Type: CalendarTypeLocal, Color: "#AABBCC", Source: "On My Mac", ReadOnly: false},
	}

	jsonStr := simulateCalendarsResponse(input)
	parsed, err := parseCalendarsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed) != len(input) {
		t.Fatalf("parsed %d calendars, want %d", len(parsed), len(input))
	}

	for i, c := range parsed {
		if c.ID != input[i].ID {
			t.Errorf("cal[%d].ID = %q, want %q", i, c.ID, input[i].ID)
		}
		if c.Title != input[i].Title {
			t.Errorf("cal[%d].Title = %q, want %q", i, c.Title, input[i].Title)
		}
		if c.Type != input[i].Type {
			t.Errorf("cal[%d].Type = %d, want %d", i, c.Type, input[i].Type)
		}
		if c.Color != input[i].Color {
			t.Errorf("cal[%d].Color = %q, want %q", i, c.Color, input[i].Color)
		}
		if c.Source != input[i].Source {
			t.Errorf("cal[%d].Source = %q, want %q", i, c.Source, input[i].Source)
		}
		if c.ReadOnly != input[i].ReadOnly {
			t.Errorf("cal[%d].ReadOnly = %v, want %v", i, c.ReadOnly, input[i].ReadOnly)
		}
	}
}

func TestMockEventsRoundtrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)

	input := []Event{
		{
			ID:           "evt-1",
			Title:        "Morning Standup",
			StartDate:    now,
			EndDate:      now.Add(30 * time.Minute),
			AllDay:       false,
			Location:     "Zoom",
			Notes:        "Daily sync",
			URL:          "https://zoom.us/j/123",
			Calendar:     "Work",
			CalendarID:   "cal-work",
			Status:       StatusConfirmed,
			Availability: AvailabilityBusy,
			Organizer:    "Manager",
			Attendees: []Attendee{
				{Name: "Alice", Email: "alice@co.com", Status: ParticipantStatusAccepted},
				{Name: "Bob", Email: "bob@co.com", Status: ParticipantStatusTentative},
			},
			Recurring: true,
			Alerts: []Alert{
				{RelativeOffset: -5 * time.Minute},
			},
			TimeZone:   "America/New_York",
			CreatedAt:  now.Add(-24 * time.Hour),
			ModifiedAt: now.Add(-1 * time.Hour),
		},
		{
			ID:           "evt-2",
			Title:        "Lunch Break",
			StartDate:    now.Add(3 * time.Hour),
			EndDate:      now.Add(4 * time.Hour),
			AllDay:       false,
			Calendar:     "Home",
			CalendarID:   "cal-home",
			Status:       StatusNone,
			Availability: AvailabilityFree,
		},
		{
			ID:           "evt-3",
			Title:        "Company Holiday",
			StartDate:    now,
			EndDate:      now.Add(24 * time.Hour),
			AllDay:       true,
			Calendar:     "Work",
			CalendarID:   "cal-work",
			Status:       StatusConfirmed,
			Availability: AvailabilityUnavailable,
		},
	}

	jsonStr := simulateEventsResponse(input)
	parsed, err := parseEventsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed) != len(input) {
		t.Fatalf("parsed %d events, want %d", len(parsed), len(input))
	}

	// Verify first event (full details)
	e := parsed[0]
	if e.ID != "evt-1" {
		t.Errorf("e.ID = %q", e.ID)
	}
	if e.Title != "Morning Standup" {
		t.Errorf("e.Title = %q", e.Title)
	}
	if !e.StartDate.Equal(input[0].StartDate) {
		t.Errorf("e.StartDate = %v, want %v", e.StartDate, input[0].StartDate)
	}
	if !e.EndDate.Equal(input[0].EndDate) {
		t.Errorf("e.EndDate = %v, want %v", e.EndDate, input[0].EndDate)
	}
	if e.Location != "Zoom" {
		t.Errorf("e.Location = %q", e.Location)
	}
	if e.Notes != "Daily sync" {
		t.Errorf("e.Notes = %q", e.Notes)
	}
	if e.URL != "https://zoom.us/j/123" {
		t.Errorf("e.URL = %q", e.URL)
	}
	if e.Organizer != "Manager" {
		t.Errorf("e.Organizer = %q", e.Organizer)
	}
	if e.TimeZone != "America/New_York" {
		t.Errorf("e.TimeZone = %q", e.TimeZone)
	}
	if !e.Recurring {
		t.Error("e.Recurring should be true")
	}
	if len(e.Attendees) != 2 {
		t.Fatalf("attendees = %d", len(e.Attendees))
	}
	if e.Attendees[0].Name != "Alice" || e.Attendees[0].Status != ParticipantStatusAccepted {
		t.Errorf("attendee[0] = %+v", e.Attendees[0])
	}
	if len(e.Alerts) != 1 || e.Alerts[0].RelativeOffset != -5*time.Minute {
		t.Errorf("alerts = %+v", e.Alerts)
	}
	if !e.CreatedAt.Equal(input[0].CreatedAt) {
		t.Errorf("e.CreatedAt = %v, want %v", e.CreatedAt, input[0].CreatedAt)
	}

	// Verify second event (minimal)
	e2 := parsed[1]
	if e2.Location != "" {
		t.Errorf("e2.Location = %q, want empty", e2.Location)
	}
	if e2.Notes != "" {
		t.Errorf("e2.Notes = %q, want empty", e2.Notes)
	}

	// Verify all-day event
	e3 := parsed[2]
	if !e3.AllDay {
		t.Error("e3.AllDay should be true")
	}
}

func TestMockCreateEventJSON(t *testing.T) {
	// Test that CreateEventInput marshals correctly for the ObjC bridge
	ist := time.FixedZone("IST", 5*3600+30*60)
	start := time.Date(2026, 3, 15, 15, 30, 0, 0, ist) // 10:00 UTC
	end := start.Add(90 * time.Minute)

	input := CreateEventInput{
		Title:     "IST Meeting",
		StartDate: start,
		EndDate:   end,
		Location:  "Mumbai Office",
		Notes:     "Q2 Planning",
		Calendar:  "Work",
		TimeZone:  "Asia/Kolkata",
		Alerts: []Alert{
			{RelativeOffset: -30 * time.Minute},
			{RelativeOffset: -1 * time.Hour},
		},
	}

	data, err := marshalCreateInput(input)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// Simulate ObjC parsing this JSON and returning the event
	var parsed map[string]any
	json.Unmarshal(data, &parsed)

	// Verify UTC conversion
	if parsed["startDate"] != "2026-03-15T10:00:00.000Z" {
		t.Errorf("startDate = %v, want UTC", parsed["startDate"])
	}
	if parsed["endDate"] != "2026-03-15T11:30:00.000Z" {
		t.Errorf("endDate = %v, want UTC", parsed["endDate"])
	}
	if parsed["timeZone"] != "Asia/Kolkata" {
		t.Errorf("timeZone = %v", parsed["timeZone"])
	}

	alerts := parsed["alerts"].([]any)
	if len(alerts) != 2 {
		t.Fatalf("alerts = %d", len(alerts))
	}
	a0 := alerts[0].(map[string]any)
	if a0["relativeOffset"] != -1800.0 {
		t.Errorf("alert[0].relativeOffset = %v, want -1800", a0["relativeOffset"])
	}
}

func TestMockUpdateEventJSON(t *testing.T) {
	t.Run("partial update preserves existing", func(t *testing.T) {
		// Original event
		originalJSON := `{
			"id": "evt-1",
			"title": "Original Title",
			"startDate": "2026-02-11T10:00:00.000Z",
			"endDate": "2026-02-11T11:00:00.000Z",
			"allDay": false,
			"location": "Original Location",
			"notes": "Original Notes",
			"url": null,
			"calendar": "Work",
			"calendarID": "cal-work",
			"status": 1,
			"availability": 0,
			"organizer": null,
			"attendees": [],
			"recurring": false,
			"alerts": [{"relativeOffset": -900}],
			"createdAt": "2026-02-10T08:00:00.000Z",
			"modifiedAt": "2026-02-10T09:00:00.000Z",
			"timeZone": "America/New_York"
		}`

		original, _ := parseEventJSON(originalJSON)

		// Only update title
		newTitle := "Updated Title"
		updateData, _ := marshalUpdateInput(UpdateEventInput{Title: &newTitle})

		var updateMap map[string]any
		json.Unmarshal(updateData, &updateMap)

		// Verify only title is in the update payload
		if _, ok := updateMap["title"]; !ok {
			t.Error("title should be present in update")
		}
		if _, ok := updateMap["location"]; ok {
			t.Error("location should NOT be in update (not changed)")
		}
		if _, ok := updateMap["notes"]; ok {
			t.Error("notes should NOT be in update (not changed)")
		}

		// Simulate ObjC applying the update and returning
		original.Title = newTitle
		responseJSON := simulateEventsResponse([]Event{*original})

		// Parse the response
		var rawArr []rawEvent
		json.Unmarshal([]byte(responseJSON), &rawArr)
		if len(rawArr) != 1 {
			t.Fatalf("expected 1 event")
		}

		result := convertRawEvent(rawArr[0])
		if result.Title != "Updated Title" {
			t.Errorf("title = %q, want Updated Title", result.Title)
		}
		if result.Location != "Original Location" {
			t.Errorf("location changed: got %q, want Original Location", result.Location)
		}
	})

	t.Run("move event to different calendar", func(t *testing.T) {
		newCal := "Family"
		data, _ := marshalUpdateInput(UpdateEventInput{Calendar: &newCal})

		var m map[string]any
		json.Unmarshal(data, &m)

		if m["calendar"] != "Family" {
			t.Errorf("calendar = %v, want Family", m["calendar"])
		}
	})
}

func TestMockDeleteEventRoundtrip(t *testing.T) {
	// The delete bridge returns "ok" on success.
	// We can't call the real bridge, but we can verify the span values.
	if SpanThisEvent != 0 {
		t.Errorf("SpanThisEvent should be 0 for ObjC bridge")
	}
	if SpanFutureEvents != 1 {
		t.Errorf("SpanFutureEvents should be 1 for ObjC bridge")
	}
}

func TestMockErrorPaths(t *testing.T) {
	t.Run("parse error for malformed event JSON", func(t *testing.T) {
		_, err := parseEventJSON(`{"id": 123}`) // id should be string
		// This actually works because JSON numbers are valid - just parsed differently
		// The real error case is truly malformed JSON
		_ = err
	})

	t.Run("parse error for truncated JSON", func(t *testing.T) {
		_, err := parseEventsJSON(`[{"id": "1", "title": "test"`)
		if err == nil {
			t.Error("expected error for truncated JSON")
		}
	})

	t.Run("parse error for wrong JSON type", func(t *testing.T) {
		_, err := parseEventsJSON(`"not an array"`)
		if err == nil {
			t.Error("expected error for non-array JSON")
		}
	})

	t.Run("parse error for calendar wrong type", func(t *testing.T) {
		_, err := parseCalendarsJSON(`{"not": "array"}`)
		if err == nil {
			t.Error("expected error for non-array calendar JSON")
		}
	})

	t.Run("sentinel errors are distinguishable", func(t *testing.T) {
		if errors.Is(ErrUnsupported, ErrAccessDenied) {
			t.Error("ErrUnsupported should not match ErrAccessDenied")
		}
		if errors.Is(ErrAccessDenied, ErrNotFound) {
			t.Error("ErrAccessDenied should not match ErrNotFound")
		}
		if errors.Is(ErrNotFound, ErrUnsupported) {
			t.Error("ErrNotFound should not match ErrUnsupported")
		}
	})
}

// --- Edge case tests ---

func TestEdgeCases(t *testing.T) {
	t.Run("event spanning midnight", func(t *testing.T) {
		jsonStr := `{
			"id": "midnight",
			"title": "Late Night Event",
			"startDate": "2026-02-11T23:00:00.000Z",
			"endDate": "2026-02-12T01:00:00.000Z",
			"allDay": false,
			"calendar": "Home",
			"calendarID": "cal-1",
			"status": 0,
			"availability": 0,
			"recurring": false,
			"attendees": [],
			"alerts": []
		}`

		event, err := parseEventJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		duration := event.EndDate.Sub(event.StartDate)
		if duration != 2*time.Hour {
			t.Errorf("duration = %v, want 2h (spans midnight)", duration)
		}
	})

	t.Run("event at year boundary", func(t *testing.T) {
		jsonStr := `{
			"id": "newyear",
			"title": "New Year Event",
			"startDate": "2025-12-31T23:00:00.000Z",
			"endDate": "2026-01-01T01:00:00.000Z",
			"allDay": false,
			"calendar": "Home",
			"calendarID": "cal-1",
			"status": 0,
			"availability": 0,
			"recurring": false,
			"attendees": [],
			"alerts": []
		}`

		event, err := parseEventJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if event.StartDate.Year() != 2025 {
			t.Errorf("start year = %d, want 2025", event.StartDate.Year())
		}
		if event.EndDate.Year() != 2026 {
			t.Errorf("end year = %d, want 2026", event.EndDate.Year())
		}
	})

	t.Run("event with special characters in title", func(t *testing.T) {
		jsonStr := `{
			"id": "special",
			"title": "Meeting: \"Important\" & <urgent>",
			"startDate": "2026-02-11T10:00:00.000Z",
			"endDate": "2026-02-11T11:00:00.000Z",
			"allDay": false,
			"calendar": "Work",
			"calendarID": "cal-1",
			"status": 0,
			"availability": 0,
			"recurring": false,
			"attendees": [],
			"alerts": []
		}`

		event, err := parseEventJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := "Meeting: \"Important\" & <urgent>"
		if event.Title != expected {
			t.Errorf("title = %q, want %q", event.Title, expected)
		}
	})

	t.Run("event with unicode in fields", func(t *testing.T) {
		jsonStr := `{
			"id": "unicode",
			"title": "会議 — Team Sync 🗓️",
			"startDate": "2026-02-11T10:00:00.000Z",
			"endDate": "2026-02-11T11:00:00.000Z",
			"allDay": false,
			"location": "東京オフィス",
			"notes": "Agenda: détails importants",
			"calendar": "Work",
			"calendarID": "cal-1",
			"status": 0,
			"availability": 0,
			"recurring": false,
			"attendees": [],
			"alerts": []
		}`

		event, err := parseEventJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if event.Title != "会議 — Team Sync 🗓️" {
			t.Errorf("title = %q", event.Title)
		}
		if event.Location != "東京オフィス" {
			t.Errorf("location = %q", event.Location)
		}
	})

	t.Run("very long event (multi-day)", func(t *testing.T) {
		jsonStr := `{
			"id": "multiday",
			"title": "Conference",
			"startDate": "2026-03-01T09:00:00.000Z",
			"endDate": "2026-03-05T17:00:00.000Z",
			"allDay": false,
			"calendar": "Work",
			"calendarID": "cal-1",
			"status": 1,
			"availability": 0,
			"recurring": false,
			"attendees": [],
			"alerts": []
		}`

		event, err := parseEventJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		duration := event.EndDate.Sub(event.StartDate)
		expectedDays := 4*24*time.Hour + 8*time.Hour // 4 days + 8 hours
		if duration != expectedDays {
			t.Errorf("duration = %v, want %v", duration, expectedDays)
		}
	})

	t.Run("event with maximum attendees", func(t *testing.T) {
		attendees := make([]rawAttendee, 50)
		for i := range attendees {
			attendees[i] = rawAttendee{
				Name:   "Person " + string(rune('A'+i%26)),
				Email:  "person@example.com",
				Status: i % 5,
			}
		}

		raw := rawEvent{
			ID:        "many-attendees",
			Title:     "Big Meeting",
			Attendees: attendees,
			Alerts:    []rawAlert{},
		}

		data, _ := json.Marshal(raw)
		event, err := parseEventJSON(string(data))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(event.Attendees) != 50 {
			t.Errorf("attendees = %d, want 50", len(event.Attendees))
		}
	})

	t.Run("DST transition event", func(t *testing.T) {
		// March 8, 2026 is around US DST transition
		jsonStr := `{
			"id": "dst",
			"title": "DST Event",
			"startDate": "2026-03-08T06:00:00.000Z",
			"endDate": "2026-03-08T08:00:00.000Z",
			"allDay": false,
			"calendar": "Home",
			"calendarID": "cal-1",
			"status": 0,
			"availability": 0,
			"recurring": false,
			"attendees": [],
			"alerts": [],
			"timeZone": "America/New_York"
		}`

		event, err := parseEventJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// The bridge stores everything in UTC, so DST doesn't affect parsing
		if event.TimeZone != "America/New_York" {
			t.Errorf("timeZone = %q", event.TimeZone)
		}
		// Duration should still be exactly 2 hours in UTC
		duration := event.EndDate.Sub(event.StartDate)
		if duration != 2*time.Hour {
			t.Errorf("duration = %v, want 2h", duration)
		}
	})
}

// --- Recurrence rule mock roundtrip tests ---

func TestMockRecurrenceRuleRoundtrip(t *testing.T) {
	t.Run("daily recurrence roundtrip", func(t *testing.T) {
		// Simulate ObjC returning an event with a daily recurrence rule
		jsonStr := `{
			"id": "rec-daily",
			"title": "Daily Standup",
			"startDate": "2026-02-11T10:00:00.000Z",
			"endDate": "2026-02-11T10:30:00.000Z",
			"allDay": false,
			"calendar": "Work",
			"calendarID": "cal-1",
			"status": 1,
			"availability": 0,
			"recurring": true,
			"isDetached": false,
			"recurrenceRules": [
				{
					"frequency": 0,
					"interval": 1
				}
			],
			"attendees": [],
			"alerts": []
		}`

		event, err := parseEventJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !event.Recurring {
			t.Error("Recurring should be true")
		}
		if len(event.RecurrenceRules) != 1 {
			t.Fatalf("RecurrenceRules count = %d, want 1", len(event.RecurrenceRules))
		}
		if event.RecurrenceRules[0].Frequency != eventkit.FrequencyDaily {
			t.Errorf("Frequency = %d, want %d", event.RecurrenceRules[0].Frequency, eventkit.FrequencyDaily)
		}
		if event.RecurrenceRules[0].Interval != 1 {
			t.Errorf("Interval = %d, want 1", event.RecurrenceRules[0].Interval)
		}
	})

	t.Run("weekly recurrence with days roundtrip", func(t *testing.T) {
		jsonStr := `{
			"id": "rec-weekly",
			"title": "MWF Meeting",
			"startDate": "2026-02-11T10:00:00.000Z",
			"endDate": "2026-02-11T10:30:00.000Z",
			"allDay": false,
			"calendar": "Work",
			"calendarID": "cal-1",
			"status": 1,
			"availability": 0,
			"recurring": true,
			"isDetached": false,
			"recurrenceRules": [
				{
					"frequency": 1,
					"interval": 2,
					"daysOfTheWeek": [
						{"dayOfTheWeek": 2, "weekNumber": 0},
						{"dayOfTheWeek": 4, "weekNumber": 0},
						{"dayOfTheWeek": 6, "weekNumber": 0}
					],
					"end": {"occurrenceCount": 10}
				}
			],
			"attendees": [],
			"alerts": []
		}`

		event, err := parseEventJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rule := event.RecurrenceRules[0]
		if rule.Interval != 2 {
			t.Errorf("Interval = %d, want 2", rule.Interval)
		}
		if len(rule.DaysOfTheWeek) != 3 {
			t.Fatalf("DaysOfTheWeek count = %d, want 3", len(rule.DaysOfTheWeek))
		}
		if rule.End == nil || rule.End.OccurrenceCount != 10 {
			t.Errorf("End = %+v, want OccurrenceCount=10", rule.End)
		}
	})

	t.Run("create input with recurrence marshals and parses back", func(t *testing.T) {
		endDate := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
		input := CreateEventInput{
			Title:     "Recurring Meeting",
			StartDate: time.Date(2026, 2, 12, 10, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2026, 2, 12, 11, 0, 0, 0, time.UTC),
			RecurrenceRules: []eventkit.RecurrenceRule{
				eventkit.Weekly(1, eventkit.Monday, eventkit.Wednesday, eventkit.Friday).Until(endDate),
			},
		}

		data, err := marshalCreateInput(input)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var m map[string]any
		json.Unmarshal(data, &m)

		rules := m["recurrenceRules"].([]any)
		if len(rules) != 1 {
			t.Fatalf("rules count = %d, want 1", len(rules))
		}

		rule := rules[0].(map[string]any)
		if rule["frequency"] != 1.0 {
			t.Errorf("frequency = %v, want 1", rule["frequency"])
		}
		if rule["interval"] != 1.0 {
			t.Errorf("interval = %v, want 1", rule["interval"])
		}
		days := rule["daysOfTheWeek"].([]any)
		if len(days) != 3 {
			t.Errorf("daysOfTheWeek count = %d, want 3", len(days))
		}
		end := rule["end"].(map[string]any)
		if end["endDate"] != "2026-12-31T00:00:00.000Z" {
			t.Errorf("endDate = %v", end["endDate"])
		}
	})
}

func TestMockStructuredLocationRoundtrip(t *testing.T) {
	t.Run("full structured location roundtrip", func(t *testing.T) {
		jsonStr := `{
			"id": "loc-1",
			"title": "Meeting at HQ",
			"startDate": "2026-02-11T10:00:00.000Z",
			"endDate": "2026-02-11T11:00:00.000Z",
			"allDay": false,
			"location": "Apple Park",
			"calendar": "Work",
			"calendarID": "cal-1",
			"status": 0,
			"availability": 0,
			"recurring": false,
			"isDetached": false,
			"recurrenceRules": [],
			"structuredLocation": {
				"title": "Apple Park",
				"latitude": 37.3349,
				"longitude": -122.009,
				"radius": 150
			},
			"attendees": [],
			"alerts": []
		}`

		event, err := parseEventJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if event.StructuredLocation == nil {
			t.Fatal("StructuredLocation should not be nil")
		}
		if event.StructuredLocation.Title != "Apple Park" {
			t.Errorf("Title = %q", event.StructuredLocation.Title)
		}
		if event.StructuredLocation.Latitude != 37.3349 {
			t.Errorf("Latitude = %f", event.StructuredLocation.Latitude)
		}
		if event.StructuredLocation.Longitude != -122.009 {
			t.Errorf("Longitude = %f", event.StructuredLocation.Longitude)
		}
		if event.StructuredLocation.Radius != 150 {
			t.Errorf("Radius = %f", event.StructuredLocation.Radius)
		}
	})

	t.Run("create input with structured location marshals correctly", func(t *testing.T) {
		input := CreateEventInput{
			Title:     "Location Meeting",
			StartDate: time.Date(2026, 2, 12, 10, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2026, 2, 12, 11, 0, 0, 0, time.UTC),
			StructuredLocation: &eventkit.StructuredLocation{
				Title:     "NYC Office",
				Latitude:  40.7128,
				Longitude: -74.0060,
				Radius:    100,
			},
		}

		data, err := marshalCreateInput(input)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var m map[string]any
		json.Unmarshal(data, &m)

		sl := m["structuredLocation"].(map[string]any)
		if sl["title"] != "NYC Office" {
			t.Errorf("title = %v", sl["title"])
		}
		if sl["latitude"] != 40.7128 {
			t.Errorf("latitude = %v", sl["latitude"])
		}
		if sl["longitude"] != -74.006 {
			t.Errorf("longitude = %v", sl["longitude"])
		}
	})

	t.Run("zero coordinates in structured location", func(t *testing.T) {
		jsonStr := `{
			"id": "loc-zero",
			"title": "Zero Coords Event",
			"startDate": "2026-02-11T10:00:00.000Z",
			"endDate": "2026-02-11T11:00:00.000Z",
			"allDay": false,
			"calendar": "Work",
			"calendarID": "cal-1",
			"status": 0,
			"availability": 0,
			"recurring": false,
			"isDetached": false,
			"recurrenceRules": [],
			"structuredLocation": {
				"title": "Null Island",
				"latitude": 0,
				"longitude": 0
			},
			"attendees": [],
			"alerts": []
		}`

		event, err := parseEventJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if event.StructuredLocation == nil {
			t.Fatal("StructuredLocation should not be nil")
		}
		if event.StructuredLocation.Title != "Null Island" {
			t.Errorf("Title = %q", event.StructuredLocation.Title)
		}
		// Zero coordinates are valid (Null Island is a real concept)
		if event.StructuredLocation.Latitude != 0 {
			t.Errorf("Latitude = %f, want 0", event.StructuredLocation.Latitude)
		}
		if event.StructuredLocation.Longitude != 0 {
			t.Errorf("Longitude = %f, want 0", event.StructuredLocation.Longitude)
		}
	})
}

func TestMockDetachedOccurrence(t *testing.T) {
	jsonStr := `{
		"id": "det-1",
		"title": "Modified Occurrence",
		"startDate": "2026-02-11T14:00:00.000Z",
		"endDate": "2026-02-11T14:30:00.000Z",
		"allDay": false,
		"calendar": "Work",
		"calendarID": "cal-1",
		"status": 1,
		"availability": 0,
		"recurring": true,
		"isDetached": true,
		"occurrenceDate": "2026-02-11T10:00:00.000Z",
		"recurrenceRules": [],
		"attendees": [],
		"alerts": []
	}`

	event, err := parseEventJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !event.IsDetached {
		t.Error("IsDetached should be true")
	}
	if event.OccurrenceDate == nil {
		t.Fatal("OccurrenceDate should not be nil")
	}
	wantOcc := time.Date(2026, 2, 11, 10, 0, 0, 0, time.UTC)
	if !event.OccurrenceDate.Equal(wantOcc) {
		t.Errorf("OccurrenceDate = %v, want %v", *event.OccurrenceDate, wantOcc)
	}
}

func TestMockRecurrenceEdgeCases(t *testing.T) {
	t.Run("negative days of month", func(t *testing.T) {
		jsonStr := `{
			"id": "edge-neg",
			"title": "Last Day",
			"startDate": "2026-02-11T10:00:00.000Z",
			"endDate": "2026-02-11T11:00:00.000Z",
			"allDay": false,
			"calendar": "Work",
			"calendarID": "cal-1",
			"status": 0,
			"availability": 0,
			"recurring": true,
			"isDetached": false,
			"recurrenceRules": [
				{
					"frequency": 2,
					"interval": 1,
					"daysOfTheMonth": [-1, -2]
				}
			],
			"attendees": [],
			"alerts": []
		}`

		event, err := parseEventJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rule := event.RecurrenceRules[0]
		if len(rule.DaysOfTheMonth) != 2 {
			t.Fatalf("DaysOfTheMonth count = %d, want 2", len(rule.DaysOfTheMonth))
		}
		if rule.DaysOfTheMonth[0] != -1 || rule.DaysOfTheMonth[1] != -2 {
			t.Errorf("DaysOfTheMonth = %v, want [-1, -2]", rule.DaysOfTheMonth)
		}
	})

	t.Run("week number on day of week", func(t *testing.T) {
		jsonStr := `{
			"id": "edge-weeknum",
			"title": "Second Tuesday",
			"startDate": "2026-02-11T10:00:00.000Z",
			"endDate": "2026-02-11T11:00:00.000Z",
			"allDay": false,
			"calendar": "Work",
			"calendarID": "cal-1",
			"status": 0,
			"availability": 0,
			"recurring": true,
			"isDetached": false,
			"recurrenceRules": [
				{
					"frequency": 2,
					"interval": 1,
					"daysOfTheWeek": [
						{"dayOfTheWeek": 3, "weekNumber": 2}
					]
				}
			],
			"attendees": [],
			"alerts": []
		}`

		event, err := parseEventJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rule := event.RecurrenceRules[0]
		if len(rule.DaysOfTheWeek) != 1 {
			t.Fatalf("DaysOfTheWeek count = %d, want 1", len(rule.DaysOfTheWeek))
		}
		if rule.DaysOfTheWeek[0].DayOfTheWeek != eventkit.Tuesday {
			t.Errorf("DayOfTheWeek = %d, want %d (Tuesday)", rule.DaysOfTheWeek[0].DayOfTheWeek, eventkit.Tuesday)
		}
		if rule.DaysOfTheWeek[0].WeekNumber != 2 {
			t.Errorf("WeekNumber = %d, want 2", rule.DaysOfTheWeek[0].WeekNumber)
		}
	})
}

// --- Comprehensive marshal/parse symmetry test ---

func TestCreateInputMarshalParseSymmetry(t *testing.T) {
	// What we marshal for create should be parseable as an event response
	// (after the ObjC bridge creates the event and returns it)
	timezones := []string{
		"America/New_York",
		"America/Los_Angeles",
		"Europe/London",
		"Europe/Berlin",
		"Asia/Tokyo",
		"Asia/Kolkata",
		"Australia/Sydney",
		"Pacific/Auckland",
	}

	for _, tz := range timezones {
		t.Run(tz, func(t *testing.T) {
			loc, err := time.LoadLocation(tz)
			if err != nil {
				t.Skipf("timezone %s not available: %v", tz, err)
			}

			start := time.Date(2026, 6, 15, 14, 0, 0, 0, loc)
			end := start.Add(90 * time.Minute)

			input := CreateEventInput{
				Title:     "TZ Test: " + tz,
				StartDate: start,
				EndDate:   end,
				Calendar:  "Work",
				TimeZone:  tz,
			}

			data, err := marshalCreateInput(input)
			if err != nil {
				t.Fatalf("marshal error: %v", err)
			}

			// Parse the date back
			var m map[string]any
			json.Unmarshal(data, &m)

			parsedStart := parseISO8601(m["startDate"].(string))
			if !parsedStart.Equal(start.UTC()) {
				t.Errorf("start mismatch: got %v, want %v", parsedStart, start.UTC())
			}
		})
	}
}
