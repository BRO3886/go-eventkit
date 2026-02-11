package reminders

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/BRO3886/go-eventkit"
)

// --- Priority Tests ---

func TestPriorityString(t *testing.T) {
	tests := []struct {
		p    Priority
		want string
	}{
		{PriorityNone, "none"},
		{PriorityHigh, "high"},
		{PriorityMedium, "medium"},
		{PriorityLow, "low"},
		{Priority(2), "high"},   // 1-4 = high
		{Priority(3), "high"},   // 1-4 = high
		{Priority(4), "high"},   // 1-4 = high
		{Priority(6), "low"},    // 6-9 = low
		{Priority(7), "low"},    // 6-9 = low
		{Priority(8), "low"},    // 6-9 = low
		{Priority(10), "none"},  // out of range
		{Priority(-1), "none"},  // negative
	}
	for _, tt := range tests {
		got := tt.p.String()
		if got != tt.want {
			t.Errorf("Priority(%d).String() = %q, want %q", tt.p, got, tt.want)
		}
	}
}

func TestParsePriority(t *testing.T) {
	tests := []struct {
		input string
		want  Priority
	}{
		{"high", PriorityHigh},
		{"h", PriorityHigh},
		{"1", PriorityHigh},
		{"medium", PriorityMedium},
		{"med", PriorityMedium},
		{"m", PriorityMedium},
		{"5", PriorityMedium},
		{"low", PriorityLow},
		{"l", PriorityLow},
		{"9", PriorityLow},
		{"", PriorityNone},
		{"unknown", PriorityNone},
		{"MEDIUM", PriorityNone}, // case-sensitive
	}
	for _, tt := range tests {
		got := ParsePriority(tt.input)
		if got != tt.want {
			t.Errorf("ParsePriority(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestPriorityEnumValues(t *testing.T) {
	// Verify enum values match Apple's EventKit priority values.
	if PriorityNone != 0 {
		t.Errorf("PriorityNone = %d, want 0", PriorityNone)
	}
	if PriorityHigh != 1 {
		t.Errorf("PriorityHigh = %d, want 1", PriorityHigh)
	}
	if PriorityMedium != 5 {
		t.Errorf("PriorityMedium = %d, want 5", PriorityMedium)
	}
	if PriorityLow != 9 {
		t.Errorf("PriorityLow = %d, want 9", PriorityLow)
	}
}

// --- Options Tests ---

func TestApplyOptions(t *testing.T) {
	t.Run("no options", func(t *testing.T) {
		o := applyOptions(nil)
		if o.listName != "" {
			t.Errorf("listName = %q, want empty", o.listName)
		}
		if o.completed != nil {
			t.Errorf("completed should be nil")
		}
	})

	t.Run("all options", func(t *testing.T) {
		before := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		after := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		o := applyOptions([]ListOption{
			WithList("Shopping"),
			WithCompleted(false),
			WithSearch("milk"),
			WithDueBefore(before),
			WithDueAfter(after),
		})
		if o.listName != "Shopping" {
			t.Errorf("listName = %q, want Shopping", o.listName)
		}
		if o.completed == nil || *o.completed != false {
			t.Error("completed should be false")
		}
		if o.search != "milk" {
			t.Errorf("search = %q, want milk", o.search)
		}
		if o.dueBefore == nil || !o.dueBefore.Equal(before) {
			t.Error("dueBefore not set correctly")
		}
		if o.dueAfter == nil || !o.dueAfter.Equal(after) {
			t.Error("dueAfter not set correctly")
		}
	})

	t.Run("list ID option", func(t *testing.T) {
		o := applyOptions([]ListOption{WithListID("cal-123")})
		if o.listID != "cal-123" {
			t.Errorf("listID = %q, want cal-123", o.listID)
		}
	})

	t.Run("last option wins", func(t *testing.T) {
		o := applyOptions([]ListOption{
			WithList("First"),
			WithList("Second"),
		})
		if o.listName != "Second" {
			t.Errorf("listName = %q, want Second (last wins)", o.listName)
		}
	})
}

// --- Parse ISO 8601 Tests ---

func TestParseISO8601(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{
			name:  "fractional seconds",
			input: "2026-02-11T14:30:00.000Z",
			want:  time.Date(2026, 2, 11, 14, 30, 0, 0, time.UTC),
		},
		{
			name:  "no fractional seconds",
			input: "2026-02-11T14:30:00Z",
			want:  time.Date(2026, 2, 11, 14, 30, 0, 0, time.UTC),
		},
		{
			name:  "with timezone offset",
			input: "2026-02-11T14:30:00+05:30",
			want:  time.Date(2026, 2, 11, 14, 30, 0, 0, time.FixedZone("", 5*3600+30*60)),
		},
		{
			name:  "empty string",
			input: "",
			want:  time.Time{},
		},
		{
			name:  "invalid string",
			input: "not-a-date",
			want:  time.Time{},
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

// --- Parse Reminder JSON Tests ---

func TestParseReminderJSON(t *testing.T) {
	t.Run("full reminder", func(t *testing.T) {
		jsonStr := `{
			"id": "550e8400-e29b-41d4-a716-446655440000",
			"title": "Buy milk",
			"notes": "Whole milk from Trader Joe's",
			"list": "Shopping",
			"listID": "cal-123",
			"dueDate": "2026-02-12T09:00:00.000Z",
			"remindMeDate": "2026-02-12T08:30:00.000Z",
			"completionDate": null,
			"createdAt": "2026-02-11T10:00:00.000Z",
			"modifiedAt": "2026-02-11T12:00:00.000Z",
			"priority": 1,
			"completed": false,
			"flagged": false,
			"url": "https://example.com",
			"hasAlarms": true,
			"alarms": [
				{"absoluteDate": "2026-02-12T08:30:00.000Z", "relativeOffset": 0}
			]
		}`
		r, err := parseReminderJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.ID != "550e8400-e29b-41d4-a716-446655440000" {
			t.Errorf("ID = %q", r.ID)
		}
		if r.Title != "Buy milk" {
			t.Errorf("Title = %q", r.Title)
		}
		if r.Notes != "Whole milk from Trader Joe's" {
			t.Errorf("Notes = %q", r.Notes)
		}
		if r.List != "Shopping" {
			t.Errorf("List = %q", r.List)
		}
		if r.Priority != PriorityHigh {
			t.Errorf("Priority = %d, want %d", r.Priority, PriorityHigh)
		}
		if r.URL != "https://example.com" {
			t.Errorf("URL = %q", r.URL)
		}
		if r.DueDate == nil {
			t.Fatal("DueDate should not be nil")
		}
		if len(r.Alarms) != 1 {
			t.Fatalf("Alarms len = %d, want 1", len(r.Alarms))
		}
		if r.Alarms[0].AbsoluteDate == nil {
			t.Error("Alarm absolute date should not be nil")
		}
	})

	t.Run("null fields", func(t *testing.T) {
		jsonStr := `{
			"id": "abc",
			"title": "Minimal",
			"notes": null,
			"list": "Default",
			"listID": "",
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
		if r.Notes != "" {
			t.Errorf("Notes should be empty for null, got %q", r.Notes)
		}
		if r.URL != "" {
			t.Errorf("URL should be empty for null, got %q", r.URL)
		}
		if r.DueDate != nil {
			t.Error("DueDate should be nil")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := parseReminderJSON("not json")
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestParseRemindersJSON(t *testing.T) {
	t.Run("multiple reminders", func(t *testing.T) {
		jsonStr := `[
			{"id": "1", "title": "First", "list": "A", "listID": "a", "priority": 0, "completed": false, "flagged": false, "hasAlarms": false, "alarms": []},
			{"id": "2", "title": "Second", "list": "B", "listID": "b", "priority": 5, "completed": true, "flagged": false, "hasAlarms": false, "alarms": []}
		]`
		items, err := parseRemindersJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("got %d reminders, want 2", len(items))
		}
		if items[0].Title != "First" {
			t.Errorf("items[0].Title = %q", items[0].Title)
		}
		if items[1].Priority != PriorityMedium {
			t.Errorf("items[1].Priority = %d", items[1].Priority)
		}
		if !items[1].Completed {
			t.Error("items[1] should be completed")
		}
	})

	t.Run("empty array", func(t *testing.T) {
		items, err := parseRemindersJSON("[]")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("got %d reminders, want 0", len(items))
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := parseRemindersJSON("invalid")
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

// --- Parse Lists JSON Tests ---

func TestParseListsJSON(t *testing.T) {
	t.Run("multiple lists", func(t *testing.T) {
		jsonStr := `[
			{"id": "cal-1", "title": "Reminders", "color": "#FF0000", "source": "iCloud", "count": 42, "readOnly": false},
			{"id": "cal-2", "title": "Work", "color": "#0000FF", "source": "iCloud", "count": 15, "readOnly": false},
			{"id": "cal-3", "title": "Birthdays", "color": "#00FF00", "source": "Other", "count": 5, "readOnly": true}
		]`
		lists, err := parseListsJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(lists) != 3 {
			t.Fatalf("got %d lists, want 3", len(lists))
		}
		if lists[0].Title != "Reminders" {
			t.Errorf("lists[0].Title = %q", lists[0].Title)
		}
		if lists[0].Count != 42 {
			t.Errorf("lists[0].Count = %d", lists[0].Count)
		}
		if lists[2].ReadOnly != true {
			t.Error("lists[2] should be read-only")
		}
	})

	t.Run("empty array", func(t *testing.T) {
		lists, err := parseListsJSON("[]")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(lists) != 0 {
			t.Errorf("got %d lists, want 0", len(lists))
		}
	})
}

// --- Marshal Create Input Tests ---

func TestMarshalCreateInput(t *testing.T) {
	t.Run("full input", func(t *testing.T) {
		due := time.Date(2026, 2, 12, 9, 0, 0, 0, time.UTC)
		remind := time.Date(2026, 2, 12, 8, 30, 0, 0, time.UTC)
		input := CreateReminderInput{
			Title:        "Buy milk",
			Notes:        "Whole milk",
			ListName:     "Shopping",
			DueDate:      &due,
			RemindMeDate: &remind,
			Priority:     PriorityHigh,
			URL:          "https://example.com",
		}
		jsonStr, err := marshalCreateInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if m["title"] != "Buy milk" {
			t.Errorf("title = %v", m["title"])
		}
		if m["notes"] != "Whole milk" {
			t.Errorf("notes = %v", m["notes"])
		}
		if m["listName"] != "Shopping" {
			t.Errorf("listName = %v", m["listName"])
		}
		if m["url"] != "https://example.com" {
			t.Errorf("url = %v", m["url"])
		}
		if m["priority"] != float64(1) {
			t.Errorf("priority = %v", m["priority"])
		}
	})

	t.Run("minimal input", func(t *testing.T) {
		input := CreateReminderInput{Title: "Quick task"}
		jsonStr, err := marshalCreateInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)
		if m["title"] != "Quick task" {
			t.Errorf("title = %v", m["title"])
		}
		// Omitted fields should not be present.
		if _, ok := m["notes"]; ok {
			t.Error("notes should not be present for empty input")
		}
		if _, ok := m["listName"]; ok {
			t.Error("listName should not be present for empty input")
		}
		if _, ok := m["priority"]; ok {
			t.Error("priority should not be present for PriorityNone")
		}
	})

	t.Run("with alarms", func(t *testing.T) {
		abs := time.Date(2026, 2, 12, 8, 0, 0, 0, time.UTC)
		input := CreateReminderInput{
			Title: "Alarmed",
			Alarms: []Alarm{
				{AbsoluteDate: &abs},
				{RelativeOffset: -15 * time.Minute},
			},
		}
		jsonStr, err := marshalCreateInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)
		alarms := m["alarms"].([]any)
		if len(alarms) != 2 {
			t.Fatalf("alarms len = %d, want 2", len(alarms))
		}
	})
}

// --- Marshal Update Input Tests ---

func TestMarshalUpdateInput(t *testing.T) {
	t.Run("title only", func(t *testing.T) {
		title := "New Title"
		input := UpdateReminderInput{Title: &title}
		jsonStr, err := marshalUpdateInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)
		if m["title"] != "New Title" {
			t.Errorf("title = %v", m["title"])
		}
		if _, ok := m["notes"]; ok {
			t.Error("notes should not be in output")
		}
	})

	t.Run("clear due date", func(t *testing.T) {
		input := UpdateReminderInput{ClearDueDate: true}
		jsonStr, err := marshalUpdateInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)
		// dueDate should be null (nil in JSON).
		if _, ok := m["dueDate"]; !ok {
			t.Error("dueDate should be present (as null)")
		}
		if m["dueDate"] != nil {
			t.Errorf("dueDate should be null, got %v", m["dueDate"])
		}
	})

	t.Run("complete and flag", func(t *testing.T) {
		completed := true
		flagged := true
		input := UpdateReminderInput{
			Completed: &completed,
			Flagged:   &flagged,
		}
		jsonStr, err := marshalUpdateInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)
		if m["completed"] != true {
			t.Errorf("completed = %v", m["completed"])
		}
		if m["flagged"] != true {
			t.Errorf("flagged = %v", m["flagged"])
		}
	})

	t.Run("move to list", func(t *testing.T) {
		list := "Work"
		input := UpdateReminderInput{ListName: &list}
		jsonStr, err := marshalUpdateInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)
		if m["listName"] != "Work" {
			t.Errorf("listName = %v", m["listName"])
		}
	})

	t.Run("empty update", func(t *testing.T) {
		input := UpdateReminderInput{}
		jsonStr, err := marshalUpdateInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)
		if len(m) != 0 {
			t.Errorf("expected empty map, got %d entries", len(m))
		}
	})

	t.Run("update remind me date", func(t *testing.T) {
		remind := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
		input := UpdateReminderInput{RemindMeDate: &remind}
		jsonStr, err := marshalUpdateInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)
		if m["remindMeDate"] != "2026-03-01T08:00:00.000Z" {
			t.Errorf("remindMeDate = %v", m["remindMeDate"])
		}
	})
}

// --- Sentinel Error Tests ---

func TestSentinelErrors(t *testing.T) {
	if ErrUnsupported.Error() != "reminders: not supported on this platform" {
		t.Errorf("ErrUnsupported = %q", ErrUnsupported.Error())
	}
	if ErrAccessDenied.Error() != "reminders: access denied" {
		t.Errorf("ErrAccessDenied = %q", ErrAccessDenied.Error())
	}
	if ErrNotFound.Error() != "reminders: not found" {
		t.Errorf("ErrNotFound = %q", ErrNotFound.Error())
	}
}

// --- Convert Raw Reminder Tests ---

func TestConvertRawReminder(t *testing.T) {
	t.Run("full conversion", func(t *testing.T) {
		due := "2026-02-12T09:00:00.000Z"
		remind := "2026-02-12T08:30:00.000Z"
		created := "2026-02-11T10:00:00.000Z"
		notes := "Some notes"
		url := "https://example.com"
		raw := rawReminder{
			ID:        "abc-123",
			Title:     "Test",
			Notes:     &notes,
			List:      "Work",
			ListID:    "cal-1",
			DueDate:   &due,
			CreatedAt: &created,
			Priority:  5,
			Completed: true,
			Flagged:   false,
			URL:       &url,
			HasAlarms: true,
			Alarms: []rawAlarm{
				{AbsoluteDate: &remind, RelativeOffset: 0},
			},
		}
		r := convertRawReminder(&raw)
		if r.ID != "abc-123" {
			t.Errorf("ID = %q", r.ID)
		}
		if r.Notes != "Some notes" {
			t.Errorf("Notes = %q", r.Notes)
		}
		if r.DueDate == nil {
			t.Fatal("DueDate should not be nil")
		}
		if r.Priority != PriorityMedium {
			t.Errorf("Priority = %d", r.Priority)
		}
		if !r.Completed {
			t.Error("should be completed")
		}
		if len(r.Alarms) != 1 {
			t.Errorf("Alarms len = %d", len(r.Alarms))
		}
	})

	t.Run("nil fields", func(t *testing.T) {
		raw := rawReminder{
			ID:    "xyz",
			Title: "Minimal",
			List:  "Default",
		}
		r := convertRawReminder(&raw)
		if r.Notes != "" {
			t.Errorf("Notes should be empty, got %q", r.Notes)
		}
		if r.DueDate != nil {
			t.Error("DueDate should be nil")
		}
		if r.URL != "" {
			t.Errorf("URL should be empty, got %q", r.URL)
		}
		if len(r.Alarms) != 0 {
			t.Errorf("Alarms should be empty, got %d", len(r.Alarms))
		}
	})
}

// --- Timezone Handling Tests ---

func TestTimezoneHandling(t *testing.T) {
	t.Run("UTC dates parsed correctly", func(t *testing.T) {
		jsonStr := `{
			"id": "tz1",
			"title": "UTC Test",
			"list": "Test",
			"listID": "t",
			"dueDate": "2026-06-15T14:00:00.000Z",
			"priority": 0,
			"completed": false,
			"flagged": false,
			"hasAlarms": false,
			"alarms": []
		}`
		r, err := parseReminderJSON(jsonStr)
		if err != nil {
			t.Fatal(err)
		}
		if r.DueDate == nil {
			t.Fatal("DueDate should not be nil")
		}
		if r.DueDate.Hour() != 14 {
			t.Errorf("Hour = %d, want 14", r.DueDate.Hour())
		}
	})

	t.Run("create input converts to UTC", func(t *testing.T) {
		ist := time.FixedZone("IST", 5*3600+30*60)
		localTime := time.Date(2026, 2, 12, 14, 30, 0, 0, ist) // 14:30 IST = 09:00 UTC
		input := CreateReminderInput{
			Title:   "IST Test",
			DueDate: &localTime,
		}
		jsonStr, err := marshalCreateInput(input)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)
		dueStr := m["dueDate"].(string)
		if dueStr != "2026-02-12T09:00:00.000Z" {
			t.Errorf("dueDate = %q, want 2026-02-12T09:00:00.000Z (UTC)", dueStr)
		}
	})
}

// --- Large Data Set Tests ---

func TestLargeReminderSet(t *testing.T) {
	// Build a JSON array of 200 reminders.
	type minReminder struct {
		ID        string     `json:"id"`
		Title     string     `json:"title"`
		List      string     `json:"list"`
		ListID    string     `json:"listID"`
		Priority  int        `json:"priority"`
		Completed bool       `json:"completed"`
		Flagged   bool       `json:"flagged"`
		HasAlarms bool       `json:"hasAlarms"`
		Alarms    []rawAlarm `json:"alarms"`
	}
	reminders := make([]minReminder, 200)
	for i := range reminders {
		reminders[i] = minReminder{
			ID:        "id-" + time.Now().String(),
			Title:     "Reminder " + time.Now().String(),
			List:      "Test",
			ListID:    "t",
			Priority:  i % 10,
			Completed: i%3 == 0,
			Alarms:    []rawAlarm{},
		}
	}
	data, _ := json.Marshal(reminders)
	items, err := parseRemindersJSON(string(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 200 {
		t.Errorf("got %d reminders, want 200", len(items))
	}
}

// --- JSON Round-trip Tests ---

func TestJSONRoundtrip(t *testing.T) {
	t.Run("special characters in title", func(t *testing.T) {
		jsonStr := `{
			"id": "special",
			"title": "Test \"quotes\" & <tags> & こんにちは",
			"list": "Default",
			"listID": "d",
			"priority": 0,
			"completed": false,
			"flagged": false,
			"hasAlarms": false,
			"alarms": []
		}`
		r, err := parseReminderJSON(jsonStr)
		if err != nil {
			t.Fatal(err)
		}
		if r.Title != "Test \"quotes\" & <tags> & こんにちは" {
			t.Errorf("Title = %q", r.Title)
		}
	})
}

// --- parseOptionalTime edge case ---

func TestParseOptionalTime(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := parseOptionalTime(nil); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("empty string returns nil", func(t *testing.T) {
		s := ""
		if got := parseOptionalTime(&s); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("invalid date returns nil", func(t *testing.T) {
		s := "not-a-date"
		if got := parseOptionalTime(&s); got != nil {
			t.Errorf("expected nil for invalid date, got %v", got)
		}
	})

	t.Run("valid date returns time", func(t *testing.T) {
		s := "2026-02-12T09:00:00.000Z"
		got := parseOptionalTime(&s)
		if got == nil {
			t.Fatal("expected non-nil time")
		}
		want := time.Date(2026, 2, 12, 9, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

// --- parseListsJSON error path ---

func TestParseListsJSONInvalid(t *testing.T) {
	_, err := parseListsJSON("not valid json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// --- List CRUD marshal tests ---

func TestMarshalCreateListInput(t *testing.T) {
	t.Run("full input", func(t *testing.T) {
		input := CreateListInput{
			Title:  "Shopping",
			Source: "iCloud",
			Color:  "#FF6961",
		}
		jsonStr, err := marshalCreateListInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if m["title"] != "Shopping" {
			t.Errorf("title = %v", m["title"])
		}
		if m["source"] != "iCloud" {
			t.Errorf("source = %v", m["source"])
		}
		if m["color"] != "#FF6961" {
			t.Errorf("color = %v", m["color"])
		}
	})

	t.Run("minimal input omits optional fields", func(t *testing.T) {
		input := CreateListInput{Title: "Work"}
		jsonStr, err := marshalCreateListInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)

		if m["title"] != "Work" {
			t.Errorf("title = %v", m["title"])
		}
		if _, ok := m["source"]; ok {
			t.Error("source should be omitted when empty")
		}
		if _, ok := m["color"]; ok {
			t.Error("color should be omitted when empty")
		}
	})
}

func TestMarshalUpdateListInput(t *testing.T) {
	t.Run("update title only", func(t *testing.T) {
		title := "New Name"
		input := UpdateListInput{Title: &title}
		jsonStr, err := marshalUpdateListInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)

		if m["title"] != "New Name" {
			t.Errorf("title = %v", m["title"])
		}
		if _, ok := m["color"]; ok {
			t.Error("color should not be present when nil")
		}
	})

	t.Run("update color only", func(t *testing.T) {
		color := "#00FF00"
		input := UpdateListInput{Color: &color}
		jsonStr, err := marshalUpdateListInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)

		if m["color"] != "#00FF00" {
			t.Errorf("color = %v", m["color"])
		}
		if _, ok := m["title"]; ok {
			t.Error("title should not be present when nil")
		}
	})

	t.Run("update both", func(t *testing.T) {
		title := "Renamed"
		color := "#AABBCC"
		input := UpdateListInput{Title: &title, Color: &color}
		jsonStr, err := marshalUpdateListInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)

		if m["title"] != "Renamed" {
			t.Errorf("title = %v", m["title"])
		}
		if m["color"] != "#AABBCC" {
			t.Errorf("color = %v", m["color"])
		}
	})

	t.Run("empty update produces empty object", func(t *testing.T) {
		input := UpdateListInput{}
		jsonStr, err := marshalUpdateListInput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if jsonStr != "{}" {
			t.Errorf("got %s, want {}", jsonStr)
		}
	})
}

func TestSentinelErrorImmutable(t *testing.T) {
	if ErrImmutable.Error() != "reminders: list is immutable" {
		t.Errorf("ErrImmutable = %q", ErrImmutable.Error())
	}
}

// --- Recurrence Rule Tests ---

func TestParseReminderWithRecurrence(t *testing.T) {
	t.Run("daily recurrence with count", func(t *testing.T) {
		jsonStr := `{
			"id": "rec-1",
			"title": "Daily standup",
			"list": "Work",
			"listID": "w",
			"priority": 0,
			"completed": false,
			"flagged": false,
			"recurring": true,
			"recurrenceRules": [
				{
					"frequency": 0,
					"interval": 1,
					"end": {"occurrenceCount": 30}
				}
			],
			"hasAlarms": false,
			"alarms": []
		}`
		r, err := parseReminderJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !r.Recurring {
			t.Error("should be recurring")
		}
		if len(r.RecurrenceRules) != 1 {
			t.Fatalf("rules = %d, want 1", len(r.RecurrenceRules))
		}
		rule := r.RecurrenceRules[0]
		if rule.Frequency != eventkit.FrequencyDaily {
			t.Errorf("frequency = %d, want daily", rule.Frequency)
		}
		if rule.Interval != 1 {
			t.Errorf("interval = %d, want 1", rule.Interval)
		}
		if rule.End == nil || rule.End.OccurrenceCount != 30 {
			t.Errorf("end = %+v, want count=30", rule.End)
		}
	})

	t.Run("weekly recurrence with days and end date", func(t *testing.T) {
		jsonStr := `{
			"id": "rec-2",
			"title": "Weekly review",
			"list": "Work",
			"listID": "w",
			"priority": 0,
			"completed": false,
			"flagged": false,
			"recurring": true,
			"recurrenceRules": [
				{
					"frequency": 1,
					"interval": 2,
					"daysOfTheWeek": [
						{"dayOfTheWeek": 2, "weekNumber": 0},
						{"dayOfTheWeek": 6, "weekNumber": 0}
					],
					"end": {"endDate": "2026-12-31T00:00:00.000Z"}
				}
			],
			"hasAlarms": false,
			"alarms": []
		}`
		r, err := parseReminderJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		rule := r.RecurrenceRules[0]
		if rule.Frequency != eventkit.FrequencyWeekly {
			t.Errorf("frequency = %d, want weekly", rule.Frequency)
		}
		if rule.Interval != 2 {
			t.Errorf("interval = %d, want 2", rule.Interval)
		}
		if len(rule.DaysOfTheWeek) != 2 {
			t.Fatalf("days = %d, want 2", len(rule.DaysOfTheWeek))
		}
		if rule.DaysOfTheWeek[0].DayOfTheWeek != eventkit.Monday {
			t.Errorf("day[0] = %d, want Monday", rule.DaysOfTheWeek[0].DayOfTheWeek)
		}
		if rule.DaysOfTheWeek[1].DayOfTheWeek != eventkit.Friday {
			t.Errorf("day[1] = %d, want Friday", rule.DaysOfTheWeek[1].DayOfTheWeek)
		}
		if rule.End == nil || rule.End.EndDate == nil {
			t.Fatal("end date should not be nil")
		}
		expected := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
		if !rule.End.EndDate.Equal(expected) {
			t.Errorf("end date = %v, want %v", rule.End.EndDate, expected)
		}
	})

	t.Run("no recurrence rules", func(t *testing.T) {
		jsonStr := `{
			"id": "no-rec",
			"title": "One-time",
			"list": "Inbox",
			"listID": "i",
			"priority": 0,
			"completed": false,
			"flagged": false,
			"recurring": false,
			"recurrenceRules": [],
			"hasAlarms": false,
			"alarms": []
		}`
		r, err := parseReminderJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Recurring {
			t.Error("should not be recurring")
		}
		if len(r.RecurrenceRules) != 0 {
			t.Errorf("rules = %d, want 0", len(r.RecurrenceRules))
		}
	})

	t.Run("monthly recurrence with day of month", func(t *testing.T) {
		jsonStr := `{
			"id": "rec-3",
			"title": "Pay rent",
			"list": "Bills",
			"listID": "b",
			"priority": 1,
			"completed": false,
			"flagged": false,
			"recurring": true,
			"recurrenceRules": [
				{
					"frequency": 2,
					"interval": 1,
					"daysOfTheMonth": [1]
				}
			],
			"hasAlarms": false,
			"alarms": []
		}`
		r, err := parseReminderJSON(jsonStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		rule := r.RecurrenceRules[0]
		if rule.Frequency != eventkit.FrequencyMonthly {
			t.Errorf("frequency = %d, want monthly", rule.Frequency)
		}
		if len(rule.DaysOfTheMonth) != 1 || rule.DaysOfTheMonth[0] != 1 {
			t.Errorf("daysOfMonth = %v, want [1]", rule.DaysOfTheMonth)
		}
	})
}

func TestMarshalCreateInputWithRecurrence(t *testing.T) {
	t.Run("create with recurrence", func(t *testing.T) {
		input := CreateReminderInput{
			Title:    "Daily task",
			ListName: "Work",
			RecurrenceRules: []eventkit.RecurrenceRule{
				eventkit.Daily(1).Count(10),
			},
		}
		jsonStr, err := marshalCreateInput(input)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)

		rules := m["recurrenceRules"].([]any)
		if len(rules) != 1 {
			t.Fatalf("rules = %d, want 1", len(rules))
		}
		rule := rules[0].(map[string]any)
		if rule["frequency"] != float64(0) {
			t.Errorf("frequency = %v", rule["frequency"])
		}
		if rule["interval"] != float64(1) {
			t.Errorf("interval = %v", rule["interval"])
		}
	})

	t.Run("create without recurrence omits field", func(t *testing.T) {
		input := CreateReminderInput{Title: "Simple task"}
		jsonStr, err := marshalCreateInput(input)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)
		if _, ok := m["recurrenceRules"]; ok {
			t.Error("recurrenceRules should not be present for empty rules")
		}
	})
}

func TestMarshalRecurrenceRulesComprehensive(t *testing.T) {
	t.Run("yearly with all constraint arrays", func(t *testing.T) {
		endDate := time.Date(2030, 12, 31, 0, 0, 0, 0, time.UTC)
		rules := []eventkit.RecurrenceRule{
			{
				Frequency:       eventkit.FrequencyYearly,
				Interval:        1,
				MonthsOfTheYear: []int{3, 6, 9, 12},
				WeeksOfTheYear:  []int{1, 26, 52},
				DaysOfTheYear:   []int{1, 100, 365},
				SetPositions:    []int{1, -1},
				DaysOfTheWeek: []eventkit.RecurrenceDayOfWeek{
					{DayOfTheWeek: eventkit.Monday, WeekNumber: 2},
				},
				End: &eventkit.RecurrenceEnd{
					EndDate: &endDate,
				},
			},
		}

		input := CreateReminderInput{
			Title:           "Complex recurrence",
			RecurrenceRules: rules,
		}
		jsonStr, err := marshalCreateInput(input)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)

		rr := m["recurrenceRules"].([]any)
		rule := rr[0].(map[string]any)

		if rule["frequency"] != float64(3) {
			t.Errorf("frequency = %v, want 3 (yearly)", rule["frequency"])
		}

		months := rule["monthsOfTheYear"].([]any)
		if len(months) != 4 {
			t.Errorf("monthsOfTheYear = %d, want 4", len(months))
		}

		weeks := rule["weeksOfTheYear"].([]any)
		if len(weeks) != 3 {
			t.Errorf("weeksOfTheYear = %d, want 3", len(weeks))
		}

		daysYear := rule["daysOfTheYear"].([]any)
		if len(daysYear) != 3 {
			t.Errorf("daysOfTheYear = %d, want 3", len(daysYear))
		}

		positions := rule["setPositions"].([]any)
		if len(positions) != 2 {
			t.Errorf("setPositions = %d, want 2", len(positions))
		}

		days := rule["daysOfTheWeek"].([]any)
		if len(days) != 1 {
			t.Errorf("daysOfTheWeek = %d, want 1", len(days))
		}

		end := rule["end"].(map[string]any)
		if end["endDate"] != "2030-12-31T00:00:00.000Z" {
			t.Errorf("endDate = %v", end["endDate"])
		}
	})

	t.Run("monthly with days of month", func(t *testing.T) {
		rules := []eventkit.RecurrenceRule{
			{
				Frequency:      eventkit.FrequencyMonthly,
				Interval:       1,
				DaysOfTheMonth: []int{1, 15, -1},
			},
		}

		input := CreateReminderInput{
			Title:           "Monthly",
			RecurrenceRules: rules,
		}
		jsonStr, err := marshalCreateInput(input)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)

		rr := m["recurrenceRules"].([]any)
		rule := rr[0].(map[string]any)

		daysMonth := rule["daysOfTheMonth"].([]any)
		if len(daysMonth) != 3 {
			t.Errorf("daysOfTheMonth = %d, want 3", len(daysMonth))
		}
	})

	t.Run("recurrence with occurrence count end", func(t *testing.T) {
		rules := []eventkit.RecurrenceRule{
			{
				Frequency: eventkit.FrequencyDaily,
				Interval:  1,
				End: &eventkit.RecurrenceEnd{
					OccurrenceCount: 10,
				},
			},
		}

		input := CreateReminderInput{
			Title:           "Count end",
			RecurrenceRules: rules,
		}
		jsonStr, err := marshalCreateInput(input)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)

		rr := m["recurrenceRules"].([]any)
		rule := rr[0].(map[string]any)
		end := rule["end"].(map[string]any)

		if end["occurrenceCount"] != float64(10) {
			t.Errorf("occurrenceCount = %v, want 10", end["occurrenceCount"])
		}
		if _, ok := end["endDate"]; ok {
			t.Error("endDate should not be present when using occurrence count")
		}
	})
}

func TestMarshalUpdateInputWithRecurrence(t *testing.T) {
	t.Run("update with recurrence", func(t *testing.T) {
		rules := []eventkit.RecurrenceRule{
			eventkit.Weekly(1, eventkit.Tuesday, eventkit.Thursday),
		}
		jsonStr, err := marshalUpdateInput(UpdateReminderInput{
			RecurrenceRules: &rules,
		})
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)

		rr := m["recurrenceRules"].([]any)
		if len(rr) != 1 {
			t.Fatalf("rules = %d, want 1", len(rr))
		}
	})

	t.Run("clear recurrence", func(t *testing.T) {
		empty := []eventkit.RecurrenceRule{}
		jsonStr, err := marshalUpdateInput(UpdateReminderInput{
			RecurrenceRules: &empty,
		})
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)

		rr := m["recurrenceRules"].([]any)
		if len(rr) != 0 {
			t.Errorf("rules = %d, want 0", len(rr))
		}
	})
}
