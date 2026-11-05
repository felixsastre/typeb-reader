package typebreader

import "time"

// Leg is one flight leg: where it leaves and arrives, and when.
type Leg struct {
	From, To           string
	Departure, Arrival TimeOfDay
}

// SSM is a Standard Schedule Message: NEW, CNL, RPL or TIM for one flight and one period.
type SSM struct {
	Action        string
	TimeMode      string
	Flight        string
	Start, End    time.Time
	Days          []time.Weekday
	ServiceType   string
	AircraftType  string
	Configuration string
	Legs          []Leg
}

// ReadSSM extracts the schedule message from a message already read. A failure is a *ParseError on
// the line that holds the value, or on the line of the message type when the message is not an SSM,
// and the SSM returned is then the zero value.
func ReadSSM(m *Message) (SSM, error) {
	if err := checkType(m, "SSM"); err != nil {
		return SSM{}, err
	}
	ssm := SSM{
		Action:        m.Value("action"),
		TimeMode:      m.Value("time_mode"),
		Flight:        m.Value("flight"),
		ServiceType:   m.Value("service"),
		AircraftType:  m.Value("aircraft"),
		Configuration: m.Value("configuration"),
	}
	var err error
	if ssm.Start, err = parseValue(m, parseDate, "start"); err != nil {
		return SSM{}, err
	}
	if ssm.End, err = parseValue(m, parseDate, "end"); err != nil {
		return SSM{}, err
	}
	if ssm.Days, err = parseValue(m, parseWeekdays, "days"); err != nil {
		return SSM{}, err
	}
	for _, line := range m.lines {
		if line.name != "leg" {
			continue
		}
		leg := Leg{From: line.values["from"], To: line.values["to"]}
		if leg.Departure, err = parseAt(line.values["dep"], line.number, parseTimeOfDay); err != nil {
			return SSM{}, err
		}
		if leg.Arrival, err = parseAt(line.values["arr"], line.number, parseTimeOfDay); err != nil {
			return SSM{}, err
		}
		ssm.Legs = append(ssm.Legs, leg)
	}
	return ssm, nil
}

// parseValue reads the value captured under name with parse. The error says on which line it was.
func parseValue[T any](m *Message, parse func(string) (T, error), name string) (T, error) {
	text, line := m.locate(name)
	return parseAt(text, line, parse)
}

// parseAt reads text with parse. The error says that it was on the given line.
func parseAt[T any](text string, line int, parse func(string) (T, error)) (T, error) {
	value, err := parse(text)
	return value, at(line, err)
}

// checkType returns a ParseError unless m is a message of type want. A message that was
// never read (nil, or the zero Message) has no line of its own, so the error is on line 1.
func checkType(m *Message, want string) error {
	switch {
	case m == nil || m.Type == "":
		return errorAt(1, "there is no message: it was not read")
	case m.Type != want:
		return errorAt(m.typeLine, "message type is %s, not %s", m.Type, want)
	}
	return nil
}
