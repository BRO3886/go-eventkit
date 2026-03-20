// Package dateparser provides natural language date parsing and formatting utilities.
//
// It supports keywords ("today", "tomorrow", "now"), relative expressions ("in 3 hours",
// "5 days ago"), weekdays ("next friday", "monday 2pm"), month-day ("mar 15", "21 march"),
// time-only ("5pm", "17:00"), and standard formats (ISO 8601, RFC 3339, US dates).
//
// Behavior can be customized via [Option] functions for different consumers:
//
//	// Calendar-style: bare dates at midnight (default)
//	dateparser.ParseDate("tomorrow")
//
//	// Reminder-style: bare dates at 9am, past times roll to tomorrow
//	dateparser.ParseDate("tomorrow",
//	    dateparser.WithDefaultHour(9),
//	    dateparser.WithSmartTimeRollover(),
//	    dateparser.WithEOWSkipToday(),
//	)
package dateparser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// config holds parser configuration set by Option functions.
type config struct {
	defaultHour       int
	smartTimeRollover bool
	eowSkipToday      bool
}

// Option configures parsing behavior.
type Option func(*config)

// WithDefaultHour sets the hour used when a bare date has no explicit time.
// For example, "today" with WithDefaultHour(9) resolves to today at 9:00 AM.
// Default is 0 (midnight).
func WithDefaultHour(h int) Option {
	return func(c *config) { c.defaultHour = h }
}

// WithSmartTimeRollover causes time-only inputs (like "9am") to roll forward
// to tomorrow if the time has already passed today.
func WithSmartTimeRollover() Option {
	return func(c *config) { c.smartTimeRollover = true }
}

// WithEOWSkipToday causes "eow"/"end of week" on a Friday to jump to next
// Friday instead of returning the same day at 5pm.
func WithEOWSkipToday() Option {
	return func(c *config) { c.eowSkipToday = true }
}

func buildConfig(opts []Option) config {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	return cfg
}

// ParseDate parses a natural language or formatted date string into a [time.Time]
// using the current wall clock as reference.
func ParseDate(input string, opts ...Option) (time.Time, error) {
	return ParseDateRelativeTo(input, time.Now(), opts...)
}

// ParseDateRelativeTo parses a date string relative to now, allowing deterministic testing.
//
// Supported inputs:
//   - Keywords: "today", "tomorrow", "yesterday", "now"
//   - End-of-period: "eod"/"end of day" (today 5pm), "eow"/"end of week" (Fri 5pm),
//     "this week" (Sun 23:59), "next week" (next Mon), "next month" (1st of next month)
//   - Relative: "in 3 hours", "in 2 weeks", "5 days ago"
//   - Weekdays: "next friday", "monday 2pm", "friday at 3:30pm"
//   - Month-day: "mar 15", "december 31 11:59pm", "21 mar", "21 march 2026"
//   - Time only: "5pm", "17:00", "3:30pm"
//   - Standard formats: ISO 8601, RFC 3339, US date, etc.
func ParseDateRelativeTo(input string, now time.Time, opts ...Option) (time.Time, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return time.Time{}, fmt.Errorf("empty date string")
	}

	cfg := buildConfig(opts)

	// Try standard formats first
	if t, err := tryStandardFormats(input, now.Location()); err == nil {
		return t, nil
	}

	lower := strings.ToLower(input)

	// Handle keywords
	switch lower {
	case "today":
		return todayAt(now, cfg.defaultHour, 0), nil
	case "tomorrow":
		return todayAt(now.AddDate(0, 0, 1), cfg.defaultHour, 0), nil
	case "yesterday":
		return todayAt(now.AddDate(0, 0, -1), cfg.defaultHour, 0), nil
	case "now":
		return now, nil
	case "eod", "end of day":
		return todayAt(now, 17, 0), nil
	case "eow", "end of week":
		daysUntilFriday := (5 - int(now.Weekday()) + 7) % 7
		if daysUntilFriday == 0 {
			if cfg.eowSkipToday {
				return todayAt(now.AddDate(0, 0, 7), 17, 0), nil
			}
			return todayAt(now, 17, 0), nil
		}
		return todayAt(now.AddDate(0, 0, daysUntilFriday), 17, 0), nil
	case "this week":
		daysUntilSunday := (7 - int(now.Weekday())) % 7
		sunday := now.AddDate(0, 0, daysUntilSunday)
		return todayAt(sunday, 23, 59), nil
	case "next week":
		return nextWeekdayAt(now, time.Monday, cfg.defaultHour, 0), nil
	case "next month":
		y, m, _ := now.Date()
		return time.Date(y, m+1, 1, cfg.defaultHour, 0, 0, 0, now.Location()), nil
	}

	// Handle "in X hours/minutes/days/weeks/months"
	if t, err := parseRelative(lower, now); err == nil {
		return t, nil
	}

	// Handle "X hours/days/... ago"
	if t, err := parseAgo(lower, now); err == nil {
		return t, nil
	}

	// Handle "next monday", "next tuesday at 2pm", etc.
	if t, err := parseNextWeekday(lower, now, cfg); err == nil {
		return t, nil
	}

	// Handle "today at 5pm", "tomorrow at 3:30pm", "today 5pm"
	if t, err := parseDateWithTime(lower, now); err == nil {
		return t, nil
	}

	// Handle "<weekday> <time>" e.g., "friday 2pm", "monday 10:00"
	if t, err := parseWeekdayWithTime(lower, now); err == nil {
		return t, nil
	}

	// Handle "<month> <day>" e.g., "mar 15", "march 15", "mar 15 2pm"
	if t, err := parseMonthDay(lower, now, cfg); err == nil {
		return t, nil
	}

	// Handle standalone weekday "monday", "friday"
	if wd, ok := weekdays[lower]; ok {
		return nextWeekdayAt(now, wd, cfg.defaultHour, 0), nil
	}

	// Handle standalone time like "5pm", "17:00", "3:30pm"
	if t, err := parseTimeOnly(lower, now, cfg); err == nil {
		return t, nil
	}

	// Handle "<date> <time>" where date is ISO-like
	if t, err := parseDateTimeParts(lower, now); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("could not parse date: %q. Try: 'today', 'tomorrow 2pm', 'this week', 'eow', 'next friday', 'in 3 hours', or '2026-03-15 14:00'", input)
}

// Standard format list

func tryStandardFormats(input string, loc *time.Location) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04",
		"2006-01-02 3:04PM",
		"2006-01-02 3:04pm",
		"2006-01-02 3PM",
		"2006-01-02 3pm",
		"2006-01-02",
		"01/02/2006",
		"01/02/2006 15:04",
		"01/02/2006 3:04PM",
		"Jan 2, 2006",
		"Jan 2, 2006 3:04PM",
		"Jan 2, 2006 15:04",
		"January 2, 2006",
		"January 2, 2006 3:04PM",
		"January 2, 2006 15:04",
		"2 Jan 2006",
		"02 Jan 2006",
	}

	for _, f := range formats {
		if t, err := time.ParseInLocation(f, input, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("no standard format matched")
}

// Relative expressions

var relativePattern = regexp.MustCompile(`^in\s+(\d+)\s+(minute|minutes|min|mins|hour|hours|hr|hrs|day|days|week|weeks|month|months)$`)

func parseRelative(input string, now time.Time) (time.Time, error) {
	matches := relativePattern.FindStringSubmatch(input)
	if matches == nil {
		return time.Time{}, fmt.Errorf("not a relative date")
	}

	amount, _ := strconv.Atoi(matches[1])
	unit := matches[2]

	switch {
	case strings.HasPrefix(unit, "min"):
		return now.Add(time.Duration(amount) * time.Minute), nil
	case strings.HasPrefix(unit, "hour"), strings.HasPrefix(unit, "hr"):
		return now.Add(time.Duration(amount) * time.Hour), nil
	case strings.HasPrefix(unit, "day"):
		return now.AddDate(0, 0, amount), nil
	case strings.HasPrefix(unit, "week"):
		return now.AddDate(0, 0, amount*7), nil
	case strings.HasPrefix(unit, "month"):
		return now.AddDate(0, amount, 0), nil
	}
	return time.Time{}, fmt.Errorf("unknown unit: %s", unit)
}

// Ago expressions

var agoPattern = regexp.MustCompile(`^(\d+)\s+(minute|minutes|min|mins|hour|hours|hr|hrs|day|days|week|weeks|month|months)\s+ago$`)

func parseAgo(input string, now time.Time) (time.Time, error) {
	matches := agoPattern.FindStringSubmatch(input)
	if matches == nil {
		return time.Time{}, fmt.Errorf("not an ago date")
	}

	amount, _ := strconv.Atoi(matches[1])
	unit := matches[2]

	switch {
	case strings.HasPrefix(unit, "min"):
		return now.Add(-time.Duration(amount) * time.Minute), nil
	case strings.HasPrefix(unit, "hour"), strings.HasPrefix(unit, "hr"):
		return now.Add(-time.Duration(amount) * time.Hour), nil
	case strings.HasPrefix(unit, "day"):
		return now.AddDate(0, 0, -amount), nil
	case strings.HasPrefix(unit, "week"):
		return now.AddDate(0, 0, -amount*7), nil
	case strings.HasPrefix(unit, "month"):
		return now.AddDate(0, -amount, 0), nil
	}
	return time.Time{}, fmt.Errorf("unknown unit: %s", unit)
}

// Weekday maps and helpers

var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "sun": time.Sunday,
	"monday": time.Monday, "mon": time.Monday,
	"tuesday": time.Tuesday, "tue": time.Tuesday,
	"wednesday": time.Wednesday, "wed": time.Wednesday,
	"thursday": time.Thursday, "thu": time.Thursday,
	"friday": time.Friday, "fri": time.Friday,
	"saturday": time.Saturday, "sat": time.Saturday,
}

func parseNextWeekday(input string, now time.Time, cfg config) (time.Time, error) {
	parts := strings.Fields(input)
	if len(parts) < 2 || parts[0] != "next" {
		return time.Time{}, fmt.Errorf("not a next weekday expression")
	}

	dayName := parts[1]
	targetDay, ok := weekdays[dayName]
	if !ok {
		return time.Time{}, fmt.Errorf("unknown weekday: %s", dayName)
	}

	result := nextWeekdayAt(now, targetDay, cfg.defaultHour, 0)

	// Check for "at" time specification: "next monday at 2pm"
	if len(parts) >= 4 && parts[2] == "at" {
		timeStr := strings.Join(parts[3:], " ")
		if hour, min, err := parseTimeStr(timeStr); err == nil {
			result = todayAt(result, hour, min)
		}
	} else if len(parts) >= 3 {
		// "next monday 2pm"
		timeStr := strings.Join(parts[2:], " ")
		if hour, min, err := parseTimeStr(timeStr); err == nil {
			result = todayAt(result, hour, min)
		}
	}

	return result, nil
}

func parseDateWithTime(input string, now time.Time) (time.Time, error) {
	// Try "today at 5pm" / "tomorrow at 3:30pm" first
	if parts := strings.SplitN(input, " at ", 2); len(parts) == 2 {
		datePart := strings.TrimSpace(parts[0])
		timePart := strings.TrimSpace(parts[1])

		baseDate, err := resolveBaseDate(datePart, now)
		if err != nil {
			return time.Time{}, err
		}
		hour, min, err := parseTimeStr(timePart)
		if err != nil {
			return time.Time{}, err
		}
		return todayAt(baseDate, hour, min), nil
	}

	// Try "today 5pm" / "tomorrow 3:30pm"
	parts := strings.Fields(input)
	if len(parts) >= 2 {
		datePart := parts[0]
		timePart := strings.Join(parts[1:], " ")
		baseDate, err := resolveBaseDate(datePart, now)
		if err != nil {
			return time.Time{}, err
		}
		hour, min, err := parseTimeStr(timePart)
		if err != nil {
			return time.Time{}, err
		}
		return todayAt(baseDate, hour, min), nil
	}

	return time.Time{}, fmt.Errorf("not a date with time")
}

func resolveBaseDate(datePart string, now time.Time) (time.Time, error) {
	switch datePart {
	case "today":
		return now, nil
	case "tomorrow":
		return now.AddDate(0, 0, 1), nil
	case "yesterday":
		return now.AddDate(0, 0, -1), nil
	}
	return time.Time{}, fmt.Errorf("unknown date base: %s", datePart)
}

func parseWeekdayWithTime(input string, now time.Time) (time.Time, error) {
	parts := strings.Fields(input)
	if len(parts) < 2 {
		return time.Time{}, fmt.Errorf("not a weekday with time")
	}

	wd, ok := weekdays[parts[0]]
	if !ok {
		return time.Time{}, fmt.Errorf("not a weekday")
	}

	timeStr := strings.Join(parts[1:], " ")
	// Strip optional "at"
	timeStr = strings.TrimPrefix(timeStr, "at ")
	hour, min, err := parseTimeStr(timeStr)
	if err != nil {
		return time.Time{}, err
	}

	return nextWeekdayAt(now, wd, hour, min), nil
}

// Month-day parsing

var months = map[string]time.Month{
	"jan": time.January, "january": time.January,
	"feb": time.February, "february": time.February,
	"mar": time.March, "march": time.March,
	"apr": time.April, "april": time.April,
	"may": time.May,
	"jun": time.June, "june": time.June,
	"jul": time.July, "july": time.July,
	"aug": time.August, "august": time.August,
	"sep": time.September, "september": time.September,
	"oct": time.October, "october": time.October,
	"nov": time.November, "november": time.November,
	"dec": time.December, "december": time.December,
}

// reMonthDay matches month-first: "mar 15", "march 15 2pm", "mar 15 at 2pm"
var reMonthDay = regexp.MustCompile(`^([a-z]+)\s+(\d{1,2})(?:\s+(.+))?$`)

// reDayMonth matches day-first: "21 mar", "21 march 2026", "21 mar 2pm", "21 mar 2026 2pm"
var reDayMonth = regexp.MustCompile(`^(\d{1,2})\s+([a-z]+)(?:\s+(.+))?$`)

func parseMonthDay(input string, now time.Time, cfg config) (time.Time, error) {
	// Try month-first: "mar 15", "march 15 2pm"
	if m := reMonthDay.FindStringSubmatch(input); m != nil {
		if mon, ok := months[m[1]]; ok {
			day, _ := strconv.Atoi(m[2])
			return buildMonthDayResult(mon, day, m[3], now, cfg)
		}
	}

	// Try day-first: "21 mar", "21 march 2026", "21 mar 2pm"
	if m := reDayMonth.FindStringSubmatch(input); m != nil {
		day, _ := strconv.Atoi(m[1])
		if mon, ok := months[m[2]]; ok {
			return buildMonthDayResult(mon, day, m[3], now, cfg)
		}
	}

	return time.Time{}, fmt.Errorf("not a month-day expression")
}

func buildMonthDayResult(mon time.Month, day int, rest string, now time.Time, cfg config) (time.Time, error) {
	if day < 1 || day > 31 {
		return time.Time{}, fmt.Errorf("invalid day: %d", day)
	}

	y := now.Year()
	rest = strings.TrimSpace(rest)

	// Check if rest starts with a year: "2026", "2026 2pm"
	if rest != "" {
		parts := strings.Fields(rest)
		if yr, err := strconv.Atoi(parts[0]); err == nil && yr >= 1000 && yr <= 9999 {
			y = yr
			rest = strings.TrimSpace(strings.Join(parts[1:], " "))
		}
	}

	hour := cfg.defaultHour
	result := time.Date(y, mon, day, hour, 0, 0, 0, now.Location())

	// Catch overflow: time.Date normalizes e.g. Feb 31 -> Mar 3
	if result.Month() != mon || result.Day() != day {
		return time.Time{}, fmt.Errorf("invalid date: %s %d", mon, day)
	}

	// Parse optional time component
	if rest != "" {
		timeStr := strings.TrimPrefix(rest, "at ")
		if h, min, err := parseTimeStr(timeStr); err == nil {
			result = time.Date(y, mon, day, h, min, 0, 0, now.Location())
		}
	}

	return result, nil
}

func parseTimeOnly(input string, now time.Time, cfg config) (time.Time, error) {
	hour, min, err := parseTimeStr(input)
	if err != nil {
		return time.Time{}, err
	}
	result := todayAt(now, hour, min)
	if cfg.smartTimeRollover && result.Before(now) {
		result = result.AddDate(0, 0, 1)
	}
	return result, nil
}

func parseDateTimeParts(input string, now time.Time) (time.Time, error) {
	parts := strings.Fields(input)
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("not a date-time pair")
	}

	d, err := tryStandardFormats(parts[0], now.Location())
	if err != nil {
		return time.Time{}, err
	}

	hour, min, err := parseTimeStr(parts[1])
	if err != nil {
		return time.Time{}, err
	}

	return todayAt(d, hour, min), nil
}

// Time string parsing

var timePatterns = []struct {
	re     *regexp.Regexp
	parser func([]string) (int, int, error)
}{
	{
		re: regexp.MustCompile(`^(\d{1,2})\s*(am|pm)$`),
		parser: func(m []string) (int, int, error) {
			h, _ := strconv.Atoi(m[1])
			return convertTo24(h, 0, m[2])
		},
	},
	{
		re: regexp.MustCompile(`^(\d{1,2}):(\d{2})\s*(am|pm)$`),
		parser: func(m []string) (int, int, error) {
			h, _ := strconv.Atoi(m[1])
			min, _ := strconv.Atoi(m[2])
			return convertTo24(h, min, m[3])
		},
	},
	{
		re: regexp.MustCompile(`^(\d{1,2}):(\d{2})$`),
		parser: func(m []string) (int, int, error) {
			h, _ := strconv.Atoi(m[1])
			min, _ := strconv.Atoi(m[2])
			if h > 23 || min > 59 {
				return 0, 0, fmt.Errorf("invalid time")
			}
			return h, min, nil
		},
	},
}

func parseTimeStr(s string) (int, int, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	for _, p := range timePatterns {
		matches := p.re.FindStringSubmatch(s)
		if matches != nil {
			return p.parser(matches)
		}
	}
	return 0, 0, fmt.Errorf("unable to parse time: %q", s)
}

func convertTo24(hour, min int, period string) (int, int, error) {
	if hour < 1 || hour > 12 {
		return 0, 0, fmt.Errorf("invalid hour: %d", hour)
	}
	if period == "am" {
		if hour == 12 {
			hour = 0
		}
	} else {
		if hour != 12 {
			hour += 12
		}
	}
	return hour, min, nil
}

func todayAt(base time.Time, hour, min int) time.Time {
	return time.Date(base.Year(), base.Month(), base.Day(), hour, min, 0, 0, base.Location())
}

func nextWeekdayAt(now time.Time, target time.Weekday, hour, min int) time.Time {
	daysAhead := int(target) - int(now.Weekday())
	if daysAhead <= 0 {
		daysAhead += 7
	}
	d := now.AddDate(0, 0, daysAhead)
	return todayAt(d, hour, min)
}
