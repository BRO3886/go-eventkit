package reminders

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseReminderJSONAllDay(t *testing.T) {
	t.Run("all-day due date", func(t *testing.T) {
		r, err := parseReminderJSON(`{"id":"A","title":"t","dueDate":"2026-09-30T04:00:00.000Z","dueDateAllDay":true}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !r.DueDateAllDay {
			t.Error("DueDateAllDay = false, want true")
		}
	})

	t.Run("timed due date", func(t *testing.T) {
		r, err := parseReminderJSON(`{"id":"A","title":"t","dueDate":"2026-09-30T04:00:00.000Z"}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.DueDateAllDay {
			t.Error("DueDateAllDay = true, want false")
		}
	})
}

func TestMarshalCreateInputAllDay(t *testing.T) {
	// 23:30 local on Sep 30 in a zone west of UTC is Oct 1 in UTC. The
	// all-day date must come from the caller's calendar date, not UTC.
	loc := time.FixedZone("EDT", -4*3600)
	due := time.Date(2026, 9, 30, 23, 30, 0, 0, loc)

	jsonStr, err := marshalCreateInput(CreateReminderInput{Title: "t", DueDate: &due, DueDateAllDay: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		t.Fatal(err)
	}
	if m["dueDateAllDay"] != "2026-09-30" {
		t.Errorf("dueDateAllDay = %v, want 2026-09-30", m["dueDateAllDay"])
	}
	if _, ok := m["dueDate"]; ok {
		t.Error("dueDate should not be sent for an all-day due date")
	}
}

func TestMarshalCreateInputAllDayWithoutDueDate(t *testing.T) {
	jsonStr, err := marshalCreateInput(CreateReminderInput{Title: "t", DueDateAllDay: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(jsonStr, "dueDate") {
		t.Errorf("no due date keys expected, got %s", jsonStr)
	}
}

func TestMarshalUpdateInputAllDay(t *testing.T) {
	due := time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)

	t.Run("all-day", func(t *testing.T) {
		jsonStr, err := marshalUpdateInput(UpdateReminderInput{DueDate: &due, DueDateAllDay: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)
		if m["dueDateAllDay"] != "2026-10-01" {
			t.Errorf("dueDateAllDay = %v, want 2026-10-01", m["dueDateAllDay"])
		}
		if _, ok := m["dueDate"]; ok {
			t.Error("dueDate should not be sent for an all-day due date")
		}
	})

	t.Run("timed", func(t *testing.T) {
		jsonStr, err := marshalUpdateInput(UpdateReminderInput{DueDate: &due})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)
		if _, ok := m["dueDateAllDay"]; ok {
			t.Error("dueDateAllDay should not be sent for a timed due date")
		}
		if m["dueDate"] == nil {
			t.Error("dueDate should be sent")
		}
	})

	t.Run("clear wins", func(t *testing.T) {
		jsonStr, err := marshalUpdateInput(UpdateReminderInput{DueDate: &due, DueDateAllDay: true, ClearDueDate: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var m map[string]any
		json.Unmarshal([]byte(jsonStr), &m)
		if v, ok := m["dueDate"]; !ok || v != nil {
			t.Errorf("dueDate = %v (present=%v), want null", v, ok)
		}
		if _, ok := m["dueDateAllDay"]; ok {
			t.Error("dueDateAllDay should not be sent when clearing")
		}
	})
}

func TestParseAmbiguousID(t *testing.T) {
	err := parseAmbiguousID("4", `[{"id":"4AAA","title":"One","list":"Inbox"},{"id":"4BBB","title":"Two","list":"Work"}]`)

	if !errors.Is(err, ErrAmbiguousID) {
		t.Fatalf("errors.Is(err, ErrAmbiguousID) = false: %v", err)
	}
	var amb *AmbiguousIDError
	if !errors.As(err, &amb) {
		t.Fatalf("errors.As AmbiguousIDError failed: %v", err)
	}
	if amb.Prefix != "4" {
		t.Errorf("Prefix = %q", amb.Prefix)
	}
	if len(amb.Candidates) != 2 || amb.Candidates[0].ID != "4AAA" || amb.Candidates[1].List != "Work" {
		t.Errorf("Candidates = %+v", amb.Candidates)
	}
	if !strings.Contains(err.Error(), "matches 2 reminders") {
		t.Errorf("Error() = %q", err.Error())
	}
}
