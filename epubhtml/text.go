package epubhtml

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// TextMode selects the cleaning profile for a run of human text.
type TextMode int8

// Text modes. They differ only in how line structure is treated; the
// character policy (what is stripped vs kept) is shared.
const (
	// DocumentText cleans text destined for the document's text nodes.
	// Newlines and tabs survive as ordinary HTML whitespace.
	DocumentText TextMode = iota
	// HeaderValue cleans single-line mail header values (mailto subject).
	// Header injection lives in line breaks, so any line structure is
	// flattened to a single space.
	HeaderValue
	// BodyValue cleans the mailto body parameter: line breaks are
	// legitimate there (form-style templates parsed by the recipient) and
	// are normalized to the CRLF form RFC 6068 requires.
	BodyValue
)

// CleanText normalizes a run of human text according to mode. It strips
// what can only deceive — C0/C1 control characters, DEL, invalid UTF-8 and
// the bidi embedding/override marks U+202A–U+202E (the "Trojan Source"
// family) — while keeping invisible characters that real scripts require:
// ZWNJ/ZWJ (Persian needs ZWNJ, Indic ligatures and emoji sequences need
// ZWJ) and the modern bidi isolates U+2066–U+2069, which mix directions
// without the override tricks.
//
// This is deliberately normalization, not rejection: human prose from
// paste or import should degrade gracefully. Structural URL parts get the
// opposite treatment (reject outright) in CanonicalizeHref.
func CleanText(s string, mode TextMode) string {
	// Fold the three line-break forms into one token first, so the mode
	// switch below sees a single case.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n':
			switch mode {
			case HeaderValue:
				b.WriteByte(' ')
			case BodyValue:
				b.WriteString("\r\n")
			default:
				b.WriteByte('\n')
			}
		case r == '\t':
			if mode == HeaderValue {
				b.WriteByte(' ')
			} else {
				b.WriteByte('\t')
			}
		case r == utf8.RuneError:
			// Invalid UTF-8 byte (or a literal replacement char): dropped.
		case r >= 0x202A && r <= 0x202E:
			// Bidi embedding/override marks: dropped.
		case unicode.IsControl(r):
			// Remaining C0, DEL, C1 (\n and \t were handled above).
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// structuralOK reports whether a structural URL part (scheme, host, path,
// query, fragment, CSS source) is free of control and invisible format
// characters. Structural strings get no cleaning: a percent-smuggled bidi
// override or a zero-width character hiding a lookalike path can only be
// an attack, so its presence rejects the whole input — reject, don't
// repair.
func structuralOK(s string) bool {
	for _, r := range s {
		if r == utf8.RuneError || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}
