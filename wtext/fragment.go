package wtext

import (
	"fmt"

	"github.com/luisfurquim/wings/epubhtml"
)

// fnode is one node of a Fragment: a text run (tag == "") or an element
// with attributes and children. Fragments are portable trees — plugins
// build and transform them natively; only the editor's materializer turns
// them into DOM nodes, re-checking hrefs and class registration on the way.
type fnode struct {
	text  string
	tag   string
	attrs map[string]string
	kids  []fnode
}

// Fragment is a safe-by-construction piece of content: everything a
// Builder can express is inside the epubhtml profile, so inserting a
// Fragment can never smuggle markup past the filter.
type Fragment struct {
	nodes []fnode
	errs  []error
}

// Empty reports whether the fragment has no content.
func (f Fragment) Empty() bool { return len(f.nodes) == 0 }

// Node is the fluent handle to one element being built. Every method
// returns the same node so calls chain; errors accumulate and surface at
// Builder.Done.
type Node interface {
	// Attr sets an allowed attribute (a/href, class). Anything else is a
	// build error — there is no method for the forbidden.
	Attr(name, value string) Node
	// Text appends a text run (cleaned as document text).
	Text(s string) Node
	// Block appends a block child and descends into it via fn.
	Block(tag string, fn func(Node)) Node
	// Mark appends an inline mark child and descends into it via fn.
	Mark(tag string, fn func(Node)) Node
	// Br appends a line break.
	Br() Node
}

// Builder assembles a Fragment. Zero value is ready to use.
type Builder struct {
	nodes  []fnode
	errs   []error
	inline bool // building inside a mark: blocks are not allowed
}

// NewFragment returns an empty Builder.
func NewFragment() *Builder { return &Builder{} }

// Text appends a top-level text run.
func (b *Builder) Text(s string) *Builder {
	b.nodes = append(b.nodes, fnode{text: epubhtml.CleanText(s, epubhtml.DocumentText)})
	return b
}

// Br appends a top-level line break.
func (b *Builder) Br() *Builder {
	b.nodes = append(b.nodes, fnode{tag: "br"})
	return b
}

// Block appends a top-level block element and builds its content via fn.
func (b *Builder) Block(tag string, fn func(Node)) *Builder {
	b.nodes = b.child(b.nodes, tag, false, fn)
	return b
}

// Mark appends a top-level inline mark and builds its content via fn.
func (b *Builder) Mark(tag string, fn func(Node)) *Builder {
	b.nodes = b.child(b.nodes, tag, true, fn)
	return b
}

// Done returns the built Fragment. Any error recorded during building
// (forbidden tag, forbidden attribute, block inside a mark, bad href)
// invalidates the whole fragment: the editor refuses it with the joined
// error — reject, don't repair.
func (b *Builder) Done() (Fragment, error) {
	if len(b.errs) > 0 {
		return Fragment{}, fmt.Errorf("%w: %v", ErrBadFragment, b.errs)
	}
	return Fragment{nodes: b.nodes}, nil
}

// child validates tag for its role and appends an element node built by fn.
func (b *Builder) child(list []fnode, tag string, mark bool, fn func(Node)) []fnode {
	switch {
	case mark && !epubhtml.IsMark(tag):
		b.errs = append(b.errs, fmt.Errorf("%q is not a mark", tag))
		return list
	case !mark && !epubhtml.IsBlock(tag):
		b.errs = append(b.errs, fmt.Errorf("%q is not a block", tag))
		return list
	case !mark && b.inline:
		b.errs = append(b.errs, fmt.Errorf("block %q inside a mark", tag))
		return list
	}
	n := fnode{tag: tag}
	if fn != nil {
		wasInline := b.inline
		b.inline = b.inline || mark
		fn(&nodeRef{n: &n, b: b})
		b.inline = wasInline
	}
	return append(list, n)
}

// nodeRef implements Node over one fnode under construction.
type nodeRef struct {
	n *fnode
	b *Builder
}

func (r *nodeRef) Attr(name, value string) Node {
	switch epubhtml.AttrFor(r.n.tag, name) {
	case epubhtml.AttrHref:
		canon, err := epubhtml.CanonicalizeHref(value, nil)
		if err != nil {
			r.b.errs = append(r.b.errs, err)
			return r
		}
		value = canon
	case epubhtml.AttrClass:
		if err := epubhtml.ValidClassName(value); err != nil {
			r.b.errs = append(r.b.errs, err)
			return r
		}
	default:
		r.b.errs = append(r.b.errs,
			fmt.Errorf("attribute %q not allowed on %q", name, r.n.tag))
		return r
	}
	if r.n.attrs == nil {
		r.n.attrs = map[string]string{}
	}
	r.n.attrs[name] = value
	return r
}

func (r *nodeRef) Text(s string) Node {
	r.n.kids = append(r.n.kids, fnode{text: epubhtml.CleanText(s, epubhtml.DocumentText)})
	return r
}

func (r *nodeRef) Br() Node {
	r.n.kids = append(r.n.kids, fnode{tag: "br"})
	return r
}

func (r *nodeRef) Block(tag string, fn func(Node)) Node {
	inMark := r.b.inline || epubhtml.IsMark(r.n.tag)
	if inMark {
		r.b.errs = append(r.b.errs, fmt.Errorf("block %q inside a mark", tag))
		return r
	}
	if !epubhtml.IsBlock(tag) {
		r.b.errs = append(r.b.errs, fmt.Errorf("%q is not a block", tag))
		return r
	}
	n := fnode{tag: tag}
	if fn != nil {
		fn(&nodeRef{n: &n, b: r.b})
	}
	r.n.kids = append(r.n.kids, n)
	return r
}

func (r *nodeRef) Mark(tag string, fn func(Node)) Node {
	if !epubhtml.IsMark(tag) {
		r.b.errs = append(r.b.errs, fmt.Errorf("%q is not a mark", tag))
		return r
	}
	n := fnode{tag: tag}
	if fn != nil {
		wasInline := r.b.inline
		r.b.inline = true
		fn(&nodeRef{n: &n, b: r.b})
		r.b.inline = wasInline
	}
	r.n.kids = append(r.n.kids, n)
	return r
}
