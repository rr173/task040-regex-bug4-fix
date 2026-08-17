package regexpl

import "fmt"

// scanUnsupportedAndDupes performs a single escape-aware pass over the pattern
// to detect conditions that regexp.Compile either misreports or does not report
// at all:
//
//   - unsupported_feature: lookahead (?= / (?!), lookbehind (?<= / (?<!), and
//     named backreferences (?P=. regexp reports lookbehind as "invalid named
//     capture" and (?P= as "unsupported Perl syntax: (?P", both of which are
//     misleading, so they are detected here instead.
//   - duplicate_name: two capture groups with the same name. Go's regexp
//     silently allows this (later definitions shadow earlier ones), so it must
//     be detected here.
//
// Backreferences of the form \1..\9 and dangling quantifiers are left to
// regexp.Compile and classified in classifyCompileError.
func scanUnsupportedAndDupes(src string) *MatchError {
	n := len(src)
	i := 0
	inClass := false
	names := map[string]bool{}

	for i < n {
		c := src[i]

		if inClass {
			if c == '\\' {
				i += 2
				continue
			}
			if c == ']' {
				inClass = false
			}
			i++
			continue
		}

		switch c {
		case '\\':
			// Skip the escaped character. If the backslash is the last byte,
			// regexp.Compile will report a parse error.
			i += 2
		case '[':
			inClass = true
			i++
			// A ']' immediately after '[' or '[^' is a literal member.
			if i < n && src[i] == '^' {
				i++
			}
			if i < n && src[i] == ']' {
				i++
			}
		case '(':
			if i+1 < n && src[i+1] == '?' {
				handled, err := scanGroupHeader(src, &i, names)
				if err != nil {
					return err
				}
				if !handled {
					// Malformed (?...; let regexp classify it.
					i++
				}
				continue
			}
			// Plain capturing group: enter it.
			i++
		default:
			i++
		}
	}
	return nil
}

// scanGroupHeader inspects the bytes following "(?". It advances *i past the
// group header (but not past the group body) for the constructs it recognizes,
// and records named groups for duplicate detection. It returns (handled, err):
// handled is false when the header is unrecognized and should be left to
// regexp.
func scanGroupHeader(src string, i *int, names map[string]bool) (bool, *MatchError) {
	// *i currently points at '?'. The caller already consumed '('.
	n := len(src)
	k := *i + 2 // index of the char after '?'
	if k >= n {
		return false, nil
	}
	switch src[k] {
	case '=', '!':
		kind := "lookahead"
		if src[k] == '!' {
			kind = "negative lookahead"
		}
		return false, &MatchError{
			Class:  "unsupported_feature",
			Reason: fmt.Sprintf("%s (?%c) is not supported: RE2 has no lookahead/lookbehind", kind, src[k]),
		}
	case '<':
		if k+1 < n && (src[k+1] == '=' || src[k+1] == '!') {
			kind := "lookbehind"
			if src[k+1] == '!' {
				kind = "negative lookbehind"
			}
			return false, &MatchError{
				Class:  "unsupported_feature",
				Reason: fmt.Sprintf("%s (?<%c) is not supported: RE2 has no lookahead/lookbehind", kind, src[k+1]),
			}
		}
		// (?<name>...) Python-style named group.
		name, end, ok := extractName(src, k+1)
		if !ok {
			return false, nil // malformed; let regexp classify
		}
		if names[name] {
			return false, &MatchError{
				Class:  "duplicate_name",
				Reason: fmt.Sprintf("duplicate capture group name %q", name),
			}
		}
		names[name] = true
		*i = end
		return true, nil
	case 'P':
		if k+1 < n && src[k+1] == '=' {
			return false, &MatchError{
				Class:  "unsupported_feature",
				Reason: "named backreference (?P=...) is not supported: RE2 has no backreferences",
			}
		}
		if k+1 < n && src[k+1] == '<' {
			name, end, ok := extractName(src, k+2)
			if !ok {
				return false, nil
			}
			if names[name] {
				return false, &MatchError{
					Class:  "duplicate_name",
					Reason: fmt.Sprintf("duplicate capture group name %q", name),
				}
			}
			names[name] = true
			*i = end
			return true, nil
		}
		// Unrecognized (?P form; let regexp classify.
		return false, nil
	default:
		// (?flags:...) or (?flags). Flag characters are scanned up to ':' or ')'.
		flagStart := k
		for k < n && src[k] != ':' && src[k] != ')' {
			k++
		}
		if k >= n {
			return false, nil // malformed; let regexp classify
		}
		_ = flagStart // flags themselves are not validated here
		if src[k] == ')' {
			*i = k + 1 // zero-width flag group consumed entirely
			return true, nil
		}
		// (?flags:...) — enter the group body.
		*i = k + 1
		return true, nil
	}
}

// extractName reads a group name starting at nameStart (the byte after '<')
// up to the closing '>'. It returns (name, indexPastCloseBracket, ok).
func extractName(src string, nameStart int) (string, int, bool) {
	n := len(src)
	j := nameStart
	for j < n && src[j] != '>' {
		j++
	}
	if j >= n {
		return "", n, false
	}
	return src[nameStart:j], j + 1, true
}
