//go:build darwin && integration

package calendar

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func identifierTestClient(t *testing.T) (*Client, string) {
	t.Helper()
	c, err := New()
	if err != nil {
		t.Fatalf("opening Calendar store: %v", err)
	}
	cals, err := c.Calendars()
	if err != nil {
		t.Fatalf("listing calendars: %v", err)
	}
	source := os.Getenv("EVENTKIT_TEST_SOURCE")
	if source == "" {
		t.Fatal("set EVENTKIT_TEST_SOURCE to a source that supports temporary calendars")
	}
	for _, cal := range cals {
		if !cal.ReadOnly && cal.Source == source {
			return c, cal.Source
		}
	}
	t.Fatal("configured source has no writable calendar")
	return nil, ""
}

func TestEventIdentifierPreflight(t *testing.T) {
	identifierTestClient(t)
}

func TestEventExactRead(t *testing.T) {
	c, _ := identifierTestClient(t)
	now := time.Now()
	events, err := c.Events(now.AddDate(-1, 0, 0), now.AddDate(1, 0, 0))
	if err != nil {
		t.Fatalf("listing events: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("no events available for read-only lookup verification")
	}
	id := events[0].ID
	got, err := c.Event(id)
	if err != nil || got == nil || got.ID != id {
		t.Fatalf("full ID lookup did not round-trip: event=%v error=%v", got != nil, err)
	}
	if len(id) <= 13 {
		t.Fatal("event ID is too short to reproduce truncated lookup")
	}
	for _, input := range []string{"", id[:13], id[:len(id)-1], id + "-stale", id + "\x00suffix"} {
		got, err := c.Event(input)
		if got != nil || !errors.Is(err, ErrNotFound) {
			t.Errorf("nonexact lookup returned event=%v error=%v; want nil and ErrNotFound", got != nil, err)
		}
	}
}

func TestEventExactMutations(t *testing.T) {
	c, source := identifierTestClient(t)
	cal, err := c.CreateCalendar(CreateCalendarInput{
		Title:  fmt.Sprintf("go-eventkit identifier test %d", time.Now().UnixNano()),
		Source: source,
	})
	if err != nil {
		t.Fatalf("creating isolated test calendar: %v", err)
	}
	t.Cleanup(func() {
		if err := c.DeleteCalendar(cal.ID); err != nil {
			t.Errorf("removing test calendar: %v", err)
			return
		}
		cals, err := c.Calendars()
		if err != nil {
			t.Errorf("verifying calendar cleanup: %v", err)
			return
		}
		for _, remaining := range cals {
			if remaining.ID == cal.ID {
				t.Error("test calendar remains after cleanup")
			}
		}
	})
	start := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	event, err := c.CreateEvent(CreateEventInput{
		Title: "Identifier safety fixture", Calendar: cal.Title,
		StartDate: start, EndDate: start.Add(30 * time.Minute),
		SuppressDefaultAlarms: true,
	})
	if err != nil {
		t.Fatalf("creating test event: %v", err)
	}
	title := "Unexpected mutation"
	for _, input := range []string{"", event.ID[:13], event.ID[:len(event.ID)-1], event.ID + "-stale", event.ID + "\x00suffix"} {
		updated, err := c.UpdateEvent(input, UpdateEventInput{Title: &title}, SpanThisEvent)
		if updated != nil || !errors.Is(err, ErrNotFound) {
			t.Errorf("nonexact update returned event=%v error=%v", updated != nil, err)
		}
		if err := c.DeleteEvent(input, SpanThisEvent); !errors.Is(err, ErrNotFound) {
			t.Errorf("nonexact delete error=%v; want ErrNotFound", err)
		}
		if errs := c.DeleteEvents([]string{input}, SpanThisEvent); len(errs) != 0 {
			t.Errorf("batch delete must skip nonexistent IDs: %v", errs)
		}
		if c.RSVPSupported() {
			if err := c.RespondToInvitation(input, ParticipantStatusAccepted); err == nil || !strings.Contains(err.Error(), "not found") {
				t.Errorf("nonexact RSVP error=%v; want not found", err)
			}
		}
		got, err := c.Event(event.ID)
		if err != nil || got == nil || got.Title != event.Title {
			t.Fatalf("fixture changed after nonexact mutation: event=%v error=%v", got != nil, err)
		}
	}
	title = "Updated fixture"
	updated, err := c.UpdateEvent(event.ID, UpdateEventInput{Title: &title}, SpanThisEvent)
	if err != nil || updated == nil || updated.Title != title {
		t.Fatalf("exact update failed: event=%v error=%v", updated != nil, err)
	}
	if err := c.DeleteEvent(event.ID, SpanThisEvent); err != nil {
		t.Fatalf("exact delete failed: %v", err)
	}
	if got, err := c.Event(event.ID); got != nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted ID returned event=%v error=%v", got != nil, err)
	}
	if errs := c.DeleteEvents([]string{event.ID}, SpanThisEvent); len(errs) != 0 {
		t.Errorf("batch delete must skip stale full IDs: %v", errs)
	}
	batchEvent, err := c.CreateEvent(CreateEventInput{
		Title: "Batch fixture", Calendar: cal.Title,
		StartDate: start, EndDate: start.Add(30 * time.Minute),
		SuppressDefaultAlarms: true,
	})
	if err != nil {
		t.Fatalf("creating batch fixture: %v", err)
	}
	if errs := c.DeleteEvents([]string{batchEvent.ID[:13], batchEvent.ID}, SpanThisEvent); len(errs) != 0 {
		t.Fatalf("exact batch delete failed: %v", errs)
	}
	if got, err := c.Event(batchEvent.ID); got != nil || !errors.Is(err, ErrNotFound) {
		t.Errorf("batch-deleted ID returned event=%v error=%v", got != nil, err)
	}
}
