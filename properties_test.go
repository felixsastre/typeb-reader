package typebreader

import (
	"fmt"
	"strings"
	"testing"
	"testing/quick"
)

// However many legs a new flight has, the reader returns exactly that many, in order.
func TestReadReturnsEveryLegInOrder(t *testing.T) {
	r := NewReader()
	property := func(n uint8) bool {
		legs := int(n)%30 + 1
		lines := []string{"QU MADRMXA", ".BCNRMXA 051530", "SSM", "UTC", "NEW", "XA123", "05NOV26 28MAR27 135", "J 738"}
		for i := 0; i < legs; i++ {
			lines = append(lines, fmt.Sprintf("MAD%02d00 BCN%02d30", i%24, i%24))
		}
		message, err := r.Read(strings.Join(lines, "\n"))
		if err != nil {
			return false
		}
		ssm, err := ReadSSM(message)
		if err != nil || len(ssm.Legs) != legs {
			return false
		}
		for i, leg := range ssm.Legs {
			if leg.Departure.Hour != i%24 || leg.Arrival.Minute != 30 {
				return false
			}
		}
		return true
	}
	if err := quick.Check(property, nil); err != nil {
		t.Fatal(err)
	}
}

// A new flight's days of operation come back as the digits written, in the same order.
func TestReadReturnsTheDaysWritten(t *testing.T) {
	r := NewReader()
	property := func(mask uint8) bool {
		days := ""
		for d := 0; d < 7; d++ {
			if mask&(1<<d) != 0 {
				days += fmt.Sprint(d + 1)
			}
		}
		if days == "" {
			return true
		}
		text := fmt.Sprintf("QU MADRMXA\n.BCNRMXA 051530\nSSM\nUTC\nCNL\nXA123\n05NOV26 28MAR27 %s", days)
		message, err := r.Read(text)
		if err != nil {
			return false
		}
		ssm, err := ReadSSM(message)
		if err != nil || len(ssm.Days) != len(days) {
			return false
		}
		for i, day := range ssm.Days {
			if int(day+6)%7+1 != int(days[i]-'0') { // Sunday is 0 in Go, 7 in a message
				return false
			}
		}
		return true
	}
	if err := quick.Check(property, nil); err != nil {
		t.Fatal(err)
	}
}

// A time written with its day reads back as the same day, hour and minute.
func TestMomentRoundTrip(t *testing.T) {
	property := func(d, h, m uint8) bool {
		day, hour, minute := int(d)%31+1, int(h)%24, int(m)%60
		got, err := parseMoment(fmt.Sprintf("%02d%02d%02d", day, hour, minute))
		return err == nil && got == Moment{Day: day, Time: TimeOfDay{hour, minute}}
	}
	if err := quick.Check(property, nil); err != nil {
		t.Fatal(err)
	}
}
