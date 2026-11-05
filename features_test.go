package typebreader_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"

	typeb "github.com/felixsastre/typeb-reader"
)

var (
	baseMVT = []string{
		"QU MADRMXA",
		".MADRMXA 050812",
		"MVT",
		"XA123/05.ECABC.MAD",
		"AD0805/0812",
	}
	baseMVTArrival = []string{
		"QU BCNRMXA",
		".BCNRMXA 050931",
		"MVT",
		"XA123/05.ECABC.BCN",
		"AA0925/0931",
	}
	baseSSMNew = []string{
		"QU MADRMXA",
		".BCNRMXA 051530",
		"SSM",
		"UTC",
		"NEW",
		"XA123",
		"05NOV26 28MAR27 135",
		"J 738 Y180",
		"MAD0810 BCN0925",
	}
	baseSSMCancel = []string{
		"QU MADRMXA",
		".BCNRMXA 051530",
		"SSM",
		"UTC",
		"CNL",
		"XA123",
		"05NOV26 28MAR27 135",
	}
	testGrammar = `
message "TST" {
  line "flight" { pattern = "^(?P<flight>[A-Z0-9]{2}[0-9]{1,4})$" }
}
`
	unanchoredGrammar = `
message "TST" {
  line "flight" { pattern = "(?P<flight>[A-Z0-9]{2}[0-9]{1,4})" }
}
`
	// What a scenario means by the ending of a message.
	endings = map[string]string{
		"a line feed":                               "\n",
		"a carriage return and a line feed":         "\r\n",
		"two line feeds":                            "\n\n",
		"three line feeds":                          "\n\n\n",
		"two carriage returns and line feeds":       "\r\n\r\n",
		"a lone carriage return":                    "\r",
		"two carriage returns before the line feed": "\r\r\n",
	}
	// What a scenario means by the layout of the priority line (the destination is MADRMXA).
	priorityLayouts = map[string]string{
		"a leading space":                         " QU MADRMXA",
		"a trailing space":                        "QU MADRMXA ",
		"a double space between the fields":       "QU  MADRMXA",
		"a double space between two destinations": "QU MADRMXA  PMIRMXA",
		"a tab between the fields":                "QU\tMADRMXA",
	}
	// What a scenario means by the tail of the message type line.
	typeTails = map[string]string{
		"a space and a word": " EXTRA",
		"a space":            " ",
	}
	periodGrammar = `
message "PER" {
  line "period" {
    pattern    = "^(?P<start>[0-9]{2}[A-Z]{3}[0-9]{2})?(?:/(?P<end>[0-9]{2}[A-Z]{3}[0-9]{2}))?$"
    kinds      = { start = "date", end = "date" }
    not_before = { end = "start" }
  }
}
`
)

// world holds what one scenario builds and reads.
type world struct {
	reader  *typeb.Reader
	text    string
	message *typeb.Message
	ssm     typeb.SSM
	mvt     typeb.MVT
	err     error
}

var w *world

func with(base []string, line int, text string) string {
	lines := slices.Clone(base)
	lines[line-1] = text
	return strings.Join(lines, "\n")
}

func without(base []string, line int) string {
	return strings.Join(slices.Delete(slices.Clone(base), line-1, line), "\n")
}

// withBody is the envelope of baseMVT followed by the given lines.
func withBody(lines ...string) string {
	return strings.Join(slices.Concat(baseMVT[:2], lines), "\n")
}

// addGrammar is a step that teaches the scenario's reader a grammar.
func addGrammar(source string) func() error {
	return func() error { return w.reader.AddGrammar(source) }
}

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: initializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
			Strict:   true,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("feature scenarios failed")
	}
}

func initializeScenario(sc *godog.ScenarioContext) {
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		w = &world{reader: typeb.NewReader()}
		return ctx, nil
	})

	// Givens: messages.
	sc.Step(`^the message$`, func(doc *godog.DocString) { w.text = doc.Content })
	sc.Step(`^a message whose priority line is "([^"]*)"$`, func(line string) { w.text = with(baseMVT, 1, line) })
	sc.Step(`^a message whose priority line has (.+)$`, func(layout string) { w.text = with(baseMVT, 1, priorityLayouts[layout]) })
	sc.Step(`^a message whose message type line is MVT followed by (.+)$`, func(tail string) { w.text = with(baseMVT, 3, "MVT"+typeTails[tail]) })
	sc.Step(`^a grammar that defines the message type PER as one period line with an optional start and an optional end$`, addGrammar(periodGrammar))
	sc.Step(`^a message of type PER whose period line is "([^"]*)"$`, func(line string) { w.text = withBody("PER", line) })
	sc.Step(`^a valid MVT departure whose movement line has a carriage return in the middle$`, func() {
		w.text = with(baseMVT, 5, "AD0805/0812\rXYZ")
	})
	sc.Step(`^a message whose origin line is "([^"]*)"$`, func(line string) { w.text = with(baseMVT, 2, line) })
	sc.Step(`^a message whose body has a line of 64 characters$`, func() {
		w.text = with(baseMVT, 4, strings.Repeat("X", 64))
	})
	sc.Step(`^a message of type XYZ$`, func() { w.text = withBody("XYZ", "XA123") })
	sc.Step(`^a message that has only the envelope$`, func() { w.text = withBody() })
	sc.Step(`^a grammar that defines the message type TST as one line holding a flight$`, addGrammar(testGrammar))
	sc.Step(`^a grammar that defines the message type TST as one flight line written without anchors$`, addGrammar(unanchoredGrammar))
	sc.Step(`^a message of type TST whose flight line is "([^"]*)"$`, func(line string) { w.text = withBody("TST", line) })
	sc.Step(`^a valid SSM NEW message whose period line is "([^"]*)"$`, func(line string) { w.text = with(baseSSMNew, 7, line) })
	sc.Step(`^a valid SSM NEW message whose routing line is "([^"]*)"$`, func(line string) { w.text = with(baseSSMNew, 9, line) })
	sc.Step(`^a valid SSM NEW message without its routing line$`, func() { w.text = without(baseSSMNew, 9) })
	sc.Step(`^a valid SSM NEW message whose (action|time mode) line is "([^"]*)"$`, func(which, line string) {
		number := map[string]int{"time mode": 4, "action": 5}[which]
		w.text = with(baseSSMNew, number, line)
	})
	sc.Step(`^a valid SSM CNL message followed by the line "([^"]*)"$`, func(line string) {
		w.text = strings.Join(slices.Concat(baseSSMCancel, []string{line}), "\n")
	})
	sc.Step(`^a valid MVT (departure|arrival) whose flight line is "([^"]*)"$`, func(kind, line string) { w.text = with(mvtBase(kind), 4, line) })
	sc.Step(`^a valid MVT (departure|arrival) whose movement line is "([^"]*)"$`, func(kind, line string) { w.text = with(mvtBase(kind), 5, line) })
	sc.Step(`^a valid MVT (departure|arrival) whose movement line is "([^"]*)" and followed by the line "([^"]*)"$`,
		func(kind, movement, line string) {
			w.text = strings.Join(append(strings.Split(with(mvtBase(kind), 5, movement), "\n"), line), "\n")
		})
	sc.Step(`^a valid MVT departure ending with (a line feed|a carriage return and a line feed|two line feeds|three line feeds|two carriage returns and line feeds|a lone carriage return|two carriage returns before the line feed)$`,
		func(ending string) { w.text = strings.Join(baseMVT, "\n") + endings[ending] })
	sc.Step(`^a valid MVT (departure|arrival) followed by the line "([^"]*)"$`, func(kind, line string) {
		w.text = strings.Join(append(mvtBase(kind), line), "\n")
	})
	sc.Step(`^a valid MVT (departure|arrival) followed by the lines$`, func(kind string, doc *godog.DocString) {
		w.text = strings.Join(append(mvtBase(kind), doc.Content), "\n")
	})
	sc.Step(`^a valid MVT departure without its movement line$`, func() { w.text = without(baseMVT, 5) })

	// When.
	sc.Step(`^the message is read$`, func() {
		w.message, w.err = w.reader.Read(w.text)
		if w.err != nil {
			return
		}
		switch w.message.Type {
		case "SSM":
			w.ssm, w.err = typeb.ReadSSM(w.message)
		case "MVT":
			w.mvt, w.err = typeb.ReadMVT(w.message)
		}
	})

	// Then: errors.
	sc.Step(`^reading succeeds$`, func() error {
		if w.err != nil {
			return fmt.Errorf("expected reading to succeed, got %v", w.err)
		}
		if w.message == nil {
			return fmt.Errorf("expected a message, got nil")
		}
		return nil
	})
	sc.Step(`^reading fails on line (\d+)$`, func(line int) error {
		var perr *typeb.ParseError
		if !asParseError(w.err, &perr) {
			return fmt.Errorf("expected a parse error, got %v", w.err)
		}
		if perr.Line != line {
			return fmt.Errorf("expected failure on line %d, got %d (%s)", line, perr.Line, perr.Detail)
		}
		return nil
	})
	sc.Step(`^the error names "([^"]*)"$`, func(name string) error { return errorContains(name) })
	sc.Step(`^the error says a leg is missing$`, func() error { return errorContains("missing leg") })
	sc.Step(`^the error says a movement line is missing$`, func() error { return errorContains("missing movement line") })

	// Then: envelope.
	sc.Step(`^the priority is (\w+) and the destination is (\w+)$`, func(priority, destination string) error {
		return all(
			equal("priority", priority, w.message.Priority),
			equal("destinations", destination, strings.Join(w.message.Destinations, " ")),
		)
	})
	sc.Step(`^the destinations are (\w+) and (\w+)$`, func(a, b string) error {
		return equal("destinations", a+" "+b, strings.Join(w.message.Destinations, " "))
	})
	sc.Step(`^the origin is (\w+), sent on day (\d+) at (\d+):(\d+)$`, func(origin string, day, hour, minute int) error {
		return all(
			equal("origin", origin, w.message.Origin),
			equal("sent", fmt.Sprint(day, hour, minute), fmt.Sprint(w.message.SentDay, w.message.SentHour, w.message.SentMinute)),
		)
	})
	sc.Step(`^the message type is (\w+)$`, func(kind string) error { return equal("type", kind, w.message.Type) })
	sc.Step(`^the flight is (\w+)$`, func(flight string) error { return equal("flight", flight, w.message.Value("flight")) })

	// Then: SSM.
	sc.Step(`^it is an SSM (\w+) for flight (\w+) in (\w+), operating every day$`, func(action, flight, mode string) error {
		return all(ssmHeader(action, flight, mode), equal("days", weekdays("every day"), weekdays(w.ssm.Days)))
	})
	sc.Step(`^it is an SSM (\w+) for flight (\w+) in (\w+)$`, func(action, flight, mode string) error {
		return ssmHeader(action, flight, mode)
	})
	sc.Step(`^it is an SSM (\w+) for flight (\w+)$`, func(action, flight string) error {
		return all(equal("action", action, w.ssm.Action), equal("flight", flight, w.ssm.Flight))
	})
	sc.Step(`^the period runs from (\d+) (\w+) (\d+) to (\d+) (\w+) (\d+) on (.+)$`,
		func(d1 int, m1 string, y1, d2 int, m2 string, y2 int, days string) error {
			return all(
				equal("start", fmt.Sprint(y1, m1, d1), dateText(w.ssm.Start)),
				equal("end", fmt.Sprint(y2, m2, d2), dateText(w.ssm.End)),
				equal("days", weekdays(days), weekdays(w.ssm.Days)),
			)
		})
	sc.Step(`^the days of operation are (.+)$`, func(days string) error {
		return equal("days", weekdays(days), weekdays(w.ssm.Days))
	})
	sc.Step(`^the aircraft is a (\w+) on a service of type (\w) with configuration "([^"]*)"$`, func(aircraft, service, config string) error {
		return all(equal("aircraft", aircraft, w.ssm.AircraftType), equal("service", service, w.ssm.ServiceType), equal("configuration", config, w.ssm.Configuration))
	})
	sc.Step(`^the aircraft is a (\w+) and the leg goes from (\w+) at (\d\d:\d\d) to (\w+) at (\d\d:\d\d)$`,
		func(aircraft, from, dep, to, arr string) error {
			return all(equal("aircraft", aircraft, w.ssm.AircraftType), legsAre(legText(from, dep, to, arr)))
		})
	sc.Step(`^the leg goes from (\w+) at (\d\d:\d\d) to (\w+) at (\d\d:\d\d)$`, func(from, dep, to, arr string) error {
		return legsAre(legText(from, dep, to, arr))
	})
	sc.Step(`^the legs are (.+)$`, func(legs string) error { return legsAre(strings.ReplaceAll(legs, ", then ", ";")) })
	sc.Step(`^there is no configuration$`, func() error { return equal("configuration", "", w.ssm.Configuration) })
	sc.Step(`^there is no aircraft$`, func() error { return equal("aircraft", "", w.ssm.AircraftType) })
	sc.Step(`^there are no legs and no aircraft$`, func() error {
		return all(equal("legs", 0, len(w.ssm.Legs)), equal("aircraft", "", w.ssm.AircraftType))
	})

	// Then: MVT.
	sc.Step(`^it is a movement of flight (\w+) on day (\d+) by aircraft (\w+) at (\w+)$`, func(flight string, day int, registration, station string) error {
		return all(equal("flight", flight, w.mvt.Flight), equal("day", day, w.mvt.Day),
			equal("registration", registration, w.mvt.Registration), equal("station", station, w.mvt.Station))
	})
	sc.Step(`^the movement is on day (\d+)$`, func(day int) error { return equal("day", day, w.mvt.Day) })
	sc.Step(`^it is a departure, off the gate at (\d\d:\d\d) and airborne at (\d\d:\d\d)$`, departureIs)
	sc.Step(`^it is an arrival, touched down at (\d\d:\d\d) and at the gate at (\d\d:\d\d)$`, arrivalIs)
	sc.Step(`^the departure shows off the gate at (.+) and airborne at (.+)$`, departureIs)
	sc.Step(`^the arrival shows touchdown at (.+) and at the gate at (.+)$`, arrivalIs)
	sc.Step(`^the estimated take-off is (.+)$`, func(at string) error { return equal("estimated take-off", at, w.mvt.EstimatedTakeOff.String()) })
	sc.Step(`^the estimated arrival is (\d\d:\d\d) at (\w+)$`, func(at, airport string) error {
		return all(equal("estimated arrival", at, w.mvt.EstimatedArrival.String()), equal("airport", airport, w.mvt.EstimatedArrivalAirport))
	})
	sc.Step(`^the return to ramp is at (.+)$`, func(at string) error { return equal("return to ramp", at, w.mvt.ReturnToRamp.String()) })
	sc.Step(`^the estimated on-block time is (.+)$`, func(at string) error { return equal("estimated on-block", at, w.mvt.EstimatedOnBlock.String()) })
	sc.Step(`^the delay is code (\d+) for (\d+) minutes$`, func(code string, minutes int) error {
		return delaysAre(typeb.Delay{Code: code, Minutes: minutes})
	})
	sc.Step(`^the delays are code (\d+) for (\d+) minutes and code (\d+) for (\d+) minutes$`, func(c1 string, m1 int, c2 string, m2 int) error {
		return delaysAre(typeb.Delay{Code: c1, Minutes: m1}, typeb.Delay{Code: c2, Minutes: m2})
	})
	sc.Step(`^there are (\d+) passengers$`, func(n int) error { return equal("passengers", fmt.Sprint([]int{n}), fmt.Sprint(w.mvt.Passengers)) })
	sc.Step(`^the passengers per destination are (\d+), (\d+) and (\d+)$`, func(a, b, c int) error {
		return equal("passengers", fmt.Sprint([]int{a, b, c}), fmt.Sprint(w.mvt.Passengers))
	})
	sc.Step(`^the information is "([^"]*)"$`, func(text string) error { return equal("information", text, w.mvt.Information) })
	sc.Step(`^the leg day is (\d+)$`, func(day int) error { return equal("leg day", day, w.mvt.LegDay) })
	sc.Step(`^it is a delay notice with the estimated departure at (.+)$`, func(at string) error {
		return all(equal("kind", "delay", w.mvt.Kind), equal("estimated departure", at, w.mvt.EstimatedDeparture.String()))
	})
	sc.Step(`^it is a delay notice with the next information at (.+)$`, func(at string) error {
		return all(equal("kind", "delay", w.mvt.Kind), equal("next information", at, w.mvt.NextInformation.String()))
	})
}

func mvtBase(kind string) []string {
	if kind == "arrival" {
		return slices.Clone(baseMVTArrival)
	}
	return slices.Clone(baseMVT)
}

func departureIs(off, air string) error {
	return all(equal("kind", "departure", w.mvt.Kind), equal("off the gate", off, w.mvt.OffBlock.String()), equal("airborne", air, w.mvt.Airborne.String()))
}

func arrivalIs(down, gate string) error {
	return all(equal("kind", "arrival", w.mvt.Kind), equal("touchdown", down, w.mvt.Touchdown.String()), equal("at the gate", gate, w.mvt.OnBlock.String()))
}

func delaysAre(want ...typeb.Delay) error {
	return equal("delays", fmt.Sprint(want), fmt.Sprint(w.mvt.Delays))
}

func asParseError(err error, target **typeb.ParseError) bool {
	perr, ok := err.(*typeb.ParseError)
	*target = perr
	return ok
}

func errorContains(text string) error {
	if w.err == nil {
		return fmt.Errorf("expected an error mentioning %q, reading succeeded", text)
	}
	if !strings.Contains(w.err.Error(), text) {
		return fmt.Errorf("error %q does not mention %q", w.err, text)
	}
	return nil
}

func equal[T comparable](what string, want, got T) error {
	if want != got {
		return fmt.Errorf("%s: want %v, got %v", what, want, got)
	}
	return nil
}

func all(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func ssmHeader(action, flight, mode string) error {
	return all(equal("action", action, w.ssm.Action), equal("flight", flight, w.ssm.Flight), equal("time mode", mode, w.ssm.TimeMode))
}

func dateText(t time.Time) string {
	return fmt.Sprint(t.Year(), t.Month().String()[:3], t.Day())
}

// weekdays turns the days written in a step ("Monday, Wednesday and Friday", "every day") or the
// days an SSM holds into one comparable text.
func weekdays(days any) string {
	var list []string
	switch d := days.(type) {
	case []time.Weekday:
		if len(d) == 7 {
			return "every day"
		}
		for _, day := range d {
			list = append(list, day.String())
		}
	case string:
		if d == "every day" {
			return d
		}
		list = strings.Split(strings.ReplaceAll(d, " and ", ", "), ", ")
	}
	return strings.Join(list, ",")
}

// legText is how the steps write a leg: "MAD 10:00 to BCN 11:15".
func legText(from, dep, to, arr any) string {
	return fmt.Sprintf("%s %s to %s %s", from, dep, to, arr)
}

// legsAre compares the legs of the SSM with "MAD 10:00 to BCN 11:15;BCN 12:00 to PMI 12:50".
func legsAre(want string) error {
	var got []string
	for _, leg := range w.ssm.Legs {
		got = append(got, legText(leg.From, leg.Departure, leg.To, leg.Arrival))
	}
	return equal("legs", want, strings.Join(got, ";"))
}
