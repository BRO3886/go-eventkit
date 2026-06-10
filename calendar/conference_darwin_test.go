//go:build darwin

package calendar

import "testing"

// TestConferenceSelectorsAvailable is an early-warning canary: it fails when
// a macOS update removes the private EKEvent conference URL accessors. The
// library still works without them (pure-Go detection fallback), but a
// failure here means the richer server-provided conference data is gone and
// the bridge should be re-introspected.
func TestConferenceSelectorsAvailable(t *testing.T) {
	if !conferenceSelectorsAvailable() {
		t.Error("private EKEvent conference URL selectors are gone on this macOS; bridge needs re-introspection (Go fallback detection still works)")
	}
}

// TestSchedulingSelectorsAvailable is a canary for the private scheduling APIs
// (attendees, RSVP, availability, inbox). A failure means a macOS update
// removed the private selectors and the corresponding feature now returns
// ErrUnsupportedFeature — the bridge should be re-introspected.
func TestSchedulingSelectorsAvailable(t *testing.T) {
	c := &Client{}
	checks := map[string]bool{
		"attendee writes": c.AttendeeWritesSupported(),
		"RSVP":            c.RSVPSupported(),
		"availability":    c.AvailabilitySupported(),
	}
	for name, ok := range checks {
		if !ok {
			t.Errorf("private %s selectors are gone on this macOS; bridge needs re-introspection", name)
		}
	}
}
