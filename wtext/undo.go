//go:build js && wasm

package wtext

import "syscall/js"

// The undo machine has ONE capture pipeline: every mutation — native
// typing, canonicalization, core methods, even Range.deleteContents —
// is recorded by the MutationObserver and converted to invertible ops by
// drainRecords. Core code therefore mutates the DOM freely and calls
// commitWrite; it never bookkeeps ops by hand.

// jsOp is one invertible primitive mutation.
type jsOp interface {
	undo()
	redo()
	// bytes approximates retained size for the undo byte budget.
	bytes() int
}

// opText is a characterData change, coalesced per node within one step:
// old is the value before the step, new the value after it (intermediate
// states are irrelevant to undo/redo semantics).
type opText struct {
	node     js.Value
	old, new string
}

func (o *opText) undo() { o.node.Set("data", o.old) }
func (o *opText) redo() { o.node.Set("data", o.new) }
func (o *opText) bytes() int {
	return len(o.old) + len(o.new) + 64
}

// opAttr is an attribute change, coalesced per (node, name).
type opAttr struct {
	node           js.Value
	name           string
	old, new       string
	hadOld, hasNew bool
}

func (o *opAttr) undo() { setOrRemoveAttr(o.node, o.name, o.old, o.hadOld) }
func (o *opAttr) redo() { setOrRemoveAttr(o.node, o.name, o.new, o.hasNew) }
func (o *opAttr) bytes() int {
	return len(o.name) + len(o.old) + len(o.new) + 64
}

func setOrRemoveAttr(node js.Value, name, val string, has bool) {
	if has {
		node.Call("setAttribute", name, val)
	} else {
		node.Call("removeAttribute", name)
	}
}

// opChild is one node inserted (added=true) or removed at a position
// described by the mutation-time siblings. Removed nodes stay referenced
// by the op — pinned until the step falls off the stack — so undo can
// reinsert the very same node.
type opChild struct {
	parent, node, prev js.Value
	added              bool
}

func (o *opChild) undo() {
	if o.added {
		removeIfChild(o.parent, o.node)
	} else {
		insertAfter(o.parent, o.node, o.prev)
	}
}

func (o *opChild) redo() {
	if o.added {
		insertAfter(o.parent, o.node, o.prev)
	} else {
		removeIfChild(o.parent, o.node)
	}
}

// bytes charges a rough per-node cost plus the retained text length.
func (o *opChild) bytes() int {
	n := 128
	// textContent is a JS string; Value.Get on it would panic. len of its
	// UTF-8 form is a fine approximation for a byte-budget heuristic.
	if txt := o.node.Get("textContent"); txt.Truthy() {
		n += len(txt.String())
	}
	return n
}

// insertAfter puts node back into parent right after prev (or first when
// prev is null). Ops are applied in reverse order on undo, so at apply
// time the DOM is in the state this record described.
func insertAfter(parent, node, prev js.Value) {
	if prev.Truthy() {
		parent.Call("insertBefore", node, prev.Get("nextSibling"))
	} else {
		parent.Call("insertBefore", node, parent.Get("firstChild"))
	}
}

// removeIfChild removes node from parent, tolerating an already-detached
// node (fail-operational: undo must never throw and kill the app).
func removeIfChild(parent, node js.Value) {
	if p := node.Get("parentNode"); p.Truthy() && p.Equal(parent) {
		parent.Call("removeChild", node)
	}
}

// ── Capture ─────────────────────────────────────────────────────────────

// drainRecords converts the observer's pending MutationRecords into ops.
// characterData and attribute records are coalesced (first oldValue →
// live value); childList records stay ordered — their sibling references
// describe the DOM at mutation time, which is exactly the state undo
// recreates by inverting later ops first.
func (e *Editor) drainRecords() []jsOp {
	records := e.observer.Call("takeRecords")
	n := records.Get("length").Int()
	if n == 0 {
		return nil
	}
	var ops []jsOp
	var textOps []*opText
	var attrOps []*opAttr
	for i := 0; i < n; i++ {
		rec := records.Index(i)
		switch rec.Get("type").String() {
		case "characterData":
			node := rec.Get("target")
			found := false
			for _, t := range textOps {
				if t.node.Equal(node) {
					found = true
					break
				}
			}
			if !found {
				textOps = append(textOps, &opText{
					node: node,
					old:  rec.Get("oldValue").String(),
				})
			}
		case "attributes":
			node := rec.Get("target")
			name := rec.Get("attributeName").String()
			found := false
			for _, a := range attrOps {
				if a.node.Equal(node) && a.name == name {
					found = true
					break
				}
			}
			if !found {
				old := rec.Get("oldValue")
				attrOps = append(attrOps, &opAttr{
					node: node, name: name,
					old: old.String(), hadOld: !old.IsNull(),
				})
			}
		case "childList":
			target := rec.Get("target")
			removed := rec.Get("removedNodes")
			for j := 0; j < removed.Get("length").Int(); j++ {
				ops = append(ops, &opChild{
					parent: target, node: removed.Index(j),
					prev: rec.Get("previousSibling"), added: false,
				})
			}
			added := rec.Get("addedNodes")
			for j := 0; j < added.Get("length").Int(); j++ {
				node := added.Index(j)
				ops = append(ops, &opChild{
					parent: target, node: node,
					prev: node.Get("previousSibling"), added: true,
				})
			}
		}
	}
	// Coalesced ops read their final state from the live DOM at drain
	// time and are appended after the structural ops: on undo they run
	// first (reverse order), setting content on nodes that may be
	// detached at that instant — the value sticks and rides along when a
	// later inverse reinserts the node.
	for _, t := range textOps {
		t.new = t.node.Get("data").String()
		if t.new != t.old {
			ops = append(ops, t)
		}
	}
	for _, a := range attrOps {
		cur := a.node.Call("getAttribute", a.name)
		a.hasNew = !cur.IsNull()
		if a.hasNew {
			a.new = cur.String()
		}
		if a.hasNew != a.hadOld || a.new != a.old {
			ops = append(ops, a)
		}
	}
	return ops
}

// discardRecords drops pending records (used right after undo/redo apply
// and content loads: those mutations must not capture themselves).
func (e *Editor) discardRecords() {
	e.observer.Call("takeRecords")
}

// ── Write bracketing ────────────────────────────────────────────────────

// beginWrite closes any pending native-typing step before a core write,
// so the user's own typing and the plugin's mutation stay separate undo
// steps.
func (e *Editor) beginWrite() {
	if ops := e.drainRecords(); len(ops) > 0 {
		e.pushNative(ops)
	}
}

// commitWrite collects everything the core write just mutated into one
// step — or into the open transaction.
func (e *Editor) commitWrite() {
	ops := e.drainRecords()
	if len(ops) == 0 {
		return
	}
	if e.inTxn {
		e.txnOps = append(e.txnOps, ops...)
		return
	}
	e.undo.Push(ops)
}

// pushNative records a step captured from native editing (typing,
// spellcheck). Canonicalization runs first, inside the same step, so
// Ctrl+Z undoes the canonicalized edit as one thing.
func (e *Editor) pushNative(ops []jsOp) {
	e.canonicalize()
	ops = append(ops, e.drainRecords()...)
	e.undo.Push(ops)
}

// rollback inverts ops right away (a failed Txn).
func (e *Editor) rollback(ops []jsOp) {
	e.applyStep(ops, true)
}

// ── Undo / Redo ─────────────────────────────────────────────────────────

// Undo reverts the newest step.
func (e *Editor) Undo() {
	e.beginWrite()
	if step, ok := e.undo.PopUndo(); ok {
		e.applyStep(step, true)
	}
}

// Redo reapplies the newest undone step.
func (e *Editor) Redo() {
	e.beginWrite()
	if step, ok := e.undo.PopRedo(); ok {
		e.applyStep(step, false)
	}
}

// applyStep runs a step's inverses (invert=true, reverse order) or its
// forward ops (invert=false, original order), keeping the observer from
// capturing the application itself.
func (e *Editor) applyStep(step []jsOp, invert bool) {
	e.applying = true
	defer func() {
		e.discardRecords()
		e.applying = false
		if r := recover(); r != nil {
			// A corrupt op must not kill the whole wasm app; the step is
			// already popped, so we log and carry on (sec-fail-operational).
			G.Logf(1, "wtext: recovered while applying undo step: %v\n", r)
		}
	}()
	if invert {
		for i := len(step) - 1; i >= 0; i-- {
			step[i].undo()
		}
	} else {
		for i := 0; i < len(step); i++ {
			step[i].redo()
		}
	}
}

// Txn groups every write fn performs into one undo step; an error rolls
// the group back.
func (e *Editor) Txn(fn func(EditorCore) error) error {
	if e.inTxn {
		return fn(e) // nested Txn joins the outer step
	}
	e.beginWrite()
	e.inTxn = true
	e.txnOps = nil
	err := fn(e)
	e.commitWrite() // catch trailing uncommitted mutations
	e.inTxn = false
	if err != nil {
		e.rollback(e.txnOps)
		e.txnOps = nil
		return err
	}
	e.undo.Push(e.txnOps)
	e.txnOps = nil
	return nil
}
