package typebreader

import (
	"fmt"
	"maps"
	"regexp"
	"slices"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

// What the HCL of a grammar file decodes into.
type grammarFile struct {
	Messages []messageSpec `hcl:"message,block"`
}

type messageSpec struct {
	Type  string     `hcl:"type,label"`
	Lines []lineSpec `hcl:"line,block"`
}

type lineSpec struct {
	Name      string            `hcl:"name,label"`
	Pattern   string            `hcl:"pattern"`
	Kinds     map[string]string `hcl:"kinds,optional"`
	NotBefore map[string]string `hcl:"not_before,optional"`
	AtMostOne []string          `hcl:"at_most_one,optional"`
	Repeat    bool              `hcl:"repeat,optional"`
	Optional  bool              `hcl:"optional,optional"`
	When      *whenSpec         `hcl:"when,block"`
}

type whenSpec struct {
	Capture string   `hcl:"capture"`
	In      []string `hcl:"in"`
}

// grammar is the checked form of a message type: its lines, in the order they must appear.
type grammar struct{ lines []lineRule }

type lineRule struct {
	name      string
	pattern   *regexp.Regexp
	kinds     []groupKind // sorted by group name, so the first error reported is always the same
	notBefore [][2]string // pairs {later, earlier}, sorted by later, so the first error reported is always the same
	atMostOne []string    // captures of which at most one may be given in the whole message
	repeat    bool
	optional  bool
	when      *whenSpec
}

type groupKind struct{ group, kind string }

// compileGrammars reads the text of a grammar file and checks every message type in it.
func compileGrammars(source string) (map[string]grammar, error) {
	var file grammarFile
	if err := hclsimple.Decode("grammar.hcl", []byte(source), nil, &file); err != nil {
		return nil, fmt.Errorf("reading grammar: %w", err)
	}
	if len(file.Messages) == 0 {
		return nil, fmt.Errorf("grammar defines no message block")
	}
	grammars := map[string]grammar{}
	for _, spec := range file.Messages {
		if _, dup := grammars[spec.Type]; dup {
			return nil, fmt.Errorf("message %s is defined twice", spec.Type)
		}
		g, err := compileMessage(spec)
		if err != nil {
			return nil, fmt.Errorf("message %s: %w", spec.Type, err)
		}
		grammars[spec.Type] = g
	}
	return grammars, nil
}

func compileMessage(spec messageSpec) (grammar, error) {
	if len(spec.Lines) == 0 {
		return grammar{}, fmt.Errorf("it has no lines")
	}
	var g grammar
	var earlierGroups []string
	for _, ls := range spec.Lines {
		rule, err := compileLine(ls, earlierGroups)
		if err != nil {
			return grammar{}, fmt.Errorf("line %s: %w", ls.Name, err)
		}
		g.lines = append(g.lines, rule)
		earlierGroups = append(earlierGroups, namedGroups(rule.pattern)...)
	}
	return g, nil
}

// namedGroups returns the names of the groups of the pattern that have one.
func namedGroups(re *regexp.Regexp) []string {
	return slices.DeleteFunc(slices.Clone(re.SubexpNames()), func(name string) bool { return name == "" })
}

func compileLine(ls lineSpec, earlierGroups []string) (lineRule, error) {
	// The pattern is compiled as written first, so that an error quotes it as the author wrote it
	// and a pattern that only balances its parentheses once wrapped is still refused.
	if _, err := regexp.Compile(ls.Pattern); err != nil {
		return lineRule{}, fmt.Errorf("pattern: %w", err)
	}
	re, err := regexp.Compile(`^(?:` + ls.Pattern + `)$`) // a line must match entirely
	if err != nil {
		return lineRule{}, fmt.Errorf("pattern: %w", err)
	}
	groups := namedGroups(re)
	rule := lineRule{name: ls.Name, pattern: re, repeat: ls.Repeat, optional: ls.Optional, when: ls.When}

	for _, group := range slices.Sorted(maps.Keys(ls.Kinds)) {
		kind := ls.Kinds[group]
		if _, known := validators[kind]; !known {
			return lineRule{}, fmt.Errorf("unknown kind %q", kind)
		}
		if !slices.Contains(groups, group) {
			return lineRule{}, fmt.Errorf("kind for %q, which is not a group of the pattern", group)
		}
		rule.kinds = append(rule.kinds, groupKind{group, kind})
	}

	for _, later := range slices.Sorted(maps.Keys(ls.NotBefore)) {
		earlier := ls.NotBefore[later]
		for _, group := range []string{later, earlier} {
			if ls.Kinds[group] != "date" {
				return lineRule{}, fmt.Errorf("not_before uses %q, which is not a date group", group)
			}
		}
		rule.notBefore = append(rule.notBefore, [2]string{later, earlier})
	}

	if len(ls.AtMostOne) > 0 {
		if err := validateAtMostOne(ls.AtMostOne, groups, earlierGroups); err != nil {
			return lineRule{}, err
		}
		rule.atMostOne = ls.AtMostOne
	}

	if ls.When != nil && !slices.Contains(earlierGroups, ls.When.Capture) {
		return lineRule{}, fmt.Errorf("when refers to %q, which no earlier line captures", ls.When.Capture)
	}
	return rule, nil
}

// validateAtMostOne checks that the names are at least two captures of the message so far, and that
// the line holds at least one of them: the error is reported on the line that completes the clash.
func validateAtMostOne(names, groups, earlierGroups []string) error {
	if len(names) < 2 {
		return fmt.Errorf("at_most_one needs at least two names")
	}
	seen := map[string]bool{}
	inThisLine := false
	for _, name := range names {
		if seen[name] {
			return fmt.Errorf("at_most_one names %q more than once", name)
		}
		seen[name] = true
		switch {
		case slices.Contains(groups, name):
			inThisLine = true
		case !slices.Contains(earlierGroups, name):
			return fmt.Errorf("at_most_one refers to %q, which neither this line nor an earlier line captures", name)
		}
	}
	if !inThisLine {
		return fmt.Errorf("at_most_one names no capture of this line, so it has no line to report")
	}
	return nil
}
