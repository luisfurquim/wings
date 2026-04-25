//go:build js && wasm

package wprana

import (
	"syscall/js"
)

// RetranslateAll re-applies the active Printer to every live custom element
// instance, re-parses the resulting strings into TextSegs / AttrBinding.Segs,
// and triggers a re-render. It is the runtime entry point used by
// wi18n.SetLang() to flip the catalog without rebuilding the DOM.
//
// The walk relies on the _wi18nSrc / _wi18nAttr_* expandos planted by
// translateTextNodes (and propagated by cloneNode → copyTranslateStash).
// Nodes without a stash are left untouched, so non-i18n text and bindings
// are preserved verbatim.
func RetranslateAll() {
	for _, instances := range instanceRegistry {
		for _, el := range instances {
			retranslateInstance(el)
		}
	}
}

// retranslateInstance walks one custom element's shadow tree in lockstep
// with its DOMRefNode tree, refreshes bindings, and triggers sync.
func retranslateInstance(el js.Value) {
	nodeID, ok := getNodeID(el)
	if !ok {
		return
	}
	st, found := nodeRegistry[nodeID]
	if !found || st.State == nil {
		return
	}
	state := st.State
	rebindWalk(state.model, state.Refs)
	applyStashSweep(state.model)
	if state.Data != nil {
		state.Data.Sync()
	}
}

// applyStashSweep walks the DOM subtree and rewrites nodeValue / attribute
// values for every textnode and element carrying _wi18nSrc / _wi18nAttr_*.
// It is the counterpart to rebindWalk: rebindWalk re-parses TextSegs for
// nodes that carry a DOMRefNode (so syncText can render placeholder-bearing
// translations), while this sweep covers the textnodes that have no ref
// because their original translation was a plain string with no `{{}}`.
//
// For nodes that DO have a ref, this sweep writes the new value first and
// the subsequent Sync() overwrites it from TextSegs — harmless redundancy.
func applyStashSweep(node js.Value) {
	if node.IsNull() || node.IsUndefined() {
		return
	}
	nt := node.Get("nodeType").Int()
	if nt == 1 { // ELEMENT_NODE
		for _, a := range TranslatableAttrs {
			srcVal := node.Get("_wi18nAttr_" + a)
			if srcVal.Type() == js.TypeString {
				node.Call("setAttribute", a, Printer(srcVal.String()))
			}
		}
	} else if nt == 3 { // TEXT_NODE
		srcVal := node.Get("_wi18nSrc")
		if srcVal.Type() == js.TypeString {
			node.Set("nodeValue", Printer(srcVal.String()))
		}
		return
	}
	kids := node.Get("childNodes")
	n := kids.Get("length").Int()
	for i := 0; i < n; i++ {
		applyStashSweep(kids.Index(i))
	}
}

// rebindWalk walks one DOM node paired with its DOMRefNode, re-parsing every
// stashed binding into fresh TextSegs / AttrBinding.Segs from the current
// Printer output.
func rebindWalk(dom js.Value, ref *DOMRefNode) {
	if ref == nil || dom.IsNull() || dom.IsUndefined() {
		return
	}

	if ref.Type == TokTxt {
		srcVal := dom.Get("_wi18nSrc")
		if srcVal.Type() == js.TypeString {
			if segs, err := cachedParseText(Printer(srcVal.String())); err == nil {
				ref.TextSegs = segs
			}
		}
		return
	}

	for attrName, ab := range ref.Attrs {
		srcVal := dom.Get("_wi18nAttr_" + attrName)
		if srcVal.Type() == js.TypeString {
			if segs, err := cachedParseText(Printer(srcVal.String())); err == nil {
				ab.Segs = segs
			}
		}
	}

	if ref.Cond != "" && nodeType(dom) == jsNodeComment {
		st := getState(dom)
		if st != nil && st.CondModel.Truthy() {
			rebindChildren(st.CondModel, ref)
		}
		return
	}

	if ref.ArrayVar != "" {
		st := getState(dom)
		if st == nil || !st.Model.Truthy() {
			return
		}
		if ref.NoSpan && ref.ModelRef != nil {
			rebindWalk(st.Model, ref.ModelRef)
		} else {
			rebindChildren(st.Model, ref)
		}
		return
	}

	rebindChildren(dom, ref)
}

func rebindChildren(dom js.Value, ref *DOMRefNode) {
	kids := dom.Get("childNodes")
	for idx, childRef := range ref.Children {
		child := kids.Index(idx)
		if child.IsUndefined() || child.IsNull() {
			continue
		}
		rebindWalk(child, childRef)
	}
}
