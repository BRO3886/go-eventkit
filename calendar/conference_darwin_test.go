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
