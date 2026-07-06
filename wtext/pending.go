//go:build js && wasm

package wtext

import (
	"sort"
	"strings"
	"syscall/js"
)

// Pending marks are the Word behaviour at a collapsed caret: toggle bold
// with nothing selected and the NEXT text typed comes out bold; toggle it
// off inside a bold run and the next typing escapes it. The mark cannot be
// applied to the tree yet (an empty mark is removed by the canonicalizer,
// and rightly so), so it is held as editor state anchored to the caret:
//
//	arm     — Wrap/Unwrap on a collapsed selection (mutate.go) record the
//	          intent per tag; InMark overlays it so the toolbar lights up.
//	consume — the first insertText beforeinput at the anchor is canceled
//	          and re-done as a core write with the pending marks applied.
//	disarm  — the caret moving anywhere else inside the editor drops the
//	          intent. A trip to a toolbar control (selection leaves the
//	          editor and comes back to the anchor) keeps it.

// pendingMark is one armed intent: on=true wraps the next typing in mark;
// on=false lifts it out of the enclosing mark element.
type pendingMark struct {
	mark Mark // set when on (carries the tag and a Link's canonical href)
	on   bool
}

// armPendingOn records "next typing gets m" at the collapsed s. When the
// caret already sits inside the mark, arming is a no-op that cancels a
// pending removal (the natural state is back).
func (e *Editor) armPendingOn(s Selection, m Mark) error {
	from, err := e.resolve(s.From)
	if err != nil {
		return err
	}
	if e.markAncestor(from, m.tag).Truthy() {
		e.dropPending(m.tag)
		return nil
	}
	e.setPending(from, s.From.Offset, m.tag, pendingMark{mark: m, on: true})
	return nil
}

// armPendingOff records "next typing escapes tag" at the collapsed s. When
// the caret is not inside the mark, it just cancels a pending application.
func (e *Editor) armPendingOff(s Selection, tag string) error {
	from, err := e.resolve(s.From)
	if err != nil {
		return err
	}
	tag = strings.ToLower(tag)
	if !e.markAncestor(from, tag).Truthy() {
		e.dropPending(tag)
		return nil
	}
	e.setPending(from, s.From.Offset, tag, pendingMark{on: false})
	return nil
}

// setPending stores one intent, resetting the set when the anchor moved
// (intents armed at another caret position are stale by definition).
func (e *Editor) setPending(node js.Value, off int, tag string, p pendingMark) {
	if len(e.pending) == 0 || !node.Equal(e.pendingNode) || off != e.pendingOff {
		e.pending = map[string]pendingMark{}
		e.pendingNode, e.pendingOff = node, off
	}
	e.pending[strings.ToLower(tag)] = p
}

// dropPending removes one tag's intent.
func (e *Editor) dropPending(tag string) {
	delete(e.pending, strings.ToLower(tag))
	if len(e.pending) == 0 {
		e.clearPending()
	}
}

// clearPending drops every intent and the anchor's node pin.
func (e *Editor) clearPending() {
	e.pending = nil
	e.pendingNode = js.Undefined()
	e.pendingOff = 0
}

// checkPendingAnchor disarms the set when a live in-editor selection sits
// anywhere but the anchor. A selection outside the editor (focus on a
// toolbar control) is NOT a move: the caret may come back via RestoreSel.
func (e *Editor) checkPendingAnchor() {
	if len(e.pending) == 0 {
		return
	}
	rng := e.selectionRange()
	if !rng.Truthy() {
		return
	}
	start, startOff := rng.Get("startContainer"), rng.Get("startOffset").Int()
	end, endOff := rng.Get("endContainer"), rng.Get("endOffset").Int()
	if !e.contains(start) || !e.contains(end) {
		return
	}
	if !start.Equal(e.pendingNode) || startOff != e.pendingOff ||
		!end.Equal(e.pendingNode) || endOff != e.pendingOff {
		e.clearPending()
	}
}

// insertPending consumes the armed set for one insertText: the native
// insertion is canceled and the text lands as a core write, wrapped in
// the pending-on marks and lifted out of the pending-off ones. Reports
// whether it consumed the event; false lets the caller fall through to
// the native path (and disarms, so a mismatch cannot loop).
func (e *Editor) insertPending(ev js.Value) bool {
	defer e.endTurn()
	data := ev.Get("data")
	if data.Type() != js.TypeString || data.String() == "" {
		return false
	}
	// The caret must still be exactly the anchor; anything else means a
	// move checkPendingAnchor has not seen yet.
	rng := e.selectionRange()
	if !rng.Truthy() {
		e.clearPending()
		return false
	}
	start, startOff := rng.Get("startContainer"), rng.Get("startOffset").Int()
	end, endOff := rng.Get("endContainer"), rng.Get("endOffset").Int()
	if !start.Equal(e.pendingNode) || startOff != e.pendingOff ||
		!end.Equal(e.pendingNode) || endOff != e.pendingOff {
		e.clearPending()
		return false
	}

	ev.Call("preventDefault")
	e.beginWrite()
	// Plain spaces become NBSP, as the browser's own editor does here: at a
	// run boundary the space would collapse, and Chromium refuses to park
	// the caret in collapsed whitespace — the setBaseAndExtent below would
	// silently not move. Native typing re-normalizes them as text grows.
	txt := e.doc.Call("createTextNode",
		strings.ReplaceAll(data.String(), " ", "\u00a0"))
	ins := e.doc.Call("createRange")
	ins.Call("setStart", e.pendingNode, e.pendingOff)
	ins.Call("collapse", true)
	ins.Call("insertNode", txt)
	// insertNode splits a text-node insertion point; a split at the end
	// leaves an empty text sibling that would ride into liftOutOf's clone
	// and keep an empty mark husk alive. Prune it now.
	pruneEmptyTextSibling(txt)
	e.dropCaretFiller(txt)
	// Offs before ons: liftOutOf climbs from txt splitting every level, so
	// wrappers added first would be split right back apart.
	for _, tag := range e.pendingTags(false) {
		if mark := e.markAncestor(txt, tag); mark.Truthy() {
			e.liftOutOf(txt, mark)
		}
	}
	for _, tag := range e.pendingTags(true) {
		if e.markAncestor(txt, tag).Truthy() {
			continue
		}
		p := e.pending[tag]
		wrapper := e.doc.Call("createElement", tag)
		if p.mark.href != "" {
			wrapper.Call("setAttribute", "href", p.mark.href)
			if strings.HasPrefix(p.mark.href, "http://") {
				wrapper.Call("setAttribute", "data-wings-insecure", "")
			}
		}
		txt.Get("parentNode").Call("insertBefore", wrapper, txt)
		wrapper.Call("appendChild", txt)
	}
	e.clearPending()
	e.commitWrite()

	// Caret after the typed text, so further typing flows natively inside
	// (or outside) the mark. setBaseAndExtent reaches into shadow trees in
	// Chromium; a refusal degrades to leaving the caret where it was.
	l := txt.Get("length").Int()
	e.guard("pending-caret", func() {
		e.selectionObj().Call("setBaseAndExtent", txt, l, txt, l)
	})
	// The native input event was suppressed with the insertion; re-emit it
	// so the widget's form value and @input trigger stay live.
	e.root.Call("dispatchEvent", js.Global().Get("Event").New("input",
		map[string]any{"bubbles": true}))
	return true
}

// pendingTags returns the armed tags with the given direction, sorted for
// deterministic nesting.
func (e *Editor) pendingTags(on bool) []string {
	var tags []string
	for tag, p := range e.pending {
		if p.on == on {
			tags = append(tags, tag)
		}
	}
	sort.Strings(tags)
	return tags
}

// pruneEmptyTextSibling removes zero-length text nodes flanking txt (the
// leftovers of Range.insertNode splitting at a text-node boundary).
func pruneEmptyTextSibling(txt js.Value) {
	for _, side := range []string{"previousSibling", "nextSibling"} {
		if sib := txt.Get(side); sib.Truthy() &&
			sib.Get("nodeType").Int() == 3 && sib.Get("length").Int() == 0 {
			sib.Call("remove")
		}
	}
}

// dropCaretFiller removes the <br> caret filler when txt just became the
// block's real content (native typing does the same cleanup on its own).
func (e *Editor) dropCaretFiller(txt js.Value) {
	next := txt.Get("nextSibling")
	if !next.Truthy() || next.Get("nodeType").Int() != 1 ||
		strings.ToLower(next.Get("tagName").String()) != "br" {
		return
	}
	if !next.Get("nextSibling").Truthy() && !txt.Get("previousSibling").Truthy() {
		next.Call("remove")
	}
}
