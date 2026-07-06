package wtext

import "github.com/luisfurquim/wings/epubhtml"

// Mark is a sealed semantic inline mark. The constructors below are the
// only way to make one, and the allowed attribute of each kind is a
// constructor parameter — so a plugin literally cannot express onclick, a
// style, or an attribute on the wrong tag; the compiler is the allowlist.
type Mark struct {
	tag  string
	href string // set by Link only
}

// Tag returns the element the mark renders as ("" for a zero Mark).
func (m Mark) Tag() string { return m.tag }

// Href returns the canonical link target ("" unless the mark is a Link).
func (m Mark) Href() string { return m.href }

// Strong is importance/bold.
func Strong() Mark { return Mark{tag: "strong"} }

// Em is emphasis/italic.
func Em() Mark { return Mark{tag: "em"} }

// Code is inline code.
func Code() Mark { return Mark{tag: "code"} }

// Sup is superscript.
func Sup() Mark { return Mark{tag: "sup"} }

// Sub is subscript.
func Sub() Mark { return Mark{tag: "sub"} }

// Link is a hyperlink. href is canonicalized here (structure, schemes,
// punycode host, mailto rebuild); the editor re-runs it against the
// profile's LinkPolicy at apply time, so an app-level host restriction
// cannot be bypassed by building the Mark early.
func Link(href string) (Mark, error) {
	canon, err := epubhtml.CanonicalizeHref(href, nil)
	if err != nil {
		return Mark{}, err
	}
	return Mark{tag: "a", href: canon}, nil
}
