// Package epubhtml is the content policy for the w-text rich editor: the
// "EPUB Content Document, no-script" profile. It decides which elements,
// attributes, URLs, characters and CSS declarations may exist in editor
// content. Nothing here touches the DOM.
//
// The package is intentionally pure (no syscall/js), following the same
// policy/mechanism split as package expr: decisions live here, testable and
// fuzzable under the native toolchain, while the js-side filtering walker
// (the w-text EditorCore) merely applies them to DOM nodes. The walker
// builds a clean tree by copying only what the policy allows — forbidden
// markup is never copied, so it cannot exist in editor content by
// construction. Sanitization always runs on a tree parsed by the browser's
// own parser (DOMParser), never on a second Go-side HTML parse, so there is
// no parser differential for mutation-XSS tricks to exploit.
package epubhtml

import "strings"

// Disposition tells the filtering walker what to do with an element.
type Disposition int8

// Dispositions, from most to least destructive.
const (
	// Drop discards the element and its whole subtree. Reserved for
	// elements whose content is dangerous or meaningless as text
	// (script, style, iframe, svg, form controls, media...).
	Drop Disposition = iota
	// Unwrap discards the element but keeps (and filters) its children —
	// the fail-toward-text default for unknown or presentational markup.
	Unwrap
	// Keep copies the element under ElementPolicy.Canonical; its
	// attributes and children are filtered in turn.
	Keep
)

// ElementPolicy is the filtering decision for one element tag.
type ElementPolicy struct {
	Disposition Disposition
	// Canonical is the tag to emit when Disposition is Keep. It differs
	// from the source tag for legacy presentational aliases (b → strong,
	// i → em, div → p).
	Canonical string
}

// marks are the semantic inline elements of the profile. Everything
// presentational (u, span, font...) is expressed as a named class instead.
var marks = map[string]bool{
	"strong": true,
	"em":     true,
	"a":      true,
	"code":   true,
	"sup":    true,
	"sub":    true,
}

// blocks are the block-level elements of the profile. Lists (ul/ol/li) are
// deliberately out of v1: their toggle semantics need nesting-aware
// commands that SetBlock does not model yet.
var blocks = map[string]bool{
	"p":          true,
	"h1":         true,
	"h2":         true,
	"h3":         true,
	"h4":         true,
	"h5":         true,
	"h6":         true,
	"blockquote": true,
	"pre":        true,
}

// classCarriers are inline containers that exist only to carry named
// classes (the inline half of a split style). One with no registered
// class says nothing and is unwrapped by the walkers — see RequiresClass.
var classCarriers = map[string]bool{
	"span": true,
}

// renames maps legacy presentational tags to their canonical semantic
// form. Browsers still generate these on their own (native Ctrl+B/Ctrl+I,
// the iOS selection callout), so the canonicalizer must know them even
// though the toolbar never emits them.
var renames = map[string]string{
	"b":   "strong",
	"i":   "em",
	"div": "p",
}

// dropped lists elements whose whole subtree is discarded: content that is
// executable, that loads external resources, that changes document
// semantics (base, meta) or that has no sensible text projection. Unknown
// tags do NOT belong here — they get the milder Unwrap.
var dropped = map[string]bool{
	// executable / style
	"script": true, "style": true, "template": true, "noscript": true,
	// nested browsing contexts and plugins
	"iframe": true, "frame": true, "frameset": true,
	"object": true, "embed": true, "applet": true, "portal": true,
	// document metadata (base rewrites relative URL resolution)
	"base": true, "link": true, "meta": true, "title": true, "head": true,
	// foreign content (classic mXSS carriers) and canvases
	"svg": true, "math": true, "canvas": true,
	// external media
	"img": true, "picture": true, "source": true, "track": true,
	"audio": true, "video": true, "map": true, "area": true,
	// form machinery
	"form": true, "input": true, "button": true, "select": true,
	"textarea": true, "option": true, "optgroup": true, "datalist": true,
	"output": true, "label": true, "fieldset": true, "legend": true,
	// interactive containers
	"dialog": true, "slot": true,
}

// ElementFor returns the filtering decision for a tag. The tag may arrive
// in any case (DOM tagName is uppercase); unknown tags fall back to Unwrap,
// the fail-toward-text default.
func ElementFor(tag string) ElementPolicy {
	tag = strings.ToLower(tag)
	switch {
	case dropped[tag]:
		return ElementPolicy{Disposition: Drop}
	case marks[tag] || blocks[tag] || classCarriers[tag] || tag == "br":
		return ElementPolicy{Disposition: Keep, Canonical: tag}
	}
	if canon, ok := renames[tag]; ok {
		return ElementPolicy{Disposition: Keep, Canonical: canon}
	}
	return ElementPolicy{Disposition: Unwrap}
}

// IsMark reports whether canonical tag is a semantic inline mark of the
// profile. The walker uses it for structure rules (marks nest in blocks,
// never the other way around).
func IsMark(tag string) bool { return marks[strings.ToLower(tag)] }

// IsBlock reports whether canonical tag is a block element of the profile.
func IsBlock(tag string) bool { return blocks[strings.ToLower(tag)] }

// BlockList returns the block tags of the profile in stable order — the
// selector vocabulary for block-half class rules.
func BlockList() []string {
	return []string{"p", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "pre"}
}

// RequiresClass reports whether canonical tag may only exist while
// carrying at least one registered class. The walkers unwrap it the
// moment its class list filters down to nothing.
func RequiresClass(tag string) bool { return classCarriers[strings.ToLower(tag)] }

// IsInline reports whether canonical tag is inline content — a semantic
// mark or a class carrier. Structure rule: inline nests in blocks, never
// the other way around.
func IsInline(tag string) bool {
	tag = strings.ToLower(tag)
	return marks[tag] || classCarriers[tag]
}

// AttrKind classifies an allowed attribute so the walker knows which
// validator must run on the value before copying it.
type AttrKind int8

// Attribute kinds. The default is AttrDrop: an attribute is copied only
// when the policy names it explicitly, so event handlers, id and friends
// are simply not expressible.
const (
	// AttrDrop marks an attribute that is never copied.
	AttrDrop AttrKind = iota
	// AttrHref marks a value that must pass CanonicalizeHref first.
	AttrHref
	// AttrClass marks a class list: every name must have been registered
	// through the editor's DefineClass before it may be copied.
	AttrClass
	// AttrStyle marks an inline style declaration list: FilterCSS reduces
	// it to the allowed properties (silently, not SanitizeCSS's reject-
	// everything-on-one-mistake contract — this is a pasted element's own
	// style, not a webdev's DefineClass call), and whatever survives is
	// registered as a class (PasteClassName) rather than copied as an
	// attribute — the element never gets a raw style="" back.
	AttrStyle
)

// AttrFor returns the attribute policy for attr on the canonical tag.
// SetBlock conversions re-run this with the *target* tag, so an attribute
// that was legal on the source block but not on the new one is dropped,
// and kept values are re-validated against the destination's rules. This
// never actually applies to "style" — SetBlock only ever re-checks
// attributes already IN the document, and this profile never leaves a raw
// style="" there in the first place (see AttrStyle).
func AttrFor(tag, attr string) AttrKind {
	tag = strings.ToLower(tag)
	attr = strings.ToLower(attr)
	switch {
	case attr == "class":
		return AttrClass
	case attr == "style":
		return AttrStyle
	case tag == "a" && attr == "href":
		return AttrHref
	}
	return AttrDrop
}
