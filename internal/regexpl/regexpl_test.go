package regexpl

import "testing"

func TestExplainSimpleLiteral(t *testing.T) {
	r := Explain(`abc`)
	if !r.OK {
		t.Fatalf("expected ok, got %v", r.Error)
	}
	if len(r.Tree) != 1 || r.Tree[0].Kind != "literal" {
		t.Fatalf("expected single literal node, got %+v", r.Tree)
	}
	n := r.Tree[0]
	if n.Text != "abc" || n.Start != 0 || n.End != 3 {
		t.Fatalf("bad node: %+v", n)
	}
}

func TestExplainAnchorAndDot(t *testing.T) {
	r := Explain(`^a.c$`)
	if !r.OK {
		t.Fatalf("expected ok, got %v", r.Error)
	}
	if len(r.Tree) != 5 {
		t.Fatalf("expected 5 nodes (^, a, ., c, $), got %d", len(r.Tree))
	}
	if r.Tree[0].Kind != "anchor" || r.Tree[2].Kind != "dot" || r.Tree[4].Kind != "anchor" {
		t.Fatalf("expected anchor, literal, dot, literal, anchor; got %+v", r.Tree)
	}
}

func TestExplainEscapeKinds(t *testing.T) {
	r := Explain(`\d\w\s\b\.\n\x41`)
	if !r.OK {
		t.Fatalf("expected ok, got %v", r.Error)
	}
	// \d \w \s are escapes; \b is an anchor; \. and \n and \x41 are escapes.
	if r.Tree[0].Kind != "escape" || r.Tree[0].Detail == "" {
		t.Fatalf("expected escape \\d, got %+v", r.Tree[0])
	}
	if r.Tree[3].Kind != "anchor" {
		t.Fatalf("expected \\b to be an anchor, got %+v", r.Tree[3])
	}
}

func TestExplainAlternation(t *testing.T) {
	r := Explain(`cat|dog|bird`)
	if !r.OK {
		t.Fatalf("expected ok, got %v", r.Error)
	}
	if len(r.Tree) != 1 || r.Tree[0].Kind != "alternate" {
		t.Fatalf("expected single alternate node, got %+v", r.Tree)
	}
	if len(r.Tree[0].Children) != 3 {
		t.Fatalf("expected 3 branches, got %d", len(r.Tree[0].Children))
	}
}

func TestExplainNestedGroups(t *testing.T) {
	r := Explain(`((a)(b))`)
	if !r.OK {
		t.Fatalf("expected ok, got %v", r.Error)
	}
	if len(r.Tree) != 1 || r.Tree[0].Kind != "group" {
		t.Fatalf("expected outer group, got %+v", r.Tree)
	}
	outer := r.Tree[0]
	if len(outer.Children) != 1 {
		t.Fatalf("expected one child (inner concat), got %d", len(outer.Children))
	}
}

func TestExplainNonCapturingAndFlags(t *testing.T) {
	r := Explain(`(?:ab)(?i:cd)(?m)`)
	if !r.OK {
		t.Fatalf("expected ok, got %v", r.Error)
	}
	if len(r.Tree) != 3 {
		t.Fatalf("expected 3 group nodes, got %d", len(r.Tree))
	}
	if !contains(r.Tree[0].Detail, "non-capturing") {
		t.Fatalf("expected non-capturing, got %q", r.Tree[0].Detail)
	}
	if !contains(r.Tree[2].Detail, "flag group") {
		t.Fatalf("expected flag group, got %q", r.Tree[2].Detail)
	}
}

func TestErrLookahead(t *testing.T) {
	for _, p := range []string{`(?=x)`, `(?!x)`} {
		r := Explain(p)
		if r.OK || r.Error == nil || r.Error.Class != "unsupported_feature" {
			t.Fatalf("expected unsupported_feature for %q, got %+v", p, r)
		}
	}
}

func TestErrLookbehind(t *testing.T) {
	for _, p := range []string{`(?<=x)`, `(?<!x)`} {
		r := Explain(p)
		if r.OK || r.Error == nil || r.Error.Class != "unsupported_feature" {
			t.Fatalf("expected unsupported_feature for %q, got %+v", p, r)
		}
	}
}

func TestErrBackreference(t *testing.T) {
	for _, p := range []string{`(a)\1`, `\1`, `\9`, `(a)\8`} {
		r := Explain(p)
		if r.OK || r.Error == nil || r.Error.Class != "unsupported_feature" {
			t.Fatalf("expected unsupported_feature for %q, got %+v", p, r)
		}
	}
}

func TestOctalIsNotBackreference(t *testing.T) {
	// \12 is a valid octal escape, not a backreference.
	for _, p := range []string{`a\12b`, `\0`, `\012`, `\100`, `\77`} {
		r := Explain(p)
		if !r.OK {
			t.Fatalf("expected ok for octal %q, got %v", p, r.Error)
		}
	}
}

func TestErrDuplicateName(t *testing.T) {
	r := Explain(`(?P<x>a)(?P<x>b)`)
	if r.OK || r.Error == nil || r.Error.Class != "duplicate_name" {
		t.Fatalf("expected duplicate_name, got %+v", r)
	}
	// Python-style named groups should also be checked.
	r = Explain(`(?<x>a)(?<x>b)`)
	if r.OK || r.Error == nil || r.Error.Class != "duplicate_name" {
		t.Fatalf("expected duplicate_name for (?<x>), got %+v", r)
	}
}

func TestErrDanglingQuantifier(t *testing.T) {
	for _, p := range []string{`*`, `+abc`, `?x`, `a**`, `a++`, `a*+`, `a+*`, `(?:*)`, `a|*`, `{2}a`, `a{2}*`, `a*{2}`} {
		r := Explain(p)
		if r.OK || r.Error == nil || r.Error.Class != "dangling_quantifier" {
			t.Fatalf("expected dangling_quantifier for %q, got %+v", p, r)
		}
	}
}

func TestBraceLiteralsAreValid(t *testing.T) {
	// {abc} and {} are literal braces, not dangling.
	for _, p := range []string{`{abc}a`, `a{}`, `a{abc}`, `{abc}`} {
		r := Explain(p)
		if !r.OK {
			t.Fatalf("expected ok for %q, got %v", p, r.Error)
		}
	}
}

func TestMatchFirst(t *testing.T) {
	r := MatchFirst(`\d+`, "abc 123 def 456")
	if !r.OK || !r.Matched || len(r.Matches) != 1 {
		t.Fatalf("expected 1 match, got %+v", r)
	}
	m := r.Matches[0]
	if m.Start != 4 || m.End != 7 || m.Text != "123" {
		t.Fatalf("bad match: %+v", m)
	}
}

func TestMatchNoMatch(t *testing.T) {
	r := MatchFirst(`\d+`, "no digits here")
	if !r.OK || r.Matched {
		t.Fatalf("expected no match, got %+v", r)
	}
}

func TestMatchGlobal(t *testing.T) {
	r := MatchAll(`\d+`, "a1 b22 c333")
	if !r.OK || !r.Matched || len(r.Matches) != 3 {
		t.Fatalf("expected 3 matches, got %+v", r)
	}
	if r.Matches[0].Text != "1" || r.Matches[1].Text != "22" || r.Matches[2].Text != "333" {
		t.Fatalf("bad matches: %v %v %v", r.Matches[0].Text, r.Matches[1].Text, r.Matches[2].Text)
	}
}

func TestMatchEmptyAdvance(t *testing.T) {
	r := MatchAll(``, "abc")
	if !r.OK {
		t.Fatalf("expected ok, got %v", r.Error)
	}
	if len(r.Matches) != 4 {
		t.Fatalf("expected 4 empty matches, got %d", len(r.Matches))
	}
	for _, m := range r.Matches {
		if m.Text != "" || m.Start != m.End {
			t.Fatalf("expected empty match, got %+v", m)
		}
	}
}

func TestMatchCaptures(t *testing.T) {
	r := MatchFirst(`(\d{4})-(\d{2})-(\d{2})`, "date: 2026-08-15 end")
	if !r.OK || !r.Matched || len(r.Matches) != 1 {
		t.Fatalf("expected 1 match, got %+v", r)
	}
	m := r.Matches[0]
	if m.Text != "2026-08-15" || len(m.Groups) != 3 {
		t.Fatalf("bad match: %+v", m)
	}
	want := []string{"2026", "08", "15"}
	for i, w := range want {
		if m.Groups[i] == nil || m.Groups[i].Text != w || m.Groups[i].Index != i+1 {
			t.Fatalf("group %d: expected %q idx %d, got %+v", i+1, w, i+1, m.Groups[i])
		}
	}
}

func TestMatchNamedGroup(t *testing.T) {
	r := MatchFirst(`(?P<year>\d{4})`, "year 2026 done")
	if !r.OK || !r.Matched {
		t.Fatalf("expected match, got %+v", r)
	}
	m := r.Matches[0]
	if len(m.Groups) != 1 || m.Groups[0] == nil {
		t.Fatalf("expected 1 group, got %+v", m.Groups)
	}
	if m.Groups[0].Name != "year" || m.Groups[0].Text != "2026" {
		t.Fatalf("bad named group: %+v", m.Groups[0])
	}
}

func TestMatchUnmatchedGroup(t *testing.T) {
	r := MatchFirst(`(a)|(b)`, "b")
	if !r.OK || !r.Matched {
		t.Fatalf("expected match, got %+v", r)
	}
	m := r.Matches[0]
	if len(m.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(m.Groups))
	}
	if m.Groups[0] != nil {
		t.Fatalf("expected group 1 null, got %+v", m.Groups[0])
	}
	if m.Groups[1] == nil || m.Groups[1].Text != "b" {
		t.Fatalf("expected group 2 = b, got %+v", m.Groups[1])
	}
}

func TestMatchUnicodeByteOffsets(t *testing.T) {
	// "héllo" — é is two bytes in UTF-8. Match "llo" which starts at byte offset 3.
	r := MatchFirst(`llo`, "héllo")
	if !r.OK || !r.Matched || len(r.Matches) != 1 {
		t.Fatalf("expected match, got %+v", r)
	}
	m := r.Matches[0]
	if m.Start != 3 || m.End != 6 || m.Text != "llo" {
		t.Fatalf("expected byte offsets 3:6, got %+v", m)
	}
}

func TestMatchInvalidPatternReturnsError(t *testing.T) {
	r := MatchFirst(`(?=x)`, "abc")
	if r.OK || r.Error == nil || r.Error.Class != "unsupported_feature" {
		t.Fatalf("expected unsupported_feature, got %+v", r)
	}
}

func TestExplainPositionSpans(t *testing.T) {
	r := Explain(`a(bc)*`)
	if !r.OK {
		t.Fatalf("expected ok, got %v", r.Error)
	}
	if len(r.Tree) != 2 {
		t.Fatalf("expected 2 top-level nodes (a, (bc)*), got %d", len(r.Tree))
	}
	if r.Tree[0].Text != "a" || r.Tree[0].Start != 0 || r.Tree[0].End != 1 {
		t.Fatalf("bad first node: %+v", r.Tree[0])
	}
	q := r.Tree[1]
	if q.Kind != "quantifier" || q.Text != "(bc)*" || q.Start != 1 || q.End != 6 {
		t.Fatalf("bad quantifier node: %+v", q)
	}
}
