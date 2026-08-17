// Package regexpl implements a small regex test-and-explain service.
//
// It provides three capabilities over RE2 patterns (Go's standard library
// regexp is an RE2 implementation):
//
//   - Explain: parse a pattern into a human-readable syntax tree, recording
//     each component's kind, source span, and a description.
//   - MatchFirst / MatchAll: execute a pattern against an input text and report
//     match spans (byte offsets) together with capture group contents.
//
// Invalid patterns are reported with a classified error:
//
//   - unsupported_feature: lookahead/lookbehind assertions and backreferences,
//     which RE2 does not support.
//   - duplicate_name: two or more capture groups sharing the same name.
//   - dangling_quantifier: a quantifier with no preceding atom, or a quantifier
//     applied directly to another quantifier.
//   - parse_error: any other malformed pattern.
package regexpl

import (
	"fmt"
	"regexp"
	"strings"
)

// MatchError is a classified error describing why a pattern is invalid.
type MatchError struct {
	Class  string `json:"class"`
	Reason string `json:"reason"`
}

func (e *MatchError) Error() string {
	return fmt.Sprintf("%s: %s", e.Class, e.Reason)
}

// ExplainNode is one node of the parsed regex syntax tree.
type ExplainNode struct {
	Kind     string         `json:"kind"`
	Text     string         `json:"text"`
	Start    int            `json:"start"`
	End      int            `json:"end"`
	Detail   string         `json:"detail"`
	Children []*ExplainNode `json:"children,omitempty"`
}

// ExplainResult is the output of Explain.
type ExplainResult struct {
	OK    bool           `json:"ok"`
	Error *MatchError    `json:"error,omitempty"`
	Tree  []*ExplainNode `json:"tree,omitempty"`
}

// Group is one capture group of a match. A group that did not participate in
// the match is represented as a nil pointer (rendered as JSON null).
type Group struct {
	Name  string `json:"name"`
	Index int    `json:"index"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Text  string `json:"text"`
}

// Match is one match occurrence.
type Match struct {
	Start  int      `json:"start"`
	End    int      `json:"end"`
	Text   string   `json:"text"`
	Groups []*Group `json:"groups,omitempty"`
}

// MatchResult is the output of MatchFirst / MatchAll.
type MatchResult struct {
	OK      bool        `json:"ok"`
	Error   *MatchError `json:"error,omitempty"`
	Matched bool        `json:"matched"`
	Matches []Match     `json:"matches,omitempty"`
}

// Explain parses the pattern and returns a human-readable syntax tree.
func Explain(pattern string) ExplainResult {
	if e := validate(pattern); e != nil {
		return ExplainResult{OK: false, Error: e}
	}
	nodes, err := parseTree(pattern)
	if err != nil {
		return ExplainResult{OK: false, Error: err}
	}
	return ExplainResult{OK: true, Tree: nodes}
}

// MatchFirst returns the first match of pattern in input.
func MatchFirst(pattern, input string) MatchResult {
	if e := validate(pattern); e != nil {
		return MatchResult{OK: false, Error: e}
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return MatchResult{OK: false, Error: classifyCompileError(err)}
	}
	idx := re.FindStringSubmatchIndex(input)
	res := MatchResult{OK: true, Matched: idx != nil}
	if idx != nil {
		res.Matches = []Match{buildMatch(input, re, idx)}
	}
	return res
}

// MatchAll returns all non-overlapping matches of pattern in input. Empty
// matches advance by one rune to avoid an infinite loop (handled by the
// standard library matcher).
func MatchAll(pattern, input string) MatchResult {
	if e := validate(pattern); e != nil {
		return MatchResult{OK: false, Error: e}
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return MatchResult{OK: false, Error: classifyCompileError(err)}
	}
	all := re.FindAllStringSubmatchIndex(input, -1)
	res := MatchResult{OK: true, Matched: len(all) > 0}
	for _, idx := range all {
		res.Matches = append(res.Matches, buildMatch(input, re, idx))
	}
	return res
}

func buildMatch(input string, re *regexp.Regexp, idx []int) Match {
	m := Match{
		Start: idx[0],
		End:   idx[1],
		Text:  input[idx[0]:idx[1]],
	}
	names := re.SubexpNames()
	for gi := 1; gi < len(names); gi++ {
		s, e := idx[2*gi], idx[2*gi+1]
		if s < 0 || e < 0 {
			// Group did not participate in this match → JSON null.
			m.Groups = append(m.Groups, nil)
			continue
		}
		m.Groups = append(m.Groups, &Group{
			Name:  names[gi],
			Index: gi,
			Start: s,
			End:   e,
			Text:  input[s:e],
		})
	}
	return m
}

// validate checks a pattern for the classified error conditions. It returns
// nil when the pattern is a valid RE2 pattern that the explainer can handle.
func validate(pattern string) *MatchError {
	if e := scanUnsupportedAndDupes(pattern); e != nil {
		return e
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return classifyCompileError(err)
	}
	return nil
}

// classifyCompileError maps a regexp.Compile error to a MatchError class.
func classifyCompileError(err error) *MatchError {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "invalid or unsupported Perl syntax"):
		return &MatchError{Class: "unsupported_feature", Reason: msg}
	case isBackrefEscapeError(msg):
		return &MatchError{
			Class:  "unsupported_feature",
			Reason: "backreferences are not supported (RE2 has no backreferences): " + msg,
		}
	case strings.Contains(msg, "missing argument to repetition operator"),
		strings.Contains(msg, "invalid nested repetition operator"):
		return &MatchError{Class: "dangling_quantifier", Reason: msg}
	default:
		return &MatchError{Class: "parse_error", Reason: msg}
	}
}

// isBackrefEscapeError reports whether msg is a regexp error of the form
// "invalid escape sequence: `\d`" where d is a digit 1-9, which the spec
// treats as a backreference request. Multi-digit octal escapes such as `\12`
// compile successfully and never reach this path.
func isBackrefEscapeError(msg string) bool {
	const marker = "invalid escape sequence: `\\"
	i := strings.Index(msg, marker)
	if i < 0 {
		return false
	}
	pos := i + len(marker)
	if pos >= len(msg) {
		return false
	}
	c := msg[pos]
	return c >= '1' && c <= '9'
}
