package typebreader

import (
	"slices"
	"testing"
	"time"
)

// mustFail checks that parse refuses every one of the texts.
func mustFail[T any](t *testing.T, name string, parse func(string) (T, error), bad ...string) {
	t.Helper()
	for _, text := range bad {
		if _, err := parse(text); err == nil {
			t.Errorf("%s(%q) should fail", name, text)
		}
	}
}

func TestParseDate(t *testing.T) {
	got, err := parseDate("05NOV26")
	if err != nil || !got.Equal(time.Date(2026, time.November, 5, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("parseDate(05NOV26) = %v, %v", got, err)
	}
	mustFail(t, "parseDate", parseDate, "31NOV26", "05XXX26", "5NOV26", "")
}

func TestParseTimeOfDay(t *testing.T) {
	got, err := parseTimeOfDay("0810")
	if err != nil || got != (TimeOfDay{8, 10}) {
		t.Fatalf("parseTimeOfDay(0810) = %v, %v", got, err)
	}
	mustFail(t, "parseTimeOfDay", parseTimeOfDay, "2575", "0875", "081", "08100", "ab12", "")
}

func TestParseWeekdays(t *testing.T) {
	got, err := parseWeekdays("1357")
	want := []time.Weekday{time.Monday, time.Wednesday, time.Friday, time.Sunday}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("parseWeekdays(1357) = %v, %v, want %v", got, err, want)
	}
	mustFail(t, "parseWeekdays", parseWeekdays, "", "0", "8", "531", "1135", "1a")
}

func TestParseDayOfMonth(t *testing.T) {
	for good, want := range map[string]int{"01": 1, "31": 31} {
		if got, err := parseDayOfMonth(good); err != nil || got != want {
			t.Errorf("parseDayOfMonth(%q) = %d, %v", good, got, err)
		}
	}
	mustFail(t, "parseDayOfMonth", parseDayOfMonth, "00", "32", "1", "ab")
}

func TestParseMoment(t *testing.T) {
	got, err := parseMoment("0805")
	if err != nil || got != (Moment{Day: 0, Time: TimeOfDay{8, 5}}) {
		t.Fatalf("parseMoment(0805) = %v, %v", got, err)
	}
	got, err = parseMoment("210805")
	if err != nil || got != (Moment{Day: 21, Time: TimeOfDay{8, 5}}) {
		t.Fatalf("parseMoment(210805) = %v, %v", got, err)
	}
	mustFail(t, "parseMoment", parseMoment, "", "805", "08055", "320805", "000805", "212505", "210875", "ab0805")
}

func TestParseDurationMinutes(t *testing.T) {
	for text, want := range map[string]int{"0015": 15, "0100": 60, "0230": 150, "1200": 720} {
		if got, err := parseDurationMinutes(text); err != nil || got != want {
			t.Errorf("parseDurationMinutes(%q) = %d, %v, want %d", text, got, err, want)
		}
	}
	mustFail(t, "parseDurationMinutes", parseDurationMinutes, "", "15", "0075", "00155", "ab15")
}
