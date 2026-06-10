package calendar

import (
	"strings"
	"testing"
)

func TestDetectConferenceURL_Providers(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"zoom join", "Join: https://zoom.us/j/123456789", "https://zoom.us/j/123456789"},
		{"zoom subdomain", "https://us02web.zoom.us/j/123456789?pwd=abcDEF123", "https://us02web.zoom.us/j/123456789?pwd=abcDEF123"},
		{"zoom personal", "https://zoom.us/my/sidd", "https://zoom.us/my/sidd"},
		{"zoom sso", "https://company.zoom.us/s/987654", "https://company.zoom.us/s/987654"},
		{"zoom webclient", "https://zoom.us/wc/join/123456789", "https://zoom.us/wc/join/123456789"},
		{"zoomgov", "https://zoomgov.com/j/1", "https://zoomgov.com/j/1"},
		{"google meet", "https://meet.google.com/abc-defg-hij", "https://meet.google.com/abc-defg-hij"},
		{"google meet lookup", "https://meet.google.com/lookup/team-sync", "https://meet.google.com/lookup/team-sync"},
		{"teams meetup-join", "https://teams.microsoft.com/l/meetup-join/19%3ameeting_abc%40thread.v2/0?context=%7b%7d", "https://teams.microsoft.com/l/meetup-join/19%3ameeting_abc%40thread.v2/0?context=%7b%7d"},
		{"teams live", "https://teams.live.com/meet/9876543210", "https://teams.live.com/meet/9876543210"},
		{"facetime", "https://facetime.apple.com/join#v=1&p=abc&k=xyz", "https://facetime.apple.com/join#v=1&p=abc&k=xyz"},
		{"webex personal room", "https://company.webex.com/meet/alice", "https://company.webex.com/meet/alice"},
		{"webex join", "https://company.webex.com/join/bob", "https://company.webex.com/join/bob"},
		{"webex mtid", "https://company.webex.com/company/j.php?MTID=m1234abcd", "https://company.webex.com/company/j.php?MTID=m1234abcd"},
		{"jitsi", "https://meet.jit.si/StandupRoom", "https://meet.jit.si/StandupRoom"},
		{"whereby", "https://whereby.com/sidd-room", "https://whereby.com/sidd-room"},
		{"chime", "https://chime.aws/1234567890", "https://chime.aws/1234567890"},
		{"gotomeet", "https://gotomeet.me/sidd", "https://gotomeet.me/sidd"},
		{"gotomeeting", "https://global.gotomeeting.com/join/123456789", "https://global.gotomeeting.com/join/123456789"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectConferenceURL(tt.text); got != tt.want {
				t.Errorf("DetectConferenceURL(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestDetectConferenceURL_NonMatches(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"empty", ""},
		{"plain text", "discuss the zoom migration plan"},
		{"non-conference url", "see https://example.com/docs for details"},
		{"zoom homepage", "https://zoom.us"},
		{"zoom homepage slash", "https://zoom.us/"},
		{"zoom pricing page", "https://zoom.us/pricing"},
		{"zoom join prefix but empty id", "https://zoom.us/j/"},
		{"meet homepage", "https://meet.google.com/"},
		{"schemeless zoom (by design)", "join at zoom.us/j/123456789"},
		{"email at zoom domain", "mail j@zoom.us about it"},
		{"zoom lookalike domain", "https://zoom.us.evil.com/j/123"},
		{"meet lookalike subdomain", "https://meet.google.com.phish.io/abc"},
		{"teams non-meeting path", "https://teams.microsoft.com/downloads"},
		{"facetime homepage", "https://facetime.apple.com/"},
		{"webex marketing", "https://www.webex.com/pricing.html"},
		{"ftp scheme", "ftp://zoom.us/j/123"},
		{"whereby homepage", "https://whereby.com/"},
		{"url split across lines", "https://zoom.us\n/j/123456789"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectConferenceURL(tt.text); got != "" {
				t.Errorf("DetectConferenceURL(%q) = %q, want \"\"", tt.text, got)
			}
		})
	}
}

func TestDetectConferenceURL_Trimming(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"trailing period", "Join https://zoom.us/j/123.", "https://zoom.us/j/123"},
		{"trailing comma", "https://zoom.us/j/123, then lunch", "https://zoom.us/j/123"},
		{"wrapped in parens", "(https://meet.google.com/abc-defg-hij)", "https://meet.google.com/abc-defg-hij"},
		{"wrapped in angle brackets", "<https://zoom.us/j/123>", "https://zoom.us/j/123"},
		{"markdown link", "[join](https://zoom.us/j/123)", "https://zoom.us/j/123"},
		{"trailing semicolon and quote", `"https://whereby.com/room";`, "https://whereby.com/room"},
		{"trailing question mark", "https://meet.google.com/abc-defg-hij?", "https://meet.google.com/abc-defg-hij"},
		{"trailing exclaim", "Join now https://meet.jit.si/Room!", "https://meet.jit.si/Room"},
		{"query survives trimming", "link: https://us02web.zoom.us/j/9?pwd=x_Y-z.", "https://us02web.zoom.us/j/9?pwd=x_Y-z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectConferenceURL(tt.text); got != tt.want {
				t.Errorf("DetectConferenceURL(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestDetectConferenceURL_CaseInsensitivity(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"uppercase scheme", "HTTPS://ZOOM.US/J/123456"},
		{"mixed case host", "https://Meet.Google.Com/abc-defg-hij"},
		{"uppercase path", "https://zoom.us/J/123456"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectConferenceURL(tt.text); got == "" {
				t.Errorf("DetectConferenceURL(%q) = \"\", want a match", tt.text)
			}
		})
	}
}

func TestDetectConferenceURL_PriorityAndMultiples(t *testing.T) {
	t.Run("first text wins over later texts", func(t *testing.T) {
		got := DetectConferenceURL(
			"https://zoom.us/j/111",
			"https://meet.google.com/abc-defg-hij",
		)
		if got != "https://zoom.us/j/111" {
			t.Errorf("got %q, want the first argument's link", got)
		}
	})
	t.Run("empty texts are skipped", func(t *testing.T) {
		got := DetectConferenceURL("", "", "https://meet.google.com/abc-defg-hij")
		if got != "https://meet.google.com/abc-defg-hij" {
			t.Errorf("got %q, want link from third argument", got)
		}
	})
	t.Run("leftmost match wins within a text", func(t *testing.T) {
		got := DetectConferenceURL("a https://whereby.com/x then https://zoom.us/j/1")
		if got != "https://whereby.com/x" {
			t.Errorf("got %q, want leftmost link", got)
		}
	})
	t.Run("non-conference link before conference link", func(t *testing.T) {
		got := DetectConferenceURL("agenda: https://example.com/doc join: https://zoom.us/j/1")
		if got != "https://zoom.us/j/1" {
			t.Errorf("got %q, want the conference link, not the doc link", got)
		}
	})
	t.Run("no args", func(t *testing.T) {
		if got := DetectConferenceURL(); got != "" {
			t.Errorf("got %q, want \"\"", got)
		}
	})
}

func TestDetectConferenceURL_ExtremeInputs(t *testing.T) {
	t.Run("unicode and emoji around link", func(t *testing.T) {
		got := DetectConferenceURL("会議 📅 → https://meet.google.com/abc-defg-hij ← 参加")
		if got != "https://meet.google.com/abc-defg-hij" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("CRLF line endings", func(t *testing.T) {
		got := DetectConferenceURL("agenda\r\nhttps://zoom.us/j/123\r\nnotes")
		if got != "https://zoom.us/j/123" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("link at end of huge notes", func(t *testing.T) {
		huge := strings.Repeat("lorem ipsum dolor sit amet ", 40000) // ~1MB
		got := DetectConferenceURL(huge + "https://zoom.us/j/42")
		if got != "https://zoom.us/j/42" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("huge notes with no link", func(t *testing.T) {
		huge := strings.Repeat("https://example.com/x ", 50000)
		if got := DetectConferenceURL(huge); got != "" {
			t.Errorf("got %q, want \"\"", got)
		}
	})
	t.Run("null bytes and control chars", func(t *testing.T) {
		got := DetectConferenceURL("x\x00y\x07 https://zoom.us/j/123 z")
		if got != "https://zoom.us/j/123" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("invalid percent encoding does not panic", func(t *testing.T) {
		if got := DetectConferenceURL("https://zoom.us/j/%zz%"); got != "" {
			t.Errorf("got %q, want \"\" for unparseable URL", got)
		}
	})
	t.Run("host with port", func(t *testing.T) {
		got := DetectConferenceURL("https://zoom.us:443/j/123")
		if got != "https://zoom.us:443/j/123" {
			t.Errorf("got %q, want match with explicit port", got)
		}
	})
	t.Run("userinfo does not fool host check", func(t *testing.T) {
		if got := DetectConferenceURL("https://zoom.us@evil.com/j/123"); got != "" {
			t.Errorf("got %q, want \"\" — host is evil.com", got)
		}
	})
}

func TestConvertRawEvent_ConferenceURL(t *testing.T) {
	bridgeURL := "https://zoom.us/j/from-bridge"
	empty := ""
	notes := "join at https://meet.google.com/abc-defg-hij"

	t.Run("bridge value wins", func(t *testing.T) {
		e := convertRawEvent(rawEvent{ConferenceURL: &bridgeURL, Notes: &notes})
		if e.ConferenceURL != bridgeURL {
			t.Errorf("got %q, want bridge value %q", e.ConferenceURL, bridgeURL)
		}
	})
	t.Run("nil bridge value falls back to detection", func(t *testing.T) {
		e := convertRawEvent(rawEvent{Notes: &notes})
		if e.ConferenceURL != "https://meet.google.com/abc-defg-hij" {
			t.Errorf("got %q, want detected link from notes", e.ConferenceURL)
		}
	})
	t.Run("empty bridge value falls back to detection", func(t *testing.T) {
		e := convertRawEvent(rawEvent{ConferenceURL: &empty, Notes: &notes})
		if e.ConferenceURL != "https://meet.google.com/abc-defg-hij" {
			t.Errorf("got %q, want detected link from notes", e.ConferenceURL)
		}
	})
	t.Run("url field beats location and notes in fallback", func(t *testing.T) {
		u := "https://zoom.us/j/from-url-field"
		loc := "https://whereby.com/from-location"
		e := convertRawEvent(rawEvent{URL: &u, Location: &loc, Notes: &notes})
		if e.ConferenceURL != u {
			t.Errorf("got %q, want URL-field link %q", e.ConferenceURL, u)
		}
	})
	t.Run("no link anywhere", func(t *testing.T) {
		plain := "quarterly planning"
		e := convertRawEvent(rawEvent{Notes: &plain})
		if e.ConferenceURL != "" {
			t.Errorf("got %q, want \"\"", e.ConferenceURL)
		}
	})
}

func TestParseEventJSON_ConferenceURL(t *testing.T) {
	t.Run("bridge field present", func(t *testing.T) {
		e, err := parseEventJSON(`{"id":"x","title":"standup","conferenceURL":"https://zoom.us/j/9"}`)
		if err != nil {
			t.Fatal(err)
		}
		if e.ConferenceURL != "https://zoom.us/j/9" {
			t.Errorf("got %q", e.ConferenceURL)
		}
	})
	t.Run("bridge field null with detectable notes", func(t *testing.T) {
		e, err := parseEventJSON(`{"id":"x","title":"standup","conferenceURL":null,"notes":"https://meet.google.com/abc-defg-hij"}`)
		if err != nil {
			t.Fatal(err)
		}
		if e.ConferenceURL != "https://meet.google.com/abc-defg-hij" {
			t.Errorf("got %q", e.ConferenceURL)
		}
	})
	t.Run("legacy bridge JSON without the key", func(t *testing.T) {
		e, err := parseEventJSON(`{"id":"x","title":"standup"}`)
		if err != nil {
			t.Fatal(err)
		}
		if e.ConferenceURL != "" {
			t.Errorf("got %q, want \"\"", e.ConferenceURL)
		}
	})
}
