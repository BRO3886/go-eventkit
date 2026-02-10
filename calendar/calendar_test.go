package calendar

import (
	"encoding/json"
	"testing"
	"time"
)

// --- Enum String() tests ---

func TestEventStatusString(t *testing.T) {
	tests := []struct {
		s    EventStatus
		want string
	}{
		{StatusNone, "none"},
		{StatusConfirmed, "confirmed"},
		{StatusTentative, "tentative"},
		{StatusCanceled, "canceled"},
		{EventStatus(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.s.String(); got != tt.want {
				t.Errorf("EventStatus(%d).String() = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

func TestAvailabilityString(t *testing.T) {
	tests := []struct {
		a    Availability
		want string
	}{
		{AvailabilityNotSupported, "notSupported"},
		{AvailabilityBusy, "busy"},
		{AvailabilityFree, "free"},
		{AvailabilityTentative, "tentative"},
		{AvailabilityUnavailable, "unavailable"},
		{Availability(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.a.String(); got != tt.want {
				t.Errorf("Availability(%d).String() = %q, want %q", tt.a, got, tt.want)
			}
		})
	}
}

func TestCalendarTypeString(t *testing.T) {
	tests := []struct {
		ct   CalendarType
		want string
	}{
		{CalendarTypeLocal, "local"},
		{CalendarTypeCalDAV, "caldav"},
		{CalendarTypeExchange, "exchange"},
		{CalendarTypeBirthday, "birthday"},
		{CalendarTypeSubscription, "subscription"},
		{CalendarType(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.ct.String(); got != tt.want {
				t.Errorf("CalendarType(%d).String() = %q, want %q", tt.ct, got, tt.want)
			}
		})
	}
}

func TestParticipantStatusString(t *testing.T) {
	tests := []struct {
		s    ParticipantStatus
		want string
	}{
		{ParticipantStatusUnknown, "unknown"},
		{ParticipantStatusPending, "pending"},
		{ParticipantStatusAccepted, "accepted"},
		{ParticipantStatusDeclined, "declined"},
		{ParticipantStatusTentative, "tentative"},
		{ParticipantStatus(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.s.String(); got != tt.want {
				t.Errorf("ParticipantStatus(%d).String() = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

func TestSpanString(t *testing.T) {
	tests := []struct {
		s    Span
		want string
	}{
		{SpanThisEvent, "thisEvent"},
		{SpanFutureEvents, "futureEvents"},
		{Span(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.s.String(); got != tt.want {
				t.Errorf("Span(%d).String() = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

// --- ISO 8601 date parsing tests ---

func TestParseISO8601(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{
			name:  "with fractional seconds",
			input: "2026-02-11T10:30:00.000Z",
			want:  time.Date(2026, 2, 11, 10, 30, 0, 0, time.UTC),
		},
		{
			name:  "without fractional seconds",
			input: "2026-02-11T10:30:00Z",
			want:  time.Date(2026, 2, 11, 10, 30, 0, 0, time.UTC),
		},
		{
			name:  "RFC3339 format",
			input: "2026-02-11T10:30:00+00:00",
			want:  time.Date(2026, 2, 11, 10, 30, 0, 0, time.UTC),
		},
		{
			name:  "empty string",
			input: "",
			want:  time.Time{},
		},
		{
			name:  "invalid format",
			input: "not-a-date",
			want:  time.Time{},
		},
		{
			name:  "midnight UTC",
			input: "2026-01-01T00:00:00.000Z",
			want:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "end of day",
			input: "2026-12-31T23:59:59.999Z",
			want:  time.Date(2026, 12, 31, 23, 59, 59, 999000000, time.UTC),
		},
		{
			name:  "RFC3339 with timezone offset",
			input: "2026-02-11T15:30:00+05:00",
			want:  time.Date(2026, 2, 11, 10, 30, 0, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseISO8601(tt.input)
			if !got.Equal(tt.want) {
				t.Errorf("parseISO8601(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// --- Event JSON parsing tests ---

func TestParseEventJSON(t *testing.T) {
	t.Run("full event", func(t *testing.T) {
		jsonStr := `{
			"id": "ABC-123-DEF",
			"title": "Team Standup",
			"startDate": "2026-02-11T10:00:00.000Z",
			"endDate": "2026-02-11T10:30:00.000Z",
			"allDay": false,
			"location": "Conference Room A",
			"notes": "Discuss sprint progress",
			"url": "https://meet.example.com/standup",
			"calendar": "Work",
			"calendarID": "cal-work-123",
			"status": 1,
			"availability": 0,
			"organizer": "Alice Smith",
			"attendees": [
				{"name": "Bob Jones", "email": "bob@example.com", "status": 2},
				{"name": "Carol White", "email": "carol@example.com", "status": 4}
			],
			"recurring": false,
			"alerts": [
				{"relativeOffset": -900},
				{"relativeOffset": -300}
			],
			"createdAt": "2026-02-10T08:00:00.000Z",
			"modifiedAt": "2026-02-10T09:00:00.000Z",
			"timeZone": "America/New_York"
		}`

		event, err := parseEventJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if event.ID != "ABC-123-DEF" {
			t.Errorf("ID = %q, want %q", event.ID, "ABC-123-DEF")
		}
		if event.Title != "Team Standup" {
			t.Errorf("Title = %q, want %q", event.Title, "Team Standup")
		}
		if event.AllDay {
			t.Error("AllDay = true, want false")
		}
		if event.Location != "Conference Room A" {
			t.Errorf("Location = %q, want %q", event.Location, "Conference Room A")
		}
		if event.Notes != "Discuss sprint progress" {
			t.Errorf("Notes = %q, want %q", event.Notes, "Discuss sprint progress")
		}
		if event.URL != "https://meet.example.com/standup" {
			t.Errorf("URL = %q, want %q", event.URL, "https://meet.example.com/standup")
		}
		if event.Calendar != "Work" {
			t.Errorf("Calendar = %q, want %q", event.Calendar, "Work")
		}
		if event.CalendarID != "cal-work-123" {
			t.Errorf("CalendarID = %q, want %q", event.CalendarID, "cal-work-123")
		}
		if event.Status != StatusConfirmed {
			t.Errorf("Status = %d, want %d (confirmed)", event.Status, StatusConfirmed)
		}
		if event.Availability != AvailabilityBusy {
			t.Errorf("Availability = %d, want %d (busy)", event.Availability, AvailabilityBusy)
		}
		if event.Organizer != "Alice Smith" {
			t.Errorf("Organizer = %q, want %q", event.Organizer, "Alice Smith")
		}
		if event.Recurring {
			t.Error("Recurring = true, want false")
		}
		if event.TimeZone != "America/New_York" {
			t.Errorf("TimeZone = %q, want %q", event.TimeZone, "America/New_York")
		}

		// Dates
		wantStart := time.Date(2026, 2, 11, 10, 0, 0, 0, time.UTC)
		if !event.StartDate.Equal(wantStart) {
			t.Errorf("StartDate = %v, want %v", event.StartDate, wantStart)
		}
		wantEnd := time.Date(2026, 2, 11, 10, 30, 0, 0, time.UTC)
		if !event.EndDate.Equal(wantEnd) {
			t.Errorf("EndDate = %v, want %v", event.EndDate, wantEnd)
		}

		// Attendees
		if len(event.Attendees) != 2 {
			t.Fatalf("Attendees count = %d, want 2", len(event.Attendees))
		}
		if event.Attendees[0].Name != "Bob Jones" {
			t.Errorf("Attendee[0].Name = %q, want %q", event.Attendees[0].Name, "Bob Jones")
		}
		if event.Attendees[0].Email != "bob@example.com" {
			t.Errorf("Attendee[0].Email = %q, want %q", event.Attendees[0].Email, "bob@example.com")
		}
		if event.Attendees[0].Status != ParticipantStatusAccepted {
			t.Errorf("Attendee[0].Status = %d, want %d (accepted)", event.Attendees[0].Status, ParticipantStatusAccepted)
		}
		if event.Attendees[1].Status != ParticipantStatusTentative {
			t.Errorf("Attendee[1].Status = %d, want %d (tentative)", event.Attendees[1].Status, ParticipantStatusTentative)
		}

		// Alerts
		if len(event.Alerts) != 2 {
			t.Fatalf("Alerts count = %d, want 2", len(event.Alerts))
		}
		if event.Alerts[0].RelativeOffset != -15*time.Minute {
			t.Errorf("Alert[0].RelativeOffset = %v, want %v", event.Alerts[0].RelativeOffset, -15*time.Minute)
		}
		if event.Alerts[1].RelativeOffset != -5*time.Minute {
			t.Errorf("Alert[1].RelativeOffset = %v, want %v", event.Alerts[1].RelativeOffset, -5*time.Minute)
		}
	})

	t.Run("minimal event with null fields", func(t *testing.T) {
		jsonStr := `{
			"id": "MIN-001",
			"title": "Quick Chat",
			"startDate": "2026-02-11T14:00:00.000Z",
			"endDate": "2026-02-11T14:15:00.000Z",
			"allDay": false,
			"location": null,
			"notes": null,
			"url": null,
			"calendar": "Home",
			"calendarID": "cal-home-456",
			"status": 0,
			"availability": 1,
			"organizer": null,
			"attendees": [],
			"recurring": false,
			"alerts": [],
			"createdAt": null,
			"modifiedAt": null,
			"timeZone": null
		}`

		event, err := parseEventJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if event.Location != "" {
			t.Errorf("Location = %q, want empty", event.Location)
		}
		if event.Notes != "" {
			t.Errorf("Notes = %q, want empty", event.Notes)
		}
		if event.URL != "" {
			t.Errorf("URL = %q, want empty", event.URL)
		}
		if event.Organizer != "" {
			t.Errorf("Organizer = %q, want empty", event.Organizer)
		}
		if event.TimeZone != "" {
			t.Errorf("TimeZone = %q, want empty", event.TimeZone)
		}
		if len(event.Attendees) != 0 {
			t.Errorf("Attendees count = %d, want 0", len(event.Attendees))
		}
		if len(event.Alerts) != 0 {
			t.Errorf("Alerts count = %d, want 0", len(event.Alerts))
		}
		if event.Status != StatusNone {
			t.Errorf("Status = %d, want %d (none)", event.Status, StatusNone)
		}
		if event.Availability != AvailabilityFree {
			t.Errorf("Availability = %d, want %d (free)", event.Availability, AvailabilityFree)
		}
	})

	t.Run("all-day event", func(t *testing.T) {
		jsonStr := `{
			"id": "ALLDAY-001",
			"title": "Holiday",
			"startDate": "2026-02-14T00:00:00.000Z",
			"endDate": "2026-02-15T00:00:00.000Z",
			"allDay": true,
			"location": null,
			"notes": null,
			"url": null,
			"calendar": "Family",
			"calendarID": "cal-fam-789",
			"status": 1,
			"availability": 3,
			"organizer": null,
			"attendees": [],
			"recurring": false,
			"alerts": [],
			"createdAt": "2026-01-01T00:00:00.000Z",
			"modifiedAt": "2026-01-01T00:00:00.000Z",
			"timeZone": null
		}`

		event, err := parseEventJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !event.AllDay {
			t.Error("AllDay = false, want true")
		}
		if event.Availability != AvailabilityUnavailable {
			t.Errorf("Availability = %d, want %d (unavailable)", event.Availability, AvailabilityUnavailable)
		}
		if event.Calendar != "Family" {
			t.Errorf("Calendar = %q, want %q", event.Calendar, "Family")
		}
	})

	t.Run("recurring event", func(t *testing.T) {
		jsonStr := `{
			"id": "REC-001",
			"title": "Weekly Standup",
			"startDate": "2026-02-11T10:00:00.000Z",
			"endDate": "2026-02-11T10:30:00.000Z",
			"allDay": false,
			"location": null,
			"notes": null,
			"url": null,
			"calendar": "Work",
			"calendarID": "cal-work-123",
			"status": 1,
			"availability": 0,
			"organizer": null,
			"attendees": [],
			"recurring": true,
			"alerts": [],
			"createdAt": null,
			"modifiedAt": null,
			"timeZone": "America/Los_Angeles"
		}`

		event, err := parseEventJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !event.Recurring {
			t.Error("Recurring = false, want true")
		}
		if event.TimeZone != "America/Los_Angeles" {
			t.Errorf("TimeZone = %q, want %q", event.TimeZone, "America/Los_Angeles")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := parseEventJSON("not valid json")
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("empty JSON object", func(t *testing.T) {
		event, err := parseEventJSON("{}")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if event.ID != "" {
			t.Errorf("ID = %q, want empty", event.ID)
		}
	})
}

func TestParseEventsJSON(t *testing.T) {
	t.Run("multiple events", func(t *testing.T) {
		jsonStr := `[
			{"id": "E1", "title": "Event 1", "startDate": "2026-02-11T10:00:00.000Z", "endDate": "2026-02-11T11:00:00.000Z", "allDay": false, "calendar": "Work", "calendarID": "c1", "status": 0, "availability": 0, "recurring": false, "attendees": [], "alerts": []},
			{"id": "E2", "title": "Event 2", "startDate": "2026-02-11T14:00:00.000Z", "endDate": "2026-02-11T15:00:00.000Z", "allDay": false, "calendar": "Home", "calendarID": "c2", "status": 1, "availability": 1, "recurring": false, "attendees": [], "alerts": []}
		]`

		events, err := parseEventsJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("events count = %d, want 2", len(events))
		}
		if events[0].Title != "Event 1" {
			t.Errorf("events[0].Title = %q, want %q", events[0].Title, "Event 1")
		}
		if events[1].Calendar != "Home" {
			t.Errorf("events[1].Calendar = %q, want %q", events[1].Calendar, "Home")
		}
	})

	t.Run("empty array", func(t *testing.T) {
		events, err := parseEventsJSON("[]")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(events) != 0 {
			t.Errorf("events count = %d, want 0", len(events))
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := parseEventsJSON("not json")
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

// --- Calendar JSON parsing tests ---

func TestParseCalendarsJSON(t *testing.T) {
	t.Run("multiple calendars", func(t *testing.T) {
		jsonStr := `[
			{"id": "cal-1", "title": "Work", "type": 1, "color": "#FF0000", "source": "iCloud", "readOnly": false},
			{"id": "cal-2", "title": "Home", "type": 1, "color": "#00FF00", "source": "iCloud", "readOnly": false},
			{"id": "cal-3", "title": "Holidays", "type": 5, "color": "#0000FF", "source": "Subscriptions", "readOnly": true},
			{"id": "cal-4", "title": "Birthdays", "type": 4, "color": "#FFFF00", "source": "Other", "readOnly": true}
		]`

		calendars, err := parseCalendarsJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(calendars) != 4 {
			t.Fatalf("calendars count = %d, want 4", len(calendars))
		}

		// Work calendar
		if calendars[0].ID != "cal-1" {
			t.Errorf("cal[0].ID = %q, want %q", calendars[0].ID, "cal-1")
		}
		if calendars[0].Title != "Work" {
			t.Errorf("cal[0].Title = %q, want %q", calendars[0].Title, "Work")
		}
		if calendars[0].Type != CalendarTypeCalDAV {
			t.Errorf("cal[0].Type = %d, want %d (CalDAV)", calendars[0].Type, CalendarTypeCalDAV)
		}
		if calendars[0].Color != "#FF0000" {
			t.Errorf("cal[0].Color = %q, want %q", calendars[0].Color, "#FF0000")
		}
		if calendars[0].Source != "iCloud" {
			t.Errorf("cal[0].Source = %q, want %q", calendars[0].Source, "iCloud")
		}
		if calendars[0].ReadOnly {
			t.Error("cal[0].ReadOnly = true, want false")
		}

		// Subscribed calendar (read-only)
		if calendars[2].Type != CalendarTypeSubscription {
			t.Errorf("cal[2].Type = %d, want %d (subscription)", calendars[2].Type, CalendarTypeSubscription)
		}
		if !calendars[2].ReadOnly {
			t.Error("cal[2].ReadOnly = false, want true")
		}

		// Birthday calendar
		if calendars[3].Type != CalendarTypeBirthday {
			t.Errorf("cal[3].Type = %d, want %d (birthday)", calendars[3].Type, CalendarTypeBirthday)
		}
	})

	t.Run("empty array", func(t *testing.T) {
		calendars, err := parseCalendarsJSON("[]")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(calendars) != 0 {
			t.Errorf("calendars count = %d, want 0", len(calendars))
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := parseCalendarsJSON("invalid")
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

// --- Marshal tests ---

func TestMarshalCreateInput(t *testing.T) {
	t.Run("full input", func(t *testing.T) {
		start := time.Date(2026, 2, 12, 10, 0, 0, 0, time.UTC)
		end := time.Date(2026, 2, 12, 11, 0, 0, 0, time.UTC)

		input := CreateEventInput{
			Title:     "Team Meeting",
			StartDate: start,
			EndDate:   end,
			AllDay:    false,
			Location:  "Room 42",
			Notes:     "Bring laptop",
			URL:       "https://meet.example.com",
			Calendar:  "Work",
			TimeZone:  "America/New_York",
			Alerts: []Alert{
				{RelativeOffset: -15 * time.Minute},
				{RelativeOffset: -5 * time.Minute},
			},
		}

		data, err := marshalCreateInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result map[string]any
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}

		if result["title"] != "Team Meeting" {
			t.Errorf("title = %v, want %q", result["title"], "Team Meeting")
		}
		if result["startDate"] != "2026-02-12T10:00:00.000Z" {
			t.Errorf("startDate = %v, want %q", result["startDate"], "2026-02-12T10:00:00.000Z")
		}
		if result["location"] != "Room 42" {
			t.Errorf("location = %v, want %q", result["location"], "Room 42")
		}
		if result["calendar"] != "Work" {
			t.Errorf("calendar = %v, want %q", result["calendar"], "Work")
		}
		if result["timeZone"] != "America/New_York" {
			t.Errorf("timeZone = %v, want %q", result["timeZone"], "America/New_York")
		}

		alerts := result["alerts"].([]any)
		if len(alerts) != 2 {
			t.Fatalf("alerts count = %d, want 2", len(alerts))
		}
		alert0 := alerts[0].(map[string]any)
		if alert0["relativeOffset"] != -900.0 {
			t.Errorf("alert[0].relativeOffset = %v, want -900", alert0["relativeOffset"])
		}
	})

	t.Run("minimal input omits empty fields", func(t *testing.T) {
		input := CreateEventInput{
			Title:     "Quick Call",
			StartDate: time.Date(2026, 2, 12, 14, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2026, 2, 12, 14, 30, 0, 0, time.UTC),
		}

		data, err := marshalCreateInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result map[string]any
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}

		if _, ok := result["location"]; ok {
			t.Error("location should be omitted for empty string")
		}
		if _, ok := result["notes"]; ok {
			t.Error("notes should be omitted for empty string")
		}
		if _, ok := result["calendar"]; ok {
			t.Error("calendar should be omitted for empty string")
		}
		if _, ok := result["alerts"]; ok {
			t.Error("alerts should be omitted when empty")
		}
	})

	t.Run("timezone conversion", func(t *testing.T) {
		// Create event in IST (UTC+5:30), verify it's serialized as UTC.
		ist := time.FixedZone("IST", 5*3600+30*60)
		start := time.Date(2026, 2, 12, 15, 30, 0, 0, ist) // 10:00 UTC

		input := CreateEventInput{
			Title:     "IST Event",
			StartDate: start,
			EndDate:   start.Add(time.Hour),
		}

		data, err := marshalCreateInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result map[string]any
		json.Unmarshal(data, &result)

		if result["startDate"] != "2026-02-12T10:00:00.000Z" {
			t.Errorf("startDate = %v, want UTC time 2026-02-12T10:00:00.000Z", result["startDate"])
		}
	})

	t.Run("all-day event", func(t *testing.T) {
		input := CreateEventInput{
			Title:     "Holiday",
			StartDate: time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
			AllDay:    true,
		}

		data, err := marshalCreateInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result map[string]any
		json.Unmarshal(data, &result)

		if result["allDay"] != true {
			t.Errorf("allDay = %v, want true", result["allDay"])
		}
	})
}

func TestMarshalUpdateInput(t *testing.T) {
	t.Run("update only title", func(t *testing.T) {
		title := "Updated Title"
		input := UpdateEventInput{
			Title: &title,
		}

		data, err := marshalUpdateInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result map[string]any
		json.Unmarshal(data, &result)

		if result["title"] != "Updated Title" {
			t.Errorf("title = %v, want %q", result["title"], "Updated Title")
		}
		if _, ok := result["startDate"]; ok {
			t.Error("startDate should not be present when nil")
		}
		if _, ok := result["location"]; ok {
			t.Error("location should not be present when nil")
		}
	})

	t.Run("update multiple fields", func(t *testing.T) {
		title := "New Title"
		loc := "New Location"
		notes := "New Notes"
		allDay := true
		tz := "Europe/London"
		start := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
		end := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
		cal := "Family"
		url := "https://example.com"
		alerts := []Alert{{RelativeOffset: -10 * time.Minute}}

		input := UpdateEventInput{
			Title:     &title,
			Location:  &loc,
			Notes:     &notes,
			AllDay:    &allDay,
			TimeZone:  &tz,
			StartDate: &start,
			EndDate:   &end,
			Calendar:  &cal,
			URL:       &url,
			Alerts:    &alerts,
		}

		data, err := marshalUpdateInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result map[string]any
		json.Unmarshal(data, &result)

		if result["title"] != "New Title" {
			t.Errorf("title = %v", result["title"])
		}
		if result["location"] != "New Location" {
			t.Errorf("location = %v", result["location"])
		}
		if result["notes"] != "New Notes" {
			t.Errorf("notes = %v", result["notes"])
		}
		if result["allDay"] != true {
			t.Errorf("allDay = %v", result["allDay"])
		}
		if result["timeZone"] != "Europe/London" {
			t.Errorf("timeZone = %v", result["timeZone"])
		}
		if result["calendar"] != "Family" {
			t.Errorf("calendar = %v", result["calendar"])
		}
		if result["url"] != "https://example.com" {
			t.Errorf("url = %v", result["url"])
		}

		alertArr := result["alerts"].([]any)
		if len(alertArr) != 1 {
			t.Fatalf("alerts count = %d, want 1", len(alertArr))
		}
	})

	t.Run("empty update produces empty JSON object", func(t *testing.T) {
		input := UpdateEventInput{}
		data, err := marshalUpdateInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != "{}" {
			t.Errorf("got %s, want {}", string(data))
		}
	})

	t.Run("empty string clears field", func(t *testing.T) {
		empty := ""
		input := UpdateEventInput{
			Location: &empty,
			Notes:    &empty,
		}

		data, err := marshalUpdateInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result map[string]any
		json.Unmarshal(data, &result)

		if result["location"] != "" {
			t.Errorf("location = %v, want empty string", result["location"])
		}
		if result["notes"] != "" {
			t.Errorf("notes = %v, want empty string", result["notes"])
		}
	})

	t.Run("empty alerts array clears alerts", func(t *testing.T) {
		emptyAlerts := []Alert{}
		input := UpdateEventInput{
			Alerts: &emptyAlerts,
		}

		data, err := marshalUpdateInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result map[string]any
		json.Unmarshal(data, &result)

		alertArr, ok := result["alerts"].([]any)
		if !ok {
			t.Fatal("alerts should be present")
		}
		if len(alertArr) != 0 {
			t.Errorf("alerts count = %d, want 0", len(alertArr))
		}
	})
}

// --- Options tests ---

func TestApplyOptions(t *testing.T) {
	t.Run("no options", func(t *testing.T) {
		o := applyOptions(nil)
		if o.calendarName != "" || o.calendarID != "" || o.searchQuery != "" {
			t.Error("expected all options to be empty")
		}
	})

	t.Run("WithCalendar", func(t *testing.T) {
		o := applyOptions([]ListOption{WithCalendar("Work")})
		if o.calendarName != "Work" {
			t.Errorf("calendarName = %q, want %q", o.calendarName, "Work")
		}
	})

	t.Run("WithCalendarID", func(t *testing.T) {
		o := applyOptions([]ListOption{WithCalendarID("cal-123")})
		if o.calendarID != "cal-123" {
			t.Errorf("calendarID = %q, want %q", o.calendarID, "cal-123")
		}
	})

	t.Run("WithSearch", func(t *testing.T) {
		o := applyOptions([]ListOption{WithSearch("standup")})
		if o.searchQuery != "standup" {
			t.Errorf("searchQuery = %q, want %q", o.searchQuery, "standup")
		}
	})

	t.Run("multiple options", func(t *testing.T) {
		o := applyOptions([]ListOption{
			WithCalendar("Work"),
			WithSearch("meeting"),
		})
		if o.calendarName != "Work" {
			t.Errorf("calendarName = %q, want %q", o.calendarName, "Work")
		}
		if o.searchQuery != "meeting" {
			t.Errorf("searchQuery = %q, want %q", o.searchQuery, "meeting")
		}
	})

	t.Run("last option wins", func(t *testing.T) {
		o := applyOptions([]ListOption{
			WithCalendar("Work"),
			WithCalendar("Home"),
		})
		if o.calendarName != "Home" {
			t.Errorf("calendarName = %q, want %q (last option should win)", o.calendarName, "Home")
		}
	})
}

// --- Sentinel error tests ---

func TestSentinelErrors(t *testing.T) {
	if ErrUnsupported.Error() != "calendar: only supported on macOS (darwin)" {
		t.Errorf("ErrUnsupported = %q", ErrUnsupported.Error())
	}
	if ErrAccessDenied.Error() != "calendar: access denied" {
		t.Errorf("ErrAccessDenied = %q", ErrAccessDenied.Error())
	}
	if ErrNotFound.Error() != "calendar: not found" {
		t.Errorf("ErrNotFound = %q", ErrNotFound.Error())
	}
}

// --- ConvertRawEvent edge cases ---

func TestConvertRawEvent(t *testing.T) {
	t.Run("nil dates produce zero time", func(t *testing.T) {
		raw := rawEvent{
			ID:    "test",
			Title: "Test",
		}
		e := convertRawEvent(raw)
		if !e.StartDate.IsZero() {
			t.Error("StartDate should be zero when nil")
		}
		if !e.EndDate.IsZero() {
			t.Error("EndDate should be zero when nil")
		}
		if !e.CreatedAt.IsZero() {
			t.Error("CreatedAt should be zero when nil")
		}
		if !e.ModifiedAt.IsZero() {
			t.Error("ModifiedAt should be zero when nil")
		}
	})

	t.Run("nil optional strings produce empty strings", func(t *testing.T) {
		raw := rawEvent{ID: "test"}
		e := convertRawEvent(raw)
		if e.Location != "" {
			t.Error("Location should be empty when nil")
		}
		if e.Notes != "" {
			t.Error("Notes should be empty when nil")
		}
		if e.URL != "" {
			t.Error("URL should be empty when nil")
		}
		if e.Organizer != "" {
			t.Error("Organizer should be empty when nil")
		}
		if e.TimeZone != "" {
			t.Error("TimeZone should be empty when nil")
		}
	})

	t.Run("nil attendees and alerts produce empty slices", func(t *testing.T) {
		raw := rawEvent{ID: "test"}
		e := convertRawEvent(raw)
		if e.Attendees == nil {
			t.Error("Attendees should not be nil")
		}
		if len(e.Attendees) != 0 {
			t.Error("Attendees should be empty")
		}
		if e.Alerts == nil {
			t.Error("Alerts should not be nil")
		}
		if len(e.Alerts) != 0 {
			t.Error("Alerts should be empty")
		}
	})

	t.Run("alert offset conversion", func(t *testing.T) {
		raw := rawEvent{
			ID: "test",
			Alerts: []rawAlert{
				{RelativeOffset: -3600}, // 1 hour before
				{RelativeOffset: 0},     // at event time
				{RelativeOffset: 600},   // 10 min after (unusual but valid)
			},
		}
		e := convertRawEvent(raw)
		if e.Alerts[0].RelativeOffset != -1*time.Hour {
			t.Errorf("alert[0] = %v, want -1h", e.Alerts[0].RelativeOffset)
		}
		if e.Alerts[1].RelativeOffset != 0 {
			t.Errorf("alert[1] = %v, want 0", e.Alerts[1].RelativeOffset)
		}
		if e.Alerts[2].RelativeOffset != 10*time.Minute {
			t.Errorf("alert[2] = %v, want 10m", e.Alerts[2].RelativeOffset)
		}
	})

	t.Run("status and availability mapping", func(t *testing.T) {
		raw := rawEvent{
			ID:           "test",
			Status:       3,  // Canceled
			Availability: 2,  // Tentative
		}
		e := convertRawEvent(raw)
		if e.Status != StatusCanceled {
			t.Errorf("Status = %d, want %d (canceled)", e.Status, StatusCanceled)
		}
		if e.Availability != AvailabilityTentative {
			t.Errorf("Availability = %d, want %d (tentative)", e.Availability, AvailabilityTentative)
		}
	})
}

// --- Timezone handling tests ---

func TestTimezoneHandling(t *testing.T) {
	t.Run("UTC dates are parsed correctly", func(t *testing.T) {
		dt := parseISO8601("2026-06-15T12:00:00.000Z")
		if dt.Location() != time.UTC {
			t.Errorf("expected UTC, got %v", dt.Location())
		}
	})

	t.Run("events from different timezones parse to correct UTC", func(t *testing.T) {
		// These all represent the same moment: 12:00 UTC
		cases := []string{
			"2026-06-15T12:00:00.000Z",
			"2026-06-15T12:00:00Z",
		}
		expected := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
		for _, c := range cases {
			dt := parseISO8601(c)
			if !dt.Equal(expected) {
				t.Errorf("parseISO8601(%q) = %v, want %v", c, dt, expected)
			}
		}
	})

	t.Run("create input converts local time to UTC", func(t *testing.T) {
		est := time.FixedZone("EST", -5*3600)
		start := time.Date(2026, 2, 12, 10, 0, 0, 0, est) // 15:00 UTC

		input := CreateEventInput{
			Title:     "EST Event",
			StartDate: start,
			EndDate:   start.Add(time.Hour),
		}

		data, _ := marshalCreateInput(input)
		var result map[string]any
		json.Unmarshal(data, &result)

		if result["startDate"] != "2026-02-12T15:00:00.000Z" {
			t.Errorf("startDate = %v, want 2026-02-12T15:00:00.000Z", result["startDate"])
		}
		if result["endDate"] != "2026-02-12T16:00:00.000Z" {
			t.Errorf("endDate = %v, want 2026-02-12T16:00:00.000Z", result["endDate"])
		}
	})

	t.Run("update input converts local time to UTC", func(t *testing.T) {
		jst := time.FixedZone("JST", 9*3600)
		start := time.Date(2026, 2, 12, 19, 0, 0, 0, jst) // 10:00 UTC

		input := UpdateEventInput{
			StartDate: &start,
		}

		data, _ := marshalUpdateInput(input)
		var result map[string]any
		json.Unmarshal(data, &result)

		if result["startDate"] != "2026-02-12T10:00:00.000Z" {
			t.Errorf("startDate = %v, want 2026-02-12T10:00:00.000Z", result["startDate"])
		}
	})
}

// --- Multiple calendars in JSON ---

func TestMultipleCalendarsInEvents(t *testing.T) {
	jsonStr := `[
		{"id": "E1", "title": "Work Event", "calendar": "Work", "calendarID": "work-123", "startDate": "2026-02-11T10:00:00.000Z", "endDate": "2026-02-11T11:00:00.000Z", "allDay": false, "status": 0, "availability": 0, "recurring": false, "attendees": [], "alerts": []},
		{"id": "E2", "title": "Home Event", "calendar": "Home", "calendarID": "home-456", "startDate": "2026-02-11T18:00:00.000Z", "endDate": "2026-02-11T19:00:00.000Z", "allDay": false, "status": 0, "availability": 0, "recurring": false, "attendees": [], "alerts": []},
		{"id": "E3", "title": "Family Event", "calendar": "Family", "calendarID": "fam-789", "startDate": "2026-02-11T20:00:00.000Z", "endDate": "2026-02-11T21:00:00.000Z", "allDay": false, "status": 0, "availability": 0, "recurring": false, "attendees": [], "alerts": []}
	]`

	events, err := parseEventsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calendarNames := make(map[string]bool)
	for _, e := range events {
		calendarNames[e.Calendar] = true
	}

	if !calendarNames["Work"] {
		t.Error("missing Work calendar event")
	}
	if !calendarNames["Home"] {
		t.Error("missing Home calendar event")
	}
	if !calendarNames["Family"] {
		t.Error("missing Family calendar event")
	}
}

// --- Alert Duration tests ---

func TestAlertDuration(t *testing.T) {
	tests := []struct {
		name     string
		offset   float64
		expected time.Duration
	}{
		{"15 minutes before", -900, -15 * time.Minute},
		{"1 hour before", -3600, -1 * time.Hour},
		{"1 day before", -86400, -24 * time.Hour},
		{"at event time", 0, 0},
		{"5 minutes before", -300, -5 * time.Minute},
		{"30 minutes before", -1800, -30 * time.Minute},
		{"2 hours before", -7200, -2 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := rawAlert{RelativeOffset: tt.offset}
			result := time.Duration(raw.RelativeOffset) * time.Second
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

// --- JSON roundtrip test ---

func TestJSONRoundtrip(t *testing.T) {
	// Create input -> marshal -> unmarshal -> verify
	start := time.Date(2026, 3, 15, 14, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 15, 15, 30, 0, 0, time.UTC)

	input := CreateEventInput{
		Title:     "Roundtrip Test",
		StartDate: start,
		EndDate:   end,
		AllDay:    false,
		Location:  "Test Location",
		Notes:     "Test Notes with special chars: <>&\"'",
		URL:       "https://example.com/test?foo=bar&baz=qux",
		Calendar:  "Work",
		TimeZone:  "Europe/Berlin",
		Alerts: []Alert{
			{RelativeOffset: -15 * time.Minute},
		},
	}

	data, err := marshalCreateInput(input)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// Verify it's valid JSON
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("produced invalid JSON: %v", err)
	}

	// Verify special characters survived
	if m["notes"] != "Test Notes with special chars: <>&\"'" {
		t.Errorf("notes = %v", m["notes"])
	}
	if m["url"] != "https://example.com/test?foo=bar&baz=qux" {
		t.Errorf("url = %v", m["url"])
	}
}

// --- Enum constant values match EventKit ---

func TestEnumValues(t *testing.T) {
	// These values must match EKEventStatus constants in EventKit.
	if StatusNone != 0 {
		t.Errorf("StatusNone = %d, want 0", StatusNone)
	}
	if StatusConfirmed != 1 {
		t.Errorf("StatusConfirmed = %d, want 1", StatusConfirmed)
	}
	if StatusTentative != 2 {
		t.Errorf("StatusTentative = %d, want 2", StatusTentative)
	}
	if StatusCanceled != 3 {
		t.Errorf("StatusCanceled = %d, want 3", StatusCanceled)
	}

	// EKEventAvailability
	if AvailabilityNotSupported != -1 {
		t.Errorf("AvailabilityNotSupported = %d, want -1", AvailabilityNotSupported)
	}
	if AvailabilityBusy != 0 {
		t.Errorf("AvailabilityBusy = %d, want 0", AvailabilityBusy)
	}
	if AvailabilityFree != 1 {
		t.Errorf("AvailabilityFree = %d, want 1", AvailabilityFree)
	}
	if AvailabilityTentative != 2 {
		t.Errorf("AvailabilityTentative = %d, want 2", AvailabilityTentative)
	}
	if AvailabilityUnavailable != 3 {
		t.Errorf("AvailabilityUnavailable = %d, want 3", AvailabilityUnavailable)
	}

	// EKCalendarType
	if CalendarTypeLocal != 0 {
		t.Errorf("CalendarTypeLocal = %d, want 0", CalendarTypeLocal)
	}
	if CalendarTypeCalDAV != 1 {
		t.Errorf("CalendarTypeCalDAV = %d, want 1", CalendarTypeCalDAV)
	}
	if CalendarTypeExchange != 2 {
		t.Errorf("CalendarTypeExchange = %d, want 2", CalendarTypeExchange)
	}
	if CalendarTypeBirthday != 4 {
		t.Errorf("CalendarTypeBirthday = %d, want 4", CalendarTypeBirthday)
	}
	if CalendarTypeSubscription != 5 {
		t.Errorf("CalendarTypeSubscription = %d, want 5", CalendarTypeSubscription)
	}

	// EKParticipantStatus
	if ParticipantStatusUnknown != 0 {
		t.Errorf("ParticipantStatusUnknown = %d, want 0", ParticipantStatusUnknown)
	}
	if ParticipantStatusPending != 1 {
		t.Errorf("ParticipantStatusPending = %d, want 1", ParticipantStatusPending)
	}
	if ParticipantStatusAccepted != 2 {
		t.Errorf("ParticipantStatusAccepted = %d, want 2", ParticipantStatusAccepted)
	}
	if ParticipantStatusDeclined != 3 {
		t.Errorf("ParticipantStatusDeclined = %d, want 3", ParticipantStatusDeclined)
	}
	if ParticipantStatusTentative != 4 {
		t.Errorf("ParticipantStatusTentative = %d, want 4", ParticipantStatusTentative)
	}

	// EKSpan
	if SpanThisEvent != 0 {
		t.Errorf("SpanThisEvent = %d, want 0", SpanThisEvent)
	}
	if SpanFutureEvents != 1 {
		t.Errorf("SpanFutureEvents = %d, want 1", SpanFutureEvents)
	}
}

// --- Large event set parsing ---

func TestLargeEventSet(t *testing.T) {
	// Generate 100 events to test parsing performance and correctness.
	events := make([]rawEvent, 100)
	for i := range events {
		start := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC).Add(time.Duration(i) * 24 * time.Hour)
		startStr := start.Format("2006-01-02T15:04:05.000Z")
		endStr := start.Add(time.Hour).Format("2006-01-02T15:04:05.000Z")
		events[i] = rawEvent{
			ID:           string(rune('A'+i%26)) + "-event",
			Title:        "Event " + string(rune('A'+i%26)),
			StartDate:    &startStr,
			EndDate:      &endStr,
			Calendar:     []string{"Work", "Home", "Family"}[i%3],
			CalendarID:   []string{"c1", "c2", "c3"}[i%3],
			Attendees:    []rawAttendee{},
			Alerts:       []rawAlert{},
		}
	}

	data, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	parsed, err := parseEventsJSON(string(data))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(parsed) != 100 {
		t.Errorf("parsed %d events, want 100", len(parsed))
	}
}
