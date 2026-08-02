package epubhtml

import (
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
)

// Size bounds for DefineClass input — bounded everything.
const (
	// MaxClassNameLen bounds a class name.
	MaxClassNameLen = 64
	// MaxCSSLen bounds one class's CSS declaration list.
	MaxCSSLen = 4096
)

// Errors returned by ValidClassName and SanitizeCSS.
var (
	// ErrClassName reports an invalid class name.
	ErrClassName = errors.New("epubhtml: class name must be an ASCII letter followed by letters, digits, '-' or '_'")
	// ErrCSSTooLong reports CSS beyond MaxCSSLen.
	ErrCSSTooLong = errors.New("epubhtml: CSS exceeds the size bound")
	// ErrCSSSyntax reports input that is not a plain declaration list.
	ErrCSSSyntax = errors.New("epubhtml: CSS must be 'property: value' declarations")
	// ErrCSSProperty reports a property outside the allowlist.
	ErrCSSProperty = errors.New("epubhtml: CSS property not allowed")
	// ErrCSSValue reports a forbidden construct inside the CSS.
	ErrCSSValue = errors.New("epubhtml: forbidden construct in CSS")
)

// ValidClassName checks a Word-style named class ("title", "highlight-1"):
// an ASCII letter followed by letters, digits, '-' or '_'. The name is the
// only selector a class rule ever gets, so this also guarantees the name
// cannot alter the selector's meaning.
func ValidClassName(name string) error {
	if name == "" || len(name) > MaxClassNameLen {
		return ErrClassName
	}
	for i, r := range name {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
		case i > 0 && ((r >= '0' && r <= '9') || r == '-' || r == '_'):
		default:
			return fmt.Errorf("%w: %q", ErrClassName, name)
		}
	}
	return nil
}

// cssProps is the allowlist of properties a DefineClass rule may set:
// typography, spacing, color and borders — the vocabulary of an EPUB
// stylesheet. Escape hatches out of "inert text styling" (position,
// content, resource loading) are simply not in the list.
var cssProps = map[string]bool{
	"color":            true,
	"background-color": true,

	"font-family":  true,
	"font-size":    true,
	"font-style":   true,
	"font-weight":  true,
	"font-variant": true,

	"letter-spacing": true,
	"word-spacing":   true,
	"line-height":    true,
	"white-space":    true,
	"hyphens":        true,

	"text-align":                true,
	"text-indent":               true,
	"text-transform":            true,
	"text-shadow":               true,
	"text-decoration":           true,
	"text-decoration-line":      true,
	"text-decoration-style":     true,
	"text-decoration-color":     true,
	"text-decoration-thickness": true,

	"margin":        true,
	"margin-top":    true,
	"margin-right":  true,
	"margin-bottom": true,
	"margin-left":   true,

	"padding":        true,
	"padding-top":    true,
	"padding-right":  true,
	"padding-bottom": true,
	"padding-left":   true,

	"border":        true,
	"border-top":    true,
	"border-right":  true,
	"border-bottom": true,
	"border-left":   true,
	"border-width":  true,
	"border-style":  true,
	"border-color":  true,
	"border-radius": true,

	// EPUB pagination hints
	"page-break-before": true,
	"page-break-after":  true,
	"page-break-inside": true,
	"break-before":      true,
	"break-after":       true,
	"break-inside":      true,

	// Flow and box. These arrived with the document-sheet pass-through:
	// once a book's own stylesheet is carried instead of reduced to named
	// styles, this list stops being "what the editor writes" and becomes
	// "what a book may say", and most of what was missing was never a
	// safety matter — it was a profile drawn for the narrower job. Measured
	// on one real book, thirteen properties were being lost and only one
	// family (position, below) had any security story at all: a drop cap
	// styled `float: left` came out sitting on the baseline, and thirty-six
	// rules were dropped whole for holding nothing but `counter-reset`.
	//
	// Deliberately still absent: position and its offsets (top/right/
	// bottom/left). `position: fixed` positions against the VIEWPORT, so a
	// rule inside the editor could cover the application around it — the
	// one case here where a declaration reaches past the text it styles.
	"float": true,
	"clear": true,

	"width": true, "height": true,
	"min-width": true, "min-height": true,
	"max-width": true, "max-height": true,

	"vertical-align": true,

	"list-style":          true,
	"list-style-type":     true,
	"list-style-position": true,

	// Inert on their own: a counter is only ever SEEN through `content`,
	// which is not allowlisted. Carrying them changes nothing visually and
	// stops a word processor's list machinery from emptying whole rules.
	"counter-reset":     true,
	"counter-increment": true,

	"grid-template-columns": true,
	"grid-template-rows":    true,
	"align-items":           true,
	"justify-items":         true,
	"gap":                   true,

	// Value-restricted — see cssPropValues.
	"display":  true,
	"overflow": true,
}

// cssPropValues restricts the VALUES of properties that are safe in
// general but not in every value. A property absent from this map accepts
// any value the rest of the pipeline allows.
//
// This is the first value-level rule in the profile: until the document
// sheet was carried verbatim, judging by property NAME alone was enough,
// because everything came from the editor itself.
var cssPropValues = map[string]func(string) bool{
	// display: none would let an imported document hide its own text —
	// which then travels, still invisible, into the next export. Every
	// other display value only changes how a box lays out.
	"display": func(v string) bool { return v != "none" },
	// The values that CONTAIN what overflows: clip it, always scroll it,
	// or scroll it when there is something to scroll. `visible` is the
	// initial value and asks for nothing, so carrying it would only add
	// noise to a rule.
	"overflow": func(v string) bool {
		return v == "hidden" || v == "scroll" || v == "auto"
	},
}

// cssValueOK reports whether val is acceptable for prop.
func cssValueOK(prop, val string) bool {
	check, restricted := cssPropValues[prop]
	if !restricted {
		return true
	}
	return check(cssValueKey(val))
}

// cssValueKey reduces a declaration's value to the form the restrictions
// test: lowercased, without !important, single-spaced. A value that keeps
// its priority marker or its capitals would slip past a plain comparison,
// which is the whole point of normalizing before the check rather than
// after.
func cssValueKey(val string) string {
	v := strings.ToLower(strings.TrimSpace(val))
	if i := strings.IndexByte(v, '!'); i >= 0 {
		v = v[:i]
	}
	return strings.Join(strings.Fields(v), " ")
}

// blockProps are the allowlisted properties that only take effect at
// paragraph level — Word's Paragraph dialog: alignment, indentation,
// spacing, box and pagination. Everything else in cssProps is character
// formatting (Word's Font dialog) and applies to inline runs.
var blockProps = map[string]bool{
	"text-align":  true,
	"text-indent": true,
	"line-height": true,
	"white-space": true,
	"hyphens":     true,

	"margin": true, "margin-top": true, "margin-right": true,
	"margin-bottom": true, "margin-left": true,

	"padding": true, "padding-top": true, "padding-right": true,
	"padding-bottom": true, "padding-left": true,

	"border": true, "border-top": true, "border-right": true,
	"border-bottom": true, "border-left": true,
	"border-width": true, "border-style": true, "border-color": true,
	"border-radius": true,

	"page-break-before": true, "page-break-after": true, "page-break-inside": true,
	"break-before": true, "break-after": true, "break-inside": true,

	// Flow and box are paragraph-level, and they travel TOGETHER: a
	// `float: right` split away from the `width` that sizes what is being
	// floated would leave a named style whose two halves each do nothing.
	// (Only named styles are split at all — a document rule is emitted
	// with the selector the book wrote, so this classification never
	// touches an imported book's own drop cap.)
	"float": true, "clear": true,
	"width": true, "height": true,
	"min-width": true, "min-height": true,
	"max-width": true, "max-height": true,
	"display": true, "overflow": true,
	"list-style": true, "list-style-type": true, "list-style-position": true,
	"counter-reset": true, "counter-increment": true,
	"grid-template-columns": true, "grid-template-rows": true,
	"align-items": true, "justify-items": true, "gap": true,
}

// PropIsBlock reports whether an allowlisted property is paragraph-level.
func PropIsBlock(prop string) bool {
	return blockProps[strings.ToLower(strings.TrimSpace(prop))]
}

// SplitCSS divides a sanitized declaration list into its character and
// paragraph halves — the two application points of one named style:
// character declarations ride a span over the exact range, paragraph
// declarations mark the touched blocks. Input is SanitizeCSS output
// (canonical form); a malformed declaration is skipped, never guessed at.
func SplitCSS(css string) (charDecls, blockDecls string) {
	var ch, bl []string
	for _, decl := range strings.Split(css, ";") {
		decl = strings.TrimSpace(decl)
		if decl == "" {
			continue
		}
		prop, _, found := strings.Cut(decl, ":")
		if !found {
			continue
		}
		if PropIsBlock(prop) {
			bl = append(bl, decl)
		} else {
			ch = append(ch, decl)
		}
	}
	return strings.Join(ch, "; "), strings.Join(bl, "; ")
}

// MergeCSS folds several sanitized declaration lists into one, later
// lists overriding earlier ones property by property — the merge rule of
// "create style from selection", where the innermost formatting wins.
// The first appearance of a property fixes its position in the output.
func MergeCSS(layers ...string) string {
	var order []string
	vals := map[string]string{}
	for _, layer := range layers {
		for _, decl := range strings.Split(layer, ";") {
			decl = strings.TrimSpace(decl)
			if decl == "" {
				continue
			}
			prop, val, found := strings.Cut(decl, ":")
			if !found {
				continue
			}
			prop = strings.ToLower(strings.TrimSpace(prop))
			val = strings.TrimSpace(val)
			if val == "" {
				continue
			}
			if _, seen := vals[prop]; !seen {
				order = append(order, prop)
			}
			vals[prop] = val
		}
	}
	out := make([]string, 0, len(order))
	for _, p := range order {
		out = append(out, p+": "+vals[p])
	}
	return strings.Join(out, "; ")
}

// PasteClassName derives a deterministic class name for a sanitized
// inline-style declaration list (FilterCSS output) — the identity a
// paste-time auto-registered class gets instead of a webdev-chosen name.
// Deterministic so identical inline styles across many pasted elements
// (every paragraph in a Google Docs export typically repeats the same
// style="" verbatim) collapse onto one shared class rather than
// registering a near-duplicate per element. Declarations are sorted by
// property name (stably, so two declarations sharing a property keep
// their relative order — see below) before hashing, so two elements
// whose styles list the same properties in a different order — a real
// possibility, since nothing about how a source serializes style=""
// guarantees one property order over another — still collapse onto one
// class instead of registering a spurious duplicate. Sorting the FULL
// declaration string (property AND value together) would risk the
// opposite mistake: "color:red;color:blue" and "color:blue;color:red"
// are NOT equivalent CSS (the cascade means whichever comes last wins,
// so one nets blue and the other red) but would sort byte-identically if
// compared as whole strings — grouping by property name with a stable
// sort keeps genuinely-duplicated properties in their original relative
// order, so those two inputs still hash differently. The "wt-" prefix
// keeps the result out of StyleToolbar's picker (an internal translation
// artifact, not a user-facing named style) and off limits to
// CreateStyle's own names.
func PasteClassName(css string) string {
	decls := strings.Split(css, ";")
	for i := range decls {
		decls[i] = strings.TrimSpace(decls[i])
	}
	propOf := func(decl string) string {
		prop, _, _ := strings.Cut(decl, ":")
		return strings.TrimSpace(prop)
	}
	sort.SliceStable(decls, func(i, j int) bool { return propOf(decls[i]) < propOf(decls[j]) })

	h := fnv.New32a()
	for _, d := range decls {
		_, _ = h.Write([]byte(d)) // hash.Hash.Write never errors
		_, _ = h.Write([]byte{';'})
	}
	return fmt.Sprintf("wt-paste-%08x", h.Sum32())
}

// cssBanned are substrings that must not appear anywhere in a rule. They
// are the escape hatches out of an inert declaration list: url()-family
// functions load remote resources (tracking/exfiltration), '@' opens
// at-rules, backslash escapes could spell any of the others past this
// scan, braces would close our rule and open a raw selector, and comments
// have historically confused CSS parsers.
var cssBanned = []string{
	"url(", "image-set(", "element(", "expression(",
	"@", "\\", "{", "}", "<", "/*", "*/",
}

// SanitizeCSS validates a declaration list for DefineClass and returns it
// in canonical "prop: value; prop: value" form. This is webdev-facing
// input, so it fails loudly and specifically — an unknown property is an
// error naming the property, never a silent drop.
func SanitizeCSS(css string) (string, error) {
	if len(css) > MaxCSSLen {
		return "", ErrCSSTooLong
	}
	css = normalizeCSSSpace(css)
	if !structuralOK(css) {
		return "", fmt.Errorf("%w: control or invisible character", ErrCSSValue)
	}
	low := strings.ToLower(css)
	for _, bad := range cssBanned {
		if strings.Contains(low, bad) {
			return "", fmt.Errorf("%w: %q", ErrCSSValue, bad)
		}
	}

	var out []string
	for _, decl := range strings.Split(css, ";") {
		decl = strings.TrimSpace(decl)
		if decl == "" {
			continue
		}
		prop, val, found := strings.Cut(decl, ":")
		if !found {
			return "", fmt.Errorf("%w: %q", ErrCSSSyntax, decl)
		}
		prop = strings.ToLower(strings.TrimSpace(prop))
		val = strings.TrimSpace(val)
		if !cssProps[prop] {
			return "", fmt.Errorf("%w: %q", ErrCSSProperty, prop)
		}
		if val == "" {
			return "", fmt.Errorf("%w: empty value for %q", ErrCSSSyntax, prop)
		}
		if !cssValueOK(prop, val) {
			return "", fmt.Errorf("%w: %q is not allowed for %q", ErrCSSValue, val, prop)
		}
		out = append(out, prop+": "+val)
	}
	if len(out) == 0 {
		return "", fmt.Errorf("%w: no declarations", ErrCSSSyntax)
	}
	return strings.Join(out, "; "), nil
}

// normalizeCSSSpace turns the ASCII whitespace CSS itself treats as plain
// separators — tab, newline, carriage return, form feed, vertical tab —
// into spaces.
//
// Without this, every rule written across more than one line is refused
// as holding a "control character": those bytes ARE control characters,
// and structuralOK cannot tell them from the invisibles it exists to stop
// (zero-width joiners, bidi overrides, the Trojan-Source family). But a
// stylesheet written by a human, or by any tool that formats its output,
// is multi-line by default — a whole book's typography was being dropped
// over its own indentation. Normalizing first keeps the check aimed at
// what it was aimed at: characters that hide meaning, not characters that
// arrange it.
func normalizeCSSSpace(css string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\t', '\n', '\r', '\f', '\v':
			return ' '
		}
		return r
	}, css)
}

// FilterCSS reduces a declaration list to the declarations this profile
// recognizes, DROPPING everything else silently — an unsupported
// property, a banned construct, a malformed declaration — rather than
// rejecting the whole list the way SanitizeCSS does. SanitizeCSS is
// deliberately strict because it validates a webdev's own DefineClass
// call, where an unrecognized property is a bug worth surfacing loudly
// and immediately. FilterCSS is for externally-authored CSS this project
// never controlled in the first place — a pasted element's inline
// style="" — where real documents routinely mix a few properties this
// profile supports (font-family, text-align) with several it was never
// meant to (vertical-align, white-space, -webkit-* prefixes); rejecting
// the whole style over one unsupported property would silently discard
// the declarations that WERE safe and useful along with it. The banned-
// construct check still runs per declaration, so one hostile declaration
// poisons only itself, not its neighbors. Returns "" when nothing
// survives — same as SanitizeCSS's ErrCSSSyntax case, just without the
// error, since "the source used only unsupported properties" is the
// ordinary case here, not a mistake to report.
func FilterCSS(css string) string {
	if len(css) > MaxCSSLen {
		return "" // pathologically long for a real clipboard style=""
	}
	css = normalizeCSSSpace(css)
	var out []string
	for _, decl := range strings.Split(css, ";") {
		decl = strings.TrimSpace(decl)
		if decl == "" || !structuralOK(decl) {
			continue
		}
		low := strings.ToLower(decl)
		banned := false
		for _, bad := range cssBanned {
			if strings.Contains(low, bad) {
				banned = true
				break
			}
		}
		if banned {
			continue
		}
		prop, val, found := strings.Cut(decl, ":")
		if !found {
			continue
		}
		prop = strings.ToLower(strings.TrimSpace(prop))
		val = strings.TrimSpace(val)
		if val == "" || !cssProps[prop] || !cssValueOK(prop, val) {
			continue
		}
		out = append(out, prop+": "+val)
	}
	return strings.Join(out, "; ")
}
