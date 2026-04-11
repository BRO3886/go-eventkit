package reminders

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/BRO3886/go-eventkit"
)

// rawReminder is the intermediate JSON representation from the ObjC bridge.
type rawReminder struct {
	ID              string              `json:"id"`
	Title           string              `json:"title"`
	Notes           *string             `json:"notes"`
	List            string              `json:"list"`
	ListID          string              `json:"listID"`
	DueDate         *string             `json:"dueDate"`
	RemindMeDate    *string             `json:"remindMeDate"`
	CompletionDate  *string             `json:"completionDate"`
	CreatedAt       *string             `json:"createdAt"`
	ModifiedAt      *string             `json:"modifiedAt"`
	Priority        int                 `json:"priority"`
	Completed       bool                `json:"completed"`
	Flagged         bool                `json:"flagged"`
	URL             *string             `json:"url"`
	Recurring       bool                `json:"recurring"`
	RecurrenceRules []rawRecurrenceRule `json:"recurrenceRules"`
	HasAlarms       bool                `json:"hasAlarms"`
	Alarms          []rawAlarm          `json:"alarms"`
}

type rawAlarm struct {
	AbsoluteDate   *string `json:"absoluteDate"`
	RelativeOffset float64 `json:"relativeOffset"`
}

type rawRecurrenceRule struct {
	Frequency       int                       `json:"frequency"`
	Interval        int                       `json:"interval"`
	DaysOfTheWeek   []rawRecurrenceDayOfWeek  `json:"daysOfTheWeek,omitempty"`
	DaysOfTheMonth  []int                     `json:"daysOfTheMonth,omitempty"`
	MonthsOfTheYear []int                     `json:"monthsOfTheYear,omitempty"`
	WeeksOfTheYear  []int                     `json:"weeksOfTheYear,omitempty"`
	DaysOfTheYear   []int                     `json:"daysOfTheYear,omitempty"`
	SetPositions    []int                     `json:"setPositions,omitempty"`
	End             *rawRecurrenceEnd         `json:"end,omitempty"`
}

type rawRecurrenceDayOfWeek struct {
	DayOfTheWeek int `json:"dayOfTheWeek"`
	WeekNumber   int `json:"weekNumber"`
}

type rawRecurrenceEnd struct {
	EndDate         *string `json:"endDate,omitempty"`
	OccurrenceCount int     `json:"occurrenceCount,omitempty"`
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
		Recurring:      r.Recurring,
		HasAlarms:      r.HasAlarms,
	}

	// Convert recurrence rules.
	rem.RecurrenceRules = make([]eventkit.RecurrenceRule, len(r.RecurrenceRules))
	for i, rr := range r.RecurrenceRules {
		rule := eventkit.RecurrenceRule{
			Frequency:       eventkit.RecurrenceFrequency(rr.Frequency),
			Interval:        rr.Interval,
			DaysOfTheMonth:  rr.DaysOfTheMonth,
			MonthsOfTheYear: rr.MonthsOfTheYear,
			WeeksOfTheYear:  rr.WeeksOfTheYear,
			DaysOfTheYear:   rr.DaysOfTheYear,
			SetPositions:    rr.SetPositions,
		}
		for _, dow := range rr.DaysOfTheWeek {
			rule.DaysOfTheWeek = append(rule.DaysOfTheWeek, eventkit.RecurrenceDayOfWeek{
				DayOfTheWeek: eventkit.Weekday(dow.DayOfTheWeek),
				WeekNumber:   dow.WeekNumber,
			})
		}
		if rr.End != nil {
			end := &eventkit.RecurrenceEnd{
				OccurrenceCount: rr.End.OccurrenceCount,
			}
			if rr.End.EndDate != nil {
				t := parseISO8601(*rr.End.EndDate)
				if !t.IsZero() {
					end.EndDate = &t
				}
			}
			rule.End = end
		}
		rem.RecurrenceRules[i] = rule
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

// --- JSON marshaling for list writes ---

type createListJSON struct {
	Title  string `json:"title"`
	Source string `json:"source,omitempty"`
	Color  string `json:"color,omitempty"`
}

func marshalCreateListInput(input CreateListInput) (string, error) {
	j := createListJSON{
		Title:  input.Title,
		Source: input.Source,
		Color:  input.Color,
	}
	data, err := json.Marshal(j)
	if err != nil {
		return "", fmt.Errorf("reminders: failed to marshal create list input: %w", err)
	}
	return string(data), nil
}

func marshalUpdateListInput(input UpdateListInput) (string, error) {
	m := make(map[string]any)
	if input.Title != nil {
		m["title"] = *input.Title
	}
	if input.Color != nil {
		m["color"] = *input.Color
	}
	data, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("reminders: failed to marshal update list input: %w", err)
	}
	return string(data), nil
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
			} else {
				alarm["relativeOffset"] = a.RelativeOffset.Seconds()
			}
			alarms[i] = alarm
		}
		m["alarms"] = alarms
	}

	if len(input.RecurrenceRules) > 0 {
		m["recurrenceRules"] = marshalRecurrenceRules(input.RecurrenceRules)
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
			} else {
				alarm["relativeOffset"] = a.RelativeOffset.Seconds()
			}
			alarms[i] = alarm
		}
		m["alarms"] = alarms
	}

	if input.RecurrenceRules != nil {
		m["recurrenceRules"] = marshalRecurrenceRules(*input.RecurrenceRules)
	}

	data, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("reminders: failed to marshal update input: %w", err)
	}
	return string(data), nil
}

// marshalRecurrenceRules converts recurrence rules to JSON-serializable format.
func marshalRecurrenceRules(rules []eventkit.RecurrenceRule) []map[string]any {
	result := make([]map[string]any, len(rules))
	for i, r := range rules {
		rj := map[string]any{
			"frequency": int(r.Frequency),
			"interval":  r.Interval,
		}
		if len(r.DaysOfTheWeek) > 0 {
			days := make([]map[string]any, len(r.DaysOfTheWeek))
			for j, dow := range r.DaysOfTheWeek {
				days[j] = map[string]any{
					"dayOfTheWeek": int(dow.DayOfTheWeek),
					"weekNumber":   dow.WeekNumber,
				}
			}
			rj["daysOfTheWeek"] = days
		}
		if len(r.DaysOfTheMonth) > 0 {
			rj["daysOfTheMonth"] = r.DaysOfTheMonth
		}
		if len(r.MonthsOfTheYear) > 0 {
			rj["monthsOfTheYear"] = r.MonthsOfTheYear
		}
		if len(r.WeeksOfTheYear) > 0 {
			rj["weeksOfTheYear"] = r.WeeksOfTheYear
		}
		if len(r.DaysOfTheYear) > 0 {
			rj["daysOfTheYear"] = r.DaysOfTheYear
		}
		if len(r.SetPositions) > 0 {
			rj["setPositions"] = r.SetPositions
		}
		if r.End != nil {
			ej := map[string]any{}
			if r.End.EndDate != nil {
				ej["endDate"] = r.End.EndDate.UTC().Format("2006-01-02T15:04:05.000Z")
			}
			if r.End.OccurrenceCount > 0 {
				ej["occurrenceCount"] = r.End.OccurrenceCount
			}
			rj["end"] = ej
		}
		result[i] = rj
	}
	return result
}
