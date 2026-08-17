package regexpl

import (
	"fmt"
	"reflect"
)

// SelfCheck runs an in-process battery of checks covering valid explanation,
// each classified error, single/global matching, capture groups, and empty-match
// advancement. It returns nil if every check passes.
func SelfCheck() error {
	checks := []struct {
		name string
		fn   func() error
	}{
		{"explain_simple", checkExplainSimple},
		{"explain_quantifier_greedy_lazy", checkExplainQuantifier},
		{"explain_groups", checkExplainGroups},
		{"explain_charclass", checkExplainCharClass},
		{"err_unsupported_lookahead", checkErrLookahead},
		{"err_unsupported_lookbehind", checkErrLookbehind},
		{"err_unsupported_backref", checkErrBackref},
		{"err_duplicate_name", checkErrDuplicateName},
		{"err_dangling_start", checkErrDanglingStart},
		{"err_dangling_nested", checkErrDanglingNested},
		{"err_dangling_braces", checkErrDanglingBraces},
		{"match_first", checkMatchFirst},
		{"match_global", checkMatchGlobal},
		{"match_empty_advance", checkMatchEmptyAdvance},
		{"match_captures", checkMatchCaptures},
		{"match_unmatched_group", checkMatchUnmatchedGroup},
		{"octal_not_backref", checkOctalNotBackref},
	}
	for _, c := range checks {
		if err := c.fn(); err != nil {
			return fmt.Errorf("%s: %w", c.name, err)
		}
	}
	return nil
}

func checkExplainSimple() error {
	res := Explain(`abc`)
	if !res.OK {
		return fmt.Errorf("expected ok, got error %v", res.Error)
	}
	if len(res.Tree) != 1 || res.Tree[0].Kind != "literal" {
		return fmt.Errorf("expected single literal node, got %+v", res.Tree)
	}
	if res.Tree[0].Text != "abc" || res.Tree[0].Start != 0 || res.Tree[0].End != 3 {
		return fmt.Errorf("bad literal node: %+v", res.Tree[0])
	}
	return nil
}

func checkExplainQuantifier() error {
	res := Explain(`a*b+c?d{2}e{3,}f{1,4}g*?`)
	if !res.OK {
		return fmt.Errorf("expected ok, got error %v", res.Error)
	}
	if len(res.Tree) != 7 {
		return fmt.Errorf("expected 7 nodes, got %d: %+v", len(res.Tree), res.Tree)
	}
	for _, n := range res.Tree {
		if n.Kind != "quantifier" {
			return fmt.Errorf("expected quantifier, got %s", n.Kind)
		}
		if len(n.Children) != 1 {
			return fmt.Errorf("quantifier should wrap one atom, got %d children", len(n.Children))
		}
	}
	// a* greedy, b+ greedy, c? greedy, d{2} exactly 2, e{3,} at least 3, f{1,4} between, g*? lazy.
	if !contains(res.Tree[0].Detail, "greedy") {
		return fmt.Errorf("expected a* greedy, got %q", res.Tree[0].Detail)
	}
	if !contains(res.Tree[6].Detail, "lazy") {
		return fmt.Errorf("expected g*? lazy, got %q", res.Tree[6].Detail)
	}
	if !contains(res.Tree[3].Detail, "exactly 2") {
		return fmt.Errorf("expected d{2} exactly 2, got %q", res.Tree[3].Detail)
	}
	if !contains(res.Tree[4].Detail, "at least 3") {
		return fmt.Errorf("expected e{3,} at least 3, got %q", res.Tree[4].Detail)
	}
	if !contains(res.Tree[5].Detail, "between 1 and 4") {
		return fmt.Errorf("expected f{1,4} between, got %q", res.Tree[5].Detail)
	}
	return nil
}

func checkExplainGroups() error {
	res := Explain(`(?P<year>\d{4})-(\d{2})`)
	if !res.OK {
		return fmt.Errorf("expected ok, got error %v", res.Error)
	}
	if len(res.Tree) != 3 {
		return fmt.Errorf("expected 3 top-level nodes, got %d", len(res.Tree))
	}
	if res.Tree[0].Kind != "group" || !contains(res.Tree[0].Detail, "named capture group") {
		return fmt.Errorf("expected named group, got %+v", res.Tree[0])
	}
	if !contains(res.Tree[0].Detail, "year") {
		return fmt.Errorf("expected group name 'year', got %q", res.Tree[0].Detail)
	}
	if res.Tree[1].Kind != "literal" || res.Tree[1].Text != "-" {
		return fmt.Errorf("expected literal '-', got %+v", res.Tree[1])
	}
	if res.Tree[2].Kind != "group" || !contains(res.Tree[2].Detail, "capturing group") {
		return fmt.Errorf("expected capturing group, got %+v", res.Tree[2])
	}
	return nil
}

func checkExplainCharClass() error {
	res := Explain(`[a-z0-9_]`)
	if !res.OK {
		return fmt.Errorf("expected ok, got error %v", res.Error)
	}
	if len(res.Tree) != 1 || res.Tree[0].Kind != "charClass" {
		return fmt.Errorf("expected charClass, got %+v", res.Tree)
	}
	if len(res.Tree[0].Children) != 3 {
		return fmt.Errorf("expected 3 class members, got %d", len(res.Tree[0].Children))
	}
	if res.Tree[0].Children[0].Kind != "range" || res.Tree[0].Children[0].Text != "a-z" {
		return fmt.Errorf("expected range a-z, got %+v", res.Tree[0].Children[0])
	}
	return nil
}

func errClass(pattern string) (string, error) {
	res := Explain(pattern)
	if res.OK {
		return "", fmt.Errorf("expected error for %q, got ok", pattern)
	}
	return res.Error.Class, nil
}

func checkErrLookahead() error {
	c, err := errClass(`(?=abc)`)
	if err != nil {
		return err
	}
	if c != "unsupported_feature" {
		return fmt.Errorf("expected unsupported_feature, got %s", c)
	}
	return nil
}

func checkErrLookbehind() error {
	c, err := errClass(`(?<=abc)`)
	if err != nil {
		return err
	}
	if c != "unsupported_feature" {
		return fmt.Errorf("expected unsupported_feature, got %s", c)
	}
	return nil
}

func checkErrBackref() error {
	c, err := errClass(`(a)\1`)
	if err != nil {
		return err
	}
	if c != "unsupported_feature" {
		return fmt.Errorf("expected unsupported_feature, got %s", c)
	}
	return nil
}

func checkErrDuplicateName() error {
	c, err := errClass(`(?P<x>a)(?P<x>b)`)
	if err != nil {
		return err
	}
	if c != "duplicate_name" {
		return fmt.Errorf("expected duplicate_name, got %s", c)
	}
	return nil
}

func checkErrDanglingStart() error {
	c, err := errClass(`*abc`)
	if err != nil {
		return err
	}
	if c != "dangling_quantifier" {
		return fmt.Errorf("expected dangling_quantifier, got %s", c)
	}
	return nil
}

func checkErrDanglingNested() error {
	for _, p := range []string{`a**`, `a++`, `a*+`, `a+*`} {
		c, err := errClass(p)
		if err != nil {
			return err
		}
		if c != "dangling_quantifier" {
			return fmt.Errorf("expected dangling_quantifier for %q, got %s", p, c)
		}
	}
	return nil
}

func checkErrDanglingBraces() error {
	c, err := errClass(`{2}a`)
	if err != nil {
		return err
	}
	if c != "dangling_quantifier" {
		return fmt.Errorf("expected dangling_quantifier, got %s", c)
	}
	return nil
}

func checkMatchFirst() error {
	res := MatchFirst(`\d+`, "abc 123 def 456")
	if !res.OK {
		return fmt.Errorf("expected ok, got %v", res.Error)
	}
	if !res.Matched || len(res.Matches) != 1 {
		return fmt.Errorf("expected 1 match, got matched=%v len=%d", res.Matched, len(res.Matches))
	}
	m := res.Matches[0]
	if m.Start != 4 || m.End != 7 || m.Text != "123" {
		return fmt.Errorf("bad match: %+v", m)
	}
	return nil
}

func checkMatchGlobal() error {
	res := MatchAll(`\d+`, "a1 b22 c333")
	if !res.OK || !res.Matched || len(res.Matches) != 3 {
		return fmt.Errorf("expected 3 global matches, got %+v", res)
	}
	want := []string{"1", "22", "333"}
	got := []string{res.Matches[0].Text, res.Matches[1].Text, res.Matches[2].Text}
	if !reflect.DeepEqual(want, got) {
		return fmt.Errorf("expected %v, got %v", want, got)
	}
	return nil
}

func checkMatchEmptyAdvance() error {
	// Empty match pattern: between every pair of runes there is an empty match.
	// Ensure it terminates and advances one rune at a time (no infinite loop).
	res := MatchAll(``, "abc")
	if !res.OK {
		return fmt.Errorf("expected ok, got %v", res.Error)
	}
	// "abc" yields 4 empty matches: before a, between a-b, between b-c, after c.
	if len(res.Matches) != 4 {
		return fmt.Errorf("expected 4 empty matches, got %d", len(res.Matches))
	}
	for _, m := range res.Matches {
		if m.Text != "" {
			return fmt.Errorf("expected empty match text, got %q", m.Text)
		}
		if m.Start != m.End {
			return fmt.Errorf("expected empty match span, got %d:%d", m.Start, m.End)
		}
	}
	return nil
}

func checkMatchCaptures() error {
	res := MatchFirst(`(\d{4})-(\d{2})-(\d{2})`, "date: 2026-08-15 end")
	if !res.OK || !res.Matched || len(res.Matches) != 1 {
		return fmt.Errorf("expected 1 match, got %+v", res)
	}
	m := res.Matches[0]
	if m.Text != "2026-08-15" {
		return fmt.Errorf("bad match text %q", m.Text)
	}
	if len(m.Groups) != 3 {
		return fmt.Errorf("expected 3 groups, got %d", len(m.Groups))
	}
	want := []string{"2026", "08", "15"}
	for i, w := range want {
		if m.Groups[i] == nil {
			return fmt.Errorf("group %d is nil", i+1)
		}
		if m.Groups[i].Text != w {
			return fmt.Errorf("group %d: expected %q, got %q", i+1, w, m.Groups[i].Text)
		}
		if m.Groups[i].Index != i+1 {
			return fmt.Errorf("group %d: bad index %d", i+1, m.Groups[i].Index)
		}
	}
	return nil
}

func checkMatchUnmatchedGroup() error {
	// Alternation with capture groups: only one side participates.
	res := MatchFirst(`(a)|(b)`, "b")
	if !res.OK || !res.Matched {
		return fmt.Errorf("expected match, got %+v", res)
	}
	m := res.Matches[0]
	if len(m.Groups) != 2 {
		return fmt.Errorf("expected 2 groups, got %d", len(m.Groups))
	}
	if m.Groups[0] != nil {
		return fmt.Errorf("expected group 1 to be null (unmatched), got %+v", m.Groups[0])
	}
	if m.Groups[1] == nil || m.Groups[1].Text != "b" {
		return fmt.Errorf("expected group 2 to match 'b', got %+v", m.Groups[1])
	}
	return nil
}

func checkOctalNotBackref() error {
	// \12 is a valid octal escape (byte 10), not a backreference.
	res := Explain(`a\12b`)
	if !res.OK {
		return fmt.Errorf("expected ok for octal \\12, got %v", res.Error)
	}
	// \1 (single digit, not octal) is a backreference → unsupported.
	res = Explain(`a\1b`)
	if res.OK || res.Error == nil || res.Error.Class != "unsupported_feature" {
		return fmt.Errorf("expected unsupported_feature for \\1, got %+v", res)
	}
	// Matching with the octal pattern should work (byte 0x0a is newline).
	mres := MatchFirst(`a\12b`, "a\nb")
	if !mres.OK || !mres.Matched {
		return fmt.Errorf("expected match for a\\12b against a\\nb, got %+v", mres)
	}
	return nil
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
