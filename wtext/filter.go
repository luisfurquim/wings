//go:build js && wasm

package wtext

import (
	"strings"
	"syscall/js"

	"github.com/luisfurquim/wings/epubhtml"
)

// The filter is allowlist-by-copy: hostile HTML is parsed by the
// browser's own parser (DOMParser — the same parser that would interpret
// the result later, so there is no differential for mutation-XSS), the
// parsed tree is walked, and a portable Fragment is built holding only
// what the policy admits. Nothing is pruned in place; the dangerous is
// simply never copied. The Fragment is then materialized into real nodes.
// Paste, drop and SetContent all take this one road.

// sanitizeHTML parses hostile markup and reduces it to a Fragment. Many
// sources (plain editors, chat apps, some word processors' partial-copy
// path) export a paragraph break as a run of consecutive <br> instead of
// separate block elements — copyChildren already preserves those runs
// faithfully inside whatever single block they landed in (or, if the
// source had no wrapping block at all, as bare top-level nodes);
// splitTopLevelBreaks (portable, fragment.go) turns any 2+ run into a
// real block boundary afterward, at the one place both shapes converge.
func (e *Editor) sanitizeHTML(html string) (Fragment, error) {
	parsed := js.Global().Get("DOMParser").New().
		Call("parseFromString", html, "text/html")
	body := parsed.Get("body")
	if !body.Truthy() {
		return Fragment{}, nil
	}
	var f Fragment
	f.nodes = splitTopLevelBreaks(e.copyChildren(body, false))
	return f, nil
}

// sanitizeText reduces plain clipboard text to a Fragment: lines become
// text runs separated by <br>, blank lines split paragraphs.
func (e *Editor) sanitizeText(text string) Fragment {
	b := NewFragment()
	for _, para := range strings.Split(text, "\n\n") {
		lines := strings.Split(para, "\n")
		b.Block("p", func(n Node) {
			for i, line := range lines {
				if i > 0 {
					n.Br()
				}
				n.Text(line)
			}
		})
	}
	f, err := b.Done()
	if err != nil {
		// Unreachable by construction (p and br are always legal), but
		// degrade to empty rather than panic.
		G.Logf(1, "wtext: sanitizeText: %v\n", err)
		return Fragment{}
	}
	return f
}

// copyChildren walks src's children and returns the policy-approved copy.
// inline marks that the surrounding context is a mark: block elements
// found there are unwrapped (their children flow up), never nested.
func (e *Editor) copyChildren(src js.Value, inline bool) []fnode {
	var out []fnode
	kids := src.Get("childNodes")
	n := kids.Get("length").Int()
	for i := 0; i < n; i++ {
		kid := kids.Index(i)
		switch kid.Get("nodeType").Int() {
		case 3: // text
			txt := epubhtml.CleanText(kid.Get("data").String(), epubhtml.DocumentText)
			if txt != "" {
				out = append(out, fnode{text: txt})
			}
		case 1: // element
			out = append(out, e.copyElement(kid, inline)...)
		}
		// Everything else (comments, PIs, CDATA) is not copied.
	}
	return out
}

// copyElement applies the element policy to one node. It returns a slice
// because Unwrap dissolves the element into its (filtered) children.
func (e *Editor) copyElement(el js.Value, inline bool) []fnode {
	tag := strings.ToLower(el.Get("tagName").String())
	pol := epubhtml.ElementFor(tag)
	switch pol.Disposition {
	case epubhtml.Drop:
		return nil
	case epubhtml.Unwrap:
		return e.copyChildren(el, inline)
	}
	canon := pol.Canonical
	if canon == "br" {
		if el.Get("classList").Call("contains", "Apple-interchange-newline").Bool() {
			// WebKit's own clipboard-boundary marker (Safari, Mail.app,
			// Notes, and anything built on WebKit's editing stack) — a
			// trailing filler <br> the paste mechanism itself adds, not
			// content the source ever displayed. Same standing as <meta>.
			return nil
		}
		return []fnode{{tag: "br"}}
	}
	if epubhtml.IsBlock(canon) && inline {
		// A block inside inline content cannot exist in the profile:
		// dissolve it.
		return e.copyChildren(el, inline)
	}
	if epubhtml.IsMark(canon) && hasBlockChild(el) {
		// A "mark" wrapper whose actual children are blocks is not real
		// inline formatting — it's a malformed clipboard wrapper. Google
		// Docs is the common real-world case: it wraps the ENTIRE copied
		// selection in a single <b style="font-weight:normal"
		// id="docs-internal-guid-...">, even when nothing was bold,
		// purely to cancel <b>'s default rendering — <b> can only hold
		// inline content per the HTML spec, but DOMParser (like every
		// browser) tolerates the block <p> children anyway. Canonicalizing
		// that wrapper to <strong> and then applying the block-inside-
		// inline dissolve above (the correct rule for content that IS
		// malformed) would both falsely bold every Docs paste and erase
		// every paragraph break in it, with nothing — not even a <br> —
		// left to show where they were. Unwrap the wrapper instead: its
		// block children survive as themselves, in the SAME inline
		// context this element itself was found in.
		return e.copyChildren(el, inline)
	}
	node := fnode{tag: canon, attrs: e.copyAttrs(el, canon)}
	if epubhtml.RequiresClass(canon) && node.attrs["class"] == "" {
		// A classless carrier says nothing: dissolve it.
		return e.copyChildren(el, inline)
	}
	node.kids = e.copyChildren(el, inline || epubhtml.IsInline(canon))
	if epubhtml.IsInline(canon) && len(node.kids) == 0 {
		return nil // empty inline element marks nothing
	}
	if epubhtml.IsBlock(canon) && !inline {
		if groups := splitOnBreaks(node.kids); len(groups) > 1 {
			out := make([]fnode, 0, len(groups))
			for _, g := range groups {
				out = append(out, fnode{tag: canon, attrs: node.attrs, kids: g})
			}
			return out
		}
	}
	return []fnode{node}
}

// hasBlockChild reports whether el's direct element children include
// anything the profile would keep as a block — the sign of a malformed
// mark-candidate wrapper (see the Google Docs case above copyElement).
func hasBlockChild(el js.Value) bool {
	kids := el.Get("children") // element children only, no text nodes
	n := kids.Get("length").Int()
	for i := 0; i < n; i++ {
		tag := strings.ToLower(kids.Index(i).Get("tagName").String())
		pol := epubhtml.ElementFor(tag)
		if pol.Disposition == epubhtml.Keep && epubhtml.IsBlock(pol.Canonical) {
			return true
		}
	}
	return false
}

// copyAttrs keeps only what the policy names for the canonical tag,
// validating each value for its kind — hrefs are canonicalized under the
// profile's LinkPolicy, class lists are cut down to registered names, and
// an inline style is filtered down to the properties the profile
// supports and turned into a registered class (see AttrStyle) rather
// than copied as an attribute. A value that fails validation is not
// copied: the element survives as text carrier, the attribute does not
// (fail toward text). class and style both contribute to the SAME
// output class list — addClass appends regardless of which attribute
// ran first, so neither overwrites the other's contribution.
func (e *Editor) copyAttrs(el js.Value, canon string) map[string]string {
	var attrs map[string]string
	addClass := func(cls string) {
		if cls == "" {
			return
		}
		if attrs == nil {
			attrs = map[string]string{}
		}
		if cur := attrs["class"]; cur != "" {
			attrs["class"] = cur + " " + cls
		} else {
			attrs["class"] = cls
		}
	}
	names := el.Call("getAttributeNames")
	n := names.Get("length").Int()
	for i := 0; i < n; i++ {
		name := strings.ToLower(names.Index(i).String())
		switch epubhtml.AttrFor(canon, name) {
		case epubhtml.AttrHref:
			canonHref, err := epubhtml.CanonicalizeHref(
				el.Call("getAttribute", name).String(), e.profile.LinkPolicy)
			if err != nil {
				continue
			}
			if attrs == nil {
				attrs = map[string]string{}
			}
			attrs["href"] = canonHref
		case epubhtml.AttrClass:
			for _, cls := range strings.Fields(el.Call("getAttribute", name).String()) {
				// A registered class, or one a preserved document rule
				// selects on: the second is what makes "p.haikai" work at
				// all, since the rule needs the element to still carry the
				// class it names (see Editor.docClasses).
				if e.classDefined(cls) || e.docClasses[cls] {
					addClass(cls)
				}
			}
		case epubhtml.AttrStyle:
			// FilterCSS, not SanitizeCSS: this is a pasted element's own
			// style="", authored by whatever app produced the clipboard
			// payload, not a webdev's DefineClass call — SanitizeCSS's
			// reject-the-whole-list-on-one-bad-property contract is wrong
			// here (real documents routinely mix a couple of properties
			// this profile supports with several it was never meant to).
			clean := epubhtml.FilterCSS(el.Call("getAttribute", name).String())
			if clean == "" {
				continue // nothing the profile recognizes survived
			}
			cls := epubhtml.PasteClassName(clean)
			if err := e.DefineClass(cls, clean); err != nil {
				continue // unreachable in practice: FilterCSS output is always valid
			}
			addClass(cls)
		}
	}
	return attrs
}

// ── Materialization ─────────────────────────────────────────────────────

// materialize turns a Fragment into a DocumentFragment. Fragments are
// valid by construction; the one live re-check is the href (a Builder
// fragment canonicalized with a nil policy, and the app's LinkPolicy must
// still have its say). Links over plain http get the core-generated
// data-wings-insecure badge — the filter never copies an incoming one, so
// the badge is trustworthy.
func (e *Editor) materialize(f Fragment) js.Value {
	frag := e.doc.Call("createDocumentFragment")
	for i := range f.nodes {
		frag.Call("appendChild", e.materializeNode(&f.nodes[i]))
	}
	return frag
}

func (e *Editor) materializeNode(n *fnode) js.Value {
	if n.tag == "" {
		return e.doc.Call("createTextNode", n.text)
	}
	el := e.doc.Call("createElement", n.tag)
	for name, val := range n.attrs {
		if name == "href" {
			canon, err := epubhtml.CanonicalizeHref(val, e.profile.LinkPolicy)
			if err != nil {
				continue // drop the link target, keep the text
			}
			el.Call("setAttribute", "href", canon)
			if strings.HasPrefix(canon, "http://") {
				el.Call("setAttribute", "data-wings-insecure", "")
			}
			continue
		}
		el.Call("setAttribute", name, val)
	}
	for i := range n.kids {
		el.Call("appendChild", e.materializeNode(&n.kids[i]))
	}
	return el
}
