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

// sanitizeHTML parses hostile markup and reduces it to a Fragment.
func (e *Editor) sanitizeHTML(html string) (Fragment, error) {
	parsed := js.Global().Get("DOMParser").New().
		Call("parseFromString", html, "text/html")
	body := parsed.Get("body")
	if !body.Truthy() {
		return Fragment{}, nil
	}
	var f Fragment
	f.nodes = e.copyChildren(body, false)
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
		return []fnode{{tag: "br"}}
	}
	if epubhtml.IsBlock(canon) && inline {
		// A block inside a mark cannot exist in the profile: dissolve it.
		return e.copyChildren(el, inline)
	}
	node := fnode{tag: canon, attrs: e.copyAttrs(el, canon)}
	node.kids = e.copyChildren(el, inline || epubhtml.IsMark(canon))
	if epubhtml.IsMark(canon) && len(node.kids) == 0 {
		return nil // empty mark: nothing to mark
	}
	return []fnode{node}
}

// copyAttrs keeps only what the policy names for the canonical tag,
// validating each value for its kind — hrefs are canonicalized under the
// profile's LinkPolicy, class lists are cut down to registered names. A
// value that fails validation is not copied: the element survives as
// text carrier, the attribute does not (fail toward text).
func (e *Editor) copyAttrs(el js.Value, canon string) map[string]string {
	var attrs map[string]string
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
			var keep []string
			for _, cls := range strings.Fields(el.Call("getAttribute", name).String()) {
				if e.classDefined(cls) {
					keep = append(keep, cls)
				}
			}
			if len(keep) > 0 {
				if attrs == nil {
					attrs = map[string]string{}
				}
				attrs["class"] = strings.Join(keep, " ")
			}
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
