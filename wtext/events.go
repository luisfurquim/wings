//go:build js && wasm

package wtext

import (
	"syscall/js"

	"github.com/luisfurquim/wings/dom"
)

// The event spine, in order of first defense:
//
//   beforeinput   — inputType allowlist: formatting (native Ctrl+B, iOS
//                   callout) is canceled, paste/drop are canceled and
//                   rerouted through the filter, only plain editing gets
//                   through. Cancelable per the W3C Input Events spec.
//   keydown       — the core's own undo/redo (historyUndo beforeinput is
//                   NOT cancelable, so Ctrl+Z must die here, where it is)
//                   and the plugins' OnKey broadcast.
//   composition   — IME sessions suspend OnKey and canonicalization; the
//                   settle happens on compositionend.
//   MutationObserver — the rearguard: whatever native editing still did
//                   becomes an undo step, is canonicalized, and OnChanged
//                   is scheduled.
//   selectionchange — document-level; filters for the editor and pings
//                   the widget's toolbar refresh.

// allowedInputTypes is the beforeinput allowlist: plain editing only.
// insertCompositionText is here for completeness — it is not cancelable
// anyway. insertReplacementText is spellcheck: blocking it would break
// the corrector, so it passes and the canonicalizer re-cleans after.
var allowedInputTypes = map[string]bool{
	"insertText":            true,
	"insertLineBreak":       true,
	"insertParagraph":       true,
	"insertCompositionText": true,
	"insertReplacementText": true,
	"deleteContentBackward": true,
	"deleteContentForward":  true,
	"deleteContent":         true,
	"deleteWordBackward":    true,
	"deleteWordForward":     true,
	"deleteByCut":           true,
	"deleteByDrag":          true,
}

// wire installs the spine. Root listeners go through dom.AddEvent (freed
// by the widget's disconnect via RmEventsUnder); the document-level
// selectionchange listener sits outside the root, so Detach releases it
// explicitly.
func (e *Editor) wire() {
	e.observerFn = js.FuncOf(func(_ js.Value, args []js.Value) any {
		// Delivery EMPTIES the observer queue: the records exist only in
		// args[0] now, so they must be converted here, not re-queried via
		// takeRecords (which would come back empty).
		e.guard("observer", func() {
			if len(args) > 0 {
				e.onMutations(args[0])
			}
		})
		return nil
	})
	e.observer = js.Global().Get("MutationObserver").New(e.observerFn)
	e.observer.Call("observe", e.root, map[string]any{
		"subtree":               true,
		"childList":             true,
		"characterData":         true,
		"characterDataOldValue": true,
		"attributes":            true,
		"attributeOldValue":     true,
	})

	e.listenerIDs = append(e.listenerIDs,
		dom.AddEvent(e.root, "beforeinput", func(_ js.Value, args []js.Value) any {
			e.guard("beforeinput", func() { e.onBeforeInput(args[0]) })
			return nil
		}, false, false),
		dom.AddEvent(e.root, "keydown", func(_ js.Value, args []js.Value) any {
			e.guard("keydown", func() { e.onKeyDown(args[0]) })
			return nil
		}, false, false),
		dom.AddEvent(e.root, "compositionstart", func(_ js.Value, _ []js.Value) any {
			e.composing = true
			return nil
		}, false, false),
		dom.AddEvent(e.root, "compositionend", func(_ js.Value, _ []js.Value) any {
			e.composing = false
			e.guard("compositionend", func() { e.flushNative() })
			return nil
		}, false, false),
		// Belt and suspenders with beforeinput's insertFromPaste/Drop:
		// both roads are canceled, both feed the same filter.
		dom.AddEvent(e.root, "paste", func(_ js.Value, args []js.Value) any {
			e.guard("paste", func() {
				e.onPasteLike(args[0], args[0].Get("clipboardData"))
			})
			return nil
		}, true, false),
		dom.AddEvent(e.root, "drop", func(_ js.Value, args []js.Value) any {
			e.guard("drop", func() {
				e.onPasteLike(args[0], args[0].Get("dataTransfer"))
			})
			return nil
		}, true, false),
	)

	e.selListener = dom.AddEvent(e.doc, "selectionchange",
		func(_ js.Value, _ []js.Value) any {
			e.guard("selectionchange", func() {
				e.checkPendingAnchor()
				if e.onSelChange != nil {
					if _, ok := e.Sel(); ok {
						e.onSelChange()
					}
				}
			})
			e.endTurn()
			return nil
		}, false, false)
}

// unwire releases every listener wire installed.
func (e *Editor) unwire() {
	for _, id := range e.listenerIDs {
		dom.RmEvent(id)
	}
	e.listenerIDs = nil
	dom.RmEvent(e.selListener)
}

// guard runs fn under a recover: a bug in the editor or a plugin must
// degrade and log, never take the whole wasm app down with it (a panic
// here is total loss — there is no process isolation to fall back on).
func (e *Editor) guard(where string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			G.Logf(1, "wtext: recovered in %s: %v\n", where, r)
		}
	}()
	fn()
}

// ── beforeinput ─────────────────────────────────────────────────────────

// onBeforeInput enforces the inputType allowlist.
func (e *Editor) onBeforeInput(ev js.Value) {
	it := ev.Get("inputType").String()
	// Armed pending marks intercept the first typing at the anchor and
	// re-do it with the marks applied (pending.go). IME composition is
	// not cancelable, so it falls through to the native path.
	if it == "insertText" && len(e.pending) > 0 && !e.composing {
		if e.insertPending(ev) {
			return
		}
	}
	if allowedInputTypes[it] {
		return
	}
	ev.Call("preventDefault")
	switch it {
	case "insertFromPaste", "insertFromDrop":
		e.onPasteLike(ev, ev.Get("dataTransfer"))
	case "historyUndo":
		e.Undo() // not cancelable per spec, but preventDefault + our own is harmless
	case "historyRedo":
		e.Redo()
	}
	// Everything else — format*, insertLink, insertHorizontalRule... —
	// simply does not happen: formatting exists only through the toolbar.
}

// ── keydown ─────────────────────────────────────────────────────────────

// onKeyDown owns undo/redo and broadcasts OnKey to the edition plugins.
func (e *Editor) onKeyDown(ev js.Value) {
	defer e.endTurn()
	if e.composing || ev.Get("isComposing").Bool() {
		return // IME session: the browser owns the keys
	}
	ctrl := ev.Get("ctrlKey").Bool() || ev.Get("metaKey").Bool()
	shift := ev.Get("shiftKey").Bool()
	if ctrl {
		switch ev.Get("key").String() {
		case "z", "Z":
			ev.Call("preventDefault")
			if shift {
				e.Redo()
			} else {
				e.Undo()
			}
			return
		case "y", "Y":
			ev.Call("preventDefault")
			e.Redo()
			return
		}
	}
	if len(e.profile.Edition) == 0 {
		return
	}
	ctx := &KeyCtx{
		Key:   ev.Get("key").String(),
		Code:  ev.Get("code").String(),
		Ctrl:  ev.Get("ctrlKey").Bool(),
		Alt:   ev.Get("altKey").Bool(),
		Shift: shift,
		Meta:  ev.Get("metaKey").Bool(),
	}
	for _, p := range e.profile.Edition {
		p.OnKey(e, ctx)
		if ctx.stopped {
			break
		}
	}
	if ctx.consumed {
		ev.Call("preventDefault")
	}
}

// ── paste / drop ────────────────────────────────────────────────────────

// onPasteLike routes clipboard-shaped payloads through the filter and the
// clipboard plugins, and inserts the result as one undo step. The raw
// markup never touches the document.
func (e *Editor) onPasteLike(ev js.Value, dt js.Value) {
	ev.Call("preventDefault")
	defer e.endTurn()
	if !dt.Truthy() {
		return
	}
	var f Fragment
	if html := dt.Call("getData", "text/html").String(); html != "" {
		var err error
		f, err = e.sanitizeHTML(html)
		if err != nil {
			G.Logf(1, "wtext: paste rejected: %v\n", err)
			return
		}
	} else if text := dt.Call("getData", "text/plain").String(); text != "" {
		f = e.sanitizeText(text)
	}
	if f.Empty() {
		return
	}
	for _, p := range e.profile.Clipboard {
		var err error
		f, err = p.OnPaste(e, f)
		if err != nil {
			G.Logf(1, "wtext: clipboard plugin rejected paste: %v\n", err)
			return
		}
	}
	sel, ok := e.Sel()
	if !ok {
		return
	}
	if err := e.Replace(sel, f); err != nil {
		G.Logf(1, "wtext: paste insert failed: %v\n", err)
	}
}

// ── MutationObserver flush ──────────────────────────────────────────────

// onMutations turns a delivered batch of native edits into an undo step
// and schedules the OnChanged broadcast. The converted ops ride through
// bufOps: during IME composition they wait there for compositionend, and
// takeOps folds them in front of any later synchronous drain.
func (e *Editor) onMutations(records js.Value) {
	if e.applying {
		return // undo/redo application discards its own mutations
	}
	e.bufOps = append(e.bufOps, e.recordsToOps(records)...)
	if e.composing {
		return // settle once, on compositionend
	}
	e.flushNative()
}

// flushNative captures pending native mutations (canonicalizing inside
// the same step) and schedules OnChanged.
func (e *Editor) flushNative() {
	ops := e.takeOps()
	if len(ops) == 0 {
		return
	}
	e.pushNative(ops)
	e.scheduleChanged()
}

// scheduleChanged coalesces OnChanged dispatches into one per frame.
func (e *Editor) scheduleChanged() {
	if e.changedQueue {
		return
	}
	e.changedQueue = true
	var cb js.Func
	cb = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		defer cb.Release()
		e.changedQueue = false
		e.guard("onchanged", func() { e.dispatchChanged() })
		return nil
	})
	js.Global().Call("requestAnimationFrame", cb)
}

// dispatchChanged broadcasts OnChanged. Writes made by handlers are core
// writes (their steps commit synchronously); whatever NATIVE records
// appear as a side effect count as a cascade, bounded so a reactive loop
// degrades into a logged stop instead of a spin.
func (e *Editor) dispatchChanged() {
	if e.inOnChanged {
		return
	}
	e.inOnChanged = true
	defer func() {
		e.inOnChanged = false
		e.cascades = 0
		e.endTurn()
	}()
	for {
		sel, ok := e.Sel()
		if !ok {
			sel = Selection{}
		}
		for _, p := range e.profile.Edition {
			p.OnChanged(e, sel)
		}
		ops := e.takeOps()
		if len(ops) == 0 {
			return
		}
		e.pushNative(ops)
		e.cascades++
		if e.cascades >= maxCascades {
			G.Logf(1, "wtext: OnChanged cascade bound (%d) hit; dropping further rounds\n",
				maxCascades)
			return
		}
	}
}
