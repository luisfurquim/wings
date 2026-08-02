package epubhtml

import (
	"fmt"
	"strings"
)

// The structural half of reading a stylesheet a DOCUMENT brought with it
// — a book from anywhere, authored against a browser and not against this
// editor.
//
// It reads the SHAPE of the sheet and nothing else: where a rule starts
// and ends, what is a comment, which part is the selector and which part
// is the declaration block. The selector itself is never interpreted. It
// is checked for the handful of constructs that would be dangerous and
// then kept VERBATIM, because the thing that has to understand it —
// matching elements, resolving specificity, breaking ties by source
// order, honouring !important — is the browser, which has done it
// correctly for twenty-five years. A selector engine and a cascade here
// would be a second, worse implementation of something already present.
//
// What keeps that safe is where the sheet lands: every wings component
// renders inside a shadow root, and CSS in a shadow root does not escape
// it. A book's `body { … }` cannot reach the page around the widget.

// Bounds for a document sheet — nothing here is unbounded.
const (
	// MaxSheetRules bounds how many rules one document sheet may carry.
	MaxSheetRules = 512
	// MaxSelectorLen bounds one selector. A selector LIST ("h1, h2, h3…")
	// counts as one, so this is roomier than a single compound needs.
	MaxSelectorLen = 1024
)

// ErrSelector reports a selector this profile will not carry.
var ErrSelector = fmt.Errorf("epubhtml: selector not allowed")

// SheetRule is one rule read out of a document's stylesheet.
//
// Drop carries the reason the rule cannot be kept, empty when it can. A
// dropped rule is RETURNED rather than silently skipped: the caller
// reports it (a book losing formatting in silence is the hardest kind of
// bug to notice), and the report is the specification of how an exported
// sheet is allowed to differ from the one that came in.
type SheetRule struct {
	Selector string // verbatim, never interpreted
	Decls    string // FilterCSS output; empty when nothing survived
	Drop     string // why this rule is not kept, or ""
}

// Kept reports whether the rule survived.
func (r SheetRule) Kept() bool { return r.Drop == "" }

// selectorBanned are substrings a selector must not contain.
//
// This is NOT cssBanned, and the difference is the point. cssBanned
// forbids a backslash because a declaration has no legitimate use for one
// and it could spell any of the other bans past a substring scan. A
// SELECTOR does have a legitimate use for it: `epub:type` is an ordinary
// attribute in EPUB, ':' is not valid in a CSS identifier, and the
// correct way to select it is `[epub\:type="chapter"]`. Banning it here
// would refuse real books to defend against nothing — the escape is
// handled where it matters, in the scanner below, which skips the escaped
// byte so `a\}` cannot desynchronize brace counting.
//
// The shadow-piercing selectors are the reason this check exists at all.
// Inside a shadow root, :host, :host-context(), ::slotted() and ::part()
// reach OUT to the host element and to the webdev's own content: a book
// carrying `:host { display: none }` would delete the widget. Everything
// else in the list would break out of our rule and open a raw one.
var selectorBanned = []string{
	":host", "::slotted", "::part",
	"{", "}", "<", "/*", "*/", "@", ";",
	"url(", "expression(",
}

// SafeSelector reports whether a selector may be carried verbatim. It
// does not parse it and makes no claim that it is valid CSS: an invalid
// selector simply matches nothing, which costs formatting and not safety.
//
// Pseudo-elements are deliberately NOT banned. ::before and ::after can
// only introduce text through `content`, which is not in the property
// allowlist, so FilterCSS has already removed the only thing that made
// them worth worrying about.
func SafeSelector(sel string) error {
	sel = strings.TrimSpace(sel)
	if sel == "" {
		return fmt.Errorf("%w: empty", ErrSelector)
	}
	if len(sel) > MaxSelectorLen {
		return fmt.Errorf("%w: longer than %d bytes", ErrSelector, MaxSelectorLen)
	}
	if !structuralOK(normalizeCSSSpace(sel)) {
		return fmt.Errorf("%w: control or invisible character", ErrSelector)
	}
	low := strings.ToLower(sel)
	for _, bad := range selectorBanned {
		if strings.Contains(low, bad) {
			return fmt.Errorf("%w: %q", ErrSelector, bad)
		}
	}
	return nil
}

// ParseSheet reads a document stylesheet into its rules, in source order
// — the order IS the sheet's meaning, since it is the browser's last
// tie-breaker, so it is preserved and never sorted.
//
// Every rule found is returned, kept or dropped, so the caller can report
// what became of the sheet. Parsing never fails as a whole: a book is not
// a bug report, and one malformed rule must not cost the other two
// hundred.
func ParseSheet(css string) []SheetRule {
	var out []SheetRule
	for _, raw := range splitRules(stripComments(css)) {
		if len(out) >= MaxSheetRules {
			out = append(out, SheetRule{
				Selector: raw.sel,
				Drop:     fmt.Sprintf("the sheet carries more than %d rules", MaxSheetRules),
			})
			return out
		}
		rule := SheetRule{Selector: strings.TrimSpace(normalizeCSSSpace(raw.sel))}
		switch {
		case raw.atRule:
			// An at-rule is a statement about the sheet, not a rule about
			// elements. @font-face is read by the font path (the family name
			// is asked of a store, the book's own file is never installed),
			// @import is a network fetch, and @media would need its own
			// nesting rules. None of them belong in a verbatim pass-through.
			rule.Drop = "at-rules are not carried"
		case raw.unterminated:
			rule.Drop = "the rule is never closed"
		default:
			if err := SafeSelector(rule.Selector); err != nil {
				rule.Drop = err.Error()
				break
			}
			rule.Decls = FilterCSS(raw.decls)
			if rule.Decls == "" {
				rule.Drop = "nothing in it is supported by this profile"
			}
		}
		out = append(out, rule)
	}
	return out
}

// ScopeSelector prefixes every part of a selector list with scope, so a
// document's rules reach only the edited text.
//
// The scope is not a security boundary — the shadow root is, and a book's
// CSS cannot leave it. It is a boundary against the WIDGET: the toolbar
// and the editor share one shadow tree, so an unscoped `p { … }` from a
// book would restyle the toolbar's own markup.
//
// Wrapping the whole thing in :is() would be shorter and would distribute
// the prefix for free, but :is() does not accept pseudo-elements: a real
// book's `p::first-line { margin-left: 56.7pt }` — an ordinary paragraph
// indent — would become invalid and be dropped by the browser without a
// word. So the list is split on its TOP-LEVEL commas instead, which is
// the one piece of selector structure this package ever needs to know:
// commas inside (), [] or a string belong to the part they sit in.
func ScopeSelector(sel, scope string) string {
	parts := splitTopLevel(sel, ',')
	for i, p := range parts {
		parts[i] = scope + " " + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}

// SelectorClasses lists the class names a selector mentions.
//
// It is the narrowest possible reading of a selector — find '.', read the
// identifier after it — and it exists for one reason: the editor's policy
// walker keeps a class attribute only for classes it knows, so a document
// rule written "p.haikai" would style nothing, because the walker would
// have stripped `class="haikai"` off the paragraph before the rule ever
// had a chance to match. The rule survives and the element it needs does
// not. (Measured: a book's title kept its font only because `.chtitle`
// happened to ALSO exist as a plain rule and so was registered; its
// haikai and drop caps, styled only through `p.haikai` and
// `span.dropcaps`, silently lost theirs.)
//
// This makes no claim to understand the selector, and it does not need
// to: a false positive keeps a class attribute nothing styles, which
// costs nothing, while a false negative loses formatting. Dots inside a
// string ([title=".x"]) are skipped, since those are not classes.
func SelectorClasses(sel string) []string {
	var out []string
	seen := map[string]bool{}
	i := 0
	for i < len(sel) {
		switch c := sel[i]; {
		case c == '\\' && i+1 < len(sel):
			i += 2
		case c == '"' || c == '\'':
			i = scanString(sel, i)
		case c == '.':
			i++
			start := i
			for i < len(sel) && (isNameByte(sel[i]) || (sel[i] == '\\' && i+1 < len(sel))) {
				if sel[i] == '\\' {
					i += 2
					continue
				}
				i++
			}
			if name := sel[start:i]; name != "" && !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		default:
			i++
		}
	}
	return out
}

// isNameByte reports whether b may continue a CSS identifier. Bytes above
// ASCII are accepted wholesale: they can only be part of a multi-byte
// rune, which CSS allows in an identifier.
func isNameByte(b byte) bool {
	return b >= 0x80 ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b == '-' || b == '_'
}

// splitTopLevel splits s on sep, ignoring separators nested in brackets,
// parentheses or string literals, and honouring backslash escapes.
func splitTopLevel(s string, sep byte) []string {
	var out []string
	var cur strings.Builder
	depth := 0
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s):
			cur.WriteByte(c)
			cur.WriteByte(s[i+1])
			i += 2
			continue
		case c == '"' || c == '\'':
			end := scanString(s, i)
			cur.WriteString(s[i:end])
			i = end
			continue
		case c == '(' || c == '[':
			depth++
		case c == ')' || c == ']':
			if depth > 0 {
				depth--
			}
		case c == sep && depth == 0:
			out = append(out, cur.String())
			cur.Reset()
			i++
			continue
		}
		cur.WriteByte(c)
		i++
	}
	out = append(out, cur.String())
	return out
}

// rawRule is one rule as the scanner found it, before any judgement.
type rawRule struct {
	sel          string
	decls        string
	atRule       bool
	unterminated bool
}

// splitRules scans a sheet into rules.
//
// It is a scanner rather than a Split on '}' because three ordinary
// things break that: a nested at-rule (@media { p { … } }) whose inner
// brace closes early, a quoted string that may hold a brace
// (`[title="{"]` is valid CSS), and a backslash escape (`a\}` is one
// escaped character, not the end of a block). Getting any of them wrong
// desynchronizes the scanner, and a desynchronized scanner puts document
// text where a selector belongs — so the quote and escape state is
// tracked even though the selector is never otherwise understood.
func splitRules(css string) []rawRule {
	var out []rawRule
	var sel strings.Builder
	i := 0
	for i < len(css) {
		c := css[i]
		switch {
		case c == '\\' && i+1 < len(css):
			sel.WriteByte(c)
			sel.WriteByte(css[i+1])
			i += 2
			continue
		case c == '"' || c == '\'':
			end := scanString(css, i)
			sel.WriteString(css[i:end])
			i = end
			continue
		case c == '{':
			decls, end, closed := scanBlock(css, i)
			out = append(out, rawRule{
				sel:          sel.String(),
				decls:        decls,
				atRule:       strings.HasPrefix(strings.TrimSpace(sel.String()), "@"),
				unterminated: !closed,
			})
			sel.Reset()
			i = end
			continue
		case c == ';':
			// An at-rule with no block — `@import "x";` — ends here. Anything
			// else at this position is stray text between rules; either way
			// what accumulated is not a selector for the rule that follows.
			if s := strings.TrimSpace(sel.String()); strings.HasPrefix(s, "@") {
				out = append(out, rawRule{sel: s, atRule: true})
			}
			sel.Reset()
			i++
			continue
		}
		sel.WriteByte(c)
		i++
	}
	// Trailing text with no block is not a rule; a selector alone declares
	// nothing, so there is nothing to report about it.
	return out
}

// scanBlock reads the declaration block that starts at the '{' at open,
// returning its contents, the index just past its '}', and whether that
// '}' was actually found. Nested blocks are consumed whole — an @media
// body arrives as one string, which is exactly what a caller that intends
// to drop it needs.
func scanBlock(css string, open int) (decls string, end int, closed bool) {
	depth := 0
	i := open
	for i < len(css) {
		c := css[i]
		switch {
		case c == '\\' && i+1 < len(css):
			i += 2
			continue
		case c == '"' || c == '\'':
			i = scanString(css, i)
			continue
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return css[open+1 : i], i + 1, true
			}
		}
		i++
	}
	return css[open+1:], len(css), false
}

// scanString returns the index just past the string literal starting at
// the quote at i. An unterminated literal runs to the end of the sheet,
// which is what a browser does with one too.
func scanString(css string, i int) int {
	quote := css[i]
	i++
	for i < len(css) {
		switch {
		case css[i] == '\\' && i+1 < len(css):
			i += 2
			continue
		case css[i] == quote:
			return i + 1
		}
		i++
	}
	return len(css)
}

// stripComments removes /* … */ before the sheet is scanned. Without it a
// commented-out rule survives as text and is then read as a rule of its
// own: a `/* body { font-family: serif } */` block becomes a rule whose
// selector is `/* body`, which a report then offers to the reader as a
// lead — and a lead pointing at commented-out code is worse than none.
//
// An unterminated comment swallows the rest of the sheet, as it does in a
// browser.
func stripComments(css string) string {
	var sb strings.Builder
	for {
		open := strings.Index(css, "/*")
		if open < 0 {
			sb.WriteString(css)
			return sb.String()
		}
		sb.WriteString(css[:open])
		rest := css[open+2:]
		end := strings.Index(rest, "*/")
		if end < 0 {
			return sb.String() // unterminated: the rest is comment
		}
		css = rest[end+2:]
	}
}
