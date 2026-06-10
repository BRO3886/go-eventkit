package calendar

import (
	"testing"
	"time"
)

func TestMarshalCreateInput_Attendees(t *testing.T) {
	t.Run("attendees serialized with name and email", func(t *testing.T) {
		in := CreateEventInput{
			Title:     "review",
			StartDate: time.Now(),
			EndDate:   time.Now().Add(time.Hour),
			Attendees: []AttendeeInput{
				{Email: "a@x.com", Name: "Alice"},
				{Email: "b@y.com"},
			},
		}
		b, err := marshalCreateInput(in)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		if !contains(s, `"email":"a@x.com"`) || !contains(s, `"name":"Alice"`) {
			t.Errorf("missing attendee fields: %s", s)
		}
		if !contains(s, `"email":"b@y.com"`) {
			t.Errorf("missing second attendee: %s", s)
		}
	})
	t.Run("no attendees key when empty", func(t *testing.T) {
		in := CreateEventInput{Title: "x", StartDate: time.Now(), EndDate: time.Now()}
		b, _ := marshalCreateInput(in)
		if contains(string(b), "attendees") {
			t.Errorf("attendees key should be omitted: %s", b)
		}
	})
	t.Run("travel time serialized as seconds", func(t *testing.T) {
		in := CreateEventInput{Title: "x", StartDate: time.Now(), EndDate: time.Now(), TravelTime: 30 * time.Minute}
		b, _ := marshalCreateInput(in)
		if !contains(string(b), `"travelTime":1800`) {
			t.Errorf("expected travelTime 1800: %s", b)
		}
	})
}

func TestMarshalUpdateInput_Scheduling(t *testing.T) {
	t.Run("attendees included when set", func(t *testing.T) {
		in := UpdateEventInput{Attendees: []AttendeeInput{{Email: "a@x.com"}}}
		b, _ := marshalUpdateInput(in)
		if !contains(string(b), `"email":"a@x.com"`) {
			t.Errorf("missing attendee: %s", b)
		}
	})
	t.Run("travel time zero is still sent when pointer set", func(t *testing.T) {
		zero := time.Duration(0)
		in := UpdateEventInput{TravelTime: &zero}
		b, _ := marshalUpdateInput(in)
		if !contains(string(b), `"travelTime":0`) {
			t.Errorf("expected travelTime:0 to clear travel: %s", b)
		}
	})
	t.Run("travel time omitted when nil", func(t *testing.T) {
		in := UpdateEventInput{}
		b, _ := marshalUpdateInput(in)
		if contains(string(b), "travelTime") {
			t.Errorf("travelTime should be omitted when nil: %s", b)
		}
	})
}

func TestConvertRawEvent_SchedulingFields(t *testing.T) {
	t.Run("travel time seconds to duration", func(t *testing.T) {
		e := convertRawEvent(rawEvent{TravelTime: 900})
		if e.TravelTime != 15*time.Minute {
			t.Errorf("got %v, want 15m", e.TravelTime)
		}
	})
	t.Run("zero travel time", func(t *testing.T) {
		e := convertRawEvent(rawEvent{TravelTime: 0})
		if e.TravelTime != 0 {
			t.Errorf("got %v, want 0", e.TravelTime)
		}
	})
	t.Run("self status mapped", func(t *testing.T) {
		e := convertRawEvent(rawEvent{SelfStatus: int(ParticipantStatusAccepted)})
		if e.SelfStatus != ParticipantStatusAccepted {
			t.Errorf("got %v, want accepted", e.SelfStatus)
		}
	})
}

func TestParseAvailabilityJSON(t *testing.T) {
	t.Run("spans parsed per address", func(t *testing.T) {
		js := `{"a@x.com":[{"startDate":"2026-06-11T10:00:00.000Z","endDate":"2026-06-11T11:00:00.000Z","type":1}],"b@y.com":[]}`
		got, err := parseAvailabilityJSON(js)
		if err != nil {
			t.Fatal(err)
		}
		if len(got["a@x.com"]) != 1 {
			t.Fatalf("want 1 span for a, got %d", len(got["a@x.com"]))
		}
		span := got["a@x.com"][0]
		if span.Type != AvailabilityTypeBusy {
			t.Errorf("want busy, got %v", span.Type)
		}
		if span.Start.IsZero() || span.End.IsZero() {
			t.Errorf("dates not parsed: %+v", span)
		}
		if got["b@y.com"] == nil || len(got["b@y.com"]) != 0 {
			t.Errorf("want empty slice for b, got %v", got["b@y.com"])
		}
	})
	t.Run("empty object", func(t *testing.T) {
		got, err := parseAvailabilityJSON(`{}`)
		if err != nil || len(got) != 0 {
			t.Errorf("want empty map, got %v err %v", got, err)
		}
	})
	t.Run("malformed JSON errors", func(t *testing.T) {
		if _, err := parseAvailabilityJSON(`{bad`); err == nil {
			t.Error("expected error on malformed JSON")
		}
	})
	t.Run("missing dates yield zero times not panic", func(t *testing.T) {
		got, err := parseAvailabilityJSON(`{"a@x.com":[{"type":0}]}`)
		if err != nil {
			t.Fatal(err)
		}
		if !got["a@x.com"][0].Start.IsZero() {
			t.Error("want zero start")
		}
	})
}

func TestParseInvitationsJSON(t *testing.T) {
	t.Run("full invitation", func(t *testing.T) {
		js := `[{"title":"Sync","startDate":"2026-06-11T10:00:00.000Z","endDate":"2026-06-11T11:00:00.000Z","location":"Room 1","organizer":"boss@x.com","status":1,"allDay":false}]`
		got, err := parseInvitationsJSON(js)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("want 1, got %d", len(got))
		}
		inv := got[0]
		if inv.Title != "Sync" || inv.Location != "Room 1" || inv.Organizer != "boss@x.com" {
			t.Errorf("fields wrong: %+v", inv)
		}
		if inv.Status != ParticipantStatusPending {
			t.Errorf("want pending, got %v", inv.Status)
		}
	})
	t.Run("empty array", func(t *testing.T) {
		got, err := parseInvitationsJSON(`[]`)
		if err != nil || len(got) != 0 {
			t.Errorf("want empty, got %v err %v", got, err)
		}
	})
	t.Run("null optional fields", func(t *testing.T) {
		js := `[{"title":"x","location":null,"organizer":null,"status":0,"allDay":true}]`
		got, err := parseInvitationsJSON(js)
		if err != nil {
			t.Fatal(err)
		}
		if got[0].Location != "" || got[0].Organizer != "" || !got[0].AllDay {
			t.Errorf("null handling wrong: %+v", got[0])
		}
	})
}

func TestAvailabilityType_String(t *testing.T) {
	cases := map[AvailabilityType]string{
		AvailabilityTypeFree:        "free",
		AvailabilityTypeBusy:        "busy",
		AvailabilityTypeTentative:   "tentative",
		AvailabilityTypeUnavailable: "unavailable",
		AvailabilityType(99):        "unknown",
	}
	for typ, want := range cases {
		if got := typ.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", typ, got, want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
