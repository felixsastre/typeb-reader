package typebreader

import (
	"fmt"
	"strconv"
	"strings"
)

// Delay is a delay code, kept as written, and how long the delay lasts.
type Delay struct {
	Code    string
	Minutes int
}

// MVT is a Movement message about one flight: a departure, an arrival, or a delay notice. A time
// the message does not give is nil. All times are UTC.
type MVT struct {
	Flight       string
	Day          int // scheduled day of departure
	Registration string
	Station      string
	Kind         string // "departure", "arrival" or "delay"

	OffBlock, Airborne             *Moment // departure
	Touchdown, OnBlock             *Moment // arrival
	EstimatedDeparture             *Moment // delay notice
	NextInformation                *Moment // delay notice for an indefinite delay
	EstimatedTakeOff, ReturnToRamp *Moment
	EstimatedArrival               *Moment
	EstimatedArrivalAirport        string
	EstimatedOnBlock               *Moment

	Delays      []Delay
	Passengers  []int
	LegDay      int
	Information string
}

// ReadMVT extracts the movement from a message already read. A failure is a *ParseError on the
// line that holds the value, or on the line of the message type when the message is not an MVT,
// and the MVT returned is then the zero value.
func ReadMVT(m *Message) (MVT, error) {
	if err := checkType(m, "MVT"); err != nil {
		return MVT{}, err
	}
	mvt := MVT{
		Flight:                  m.Value("flight"),
		Registration:            m.Value("registration"),
		Station:                 m.Value("station"),
		EstimatedArrivalAirport: m.Value("est_arrival_airport"),
		Information:             m.Value("information"),
	}
	r := &valueReader{message: m}
	mvt.Day = r.dayOfMonth("day")
	mvt.LegDay = r.dayOfMonth("leg_day")
	switch code, line := m.locate("code", "delay_code"); code {
	case "AD":
		mvt.Kind = "departure"
		mvt.OffBlock, mvt.Airborne = r.moment("first"), r.moment("second", "only_second")
	case "AA":
		mvt.Kind = "arrival"
		mvt.Touchdown, mvt.OnBlock = r.moment("first"), r.moment("second", "only_second")
	case "ED":
		mvt.Kind = "delay"
		mvt.EstimatedDeparture = r.moment("delay_time")
	case "NI":
		mvt.Kind = "delay"
		mvt.NextInformation = r.moment("delay_time")
	default:
		r.fail(line, fmt.Errorf("movement code %q is not AD, AA, ED or NI", code))
	}
	mvt.EstimatedTakeOff, mvt.ReturnToRamp = r.moment("est_takeoff"), r.moment("return_ramp")
	mvt.EstimatedArrival, mvt.EstimatedOnBlock = r.moment("est_arrival"), r.moment("est_onblock", "est_onblock_line")
	for _, pair := range [][2]string{{"code_1", "duration_1"}, {"pair_code_1", "pair_duration_1"}, {"pair_code_2", "pair_duration_2"}} {
		if delay, given := r.delay(pair[0], pair[1]); given {
			mvt.Delays = append(mvt.Delays, delay)
		}
	}
	mvt.Passengers = r.numbers("passengers")
	if r.err != nil {
		return MVT{}, r.err
	}
	return mvt, nil
}

// valueReader takes typed values out of a message and keeps the first error it meets.
type valueReader struct {
	message *Message
	err     error
}

// fail keeps the first error, as a ParseError on the given line.
func (r *valueReader) fail(line int, err error) {
	if r.err == nil {
		r.err = at(line, err)
	}
}

// moment returns the first non-empty value among names read as a moment, or nil if there is none.
func (r *valueReader) moment(names ...string) *Moment {
	text, line := r.message.locate(names...)
	if text == "" {
		return nil
	}
	moment, err := parseMoment(text)
	r.fail(line, err)
	return &moment
}

func (r *valueReader) dayOfMonth(name string) int {
	text, line := r.message.locate(name)
	if text == "" {
		return 0
	}
	day, err := parseDayOfMonth(text)
	r.fail(line, err)
	return day
}

// delay returns the delay held by a pair of groups, and false when the message did not give it.
func (r *valueReader) delay(codeName, durationName string) (Delay, bool) {
	code, _ := r.message.locate(codeName)
	if code == "" {
		return Delay{}, false
	}
	duration, line := r.message.locate(durationName)
	minutes, err := parseDurationMinutes(duration)
	r.fail(line, err)
	return Delay{Code: code, Minutes: minutes}, true
}

// numbers returns the numbers of a value written 12/134/10.
func (r *valueReader) numbers(name string) []int {
	text, line := r.message.locate(name)
	if text == "" {
		return nil
	}
	var numbers []int
	for _, part := range strings.Split(text, "/") {
		n, err := strconv.Atoi(part)
		r.fail(line, err)
		numbers = append(numbers, n)
	}
	return numbers
}
