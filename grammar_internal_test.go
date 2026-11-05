package typebreader

import (
	"reflect"
	"strings"
	"testing"
)

func TestCompileGrammarAcceptsAWellFormedMessage(t *testing.T) {
	src := `message "TST" {
  line "flight" { pattern = "^(?P<flight>[A-Z]{2}[0-9]{3})$" }
  line "day"    {
    pattern = "^(?P<day>[0-9]{2})$"
    kinds   = { day = "day_of_month" }
  }
}`
	grammars, err := compileGrammars(src)
	if err != nil || len(grammars) != 1 || len(grammars["TST"].lines) != 2 {
		t.Fatalf("compileGrammars = %v, %v", grammars, err)
	}
}

// grammarOf is a grammar with one message type, "T", made of the given lines.
func grammarOf(lines ...string) string {
	return "message \"T\" {\n" + strings.Join(lines, "\n") + "\n}"
}

// lineOf is the HCL of a line with the given attributes, one per line.
func lineOf(name string, attributes ...string) string {
	return "line \"" + name + "\" {\n" + strings.Join(attributes, "\n") + "\n}"
}

func TestCompileGrammarRejects(t *testing.T) {
	x := `pattern = "^(?P<x>.)$"`
	cases := map[string]struct{ src, reason string }{
		"bad pattern":        {grammarOf(lineOf("a", `pattern = "("`)), "pattern"},
		"unknown kind":       {grammarOf(lineOf("a", x, `kinds = { x = "colour" }`)), "unknown kind"},
		"kind on no group":   {grammarOf(lineOf("a", `pattern = "^.$"`, `kinds = { x = "date" }`)), "not a group"},
		"not_before unknown": {grammarOf(lineOf("a", x, `not_before = { x = "y" }`)), "not a date group"},
		"no lines":           {grammarOf(), "no lines"},
		"not hcl":            {`message {{{`, "reading grammar"},
		"when on no capture": {grammarOf(lineOf("a", x),
			lineOf("b", `pattern = "^.$"`, "when {\ncapture = \"y\"\nin = [\"1\"]\n}")), "no earlier line captures"},
		"when on the empty name": {grammarOf(lineOf("a", x),
			lineOf("b", `pattern = "^.$"`, "when {\ncapture = \"\"\nin = [\"\"]\n}")), "no earlier line captures"},
		"at_most_one unknown name":        {grammarOf(lineOf("a", x, `at_most_one = ["x", "y"]`)), "at_most_one"},
		"at_most_one one name":            {grammarOf(lineOf("a", x, `at_most_one = ["x"]`)), "at_most_one"},
		"at_most_one twice the same name": {grammarOf(lineOf("a", x, `at_most_one = ["x", "x"]`)), "more than once"},
		"at_most_one a name repeated":     {grammarOf(lineOf("a", `pattern = "^(?P<x>.)(?P<y>.)$"`, `at_most_one = ["x", "y", "x"]`)), "more than once"},
		"at_most_one none in this line": {grammarOf(lineOf("a", x), lineOf("b", `pattern = "^(?P<y>.)$"`),
			lineOf("c", `pattern = "^(?P<z>.)$"`, `at_most_one = ["x", "y"]`)), "at_most_one"},
	}
	for name, c := range cases {
		_, err := compileGrammars(c.src)
		if err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: compileGrammars = %v, want an error mentioning %q", name, err, c.reason)
		}
	}
}

func TestEmbeddedGrammarsLoad(t *testing.T) {
	r := NewReader()
	for _, kind := range []string{"SSM", "MVT"} {
		if _, ok := r.grammars[kind]; !ok {
			t.Errorf("no built-in grammar for %s", kind)
		}
	}
}

func TestReadStopsAtTheFirstBadLine(t *testing.T) {
	text := strings.Join([]string{"ZZ MADRMXA", ".BCNRMXA 051530", "MVT"}, "\n")
	_, err := NewReader().Read(text)
	perr, ok := err.(*ParseError)
	if !ok || perr.Line != 1 {
		t.Fatalf("Read = %v, want a ParseError on line 1", err)
	}
}

func TestOptionalLineMayBeAbsent(t *testing.T) {
	r := NewReader()
	mustAdd(t, r, `message "OPT" {
  line "first"  { pattern = "^(?P<first>A)$" }
  line "extra"  {
    pattern  = "^(?P<extra>B)$"
    optional = true
  }
  line "last"   { pattern = "^(?P<last>C)$" }
}`)
	for _, body := range []string{"A\nC", "A\nB\nC"} {
		if _, err := r.Read(typedEnvelope + "OPT\n" + body); err != nil {
			t.Errorf("body %q: %v", body, err)
		}
	}
	if _, err := r.Read(typedEnvelope + "OPT\nA\nB"); err == nil {
		t.Error("a missing required line after an optional one should still fail")
	}
}

// not_before is stored in a map; the first error reported must not depend on its iteration order.
func TestFirstDateErrorDoesNotDependOnMapOrder(t *testing.T) {
	r := NewReader()
	mustAdd(t, r, `message "TST" {
  line "dates" {
    pattern    = "^(?P<a>[0-9A-Z]{7}) (?P<b>[0-9A-Z]{7}) (?P<c>[0-9A-Z]{7}) (?P<d>[0-9A-Z]{7})$"
    kinds      = { a = "date", b = "date", c = "date", d = "date" }
    not_before = { d = "c", b = "a" }
  }
}`)
	text := typedEnvelope + "TST\n05NOV26 04NOV26 10NOV26 09NOV26"
	for i := 0; i < 200; i++ {
		_, err := r.Read(text)
		if err == nil || !strings.Contains(err.Error(), `date "04NOV26" is before "05NOV26"`) {
			t.Fatalf("Read = %v, want the error about b and a, the first in name order", err)
		}
	}
}

// A group that did not take part in the match must not hide the value a later line captures
// under the same name.
func TestUnmatchedGroupDoesNotHideALaterValue(t *testing.T) {
	r := NewReader()
	mustAdd(t, r, `message "TST" {
  line "first"  { pattern = "^(?:A(?P<x>[0-9]+)|B)$" }
  line "second" { pattern = "^C(?P<x>[0-9]+)$" }
}`)
	message, err := r.Read(typedEnvelope + "TST\nB\nC42")
	if err != nil {
		t.Fatal(err)
	}
	if got := message.Value("x"); got != "42" {
		t.Errorf("Value(x) = %q, want 42", got)
	}
}

func TestPatternIsAnchoredAtBothEnds(t *testing.T) {
	grammars, err := compileGrammars(grammarOf(lineOf("a", `pattern = "A|B"`)))
	if err != nil {
		t.Fatal(err)
	}
	re := grammars["T"].lines[0].pattern
	for text, want := range map[string]bool{"A": true, "B": true, "AB": false, "xA": false, "Bx": false, "": false} {
		if re.MatchString(text) != want {
			t.Errorf("pattern A|B on %q: want %v", text, !want)
		}
	}
	if _, err := compileGrammars(grammarOf(lineOf("a", `pattern = "^(?P<x>A)$"`))); err != nil {
		t.Errorf("a pattern that already has its anchors: %v", err)
	}
	if _, err := compileGrammars(grammarOf(lineOf("a", `pattern = "a)(b"`))); err == nil {
		t.Error("a pattern whose parentheses only balance once wrapped should fail")
	}
}

const typedEnvelope = "QU MADRMXA\n.BCNRMXA 051530\n"

// looseGrammar is an SSM and an MVT grammar that check nothing, so the typed readers meet text that
// the engine would normally have refused.
const looseGrammar = `message "SSM" {
  line "flight" { pattern = "^(?P<flight>.+)$" }
  line "period" { pattern = "^(?P<start>.+) (?P<end>.+) (?P<days>.+)$" }
}
message "MVT" {
  line "flight"   { pattern = "^(?P<flight>.+)$" }
  line "movement" { pattern = "^(?P<code>.+)$" }
  line "day"      { pattern = "^(?P<day>.+)$" }
}`

// mustAdd teaches r the grammar, or stops the test.
func mustAdd(t *testing.T, r *Reader, source string) {
	t.Helper()
	if err := r.AddGrammar(source); err != nil {
		t.Fatal(err)
	}
}

// readerOf builds a Reader that knows only the given grammar.
func readerOf(t *testing.T, source string) *Reader {
	t.Helper()
	r := &Reader{}
	mustAdd(t, r, source)
	return r
}

func TestTypedReaderErrorsCarryTheLine(t *testing.T) {
	r := readerOf(t, looseGrammar)
	cases := []struct {
		name string
		text string
		read func(*Message) error
		line int
	}{
		{"SSM with a bad date", "SSM\nXA123\nnot a date 28MAR27 135", func(m *Message) error { _, err := ReadSSM(m); return err }, 5},
		{"MVT with an unknown code", "MVT\nXA123\nZZ\n05", func(m *Message) error { _, err := ReadMVT(m); return err }, 5},
		{"MVT with a bad day", "MVT\nXA123\nAD\n99", func(m *Message) error { _, err := ReadMVT(m); return err }, 6},
		{"MVT read as SSM", "MVT\nXA123\nAD\n05", func(m *Message) error { _, err := ReadSSM(m); return err }, 3},
		{"SSM read as MVT", "SSM\nXA123\n05NOV26 28MAR27 135", func(m *Message) error { _, err := ReadMVT(m); return err }, 3},
	}
	for _, c := range cases {
		message, err := r.Read(typedEnvelope + c.text)
		if err != nil {
			t.Fatalf("%s: Read = %v", c.name, err)
		}
		perr, ok := c.read(message).(*ParseError)
		if !ok || perr.Line != c.line {
			t.Errorf("%s: error = %#v, want a ParseError on line %d", c.name, perr, c.line)
		}
	}
}

func TestAtMostOneRejectsASecondCapture(t *testing.T) {
	r := NewReader()
	mustAdd(t, r, `message "TST" {
  line "a" {
    pattern  = "^A(?P<x>[0-9])?$"
    optional = true
  }
  line "b" {
    pattern     = "^B(?P<y>[0-9])?$"
    at_most_one = ["x", "y"]
  }
}`)
	for body, wantLine := range map[string]int{"A1\nB2": 5, "A1\nB": 0, "A\nB2": 0, "B2": 0} {
		_, err := r.Read(typedEnvelope + "TST\n" + body)
		if wantLine == 0 {
			if err != nil {
				t.Errorf("%q: %v", body, err)
			}
			continue
		}
		if perr, ok := err.(*ParseError); !ok || perr.Line != wantLine {
			t.Errorf("%q: error = %v, want a ParseError on line %d", body, err, wantLine)
		}
	}
}

func TestCompileGrammarRejectsASourceWithNoMessage(t *testing.T) {
	for name, src := range map[string]string{
		"empty":        "",
		"blank":        " \n\t\n",
		"only comment": "# no message here\n",
	} {
		if _, err := compileGrammars(src); err == nil || !strings.Contains(err.Error(), "no message") {
			t.Errorf("%s: compileGrammars = %v, want an error saying there is no message", name, err)
		}
		r := NewReader()
		before := len(r.grammars)
		if err := r.AddGrammar(src); err == nil || len(r.grammars) != before {
			t.Errorf("%s: AddGrammar = %v, grammars %d -> %d; want an error and no change", name, err, before, len(r.grammars))
		}
	}
}

// A Reader that was not made by NewReader knows no message type, but it can be given some.
func TestZeroReaderWorks(t *testing.T) {
	var r Reader
	text := typedEnvelope + "TST\nXA123"
	if _, err := r.Read(text); err == nil || !strings.Contains(err.Error(), "no grammar for message type") {
		t.Fatalf("Read on the zero Reader = %v, want a ParseError about the missing grammar", err)
	}
	if err := r.AddGrammar(`message "TST" {
  line "flight" { pattern = "(?P<flight>[A-Z0-9]{2}[0-9]{1,4})" }
}`); err != nil {
		t.Fatalf("AddGrammar on the zero Reader = %v", err)
	}
	message, err := r.Read(text)
	if err != nil || message.Value("flight") != "XA123" {
		t.Fatalf("Read = %v, %v", message, err)
	}
	if err := r.AddGrammar(`message "TST" {
  line "other" { pattern = "." }
}`); err == nil {
		t.Error("a message type already known should be refused")
	}
}

// The typed readers refuse a message that was never read, with a ParseError on line 1.
func TestTypedReadersRefuseNoMessage(t *testing.T) {
	for name, m := range map[string]*Message{"nil": nil, "zero": {}} {
		_, ssmErr := ReadSSM(m)
		_, mvtErr := ReadMVT(m)
		for reader, err := range map[string]error{"ReadSSM": ssmErr, "ReadMVT": mvtErr} {
			if perr, ok := err.(*ParseError); !ok || perr.Line != 1 {
				t.Errorf("%s message, %s: error = %#v, want a ParseError on line 1", name, reader, err)
			}
		}
	}
}

// A typed reader that fails returns the zero value, never a half-filled one.
func TestTypedReadersReturnTheZeroValueOnError(t *testing.T) {
	r := readerOf(t, looseGrammar)
	mvt, err := r.Read(typedEnvelope + "MVT\nXA123\nAD\n99")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ReadMVT(mvt); err == nil || !reflect.DeepEqual(got, MVT{}) {
		t.Errorf("ReadMVT = %+v, %v; want the zero MVT and an error", got, err)
	}
	ssm, err := r.Read(typedEnvelope + "SSM\nXA123\nnot a date 28MAR27 135")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ReadSSM(ssm); err == nil || !reflect.DeepEqual(got, SSM{}) {
		t.Errorf("ReadSSM = %+v, %v; want the zero SSM and an error", got, err)
	}
}

// Both dates of a not_before rule must be given for it to apply, and a date that is given but cannot
// be read is an error, never taken as the zero date. The kinds normally refuse such a value first.
func TestNotBeforeNeedsBothDatesAndReadsThemStrictly(t *testing.T) {
	rule := lineRule{notBefore: [][2]string{{"end", "start"}}}
	for name, values := range map[string]map[string]string{
		"start only": {"start": "05NOV26"},
		"end only":   {"end": "06NOV26"},
		"neither":    {},
		"in order":   {"start": "05NOV26", "end": "05NOV26"},
	} {
		if err := rule.check(values, "line", 4); err != nil {
			t.Errorf("%s: check = %v, want none", name, err)
		}
	}
	for name, values := range map[string]map[string]string{
		"end before start": {"start": "05NOV26", "end": "04NOV26"},
		"bad end":          {"start": "05NOV26", "end": "31NOV26"},
		"bad start":        {"start": "xx", "end": "05NOV26"},
	} {
		if perr, ok := rule.check(values, "line", 4).(*ParseError); !ok || perr.Line != 4 {
			t.Errorf("%s: check = %#v, want a ParseError on line 4", name, perr)
		}
	}
}

// A carriage return is never stripped unless it is the one of a CRLF. The line that holds any
// other is refused, whichever part of the message it is in.
func TestCarriageReturnIsRefusedOnItsLine(t *testing.T) {
	r := NewReader()
	for name, c := range map[string]struct {
		text string
		line int
	}{
		"in the priority line":      {"QU MADRMXA\r\r\n.BCNRMXA 051530\r\nMVT\r\nXA123/05.ECABC.MAD\r\nAD0805/0812", 1},
		"in the middle of the line": {"QU MADR\rMXA\n.BCNRMXA 051530\nMVT\nXA123/05.ECABC.MAD\nAD0805/0812", 1},
		"in the origin line":        {"QU MADRMXA\n.BCNRMXA\r 051530\nMVT\nXA123/05.ECABC.MAD\nAD0805/0812", 2},
		"in the message type":       {"QU MADRMXA\n.BCNRMXA 051530\nMVT\r\r\nXA123/05.ECABC.MAD\nAD0805/0812", 3},
		"alone in the last line":    {"QU MADRMXA\n.BCNRMXA 051530\nMVT\nXA123/05.ECABC.MAD\nAD0805/0812\n\r", 6},
	} {
		_, err := r.Read(c.text)
		perr, ok := err.(*ParseError)
		if !ok || perr.Line != c.line || !strings.Contains(perr.Detail, "carriage return") {
			t.Errorf("%s: Read = %v, want a ParseError on line %d naming a carriage return", name, err, c.line)
		}
	}
	// CRLF throughout is fine.
	if _, err := r.Read("QU MADRMXA\r\n.BCNRMXA 051530\r\nMVT\r\nXA123/05.ECABC.MAD\r\nAD0805/0812\r\n"); err != nil {
		t.Errorf("CRLF message: %v", err)
	}
}
