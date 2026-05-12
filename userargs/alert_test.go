package userargs

import (
	"strings"
	"testing"
	"time"
)

func TestParseAlertOffsets_ValidInputs(t *testing.T) {
	got, err := ParseAlertOffsets([]string{"15m", "1h", "1d"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 durations, got %d", len(got))
	}
	want := []time.Duration{15 * time.Minute, 1 * time.Hour, 24 * time.Hour}
	for i, d := range got {
		if d != want[i] {
			t.Errorf("offset[%d]: got %v, want %v", i, d, want[i])
		}
	}
}

func TestParseAlertOffsets_EmptySlice(t *testing.T) {
	got, err := ParseAlertOffsets([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestParseAlertOffsets_InvalidInput(t *testing.T) {
	_, err := ParseAlertOffsets([]string{"abc"})
	if err == nil {
		t.Fatal("expected error for invalid alert offset")
	}
	if !strings.Contains(err.Error(), "invalid alert offset") {
		t.Errorf("error should contain 'invalid alert offset', got: %v", err)
	}
}

func TestParseAlertOffsets_ZeroDuration(t *testing.T) {
	// ParseAlertDuration("0m") returns 0, which should fail the positivity check.
	_, err := ParseAlertOffsets([]string{"0m"})
	if err == nil {
		t.Fatal("expected error for zero duration")
	}
	if !strings.Contains(err.Error(), "must be positive") {
		t.Errorf("error should contain 'must be positive', got: %v", err)
	}
}

func TestParseAlertOffsets_ExceedsOneYear(t *testing.T) {
	_, err := ParseAlertOffsets([]string{"999d"})
	if err == nil {
		t.Fatal("expected error for offset exceeding one year")
	}
	if !strings.Contains(err.Error(), "exceeds one year") {
		t.Errorf("error should contain 'exceeds one year', got: %v", err)
	}
}

func TestParseAlertOffsets_OneDayExact24h(t *testing.T) {
	// Regression: time.ParseDuration rejects "d" suffix; our parser must handle it.
	got, err := ParseAlertOffsets([]string{"1d"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != 24*time.Hour {
		t.Errorf("expected 24h, got %v", got)
	}
}

func TestParseAlertOffsets_Whitespace(t *testing.T) {
	got, err := ParseAlertOffsets([]string{" 15m "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != 15*time.Minute {
		t.Errorf("expected 15m, got %v", got)
	}
}
