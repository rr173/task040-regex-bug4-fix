package regexpl

import "fmt"

// parseTree builds a human-readable syntax tree for a pattern that has already
// been validated by regexp.Compile. It returns a parse_error only for patterns
// that are valid RE2 but contain constructs the explainer does not model; the
// common subset (literals, escapes, character classes, quantifiers, anchors,
// groups, alternation, inline flags) is fully supported.
func parseTree(src string) ([]*ExplainNode, *MatchError) {
	p := &parser{src: src}
	nodes, err := p.parseAlternate()
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.src) {
		return nil, &MatchError{
			Class:  "parse_error",
			Reason: fmt.Sprintf("unexpected %q at position %d", p.src[p.pos], p.pos),
		}
	}
	return nodes, nil
}

type parser struct {
	src string
	pos int
}

// parseAlternate parses a top-level alternation. It returns a concat-level
// list: either the single branch's nodes, or a single "alternate" node.
func (p *parser) parseAlternate() ([]*ExplainNode, *MatchError) {
	start := p.pos
	first, err := p.parseConcat()
	if err != nil {
		return nil, err
	}
	if p.pos >= len(p.src) || p.src[p.pos] != '|' {
		return first, nil
	}
	branches := []*ExplainNode{wrapConcat(first, start, p.pos)}
	for p.pos < len(p.src) && p.src[p.pos] == '|' {
		branchStart := p.pos
		p.pos++ // consume '|'
		b, err := p.parseConcat()
		if err != nil {
			return nil, err
		}
		branches = append(branches, wrapConcat(b, branchStart, p.pos))
	}
	alt := &ExplainNode{
		Kind:     "alternate",
		Text:     p.src[start:p.pos],
		Start:    start,
		End:      p.pos,
		Detail:   fmt.Sprintf("alternation of %d branches", len(branches)),
		Children: branches,
	}
	return []*ExplainNode{alt}, nil
}

// parseConcat parses a sequence of repeated atoms up to '|' or ')'.
func (p *parser) parseConcat() ([]*ExplainNode, *MatchError) {
	var nodes []*ExplainNode
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == '|' || c == ')' {
			break
		}
		atom, err := p.parseRepeat()
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, atom)
	}
	return mergeLiterals(nodes), nil
}

// parseRepeat parses a single atom followed by an optional quantifier. When a
// quantifier is present, the result is a "quantifier" node whose single child
// is the atom it modifies.
func (p *parser) parseRepeat() (*ExplainNode, *MatchError) {
	atomStart := p.pos
	atom, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	_, qDetail, ok := p.tryQuantifier()
	if !ok {
		return atom, nil
	}
	return &ExplainNode{
		Kind:     "quantifier",
		Text:     p.src[atomStart:p.pos],
		Start:    atomStart,
		End:      p.pos,
		Detail:   qDetail,
		Children: []*ExplainNode{atom},
	}, nil
}

// tryQuantifier consumes a quantifier if one is present at p.pos and returns
// its text and a human-readable detail. A '{' that does not form a valid
// repeat (no closing '}' or invalid body) is treated as a literal and not
// consumed.
func (p *parser) tryQuantifier() (text, detail string, ok bool) {
	if p.pos >= len(p.src) {
		return "", "", false
	}
	start := p.pos
	switch p.src[p.pos] {
	case '*', '+', '?':
		op := p.src[p.pos]
		p.pos++
		greedy := true
		if p.pos < len(p.src) && p.src[p.pos] == '?' {
			p.pos++
			greedy = false
		}
		return p.src[start:p.pos], quantDetailSimple(op, greedy), true
	case '{':
		j := p.pos + 1
		bodyStart := j
		for j < len(p.src) && p.src[j] != '}' {
			j++
		}
		if j >= len(p.src) {
			return "", "", false // no closing '}': literal '{'
		}
		body := p.src[bodyStart:j]
		min, max, hasMax, valid := parseRepeatBody(body)
		if !valid {
			return "", "", false // invalid body: literal '{'
		}
		p.pos = j + 1 // consume '}'
		greedy := true
		if p.pos < len(p.src) && p.src[p.pos] == '?' {
			p.pos++
			greedy = false
		}
		return p.src[start:p.pos], quantDetailRepeat(min, max, hasMax, greedy), true
	}
	return "", "", false
}

func quantDetailSimple(op byte, greedy bool) string {
	var rangeText string
	switch op {
	case '*':
		rangeText = "zero or more"
	case '+':
		rangeText = "one or more"
	case '?':
		rangeText = "zero or one"
	}
	mode := "greedy"
	if !greedy {
		mode = "lazy"
	}
	return fmt.Sprintf("repeat %s (%s)", rangeText, mode)
}

func quantDetailRepeat(min int, max int, hasMax bool, greedy bool) string {
	var rangeText string
	switch {
	case !hasMax:
		rangeText = fmt.Sprintf("at least %d", min)
	case min == max:
		rangeText = fmt.Sprintf("exactly %d", min)
	default:
		rangeText = fmt.Sprintf("between %d and %d", min, max)
	}
	mode := "greedy"
	if !greedy {
		mode = "lazy"
	}
	return fmt.Sprintf("repeat %s (%s)", rangeText, mode)
}

// parseRepeatBody parses a {n}, {n,}, or {n,m} body. It returns
// (min, max, hasMax, valid). hasMax is false for {n,} (unbounded).
func parseRepeatBody(body string) (min, max int, hasMax, valid bool) {
	if body == "" {
		return 0, 0, false, false
	}
	i := 0
	for i < len(body) && body[i] >= '0' && body[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, 0, false, false
	}
	min = atoi(body[:i])
	if i == len(body) {
		return min, min, true, true // {n}
	}
	if body[i] != ',' {
		return 0, 0, false, false
	}
	i++ // skip ','
	if i == len(body) {
		return min, 0, false, true // {n,}
	}
	j := i
	for i < len(body) && body[i] >= '0' && body[i] <= '9' {
		i++
	}
	if i != len(body) {
		return 0, 0, false, false
	}
	max = atoi(body[j:i])
	if max < min {
		return 0, 0, false, false // regexp rejects this; treat as non-quantifier here
	}
	return min, max, true, true
}

func atoi(s string) int {
	v := 0
	for i := 0; i < len(s); i++ {
		v = v*10 + int(s[i]-'0')
	}
	return v
}

// parseAtom parses a single non-quantifier atom.
func (p *parser) parseAtom() (*ExplainNode, *MatchError) {
	if p.pos >= len(p.src) {
		return nil, &MatchError{Class: "parse_error", Reason: "unexpected end of pattern"}
	}
	start := p.pos
	switch p.src[p.pos] {
	case '(':
		return p.parseGroup()
	case '[':
		return p.parseCharClass()
	case '\\':
		return p.parseEscape()
	case '.':
		p.pos++
		return &ExplainNode{
			Kind:   "dot",
			Text:   ".",
			Start:  start,
			End:    p.pos,
			Detail: "any character (excluding newline unless dotall)",
		}, nil
	case '^':
		p.pos++
		return &ExplainNode{
			Kind:   "anchor",
			Text:   "^",
			Start:  start,
			End:    p.pos,
			Detail: "start of line (or start of text)",
		}, nil
	case '$':
		p.pos++
		return &ExplainNode{
			Kind:   "anchor",
			Text:   "$",
			Start:  start,
			End:    p.pos,
			Detail: "end of line (or end of text)",
		}, nil
	default:
		// A single literal character (one per atom so quantifiers attach to the
		// correct character; consecutive literals are merged in parseConcat).
		ch := p.src[p.pos]
		p.pos++
		return &ExplainNode{
			Kind:   "literal",
			Text:   string(ch),
			Start:  start,
			End:    p.pos,
			Detail: fmt.Sprintf("literal %q", string(ch)),
		}, nil
	}
}

// parseGroup parses (...), (?:...), (?P<name>...), (?<name>...), (?flags:...),
// and (?flags).
func (p *parser) parseGroup() (*ExplainNode, *MatchError) {
	start := p.pos
	p.pos++ // consume '('
	nonCap := false
	name := ""
	flags := ""
	if p.pos < len(p.src) && p.src[p.pos] == '?' {
		p.pos++ // consume '?'
		if p.pos >= len(p.src) {
			return nil, &MatchError{Class: "parse_error", Reason: "unterminated group flag"}
		}
		switch p.src[p.pos] {
		case ':':
			p.pos++ // consume ':'
			nonCap = true
		case 'P':
			p.pos++ // consume 'P'
			if p.pos < len(p.src) && p.src[p.pos] == '<' {
				p.pos++ // consume '<'
				extracted, end, ok := extractName(p.src, p.pos)
				if !ok {
					return nil, &MatchError{Class: "parse_error", Reason: "unterminated named group"}
				}
				name = extracted
				p.pos = end
			}
		case '<':
			p.pos++ // consume '<'
			if p.pos < len(p.src) && (p.src[p.pos] == '=' || p.src[p.pos] == '!') {
				// Lookbehind: pre-scan should have caught this; defensive.
				return nil, &MatchError{Class: "unsupported_feature", Reason: "lookbehind is not supported"}
			}
			extracted, end, ok := extractName(p.src, p.pos)
			if !ok {
				return nil, &MatchError{Class: "parse_error", Reason: "unterminated named group"}
			}
			name = extracted
			p.pos = end
		default:
			// (?flags:...) or (?flags)
			flagStart := p.pos
			for p.pos < len(p.src) && p.src[p.pos] != ':' && p.src[p.pos] != ')' {
				p.pos++
			}
			flags = p.src[flagStart:p.pos]
			if p.pos >= len(p.src) {
				return nil, &MatchError{Class: "parse_error", Reason: "unterminated flag group"}
			}
			if p.src[p.pos] == ')' {
				p.pos++ // consume ')'
				return &ExplainNode{
					Kind:   "group",
					Text:   p.src[start:p.pos],
					Start:  start,
					End:    p.pos,
					Detail: fmt.Sprintf("flag group (flags: %s)", flags),
				}, nil
			}
			p.pos++ // consume ':'
			nonCap = true
		}
	}

	inner, err := p.parseAlternate()
	if err != nil {
		return nil, err
	}
	if p.pos >= len(p.src) || p.src[p.pos] != ')' {
		return nil, &MatchError{Class: "parse_error", Reason: "missing closing ')'"}
	}
	p.pos++ // consume ')'

	var detail string
	switch {
	case name != "":
		detail = fmt.Sprintf("named capture group %q", name)
	case nonCap:
		detail = "non-capturing group"
	case flags != "":
		detail = fmt.Sprintf("non-capturing group (flags: %s)", flags)
	default:
		detail = "capturing group"
	}
	innerNode := wrapConcat(inner, start+1, p.pos-1)
	return &ExplainNode{
		Kind:     "group",
		Text:     p.src[start:p.pos],
		Start:    start,
		End:      p.pos,
		Detail:   detail,
		Children: []*ExplainNode{innerNode},
	}, nil
}

// parseCharClass parses a [...] character class into a node whose children are
// the individual members (ranges, single characters, and escapes).
func (p *parser) parseCharClass() (*ExplainNode, *MatchError) {
	start := p.pos
	p.pos++ // consume '['
	negated := false
	if p.pos < len(p.src) && p.src[p.pos] == '^' {
		negated = true
		p.pos++
	}
	var children []*ExplainNode
	first := true
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == ']' && !first {
			break
		}
		first = false
		if c == '\\' {
			esc, err := p.parseEscape()
			if err != nil {
				return nil, err
			}
			children = append(children, esc)
			continue
		}
		chStart := p.pos
		p.pos++
		// Range: ch '-' endChar, where endChar is not ']'.
		if p.pos+1 < len(p.src) && p.src[p.pos] == '-' && p.src[p.pos+1] != ']' {
			p.pos++ // consume '-'
			if p.src[p.pos] == '\\' {
				p.pos++
				if p.pos < len(p.src) {
					p.pos++
				}
			} else {
				p.pos++
			}
			children = append(children, &ExplainNode{
				Kind:   "range",
				Text:   p.src[chStart:p.pos],
				Start:  chStart,
				End:    p.pos,
				Detail: fmt.Sprintf("range %s", p.src[chStart:p.pos]),
			})
			continue
		}
		children = append(children, &ExplainNode{
			Kind:   "char",
			Text:   string(c),
			Start:  chStart,
			End:    p.pos,
			Detail: fmt.Sprintf("character %q", string(c)),
		})
	}
	if p.pos >= len(p.src) {
		return nil, &MatchError{Class: "parse_error", Reason: "unterminated character class"}
	}
	p.pos++ // consume ']'
	detail := "character class"
	if negated {
		detail = "negated character class"
	}
	return &ExplainNode{
		Kind:     "charClass",
		Text:     p.src[start:p.pos],
		Start:    start,
		End:      p.pos,
		Detail:   detail,
		Children: children,
	}, nil
}

// parseEscape parses a backslash escape.
func (p *parser) parseEscape() (*ExplainNode, *MatchError) {
	start := p.pos
	p.pos++ // consume '\'
	if p.pos >= len(p.src) {
		return nil, &MatchError{Class: "parse_error", Reason: "trailing backslash"}
	}
	c := p.src[p.pos]
	p.pos++
	kind := "escape"
	detail := ""
	switch c {
	case 'd':
		detail = "decimal digit [0-9]"
	case 'D':
		detail = "non-digit [^0-9]"
	case 'w':
		detail = "word character [0-9A-Za-z_]"
	case 'W':
		detail = "non-word character"
	case 's':
		detail = "whitespace"
	case 'S':
		detail = "non-whitespace"
	case 'b':
		kind = "anchor"
		detail = "word boundary"
	case 'B':
		kind = "anchor"
		detail = "non-word-boundary"
	case 'A':
		kind = "anchor"
		detail = "start of text"
	case 'z':
		kind = "anchor"
		detail = "end of text"
	case 'Z':
		kind = "anchor"
		detail = "end of text (before final newline)"
	case 'n':
		detail = "newline"
	case 't':
		detail = "tab"
	case 'r':
		detail = "carriage return"
	case 'f':
		detail = "form feed"
	case 'v':
		detail = "vertical tab"
	case 'a':
		detail = "bell"
	case '0', '1', '2', '3', '4', '5', '6', '7':
		// Octal escape: consume up to two further octal digits.
		for k := 0; k < 2 && p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '7'; k++ {
			p.pos++
		}
		detail = "octal escape"
	case 'x':
		for k := 0; k < 2 && p.pos < len(p.src) && isHex(p.src[p.pos]); k++ {
			p.pos++
		}
		detail = "hex escape"
	case 'u':
		for k := 0; k < 4 && p.pos < len(p.src) && isHex(p.src[p.pos]); k++ {
			p.pos++
		}
		detail = "unicode hex escape"
	case 'p', 'P':
		if p.pos < len(p.src) && p.src[p.pos] == '{' {
			p.pos++
			for p.pos < len(p.src) && p.src[p.pos] != '}' {
				p.pos++
			}
			if p.pos < len(p.src) {
				p.pos++ // consume '}'
			}
		}
		detail = "unicode category class"
	case 'Q':
		// Literal escape: consume up to \E.
		for p.pos+1 < len(p.src) {
			if p.src[p.pos] == '\\' && p.src[p.pos+1] == 'E' {
				p.pos += 2
				break
			}
			p.pos++
		}
		detail = "literal escape (\\Q...\\E)"
	case '8', '9':
		// Backreference-style escapes; the validator rejects these before
		// parsing, so reaching here is unexpected.
		return nil, &MatchError{Class: "unsupported_feature", Reason: "backreferences are not supported"}
	default:
		detail = fmt.Sprintf("escaped literal %q", string(c))
	}
	return &ExplainNode{
		Kind:   kind,
		Text:   p.src[start:p.pos],
		Start:  start,
		End:    p.pos,
		Detail: detail,
	}, nil
}

func isHex(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

// wrapConcat wraps a list of sibling nodes into a single concat node, unless
// the list contains exactly one node, in which case that node is returned
// directly. An empty list becomes an "empty" node.
func wrapConcat(nodes []*ExplainNode, start, end int) *ExplainNode {
	switch len(nodes) {
	case 0:
		return &ExplainNode{
			Kind:   "empty",
			Text:   "",
			Start:  start,
			End:    end,
			Detail: "empty alternative",
		}
	case 1:
		return nodes[0]
	default:
		return &ExplainNode{
			Kind:     "concat",
			Text:     "", // span text is not always contiguous across children; omit
			Start:    start,
			End:      end,
			Detail:   "sequence",
			Children: nodes,
		}
	}
}

// mergeLiterals coalesces adjacent literal nodes into a single literal node so
// that runs of literal text appear as one tree node.
func mergeLiterals(nodes []*ExplainNode) []*ExplainNode {
	if len(nodes) == 0 {
		return nodes
	}
	out := make([]*ExplainNode, 0, len(nodes))
	for _, n := range nodes {
		last := len(out) > 0
		if last && out[len(out)-1].Kind == "literal" && n.Kind == "literal" {
			prev := out[len(out)-1]
			prev.Text = prev.Text + n.Text
			prev.End = n.End
			prev.Detail = fmt.Sprintf("literal %q", prev.Text)
			continue
		}
		out = append(out, n)
	}
	return out
}
