package typebreader

import (
	"embed"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

//go:embed grammars/*.hcl
var builtinGrammars embed.FS

const maxLineLength = 63

// ParseError says on which line of the message reading failed, and why.
type ParseError struct {
	Line   int
	Detail string
}

func (e *ParseError) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Detail) }

// errorAt makes a ParseError for the given line, with a detail written like fmt.Sprintf.
func errorAt(line int, format string, args ...any) *ParseError {
	return &ParseError{line, fmt.Sprintf(format, args...)}
}

// at reports a plain error as a ParseError for the given line. No error stays no error.
func at(line int, err error) error {
	if err == nil {
		return nil
	}
	return &ParseError{line, err.Error()}
}

// Message is a message split into its envelope and its body, read with the grammar of its type.
type Message struct {
	Priority     string
	Destinations []string
	Origin       string
	SentDay      int
	SentHour     int
	SentMinute   int
	Type         string

	typeLine int // the line of the message type, where the body starts
	lines    []bodyLine
}

// bodyLine is one line of the body that matched a line of the grammar.
type bodyLine struct {
	number int
	name   string
	values map[string]string // only the groups that took part in the match
}

// Value returns the first non-empty text captured under name by the grammar of the message type,
// or "".
func (m *Message) Value(name string) string {
	value, _ := m.locate(name)
	return value
}

// locate returns the first non-empty text captured under one of the names and the line it was on.
// When nothing was captured the line is that of the message type, the best place to report it.
func (m *Message) locate(names ...string) (value string, line int) {
	for _, name := range names {
		for _, bl := range m.lines {
			if v := bl.values[name]; v != "" {
				return v, bl.number
			}
		}
	}
	return "", m.typeLine
}

// Reader reads messages with the grammars it knows: the built-in ones and those added to it.
type Reader struct {
	grammars map[string]grammar
}

// NewReader returns a Reader that knows the built-in message types.
func NewReader() *Reader {
	r := &Reader{}
	files, err := builtinGrammars.ReadDir("grammars")
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		source, err := builtinGrammars.ReadFile("grammars/" + file.Name())
		if err != nil {
			panic(err)
		}
		if err := r.AddGrammar(string(source)); err != nil {
			panic(fmt.Sprintf("built-in grammar %s: %v", file.Name(), err))
		}
	}
	return r
}

// AddGrammar teaches the Reader a message type from the text of its grammar file. The zero Reader
// has no message types and can be given them here.
func (r *Reader) AddGrammar(source string) error {
	added, err := compileGrammars(source)
	if err != nil {
		return err
	}
	if r.grammars == nil {
		r.grammars = map[string]grammar{}
	}
	for kind := range added {
		if _, exists := r.grammars[kind]; exists {
			return fmt.Errorf("message %s is already known", kind)
		}
	}
	maps.Copy(r.grammars, added)
	return nil
}

// Read splits text into envelope and body and checks the body against the grammar of its type.
func (r *Reader) Read(text string) (*Message, error) {
	c := &cursor{lines: splitLines(text)}
	m := &Message{}
	if err := readEnvelope(c, m); err != nil {
		return nil, err
	}
	kind, err := c.need("message type")
	if err != nil {
		return nil, err
	}
	g, known := r.grammars[kind]
	if !known {
		return nil, c.errorf("no grammar for message type %q", kind)
	}
	m.Type, m.typeLine = kind, c.number()
	c.advance()
	if err := readBody(c, g, m); err != nil {
		return nil, err
	}
	return m, nil
}

// splitLines cuts text into lines. Only a line feed, or a carriage return and a line feed, ends a
// line: of the carriage return in the second form, exactly one is dropped. One line break at the end
// of the text ends the last line and is not a line; a second one makes a blank line. A carriage
// return anywhere else stays in its line, and peek refuses that line.
func splitLines(text string) []string {
	lines := strings.Split(text, "\n")
	last := len(lines) - 1
	for i := 0; i < last; i++ {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	if last > 0 && lines[last] == "" { // the text ended with a line feed
		lines = lines[:last]
	}
	return lines
}

// cursor walks the lines of a message and counts them from 1.
type cursor struct {
	lines []string
	pos   int
}

func (c *cursor) number() int { return c.pos + 1 }
func (c *cursor) advance()    { c.pos++ }

// errorf makes a ParseError for the current line.
func (c *cursor) errorf(format string, args ...any) *ParseError {
	return errorAt(c.number(), format, args...)
}

// peek returns the current line without consuming it. It fails on a line that holds a carriage
// return, or that is too long.
func (c *cursor) peek() (line string, exists bool, err error) {
	if c.pos >= len(c.lines) {
		return "", false, nil
	}
	line = c.lines[c.pos]
	if strings.Contains(line, "\r") {
		return "", false, c.errorf("carriage return outside a line break")
	}
	if n := utf8.RuneCountInString(line); n > maxLineLength {
		return "", false, c.errorf("line has %d characters, the limit is %d", n, maxLineLength)
	}
	return line, true, nil
}

// need is peek for a line that must be there: the error says that the line of the given kind is
// missing.
func (c *cursor) need(what string) (string, error) {
	line, exists, err := c.peek()
	if err != nil {
		return "", err
	}
	if !exists {
		return "", c.errorf("missing %s line", what)
	}
	return line, nil
}

var (
	// An address is 3 letters of location, 2 of function and 2 of airline.
	address        = `[A-Z]{3}[A-Z]{2}[A-Z0-9]{2}`
	addressPattern = regexp.MustCompile(`^` + address + `$`)
	originPattern  = regexp.MustCompile(`^\.(` + address + `) ([0-9]{2})([0-9]{4})$`)
)

// singleSpaced says whether the fields of the line are separated by exactly one space: no leading or
// trailing space, no two spaces in a row, and no whitespace of another kind, such as a tab.
func singleSpaced(line string) bool {
	if strings.HasPrefix(line, " ") || strings.HasSuffix(line, " ") || strings.Contains(line, "  ") {
		return false
	}
	return !strings.ContainsFunc(line, func(r rune) bool { return r != ' ' && unicode.IsSpace(r) })
}

func readEnvelope(c *cursor, m *Message) error {
	if err := readPriority(c, m); err != nil {
		return err
	}
	return readOrigin(c, m)
}

func readPriority(c *cursor, m *Message) error {
	line, err := c.need("priority")
	if err != nil {
		return err
	}
	if !singleSpaced(line) {
		return c.errorf("%q: fields are separated by exactly one space, with no leading or trailing whitespace", line)
	}
	fields := strings.Split(line, " ")
	if len(fields) < 2 {
		return c.errorf("%q has no destination address", line)
	}
	switch fields[0] {
	case "QD", "QK", "QU":
		m.Priority = fields[0]
	default:
		return c.errorf("unknown priority code %q", fields[0])
	}
	for _, destination := range fields[1:] {
		if !addressPattern.MatchString(destination) {
			return c.errorf("address %q is not 3 letters of location, 2 of function and 2 of airline", destination)
		}
	}
	m.Destinations = fields[1:]
	c.advance()
	return nil
}

func readOrigin(c *cursor, m *Message) error {
	line, err := c.need("origin")
	if err != nil {
		return err
	}
	parts := originPattern.FindStringSubmatch(line)
	if parts == nil {
		return c.errorf("%q is not an origin line (a period, the origin address and ddhhmm)", line)
	}
	day, dayErr := parseDayOfMonth(parts[2])
	clock, clockErr := parseTimeOfDay(parts[3])
	if dayErr != nil || clockErr != nil {
		return c.errorf("%q has a timestamp that is not a day, hour and minute", line)
	}
	m.Origin, m.SentDay, m.SentHour, m.SentMinute = parts[1], day, clock.Hour, clock.Minute
	c.advance()
	return nil
}

func readBody(c *cursor, g grammar, m *Message) error {
	for _, rule := range g.lines {
		if !rule.applies(m) {
			continue
		}
		if err := readRule(c, rule, m); err != nil {
			return err
		}
	}
	if line, exists, err := c.peek(); err != nil {
		return err
	} else if exists {
		return c.errorf("unexpected line %q", line)
	}
	return nil
}

// readRule reads the lines that match the rule: one, or as many as there are when it repeats. The
// line that does not match ends the rule. That is an error unless the rule took a line or is
// optional.
func readRule(c *cursor, rule lineRule, m *Message) error {
	for matched := 0; ; matched++ {
		line, exists, err := c.peek()
		if err != nil {
			return err
		}
		var found []int
		if exists {
			found = rule.pattern.FindStringSubmatchIndex(line)
		}
		if found == nil {
			switch {
			case matched > 0 || rule.optional:
				return nil
			case exists:
				return c.errorf("%q is not a valid %s line", line, rule.name)
			}
			return c.errorf("missing %s line", rule.name)
		}
		values := rule.valuesOf(line, found)
		if err := rule.check(values, line, c.number()); err != nil {
			return err
		}
		if err := rule.checkAtMostOne(m, values, line, c.number()); err != nil {
			return err
		}
		m.lines = append(m.lines, bodyLine{c.number(), rule.name, values})
		c.advance()
		if !rule.repeat {
			return nil
		}
	}
}

// applies says whether the line is expected, given what the earlier lines captured.
func (rule lineRule) applies(m *Message) bool {
	return rule.when == nil || slices.Contains(rule.when.In, m.Value(rule.when.Capture))
}

// valuesOf returns the groups of the pattern that took part in the match, by name. found holds the
// start and end of the whole match and of each group, and -1 for a group that took no part.
func (rule lineRule) valuesOf(line string, found []int) map[string]string {
	values := map[string]string{}
	for i, name := range rule.pattern.SubexpNames() {
		if name != "" && found[2*i] >= 0 {
			values[name] = line[found[2*i]:found[2*i+1]]
		}
	}
	return values
}

// check applies the kinds and the not_before rules of the line to the values it captured.
func (rule lineRule) check(values map[string]string, line string, number int) error {
	for _, gk := range rule.kinds {
		value := values[gk.group]
		if value == "" {
			continue
		}
		if err := validators[gk.kind](value); err != nil {
			return errorAt(number, "invalid %s %q in %q", gk.kind, value, line)
		}
	}
	for _, pair := range rule.notBefore {
		laterText, earlierText := values[pair[0]], values[pair[1]]
		if laterText == "" || earlierText == "" {
			continue // a group that did not take part has no date to compare
		}
		later, err := dateOf(laterText, line, number)
		if err != nil {
			return err
		}
		earlier, err := dateOf(earlierText, line, number)
		if err != nil {
			return err
		}
		if later.Before(earlier) {
			return errorAt(number, "date %q is before %q in %q", laterText, earlierText, line)
		}
	}
	return nil
}

// dateOf reads the text of a date captured in the line, or fails on the line.
func dateOf(text, line string, number int) (time.Time, error) {
	date, err := parseDate(text)
	if err != nil {
		return time.Time{}, errorAt(number, "invalid date %q in %q", text, line)
	}
	return date, nil
}

// checkAtMostOne rejects the line when it makes a second of the at_most_one captures appear in the
// message: one given by an earlier line and one given by this line, or two on this line.
func (rule lineRule) checkAtMostOne(m *Message, values map[string]string, line string, number int) error {
	var given []string
	for _, name := range rule.atMostOne {
		if values[name] != "" || m.Value(name) != "" {
			given = append(given, name)
		}
	}
	if len(given) > 1 {
		return errorAt(number, "%s are both given, and only one is allowed, in %q", strings.Join(given, " and "), line)
	}
	return nil
}
