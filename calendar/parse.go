package calendar

import (
	"encoding/json"
	"fmt"
	"time"
)

// rawEvent is the intermediate JSON representation from the ObjC bridge.
type rawEvent struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	StartDate    *string       `json:"startDate"`
	EndDate      *string       `json:"endDate"`
	AllDay       bool          `json:"allDay"`
	Location     *string       `json:"location"`
	Notes        *string       `json:"notes"`
	URL          *string       `json:"url"`
	Calendar     string        `json:"calendar"`
	CalendarID   string        `json:"calendarID"`
	Status       int           `json:"status"`
	Availability int           `json:"availability"`
	Organizer    *string       `json:"organizer"`
	Attendees    []rawAttendee `json:"attendees"`
	Recurring    bool          `json:"recurring"`
	Alerts       []rawAlert    `json:"alerts"`
	CreatedAt    *string       `json:"createdAt"`
	ModifiedAt   *string       `json:"modifiedAt"`
	TimeZone     *string       `json:"timeZone"`
}

type rawAttendee struct {
	Name   string `json:"name"`
	Email  string `json:"email"`
	Status int    `json:"status"`
}

type rawAlert struct {
	RelativeOffset float64 `json:"relativeOffset"`
}

type rawCalendar struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Type     int    `json:"type"`
	Color    string `json:"color"`
	Source   string `json:"source"`
	ReadOnly bool   `json:"readOnly"`
}

// parseISO8601 parses an ISO 8601 date string from the bridge.
func parseISO8601(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// Try with fractional seconds first.
	t, err := time.Parse("2006-01-02T15:04:05.000Z", s)
	if err == nil {
		return t
	}
	// Try without fractional seconds.
	t, err = time.Parse("2006-01-02T15:04:05Z", s)
	if err == nil {
		return t
	}
	// Try standard RFC3339.
	t, err = time.Parse(time.RFC3339, s)
	if err == nil {
		return t
	}
	return time.Time{}
}

func convertRawEvent(r rawEvent) Event {
	e := Event{
		ID:           r.ID,
		Title:        r.Title,
		AllDay:       r.AllDay,
		Calendar:     r.Calendar,
		CalendarID:   r.CalendarID,
		Status:       EventStatus(r.Status),
		Availability: Availability(r.Availability),
		Recurring:    r.Recurring,
	}

	if r.StartDate != nil {
		e.StartDate = parseISO8601(*r.StartDate)
	}
	if r.EndDate != nil {
		e.EndDate = parseISO8601(*r.EndDate)
	}
	if r.Location != nil {
		e.Location = *r.Location
	}
	if r.Notes != nil {
		e.Notes = *r.Notes
	}
	if r.URL != nil {
		e.URL = *r.URL
	}
	if r.Organizer != nil {
		e.Organizer = *r.Organizer
	}
	if r.CreatedAt != nil {
		e.CreatedAt = parseISO8601(*r.CreatedAt)
	}
	if r.ModifiedAt != nil {
		e.ModifiedAt = parseISO8601(*r.ModifiedAt)
	}
	if r.TimeZone != nil {
		e.TimeZone = *r.TimeZone
	}

	// Convert attendees.
	e.Attendees = make([]Attendee, len(r.Attendees))
	for i, a := range r.Attendees {
		e.Attendees[i] = Attendee{
			Name:   a.Name,
			Email:  a.Email,
			Status: ParticipantStatus(a.Status),
		}
	}

	// Convert alerts.
	e.Alerts = make([]Alert, len(r.Alerts))
	for i, a := range r.Alerts {
		e.Alerts[i] = Alert{
			RelativeOffset: time.Duration(a.RelativeOffset) * time.Second,
		}
	}

	return e
}

func parseEventsJSON(jsonStr string) ([]Event, error) {
	var raw []rawEvent
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("calendar: failed to parse events JSON: %w", err)
	}

	events := make([]Event, len(raw))
	for i, r := range raw {
		events[i] = convertRawEvent(r)
	}
	return events, nil
}

func parseEventJSON(jsonStr string) (*Event, error) {
	var raw rawEvent
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("calendar: failed to parse event JSON: %w", err)
	}

	e := convertRawEvent(raw)
	return &e, nil
}

func parseCalendarsJSON(jsonStr string) ([]Calendar, error) {
	var raw []rawCalendar
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("calendar: failed to parse calendars JSON: %w", err)
	}

	calendars := make([]Calendar, len(raw))
	for i, r := range raw {
		calendars[i] = Calendar{
			ID:       r.ID,
			Title:    r.Title,
			Type:     CalendarType(r.Type),
			Color:    r.Color,
			Source:   r.Source,
			ReadOnly: r.ReadOnly,
		}
	}
	return calendars, nil
}

// --- JSON marshaling for writes ---

type createEventJSON struct {
	Title     string      `json:"title"`
	StartDate string      `json:"startDate"`
	EndDate   string      `json:"endDate"`
	AllDay    bool        `json:"allDay"`
	Location  string      `json:"location,omitempty"`
	Notes     string      `json:"notes,omitempty"`
	URL       string      `json:"url,omitempty"`
	Calendar  string      `json:"calendar,omitempty"`
	Alerts    []alertJSON `json:"alerts,omitempty"`
	TimeZone  string      `json:"timeZone,omitempty"`
}

type alertJSON struct {
	RelativeOffset float64 `json:"relativeOffset"`
}

func marshalCreateInput(input CreateEventInput) ([]byte, error) {
	j := createEventJSON{
		Title:     input.Title,
		StartDate: input.StartDate.UTC().Format("2006-01-02T15:04:05.000Z"),
		EndDate:   input.EndDate.UTC().Format("2006-01-02T15:04:05.000Z"),
		AllDay:    input.AllDay,
		Location:  input.Location,
		Notes:     input.Notes,
		URL:       input.URL,
		Calendar:  input.Calendar,
		TimeZone:  input.TimeZone,
	}

	if len(input.Alerts) > 0 {
		j.Alerts = make([]alertJSON, len(input.Alerts))
		for i, a := range input.Alerts {
			j.Alerts[i] = alertJSON{RelativeOffset: a.RelativeOffset.Seconds()}
		}
	}

	return json.Marshal(j)
}

func marshalUpdateInput(input UpdateEventInput) ([]byte, error) {
	m := make(map[string]any)

	if input.Title != nil {
		m["title"] = *input.Title
	}
	if input.StartDate != nil {
		m["startDate"] = input.StartDate.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	if input.EndDate != nil {
		m["endDate"] = input.EndDate.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	if input.AllDay != nil {
		m["allDay"] = *input.AllDay
	}
	if input.Location != nil {
		m["location"] = *input.Location
	}
	if input.Notes != nil {
		m["notes"] = *input.Notes
	}
	if input.URL != nil {
		m["url"] = *input.URL
	}
	if input.Calendar != nil {
		m["calendar"] = *input.Calendar
	}
	if input.TimeZone != nil {
		m["timeZone"] = *input.TimeZone
	}
	if input.Alerts != nil {
		alerts := make([]alertJSON, len(*input.Alerts))
		for i, a := range *input.Alerts {
			alerts[i] = alertJSON{RelativeOffset: a.RelativeOffset.Seconds()}
		}
		m["alerts"] = alerts
	}

	return json.Marshal(m)
}
