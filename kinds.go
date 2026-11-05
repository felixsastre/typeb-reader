package typebreader

import (
	"fmt"
	"strings"
	"time"
)

// validators check the text of a group by kind. They are the kinds a grammar can name.
var validators = map[string]func(string) error{
	"date":         valid(parseDate),
	"time":         valid(parseTimeOfDay),
	"weekdays":     valid(parseWeekdays),
	"day_of_month": valid(parseDayOfMonth),
	"moment":       valid(parseMoment),
	"duration":     valid(parseDurationMinutes),
}

// valid turns a parser into a validator: the text must be one the parser reads.
func valid[T any](parse func(string) (T, error)) func(string) error {
	return func(s string) error {
		_, err := parse(s)
		return err
	}
}

// TimeOfDay is an hour and a minute, with no date and no zone.
type TimeOfDay struct{ Hour, Minute int }

func (t TimeOfDay) String() string { return fmt.Sprintf("%02d:%02d", t.Hour, t.Minute) }

// Moment is a time of day, with the day of the month when the message wrote it (ddhhmm).
type Moment struct {
	Day  int // 0 when the message gave only the time
	Time TimeOfDay
}

// String shows "08:05", or "08:05 on day 21" when the day was written, and "none" for a time
// the message did not give.
func (m *Moment) String() string {
	switch {
	case m == nil:
		return "none"
	case m.Day == 0:
		return m.Time.String()
	default:
		return fmt.Sprintf("%s on day %d", m.Time, m.Day)
	}
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// twoDigits is the number written by the first two characters of s, which must be digits.
func twoDigits(s string) int { return int(s[0]-'0')*10 + int(s[1]-'0') }

// parseDate reads a date written ddMMMyy, such as 05NOV26. Two digits of year 00 to 68 are 20xx.
func parseDate(s string) (time.Time, error) {
	if len(s) != 7 {
		return time.Time{}, fmt.Errorf("%q is not a date written ddMMMyy", s)
	}
	titled := s[:3] + strings.ToLower(s[3:5]) + s[5:]
	date, err := time.Parse("02Jan06", titled)
	if err != nil {
		return time.Time{}, fmt.Errorf("%q is not a date: %w", s, err)
	}
	return date, nil
}

// parseTimeOfDay reads a time written hhmm.
func parseTimeOfDay(s string) (TimeOfDay, error) {
	if len(s) != 4 || !allDigits(s) {
		return TimeOfDay{}, fmt.Errorf("%q is not a time written hhmm", s)
	}
	t := TimeOfDay{Hour: twoDigits(s), Minute: twoDigits(s[2:])}
	if t.Hour > 23 || t.Minute > 59 {
		return TimeOfDay{}, fmt.Errorf("%q is not a time of day", s)
	}
	return t, nil
}

// parseWeekdays reads days of operation: digits 1 (Monday) to 7 (Sunday), ascending, each once.
func parseWeekdays(s string) ([]time.Weekday, error) {
	if s == "" {
		return nil, fmt.Errorf("no days of operation")
	}
	days := make([]time.Weekday, 0, len(s))
	previous := byte('0')
	for i := 0; i < len(s); i++ {
		if s[i] < '1' || s[i] > '7' || s[i] <= previous {
			return nil, fmt.Errorf("%q is not a list of days 1 to 7 in ascending order", s)
		}
		previous = s[i]
		days = append(days, time.Weekday(int(s[i]-'0')%7))
	}
	return days, nil
}

// parseDayOfMonth reads a day written dd, 01 to 31.
func parseDayOfMonth(s string) (int, error) {
	if len(s) != 2 || !allDigits(s) {
		return 0, fmt.Errorf("%q is not a day written dd", s)
	}
	day := twoDigits(s)
	if day < 1 || day > 31 {
		return 0, fmt.Errorf("%q is not a day of the month", s)
	}
	return day, nil
}

// parseMoment reads a time written hhmm, or ddhhmm when it carries the day of the month.
func parseMoment(s string) (Moment, error) {
	switch len(s) {
	case 4:
		clock, err := parseTimeOfDay(s)
		return Moment{Time: clock}, err
	case 6:
		day, err := parseDayOfMonth(s[:2])
		if err != nil {
			return Moment{}, err
		}
		clock, err := parseTimeOfDay(s[2:])
		return Moment{Day: day, Time: clock}, err
	}
	return Moment{}, fmt.Errorf("%q is not a time written hhmm or ddhhmm", s)
}

// parseDurationMinutes reads a length of time written hhmm and returns it in minutes.
func parseDurationMinutes(s string) (int, error) {
	if len(s) != 4 || !allDigits(s) {
		return 0, fmt.Errorf("%q is not a duration written hhmm", s)
	}
	hours, minutes := twoDigits(s), twoDigits(s[2:])
	if minutes > 59 {
		return 0, fmt.Errorf("%q has more than 59 minutes", s)
	}
	return hours*60 + minutes, nil
}
