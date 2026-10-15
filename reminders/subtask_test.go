package reminders

import (
	"encoding/json"
	"testing"
)

func TestParseReminderJSONParentID(t *testing.T) {
	r, err := parseReminderJSON(`{"id":"CHILD","title":"t","parentID":"PARENT"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ParentID != "PARENT" {
		t.Errorf("ParentID = %q, want PARENT", r.ParentID)
	}

	r, err = parseReminderJSON(`{"id":"TOP","title":"t"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ParentID != "" {
		t.Errorf("ParentID = %q, want empty", r.ParentID)
	}
}

func TestMarshalUpdateInputParentID(t *testing.T) {
	parent := "PARENT"
	none := ""
	tests := []struct {
		name    string
		input   UpdateReminderInput
		present bool
		want    any
	}{
		{"unchanged", UpdateReminderInput{}, false, nil},
		{"set parent", UpdateReminderInput{ParentID: &parent}, true, "PARENT"},
		{"outdent", UpdateReminderInput{ParentID: &none}, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonStr, err := marshalUpdateInput(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			var m map[string]any
			if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
				t.Fatal(err)
			}
			got, ok := m["parentID"]
			if ok != tt.present {
				t.Fatalf("parentID present = %v, want %v (%s)", ok, tt.present, jsonStr)
			}
			if got != tt.want {
				t.Errorf("parentID = %#v, want %#v", got, tt.want)
			}
		})
	}
}
