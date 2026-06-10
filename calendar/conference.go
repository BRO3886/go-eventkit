package calendar

import (
	"net/url"
	"regexp"
	"strings"
)

// urlCandidatePattern matches http(s) URLs in free text. Angle brackets and
// quotes terminate a match so wrapped links (`<https://...>`, markdown) yield
// the bare URL; trailing punctuation is trimmed afterwards.
var urlCandidatePattern = regexp.MustCompile(`(?i)\bhttps?://[^\s<>"']+`)

// DetectConferenceURL scans the given texts in order and returns the first
// link that belongs to a known video-conference provider (Zoom, Google Meet,
// Microsoft Teams, FaceTime, Webex, Jitsi, Whereby, Amazon Chime,
// GoToMeeting). It returns "" when no conference link is found.
//
// Within a single text the leftmost match wins; across texts the order of the
// arguments is the priority order. Only schemed http(s) links are detected —
// bare domains like "zoom.us/j/123" are intentionally ignored to avoid false
// positives in prose.
//
// This is the pure-Go fallback behind [Event.ConferenceURL]: when the private
// EventKit accessor is unavailable (or returns nothing), the field is
// populated by running this detector over the event's URL, location, and
// notes.
func DetectConferenceURL(texts ...string) string {
	for _, text := range texts {
		if text == "" {
			continue
		}
		for _, candidate := range urlCandidatePattern.FindAllString(text, -1) {
			trimmed := strings.TrimRight(candidate, `.,;:!?)]}>'"`)
			if isConferenceURL(trimmed) {
				return trimmed
			}
		}
	}
	return ""
}

func isConferenceURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	path := strings.ToLower(u.EscapedPath())

	switch {
	case host == "zoom.us" || strings.HasSuffix(host, ".zoom.us") ||
		host == "zoomgov.com" || strings.HasSuffix(host, ".zoomgov.com"):
		// Meeting joins only — the bare homepage or marketing pages don't count.
		return hasNonEmptyPathPrefix(path, "/j/", "/my/", "/s/", "/w/") ||
			strings.HasPrefix(path, "/wc/join/")
	case host == "meet.google.com":
		return strings.Trim(path, "/") != ""
	case host == "teams.microsoft.com":
		return strings.HasPrefix(path, "/l/meetup-join") || strings.HasPrefix(path, "/meet")
	case host == "teams.live.com":
		return strings.HasPrefix(path, "/meet")
	case host == "facetime.apple.com":
		return strings.HasPrefix(path, "/join")
	case host == "webex.com" || strings.HasSuffix(host, ".webex.com"):
		return strings.Contains(path, "/meet/") || strings.Contains(path, "/join/") ||
			strings.Contains(strings.ToLower(u.RawQuery), "mtid=")
	case host == "meet.jit.si" || host == "whereby.com" || host == "chime.aws" ||
		host == "gotomeet.me" || host == "global.gotomeeting.com":
		return strings.Trim(path, "/") != ""
	}
	return false
}

// hasNonEmptyPathPrefix reports whether path starts with one of the prefixes
// and has at least one character after it (so "/j/" alone doesn't match).
func hasNonEmptyPathPrefix(path string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) && len(path) > len(p) {
			return true
		}
	}
	return false
}
