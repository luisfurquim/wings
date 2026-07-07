//go:build js && wasm

package wtext

import (
	"strings"
	"syscall/js"

	"github.com/luisfurquim/wings/epubhtml"
)

// The canonicalizer is the rearguard behind native editing. The
// beforeinput allowlist blocks what it can, but the browser's editing
// engine still produces markup on its own — <b>/<i> from built-in
// shortcuts and mobile callouts, <span style> when a backspace merges
// blocks, <div> on Enter, replaced nodes from spellcheck. This pass runs
// on every observer flush, inside the same undo step as the edit that
// caused it, and folds the tree back into the profile.

// canonicalize normalizes the whole editor tree in place.
func (e *Editor) canonicalize() {
	e.canonChildren(e.root, false)
	e.ensureBlockShape()
	e.root.Call("normalize") // merge adjacent text nodes
}

// canonChildren walks node's children bottom-up (children first, so an
// element is judged after its content settled).
func (e *Editor) canonChildren(node js.Value, inline bool) {
	kids := node.Get("childNodes")
	// Iterate over a snapshot: canonicalization rewrites the child list.
	n := kids.Get("length").Int()
	snapshot := make([]js.Value, n)
	for i := 0; i < n; i++ {
		snapshot[i] = kids.Index(i)
	}
	for _, kid := range snapshot {
		if !kid.Get("isConnected").Bool() {
			continue // removed by an earlier sibling's canonicalization
		}
		switch kid.Get("nodeType").Int() {
		case 3:
			e.canonText(kid)
		case 1:
			e.canonElement(kid, inline)
		default:
			kid.Call("remove") // comments and friends
		}
	}
}

// canonText re-cleans a text node (spellcheck and IME can write anything).
func (e *Editor) canonText(node js.Value) {
	data := node.Get("data").String()
	clean := epubhtml.CleanText(data, epubhtml.DocumentText)
	if clean != data {
		node.Set("data", clean)
	}
}

// canonElement applies the element policy to a live node.
func (e *Editor) canonElement(el js.Value, inline bool) {
	tag := strings.ToLower(el.Get("tagName").String())
	pol := epubhtml.ElementFor(tag)
	switch {
	case pol.Disposition == epubhtml.Drop:
		el.Call("remove")
		return
	case pol.Disposition == epubhtml.Unwrap,
		epubhtml.IsBlock(pol.Canonical) && inline:
		e.canonChildren(el, inline)
		unwrapElement(el)
		return
	}
	canon := pol.Canonical
	if canon != tag {
		el = e.renameElement(el, canon)
	}
	e.canonAttrs(el, canon)
	isInline := epubhtml.IsInline(canon)
	e.canonChildren(el, inline || isInline)
	switch {
	case isInline && !el.Call("hasChildNodes").Bool():
		el.Call("remove") // an empty inline element marks nothing
	case epubhtml.RequiresClass(canon) && !el.Call("hasAttribute", "class").Bool():
		// canonAttrs cut its class list down to nothing: a classless
		// carrier says nothing — dissolve it, children stay.
		unwrapElement(el)
	}
}

// canonAttrs drops every attribute the policy does not name for the tag —
// with one editor-internal exception, the insecure-link badge, which only
// this package sets.
func (e *Editor) canonAttrs(el js.Value, canon string) {
	names := el.Call("getAttributeNames")
	n := names.Get("length").Int()
	for i := 0; i < n; i++ {
		name := strings.ToLower(names.Index(i).String())
		if name == "data-wings-insecure" && canon == "a" {
			continue
		}
		switch epubhtml.AttrFor(canon, name) {
		case epubhtml.AttrClass:
			e.canonClassAttr(el, name)
		case epubhtml.AttrHref:
			// Set by this package after full canonicalization; native
			// editing cannot invent one (beforeinput blocks insertLink),
			// but the rearguard re-checks anyway.
			if _, err := epubhtml.CanonicalizeHref(
				el.Call("getAttribute", name).String(), e.profile.LinkPolicy); err != nil {
				el.Call("removeAttribute", name)
			}
		default:
			el.Call("removeAttribute", name)
		}
	}
}

// canonClassAttr cuts a class list down to registered names.
func (e *Editor) canonClassAttr(el js.Value, name string) {
	val := el.Call("getAttribute", name).String()
	var keep []string
	for _, cls := range strings.Fields(val) {
		if e.classDefined(cls) {
			keep = append(keep, cls)
		}
	}
	switch {
	case len(keep) == 0:
		el.Call("removeAttribute", name)
	case strings.Join(keep, " ") != val:
		el.Call("setAttribute", name, strings.Join(keep, " "))
	}
}

// renameElement replaces el with a fresh element of the canonical tag
// (DOM cannot rename in place), moving children over. Attributes are NOT
// copied: the caller re-filters what survives on the new tag — the same
// re-filtering SetBlock does.
func (e *Editor) renameElement(el js.Value, canon string) js.Value {
	repl := e.doc.Call("createElement", canon)
	for el.Call("hasChildNodes").Bool() {
		repl.Call("appendChild", el.Get("firstChild"))
	}
	el.Get("parentNode").Call("replaceChild", repl, el)
	return repl
}

// unwrapElement dissolves el, flowing its children into its place.
func unwrapElement(el js.Value) {
	parent := el.Get("parentNode")
	if !parent.Truthy() {
		return
	}
	for el.Call("hasChildNodes").Bool() {
		parent.Call("insertBefore", el.Get("firstChild"), el)
	}
	parent.Call("removeChild", el)
}

// ensureBlockShape enforces the two top-level invariants contenteditable
// tends to erode: the root's direct children are blocks (stray text or
// inline content is wrapped into a fresh <p>), and no block is left
// caret-less (an empty block gets the conventional <br> filler).
func (e *Editor) ensureBlockShape() {
	kids := e.root.Get("childNodes")
	n := kids.Get("length").Int()
	snapshot := make([]js.Value, n)
	for i := 0; i < n; i++ {
		snapshot[i] = kids.Index(i)
	}
	var pending js.Value // open <p> collecting stray inline nodes
	for _, kid := range snapshot {
		if !kid.Get("isConnected").Bool() {
			continue
		}
		isElement := kid.Get("nodeType").Int() == 1
		if isElement && epubhtml.IsBlock(strings.ToLower(kid.Get("tagName").String())) {
			pending = js.Undefined()
			continue
		}
		if !pending.Truthy() {
			pending = e.doc.Call("createElement", "p")
			e.root.Call("insertBefore", pending, kid)
		}
		pending.Call("appendChild", kid)
	}
	// Caret filler for empty blocks; empty root gets one empty paragraph.
	if !e.root.Call("hasChildNodes").Bool() {
		e.clearContent()
		return
	}
	blocks := e.root.Get("children")
	for i := 0; i < blocks.Get("length").Int(); i++ {
		b := blocks.Index(i)
		if !b.Call("hasChildNodes").Bool() {
			b.Call("appendChild", e.doc.Call("createElement", "br"))
		}
	}
}
