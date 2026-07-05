package epubhtml

import (
	"errors"
	"fmt"
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

// ValidClassName checks a Word-style named class ("titulo", "destaque-1"):
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
		out = append(out, prop+": "+val)
	}
	if len(out) == 0 {
		return "", fmt.Errorf("%w: no declarations", ErrCSSSyntax)
	}
	return strings.Join(out, "; "), nil
}
