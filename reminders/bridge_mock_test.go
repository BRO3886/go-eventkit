package reminders

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/BRO3886/go-eventkit"
)

// These tests use a mock bridge layer to test the JSON round-trip,
// error handling, and type conversion that happens in the bridge layer
// without requiring real EventKit access.

// --- Mock bridge: simulates the ObjC bridge returning JSON ---

// simulateRemindersResponse simulates what the ObjC bridge returns for Reminders().
func simulateRemindersResponse(reminders []Reminder) string {
	raw := make([]rawReminder, len(reminders))
	for i, r := range reminders {
		rr := rawReminder{
			ID:        r.ID,
			Title:     r.Title,
			List:      r.List,
			ListID:    r.ListID,
			Priority:  int(r.Priority),
			Completed: r.Completed,
			Flagged:   r.Flagged,
			Recurring: r.Recurring,
			HasAlarms: r.HasAlarms,
			Alarms:    []rawAlarm{},
		}

		// Convert recurrence rules to raw format.
		for _, rule := range r.RecurrenceRules {
			rawRule := rawRecurrenceRule{
				Frequency:       int(rule.Frequency),
				Interval:        rule.Interval,
				DaysOfTheMonth:  rule.DaysOfTheMonth,
				MonthsOfTheYear: rule.MonthsOfTheYear,
				WeeksOfTheYear:  rule.WeeksOfTheYear,
				DaysOfTheYear:   rule.DaysOfTheYear,
				SetPositions:    rule.SetPositions,
			}
			for _, dow := range rule.DaysOfTheWeek {
				rawRule.DaysOfTheWeek = append(rawRule.DaysOfTheWeek, rawRecurrenceDayOfWeek{
					DayOfTheWeek: int(dow.DayOfTheWeek),
					WeekNumber:   dow.WeekNumber,
				})
			}
			if rule.End != nil {
				rawEnd := &rawRecurrenceEnd{
					OccurrenceCount: rule.End.OccurrenceCount,
				}
				if rule.End.EndDate != nil {
					s := rule.End.EndDate.UTC().Format("2006-01-02T15:04:05.000Z")
					rawEnd.EndDate = &s
				}
				rawRule.End = rawEnd
			}
			rr.RecurrenceRules = append(rr.RecurrenceRules, rawRule)
		}

		if r.Notes != "" {
			rr.Notes = &r.Notes
		}
		if r.URL != "" {
			rr.URL = &r.URL
		}
		if r.DueDate != nil {
			s := r.DueDate.UTC().Format("2006-01-02T15:04:05.000Z")
			rr.DueDate = &s
		}
		if r.RemindMeDate != nil {
			s := r.RemindMeDate.UTC().Format("2006-01-02T15:04:05.000Z")
			rr.RemindMeDate = &s
		}
		if r.CompletionDate != nil {
			s := r.CompletionDate.UTC().Format("2006-01-02T15:04:05.000Z")
			rr.CompletionDate = &s
		}
		if r.CreatedAt != nil {
			s := r.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z")
			rr.CreatedAt = &s
		}
		if r.ModifiedAt != nil {
			s := r.ModifiedAt.UTC().Format("2006-01-02T15:04:05.000Z")
			rr.ModifiedAt = &s
		}

		for _, a := range r.Alarms {
			ra := rawAlarm{RelativeOffset: a.RelativeOffset.Seconds()}
			if a.AbsoluteDate != nil {
				s := a.AbsoluteDate.UTC().Format("2006-01-02T15:04:05.000Z")
				ra.AbsoluteDate = &s
			}
			rr.Alarms = append(rr.Alarms, ra)
		}

		raw[i] = rr
	}
	data, _ := json.Marshal(raw)
	return string(data)
}

// simulateListsResponse simulates what the ObjC bridge returns for Lists().
func simulateListsResponse(lists []List) string {
	raw := make([]rawList, len(lists))
	for i, l := range lists {
		raw[i] = rawList{
			ID:       l.ID,
			Title:    l.Title,
			Color:    l.Color,
			Source:   l.Source,
			Count:    l.Count,
			ReadOnly: l.ReadOnly,
		}
	}
	data, _ := json.Marshal(raw)
	return string(data)
}

// --- Mock-based bridge response tests ---

func TestMockListsRoundtrip(t *testing.T) {
	input := []List{
		{ID: "list-1", Title: "Inbox", Color: "#FF0000", Source: "iCloud", Count: 42, ReadOnly: false},
		{ID: "list-2", Title: "Shopping", Color: "#00FF00", Source: "iCloud", Count: 7, ReadOnly: false},
		{ID: "list-3", Title: "Work", Color: "#0000FF", Source: "Exchange", Count: 15, ReadOnly: false},
		{ID: "list-4", Title: "Shared", Color: "#FFFF00", Source: "iCloud", Count: 3, ReadOnly: true},
	}

	jsonStr := simulateListsResponse(input)
	parsed, err := parseListsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed) != len(input) {
		t.Fatalf("parsed %d lists, want %d", len(parsed), len(input))
	}

	for i, l := range parsed {
		if l.ID != input[i].ID {
			t.Errorf("list[%d].ID = %q, want %q", i, l.ID, input[i].ID)
		}
		if l.Title != input[i].Title {
			t.Errorf("list[%d].Title = %q, want %q", i, l.Title, input[i].Title)
		}
		if l.Color != input[i].Color {
			t.Errorf("list[%d].Color = %q, want %q", i, l.Color, input[i].Color)
		}
		if l.Source != input[i].Source {
			t.Errorf("list[%d].Source = %q, want %q", i, l.Source, input[i].Source)
		}
		if l.Count != input[i].Count {
			t.Errorf("list[%d].Count = %d, want %d", i, l.Count, input[i].Count)
		}
		if l.ReadOnly != input[i].ReadOnly {
			t.Errorf("list[%d].ReadOnly = %v, want %v", i, l.ReadOnly, input[i].ReadOnly)
		}
	}
}

func TestMockRemindersRoundtrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	due := now.Add(48 * time.Hour)
	remind := now.Add(47 * time.Hour)
	completed := now.Add(-24 * time.Hour)

	input := []Reminder{
		{
			ID:             "rem-1",
			Title:          "Buy groceries",
			Notes:          "Milk, eggs, bread",
			List:           "Shopping",
			ListID:         "list-2",
			DueDate:        &due,
			RemindMeDate:   &remind,
			CreatedAt:      &now,
			ModifiedAt:     &now,
			Priority:       PriorityHigh,
			Completed:      false,
			Flagged:        false,
			URL:            "https://example.com/list",
			HasAlarms:      true,
			Alarms: []Alarm{
				{RelativeOffset: -30 * time.Minute},
			},
		},
		{
			ID:             "rem-2",
			Title:          "Call dentist",
			List:           "Inbox",
			ListID:         "list-1",
			CompletionDate: &completed,
			CreatedAt:      &now,
			Priority:       PriorityNone,
			Completed:      true,
			Alarms:         []Alarm{},
		},
		{
			ID:        "rem-3",
			Title:     "Read chapter 5",
			List:      "Books",
			ListID:    "list-5",
			Priority:  PriorityLow,
			Completed: false,
			Alarms:    []Alarm{},
		},
	}

	jsonStr := simulateRemindersResponse(input)
	parsed, err := parseRemindersJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed) != len(input) {
		t.Fatalf("parsed %d reminders, want %d", len(parsed), len(input))
	}

	// Verify first reminder (full details)
	r := parsed[0]
	if r.ID != "rem-1" {
		t.Errorf("r.ID = %q", r.ID)
	}
	if r.Title != "Buy groceries" {
		t.Errorf("r.Title = %q", r.Title)
	}
	if r.Notes != "Milk, eggs, bread" {
		t.Errorf("r.Notes = %q", r.Notes)
	}
	if r.List != "Shopping" {
		t.Errorf("r.List = %q", r.List)
	}
	if r.Priority != PriorityHigh {
		t.Errorf("r.Priority = %d, want %d", r.Priority, PriorityHigh)
	}
	if r.URL != "https://example.com/list" {
		t.Errorf("r.URL = %q", r.URL)
	}
	if r.DueDate == nil || !r.DueDate.Equal(due) {
		t.Errorf("r.DueDate = %v, want %v", r.DueDate, due)
	}
	if r.RemindMeDate == nil || !r.RemindMeDate.Equal(remind) {
		t.Errorf("r.RemindMeDate = %v, want %v", r.RemindMeDate, remind)
	}
	if !r.HasAlarms {
		t.Error("r.HasAlarms should be true")
	}
	if len(r.Alarms) != 1 || r.Alarms[0].RelativeOffset != -30*time.Minute {
		t.Errorf("r.Alarms = %+v", r.Alarms)
	}

	// Verify completed reminder
	r2 := parsed[1]
	if !r2.Completed {
		t.Error("r2.Completed should be true")
	}
	if r2.CompletionDate == nil || !r2.CompletionDate.Equal(completed) {
		t.Errorf("r2.CompletionDate = %v, want %v", r2.CompletionDate, completed)
	}
	if r2.Notes != "" {
		t.Errorf("r2.Notes = %q, want empty", r2.Notes)
	}

	// Verify minimal reminder
	r3 := parsed[2]
	if r3.DueDate != nil {
		t.Errorf("r3.DueDate = %v, want nil", r3.DueDate)
	}
	if r3.Priority != PriorityLow {
		t.Errorf("r3.Priority = %d, want %d", r3.Priority, PriorityLow)
	}
}

func TestMockCreateReminderJSON(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+30*60)
	due := time.Date(2026, 3, 15, 15, 30, 0, 0, ist)
	alarm := time.Date(2026, 3, 15, 15, 0, 0, 0, ist)

	input := CreateReminderInput{
		Title:    "IST Reminder",
		Notes:    "Test notes",
		ListName: "Work",
		DueDate:  &due,
		Priority: PriorityMedium,
		URL:      "https://example.com",
		Alarms: []Alarm{
			{AbsoluteDate: &alarm},
			{RelativeOffset: -15 * time.Minute},
		},
	}

	jsonStr, err := marshalCreateInput(input)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal([]byte(jsonStr), &parsed)

	// Verify UTC conversion
	if parsed["dueDate"] != "2026-03-15T10:00:00.000Z" {
		t.Errorf("dueDate = %v, want UTC", parsed["dueDate"])
	}
	if parsed["title"] != "IST Reminder" {
		t.Errorf("title = %v", parsed["title"])
	}
	if parsed["listName"] != "Work" {
		t.Errorf("listName = %v", parsed["listName"])
	}
	if parsed["priority"] != float64(5) {
		t.Errorf("priority = %v, want 5", parsed["priority"])
	}

	alarms := parsed["alarms"].([]any)
	if len(alarms) != 2 {
		t.Fatalf("alarms = %d", len(alarms))
	}

	a0 := alarms[0].(map[string]any)
	if a0["absoluteDate"] != "2026-03-15T09:30:00.000Z" {
		t.Errorf("alarm[0].absoluteDate = %v", a0["absoluteDate"])
	}

	a1 := alarms[1].(map[string]any)
	if a1["relativeOffset"] != -900.0 {
		t.Errorf("alarm[1].relativeOffset = %v, want -900", a1["relativeOffset"])
	}
}

func TestMockUpdateReminderJSON(t *testing.T) {
	t.Run("partial update preserves existing", func(t *testing.T) {
		// Only update title and priority
		newTitle := "Updated Title"
		newPriority := PriorityHigh
		data, err := marshalUpdateInput(UpdateReminderInput{
			Title:    &newTitle,
			Priority: &newPriority,
		})
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var m map[string]any
		json.Unmarshal([]byte(data), &m)

		if m["title"] != "Updated Title" {
			t.Errorf("title = %v", m["title"])
		}
		if m["priority"] != float64(1) {
			t.Errorf("priority = %v, want 1", m["priority"])
		}
		// These should not be in the payload
		if _, ok := m["notes"]; ok {
			t.Error("notes should NOT be in update")
		}
		if _, ok := m["listName"]; ok {
			t.Error("listName should NOT be in update")
		}
	})

	t.Run("move to different list", func(t *testing.T) {
		newList := "Shopping"
		data, _ := marshalUpdateInput(UpdateReminderInput{ListName: &newList})

		var m map[string]any
		json.Unmarshal([]byte(data), &m)

		if m["listName"] != "Shopping" {
			t.Errorf("listName = %v", m["listName"])
		}
	})

	t.Run("clear due date", func(t *testing.T) {
		data, _ := marshalUpdateInput(UpdateReminderInput{ClearDueDate: true})

		var m map[string]any
		json.Unmarshal([]byte(data), &m)

		if _, ok := m["dueDate"]; !ok {
			t.Error("dueDate should be present (set to null)")
		}
		if m["dueDate"] != nil {
			t.Errorf("dueDate = %v, want nil", m["dueDate"])
		}
	})

	t.Run("replace all alarms", func(t *testing.T) {
		alarm := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
		alarms := []Alarm{{AbsoluteDate: &alarm}}
		data, _ := marshalUpdateInput(UpdateReminderInput{Alarms: &alarms})

		var m map[string]any
		json.Unmarshal([]byte(data), &m)

		a := m["alarms"].([]any)
		if len(a) != 1 {
			t.Fatalf("alarms = %d, want 1", len(a))
		}
	})

	t.Run("remove all alarms", func(t *testing.T) {
		empty := []Alarm{}
		data, _ := marshalUpdateInput(UpdateReminderInput{Alarms: &empty})

		var m map[string]any
		json.Unmarshal([]byte(data), &m)

		a := m["alarms"].([]any)
		if len(a) != 0 {
			t.Errorf("alarms = %d, want 0", len(a))
		}
	})

	t.Run("zero relative offset alarm", func(t *testing.T) {
		alarms := []Alarm{{RelativeOffset: 0}}
		data, _ := marshalUpdateInput(UpdateReminderInput{Alarms: &alarms})

		var m map[string]any
		json.Unmarshal([]byte(data), &m)

		a := m["alarms"].([]any)
		if len(a) != 1 {
			t.Fatalf("alarms = %d, want 1", len(a))
		}
		alarm := a[0].(map[string]any)
		offset, ok := alarm["relativeOffset"]
		if !ok {
			t.Fatal("relativeOffset key missing — zero offset must be serialized")
		}
		if offset != 0.0 {
			t.Errorf("relativeOffset = %v, want 0", offset)
		}
	})
}

func TestMockAlarmRoundtrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	due := now.Add(24 * time.Hour)
	alarm1 := now.Add(23 * time.Hour)

	input := []Reminder{
		{
			ID:        "rem-alarms",
			Title:     "Alarm Test",
			List:      "Inbox",
			ListID:    "list-1",
			DueDate:   &due,
			HasAlarms: true,
			Alarms: []Alarm{
				{AbsoluteDate: &alarm1},
				{RelativeOffset: -15 * time.Minute},
				{RelativeOffset: -1 * time.Hour},
			},
		},
	}

	jsonStr := simulateRemindersResponse(input)
	parsed, err := parseRemindersJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := parsed[0]
	if len(r.Alarms) != 3 {
		t.Fatalf("alarms = %d, want 3", len(r.Alarms))
	}

	// First alarm: absolute date
	if r.Alarms[0].AbsoluteDate == nil || !r.Alarms[0].AbsoluteDate.Equal(alarm1) {
		t.Errorf("alarm[0].AbsoluteDate = %v, want %v", r.Alarms[0].AbsoluteDate, alarm1)
	}

	// Second alarm: relative offset
	if r.Alarms[1].RelativeOffset != -15*time.Minute {
		t.Errorf("alarm[1].RelativeOffset = %v, want -15m", r.Alarms[1].RelativeOffset)
	}

	// Third alarm: relative offset
	if r.Alarms[2].RelativeOffset != -1*time.Hour {
		t.Errorf("alarm[2].RelativeOffset = %v, want -1h", r.Alarms[2].RelativeOffset)
	}
}

func TestMockTimezoneSymmetry(t *testing.T) {
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

			due := time.Date(2026, 6, 15, 14, 0, 0, 0, loc)
			input := CreateReminderInput{
				Title:    "TZ Test: " + tz,
				ListName: "Inbox",
				DueDate:  &due,
			}

			jsonStr, err := marshalCreateInput(input)
			if err != nil {
				t.Fatalf("marshal error: %v", err)
			}

			var m map[string]any
			json.Unmarshal([]byte(jsonStr), &m)

			parsedDue := parseISO8601(m["dueDate"].(string))
			if !parsedDue.Equal(due.UTC()) {
				t.Errorf("due mismatch: got %v, want %v", parsedDue, due.UTC())
			}
		})
	}
}

// --- List CRUD mock tests ---

func TestMockCreateListRoundtrip(t *testing.T) {
	// Simulate: Go marshals CreateListInput -> ObjC creates list -> returns JSON -> Go parses
	input := CreateListInput{
		Title:  "Shopping",
		Source: "iCloud",
		Color:  "#FF6961",
	}

	jsonStr, err := marshalCreateListInput(input)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var m map[string]any
	json.Unmarshal([]byte(jsonStr), &m)
	if m["title"] != "Shopping" {
		t.Errorf("title = %v", m["title"])
	}
	if m["source"] != "iCloud" {
		t.Errorf("source = %v", m["source"])
	}
	if m["color"] != "#FF6961" {
		t.Errorf("color = %v", m["color"])
	}

	// Simulate ObjC response: a single list JSON wrapped in array for parsing
	responseJSON := simulateListsResponse([]List{
		{ID: "new-list-123", Title: "Shopping", Color: "#FF6961", Source: "iCloud", Count: 0, ReadOnly: false},
	})

	lists, err := parseListsJSON(responseJSON)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(lists) != 1 {
		t.Fatalf("expected 1 list, got %d", len(lists))
	}
	if lists[0].ID != "new-list-123" {
		t.Errorf("ID = %q", lists[0].ID)
	}
	if lists[0].Title != "Shopping" {
		t.Errorf("Title = %q", lists[0].Title)
	}
	if lists[0].Count != 0 {
		t.Errorf("Count = %d, want 0 for new list", lists[0].Count)
	}
}

func TestMockUpdateListRoundtrip(t *testing.T) {
	t.Run("rename list", func(t *testing.T) {
		title := "Groceries"
		jsonStr, err := marshalUpdateListInput(UpdateListInput{Title: &title})
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)
		if m["title"] != "Groceries" {
			t.Errorf("title = %v", m["title"])
		}
		if _, ok := m["color"]; ok {
			t.Error("color should not be in update when nil")
		}

		// Simulate ObjC response after update
		responseJSON := simulateListsResponse([]List{
			{ID: "list-123", Title: "Groceries", Color: "#FF0000", Source: "iCloud", Count: 5, ReadOnly: false},
		})

		lists, err := parseListsJSON(responseJSON)
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if lists[0].Title != "Groceries" {
			t.Errorf("Title = %q", lists[0].Title)
		}
	})

	t.Run("change color", func(t *testing.T) {
		color := "#00FF00"
		jsonStr, err := marshalUpdateListInput(UpdateListInput{Color: &color})
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)
		if m["color"] != "#00FF00" {
			t.Errorf("color = %v", m["color"])
		}
	})
}

func TestMockListCRUDSentinelErrors(t *testing.T) {
	if ErrImmutable.Error() != "reminders: list is immutable" {
		t.Errorf("ErrImmutable = %q", ErrImmutable.Error())
	}
}

func TestMockEdgeCases(t *testing.T) {
	t.Run("reminder with all nil optional fields", func(t *testing.T) {
		jsonStr := `{
			"id": "minimal",
			"title": "Simple task",
			"notes": null,
			"list": "Inbox",
			"listID": "list-1",
			"dueDate": null,
			"remindMeDate": null,
			"completionDate": null,
			"createdAt": null,
			"modifiedAt": null,
			"priority": 0,
			"completed": false,
			"flagged": false,
			"url": null,
			"hasAlarms": false,
			"alarms": []
		}`

		r, err := parseReminderJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if r.DueDate != nil {
			t.Errorf("DueDate = %v, want nil", r.DueDate)
		}
		if r.Notes != "" {
			t.Errorf("Notes = %q, want empty", r.Notes)
		}
		if r.URL != "" {
			t.Errorf("URL = %q, want empty", r.URL)
		}
		if r.Priority != PriorityNone {
			t.Errorf("Priority = %d, want 0", r.Priority)
		}
	})

	t.Run("reminder with unicode in fields", func(t *testing.T) {
		jsonStr := `{
			"id": "unicode",
			"title": "買い物リスト 🛒",
			"notes": "牛乳を買う — très important",
			"list": "買い物",
			"listID": "list-jp",
			"priority": 1,
			"completed": false,
			"flagged": false,
			"hasAlarms": false,
			"alarms": []
		}`

		r, err := parseReminderJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if r.Title != "買い物リスト 🛒" {
			t.Errorf("Title = %q", r.Title)
		}
		if r.Notes != "牛乳を買う — très important" {
			t.Errorf("Notes = %q", r.Notes)
		}
		if r.List != "買い物" {
			t.Errorf("List = %q", r.List)
		}
	})

	t.Run("priority range values", func(t *testing.T) {
		// EventKit uses 0-9 but only 0, 1, 5, 9 are canonical
		// Verify intermediate values parse correctly
		for prio := 0; prio <= 9; prio++ {
			p := Priority(prio)
			s := p.String()
			if s == "" {
				t.Errorf("Priority(%d).String() returned empty", prio)
			}
		}
	})
}

func TestMockRecurrenceRoundtrip(t *testing.T) {
	endDate := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)

	input := []Reminder{
		{
			ID:        "rec-daily",
			Title:     "Daily standup",
			List:      "Work",
			ListID:    "list-1",
			Recurring: true,
			RecurrenceRules: []eventkit.RecurrenceRule{
				eventkit.Daily(1).Count(30),
			},
			Alarms: []Alarm{},
		},
		{
			ID:        "rec-weekly",
			Title:     "Weekly review",
			List:      "Work",
			ListID:    "list-1",
			Recurring: true,
			RecurrenceRules: []eventkit.RecurrenceRule{
				eventkit.Weekly(2, eventkit.Monday, eventkit.Friday).Until(endDate),
			},
			Alarms: []Alarm{},
		},
		{
			ID:        "rec-monthly",
			Title:     "Pay rent",
			List:      "Bills",
			ListID:    "list-2",
			Recurring: true,
			RecurrenceRules: []eventkit.RecurrenceRule{
				eventkit.Monthly(1, 1),
			},
			Alarms: []Alarm{},
		},
		{
			ID:     "no-rec",
			Title:  "One-time task",
			List:   "Inbox",
			ListID: "list-3",
			Alarms: []Alarm{},
		},
	}

	jsonStr := simulateRemindersResponse(input)
	parsed, err := parseRemindersJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed) != 4 {
		t.Fatalf("parsed %d reminders, want 4", len(parsed))
	}

	// Daily with count.
	r0 := parsed[0]
	if !r0.Recurring {
		t.Error("r0 should be recurring")
	}
	if len(r0.RecurrenceRules) != 1 {
		t.Fatalf("r0 rules = %d, want 1", len(r0.RecurrenceRules))
	}
	if r0.RecurrenceRules[0].Frequency != eventkit.FrequencyDaily {
		t.Errorf("r0 frequency = %d, want daily", r0.RecurrenceRules[0].Frequency)
	}
	if r0.RecurrenceRules[0].Interval != 1 {
		t.Errorf("r0 interval = %d, want 1", r0.RecurrenceRules[0].Interval)
	}
	if r0.RecurrenceRules[0].End == nil || r0.RecurrenceRules[0].End.OccurrenceCount != 30 {
		t.Errorf("r0 end = %+v, want count=30", r0.RecurrenceRules[0].End)
	}

	// Weekly with days and end date.
	r1 := parsed[1]
	if !r1.Recurring {
		t.Error("r1 should be recurring")
	}
	if len(r1.RecurrenceRules) != 1 {
		t.Fatalf("r1 rules = %d, want 1", len(r1.RecurrenceRules))
	}
	rule := r1.RecurrenceRules[0]
	if rule.Frequency != eventkit.FrequencyWeekly {
		t.Errorf("r1 frequency = %d, want weekly", rule.Frequency)
	}
	if rule.Interval != 2 {
		t.Errorf("r1 interval = %d, want 2", rule.Interval)
	}
	if len(rule.DaysOfTheWeek) != 2 {
		t.Fatalf("r1 days = %d, want 2", len(rule.DaysOfTheWeek))
	}
	if rule.DaysOfTheWeek[0].DayOfTheWeek != eventkit.Monday {
		t.Errorf("r1 day[0] = %d, want Monday", rule.DaysOfTheWeek[0].DayOfTheWeek)
	}
	if rule.DaysOfTheWeek[1].DayOfTheWeek != eventkit.Friday {
		t.Errorf("r1 day[1] = %d, want Friday", rule.DaysOfTheWeek[1].DayOfTheWeek)
	}
	if rule.End == nil || rule.End.EndDate == nil || !rule.End.EndDate.Equal(endDate) {
		t.Errorf("r1 end date mismatch: %+v", rule.End)
	}

	// Monthly with day of month.
	r2 := parsed[2]
	if !r2.Recurring {
		t.Error("r2 should be recurring")
	}
	if len(r2.RecurrenceRules[0].DaysOfTheMonth) != 1 || r2.RecurrenceRules[0].DaysOfTheMonth[0] != 1 {
		t.Errorf("r2 daysOfMonth = %v, want [1]", r2.RecurrenceRules[0].DaysOfTheMonth)
	}

	// Non-recurring.
	r3 := parsed[3]
	if r3.Recurring {
		t.Error("r3 should not be recurring")
	}
	if len(r3.RecurrenceRules) != 0 {
		t.Errorf("r3 rules = %d, want 0", len(r3.RecurrenceRules))
	}
}

func TestMockCreateReminderWithRecurrence(t *testing.T) {
	input := CreateReminderInput{
		Title:    "Daily review",
		ListName: "Work",
		RecurrenceRules: []eventkit.RecurrenceRule{
			eventkit.Daily(1).Count(5),
		},
	}

	jsonStr, err := marshalCreateInput(input)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal([]byte(jsonStr), &parsed)

	rules, ok := parsed["recurrenceRules"].([]any)
	if !ok {
		t.Fatal("recurrenceRules not present or wrong type")
	}
	if len(rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(rules))
	}

	rule := rules[0].(map[string]any)
	if rule["frequency"] != float64(0) {
		t.Errorf("frequency = %v, want 0 (daily)", rule["frequency"])
	}
	if rule["interval"] != float64(1) {
		t.Errorf("interval = %v, want 1", rule["interval"])
	}
	end := rule["end"].(map[string]any)
	if end["occurrenceCount"] != float64(5) {
		t.Errorf("occurrenceCount = %v, want 5", end["occurrenceCount"])
	}
}

func TestMockUpdateReminderWithRecurrence(t *testing.T) {
	t.Run("add recurrence rules", func(t *testing.T) {
		rules := []eventkit.RecurrenceRule{
			eventkit.Weekly(1, eventkit.Monday, eventkit.Wednesday),
		}
		data, err := marshalUpdateInput(UpdateReminderInput{
			RecurrenceRules: &rules,
		})
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var m map[string]any
		json.Unmarshal([]byte(data), &m)

		rr := m["recurrenceRules"].([]any)
		if len(rr) != 1 {
			t.Fatalf("rules = %d, want 1", len(rr))
		}
		rule := rr[0].(map[string]any)
		if rule["frequency"] != float64(1) {
			t.Errorf("frequency = %v, want 1 (weekly)", rule["frequency"])
		}
		days := rule["daysOfTheWeek"].([]any)
		if len(days) != 2 {
			t.Fatalf("days = %d, want 2", len(days))
		}
	})

	t.Run("remove recurrence rules", func(t *testing.T) {
		empty := []eventkit.RecurrenceRule{}
		data, _ := marshalUpdateInput(UpdateReminderInput{
			RecurrenceRules: &empty,
		})

		var m map[string]any
		json.Unmarshal([]byte(data), &m)

		rr := m["recurrenceRules"].([]any)
		if len(rr) != 0 {
			t.Errorf("rules = %d, want 0", len(rr))
		}
	})

	t.Run("nil leaves recurrence unchanged", func(t *testing.T) {
		data, _ := marshalUpdateInput(UpdateReminderInput{})

		var m map[string]any
		json.Unmarshal([]byte(data), &m)

		if _, ok := m["recurrenceRules"]; ok {
			t.Error("recurrenceRules should NOT be in update when nil")
		}
	})
}
