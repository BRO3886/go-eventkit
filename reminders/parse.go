package reminders

import (
	"encoding/json"
	"fmt"
	"time"
)

// rawReminder is the intermediate JSON representation from the ObjC bridge.
type rawReminder struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Notes          *string    `json:"notes"`
	List           string     `json:"list"`
	ListID         string     `json:"listID"`
	DueDate        *string    `json:"dueDate"`
	RemindMeDate   *string    `json:"remindMeDate"`
	CompletionDate *string    `json:"completionDate"`
	CreatedAt      *string    `json:"createdAt"`
	ModifiedAt     *string    `json:"modifiedAt"`
	Priority       int        `json:"priority"`
	Completed      bool       `json:"completed"`
	Flagged        bool       `json:"flagged"`
	URL            *string    `json:"url"`
	HasAlarms      bool       `json:"hasAlarms"`
	Alarms         []rawAlarm `json:"alarms"`
}

type rawAlarm struct {
	AbsoluteDate   *string `json:"absoluteDate"`
	RelativeOffset float64 `json:"relativeOffset"`
}

// rawList is the intermediate JSON representation of a reminder list.
type rawList struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Color    string `json:"color"`
	Source   string `json:"source"`
	Count    int    `json:"count"`
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
	// Try RFC3339 with timezone offset.
	t, err = time.Parse(time.RFC3339, s)
	if err == nil {
		return t
	}
	return time.Time{}
}

func parseOptionalTime(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t := parseISO8601(*s)
	if t.IsZero() {
		return nil
	}
	return &t
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func convertRawReminder(r *rawReminder) Reminder {
	rem := Reminder{
		ID:             r.ID,
		Title:          r.Title,
		Notes:          derefString(r.Notes),
		List:           r.List,
		ListID:         r.ListID,
		DueDate:        parseOptionalTime(r.DueDate),
		RemindMeDate:   parseOptionalTime(r.RemindMeDate),
		CompletionDate: parseOptionalTime(r.CompletionDate),
		CreatedAt:      parseOptionalTime(r.CreatedAt),
		ModifiedAt:     parseOptionalTime(r.ModifiedAt),
		Priority:       Priority(r.Priority),
		Completed:      r.Completed,
		Flagged:        r.Flagged,
		URL:            derefString(r.URL),
		HasAlarms:      r.HasAlarms,
	}

	// Convert alarms.
	if len(r.Alarms) > 0 {
		rem.Alarms = make([]Alarm, len(r.Alarms))
		for i, a := range r.Alarms {
			rem.Alarms[i] = Alarm{
				AbsoluteDate:   parseOptionalTime(a.AbsoluteDate),
				RelativeOffset: time.Duration(a.RelativeOffset) * time.Second,
			}
		}
	} else {
		rem.Alarms = []Alarm{}
	}

	return rem
}

// parseRemindersJSON parses a JSON array of reminders.
func parseRemindersJSON(jsonStr string) ([]Reminder, error) {
	var raw []rawReminder
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("reminders: failed to parse reminders JSON: %w", err)
	}
	result := make([]Reminder, len(raw))
	for i := range raw {
		result[i] = convertRawReminder(&raw[i])
	}
	return result, nil
}

// parseReminderJSON parses a single reminder JSON object.
func parseReminderJSON(jsonStr string) (*Reminder, error) {
	var raw rawReminder
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("reminders: failed to parse reminder JSON: %w", err)
	}
	r := convertRawReminder(&raw)
	return &r, nil
}

// parseListsJSON parses a JSON array of reminder lists.
func parseListsJSON(jsonStr string) ([]List, error) {
	var raw []rawList
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("reminders: failed to parse lists JSON: %w", err)
	}
	result := make([]List, len(raw))
	for i, r := range raw {
		result[i] = List{
			ID:       r.ID,
			Title:    r.Title,
			Color:    r.Color,
			Source:   r.Source,
			Count:    r.Count,
			ReadOnly: r.ReadOnly,
		}
	}
	return result, nil
}

// marshalCreateInput converts CreateReminderInput to JSON for the bridge.
func marshalCreateInput(input CreateReminderInput) (string, error) {
	m := map[string]any{
		"title": input.Title,
	}

	if input.Notes != "" {
		m["notes"] = input.Notes
	}
	if input.ListName != "" {
		m["listName"] = input.ListName
	}
	if input.URL != "" {
		m["url"] = input.URL
	}
	if input.Priority != PriorityNone {
		m["priority"] = int(input.Priority)
	}

	if input.DueDate != nil {
		m["dueDate"] = input.DueDate.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	if input.RemindMeDate != nil {
		m["remindMeDate"] = input.RemindMeDate.UTC().Format("2006-01-02T15:04:05.000Z")
	}

	if len(input.Alarms) > 0 {
		alarms := make([]map[string]any, len(input.Alarms))
		for i, a := range input.Alarms {
			alarm := map[string]any{}
			if a.AbsoluteDate != nil {
				alarm["absoluteDate"] = a.AbsoluteDate.UTC().Format("2006-01-02T15:04:05.000Z")
			}
			if a.RelativeOffset != 0 {
				alarm["relativeOffset"] = a.RelativeOffset.Seconds()
			}
			alarms[i] = alarm
		}
		m["alarms"] = alarms
	}

	data, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("reminders: failed to marshal create input: %w", err)
	}
	return string(data), nil
}

// marshalUpdateInput converts UpdateReminderInput to JSON for the bridge.
// Only non-nil fields are included.
func marshalUpdateInput(input UpdateReminderInput) (string, error) {
	m := map[string]any{}

	if input.Title != nil {
		m["title"] = *input.Title
	}
	if input.Notes != nil {
		m["notes"] = *input.Notes
	}
	if input.ListName != nil {
		m["listName"] = *input.ListName
	}
	if input.URL != nil {
		m["url"] = *input.URL
	}
	if input.Priority != nil {
		m["priority"] = int(*input.Priority)
	}
	if input.Completed != nil {
		m["completed"] = *input.Completed
	}
	if input.Flagged != nil {
		m["flagged"] = *input.Flagged
	}

	if input.ClearDueDate {
		m["dueDate"] = nil
	} else if input.DueDate != nil {
		m["dueDate"] = input.DueDate.UTC().Format("2006-01-02T15:04:05.000Z")
	}

	if input.RemindMeDate != nil {
		m["remindMeDate"] = input.RemindMeDate.UTC().Format("2006-01-02T15:04:05.000Z")
	}

	if input.Alarms != nil {
		alarms := make([]map[string]any, len(*input.Alarms))
		for i, a := range *input.Alarms {
			alarm := map[string]any{}
			if a.AbsoluteDate != nil {
				alarm["absoluteDate"] = a.AbsoluteDate.UTC().Format("2006-01-02T15:04:05.000Z")
			}
			if a.RelativeOffset != 0 {
				alarm["relativeOffset"] = a.RelativeOffset.Seconds()
			}
			alarms[i] = alarm
		}
		m["alarms"] = alarms
	}

	data, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("reminders: failed to marshal update input: %w", err)
	}
	return string(data), nil
}
