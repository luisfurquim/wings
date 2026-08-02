//go:build js && wasm

package text

import (
	"strconv"
	"strings"
	"syscall/js"

	"github.com/luisfurquim/wings/dom"
	"github.com/luisfurquim/wings/internal/jsguard"
)

// The CSS inspector: a toolbar toggle that, while on, shows the selectors
// in effect wherever the mouse rests in the text.
//
// It exists because of what carrying a document's own stylesheet changed.
// Most of what formats an imported book is now rules the reader can
// neither apply nor remove — "p.haikai", ".chtitle *" — and which stay
// out of the style picker on purpose, so there was no way left to find
// out why a paragraph looks the way it does. A drop cap losing its float
// took two rounds to diagnose for exactly this reason.
//
// Opt-in per instance, through the `inspect` attribute on <w-text>:
// inspecting CSS is an affordance for someone editing a document that
// came from elsewhere, and a permanent extra button in every toolbar
// would be a cost paid by every app that uses the widget.

// maxTipSelectors bounds what one tooltip lists. Past a handful the
// answer stops being readable, and "what is in effect here" is a question
// about a few rules, not about a stylesheet.
const maxTipSelectors = 12

// inspectEnabled reports whether this instance asked for the inspector.
//
// Read straight off the host rather than through an observed attribute:
// observing it would put the value in the reactive store, where the
// TEMPLATE bindings react — and the toolbar is built imperatively, so it
// would not. An attribute that looks live and is not would be worse than
// one that is honestly read at mount.
func (t *toolbar) inspectEnabled() bool {
	return t.host.Call("hasAttribute", "inspect").Bool()
}

// renderInspect appends the toggle, following the composed-help button's
// precedent (renderHelp): a control the widget owns, outside the plugin
// system, since the mode is about looking at the document rather than
// about changing it.
func (t *toolbar) renderInspect() {
	if !t.inspectEnabled() {
		return
	}
	sep := t.doc().Call("createElement", "div")
	sep.Call("setAttribute", "class", "wt-sep")
	t.container.Call("appendChild", sep)

	label := t.resolveLabel("wtext-inspect")
	btn := t.doc().Call("createElement", "w-button")
	btn.Call("setAttribute", "type", "button")
	btn.Call("setAttribute", "variant", "ghost")
	btn.Call("setAttribute", "size", "sm")
	btn.Call("setAttribute", "data-item", "inspect")
	btn.Call("setAttribute", "aria-label", label)
	btn.Call("setAttribute", "title", label)
	btn.Set("textContent", "{}")
	// mousedown is swallowed so opening the mode does not move the caret
	// out of the text the user is about to inspect — the same guard every
	// other toolbar control uses.
	dom.AddEvent(btn, "mousedown", func(_ js.Value, _ []js.Value) any { return nil }, true, false)
	dom.AddEvent(btn, "click", func(_ js.Value, _ []js.Value) any {
		t.toggleInspect()
		return nil
	}, false, false)
	t.container.Call("appendChild", btn)
	t.inspectBtn = btn

	// A re-render (the re-translation path) rebuilt the button, so an
	// inspection already running has to be re-armed against the new one.
	t.markInspect()
	if t.inspect {
		t.armInspect()
	}
}

// toggleInspect flips the mode.
func (t *toolbar) toggleInspect() {
	t.inspect = !t.inspect
	if t.inspect {
		t.armInspect()
	} else {
		t.disarmInspect()
	}
	t.markInspect()
}

// markInspect reflects the mode on the button: data-active for the visual
// the plugin toggles already use, aria-pressed for assistive tech, which
// data-active says nothing to.
func (t *toolbar) markInspect() {
	if !t.inspectBtn.Truthy() {
		return
	}
	if t.inspect {
		t.inspectBtn.Call("setAttribute", "data-active", "")
		t.inspectBtn.Call("setAttribute", "aria-pressed", "true")
		return
	}
	t.inspectBtn.Call("removeAttribute", "data-active")
	t.inspectBtn.Call("setAttribute", "aria-pressed", "false")
}

// armInspect starts watching the pointer. The listeners exist only while
// the mode is on: a mousemove handler is on the hottest path in the
// widget, and an editor nobody is inspecting must not pay for one.
func (t *toolbar) armInspect() {
	t.disarmInspect() // never stack a second set
	ed := t.editorEl()
	if !ed.Truthy() {
		return
	}
	t.inspectIDs = append(t.inspectIDs,
		dom.AddEvent(ed, "mousemove", func(_ js.Value, args []js.Value) any {
			if len(args) > 0 {
				t.inspectMove(args[0])
			}
			return nil
		}, false, false),
		dom.AddEvent(ed, "mouseleave", func(_ js.Value, _ []js.Value) any {
			t.hideTip()
			return nil
		}, false, false),
	)
}

// disarmInspect releases the listeners and the tooltip WITHOUT clearing
// the mode flag: a toolbar rebuild disarms and re-arms, and the user's
// choice has to survive it.
func (t *toolbar) disarmInspect() {
	for _, id := range t.inspectIDs {
		dom.RmEvent(id)
	}
	t.inspectIDs = nil
	t.hideTip()
	if t.tip.Truthy() {
		t.tip.Call("remove")
		t.tip = js.Undefined()
	}
}

// inspectMove handles one pointer move.
func (t *toolbar) inspectMove(evt js.Value) {
	el := elementUnder(evt)
	if !el.Truthy() {
		t.hideTip()
		return
	}
	// Recompute only when the element under the pointer CHANGES. Matching
	// is every selector against every ancestor — a real book reaches a few
	// hundred comparisons — and the answer cannot change while the pointer
	// travels across one element. Moving within it only repositions.
	if !el.Equal(t.tipFor) {
		t.tipFor = el
		sels := t.selectorsFor(el)
		if len(sels) == 0 {
			// Nothing reaches this text. Say nothing: a tooltip announcing
			// its own emptiness is noise following the pointer around.
			t.hideTip()
			return
		}
		t.showTip(strings.Join(sels, "\n"))
	}
	if t.tip.Truthy() {
		t.placeTip(evt)
	}
}

// elementUnder returns the element the event landed on, stepping up from
// a text node — a mousemove over text targets the element, but a
// defensive parentElement costs nothing and covers the node case.
func elementUnder(evt js.Value) js.Value {
	target := evt.Get("target")
	if !target.Truthy() {
		return js.Undefined()
	}
	if target.Get("nodeType").Int() != 1 {
		return target.Get("parentElement")
	}
	return target
}

// selectorsFor lists the selectors in effect at el: those matching it,
// then those matching each ancestor up to (but not including) the editor
// root, which is the widget's own container rather than document content.
// Innermost first, deduplicated — an ancestor repeating a selector adds
// nothing to the answer.
func (t *toolbar) selectorsFor(el js.Value) []string {
	probes := t.editor.StyleProbes()
	if len(probes) == 0 {
		return nil
	}
	root := t.editorEl()
	var out []string
	seen := map[string]bool{}
	for cur := el; cur.Truthy() && !cur.Equal(root); cur = cur.Get("parentElement") {
		tag := strings.ToLower(cur.Get("tagName").String())
		for _, p := range probes {
			// A named style's paragraph half is rendered as an :is() over
			// every block tag — true, and unreadable. Its Show carries a
			// "%s" for the tag that actually matched; every other probe
			// shows itself, and the replace is then a no-op.
			show := strings.Replace(p.Show, "%s", tag, 1)
			if seen[show] || !matchesSelector(cur, p.Match) {
				continue
			}
			seen[show] = true
			out = append(out, show)
			if len(out) >= maxTipSelectors {
				return out
			}
		}
	}
	return out
}

// matchesSelector asks the BROWSER whether el matches sel — the selector
// was never parsed by us, which is the whole design of carrying a
// document's stylesheet.
//
// It therefore may not even be valid CSS, and matches() THROWS on an
// invalid selector. Through syscall/js a JS exception is a panic, which
// in a wasm editor loses the user's document; hence the recover. A
// selector the browser cannot read matches nothing, which is also what it
// does when rendered into the sheet.
func matchesSelector(el js.Value, sel string) bool {
	hit, err := jsguard.Value("matches", func() bool {
		return el.Call("matches", sel).Bool()
	})
	if err != nil {
		G.Logf(2, "w-text: selector %q is not valid CSS (%v); it matches nothing\n", sel, err)
		return false
	}
	return hit
}

// showTip puts text in the tooltip, creating it on first use.
//
// The tooltip is a sibling of the editing surface inside .wt-field, never
// a child of it: inside the contenteditable it would become part of the
// document and travel out through Content(). Text goes in through
// textContent — the selectors are foreign text, and this widget never
// writes innerHTML.
func (t *toolbar) showTip(text string) {
	if !t.tip.Truthy() {
		field := t.container.Get("parentNode")
		if !field.Truthy() {
			return
		}
		t.tip = t.doc().Call("createElement", "div")
		t.tip.Call("setAttribute", "class", "wt-tip")
		t.tip.Call("setAttribute", "part", "tip")
		t.tip.Call("setAttribute", "aria-hidden", "true")
		field.Call("appendChild", t.tip)
	}
	t.tip.Set("textContent", text)
	t.tip.Call("removeAttribute", "hidden")
}

// hideTip hides the tooltip and forgets what it described, so the next
// move over the same element recomputes instead of resurrecting a stale
// answer.
func (t *toolbar) hideTip() {
	t.tipFor = js.Undefined()
	if t.tip.Truthy() {
		t.tip.Call("setAttribute", "hidden", "")
	}
}

// placeTip positions the tooltip near the pointer, in coordinates
// relative to .wt-field.
//
// Absolute, never fixed. A fixed-position element resolves against the
// nearest ancestor that establishes a containing block rather than
// against the viewport, and w-tab's atmosphere opt-in sets a non-"none"
// backdrop-filter unconditionally — the trap already documented at
// openHelp, which sent a dialog off-screen.
func (t *toolbar) placeTip(evt js.Value) {
	field := t.container.Get("parentNode")
	if !field.Truthy() {
		return
	}
	box := field.Call("getBoundingClientRect")
	x := evt.Get("clientX").Float() - box.Get("left").Float() + 12
	y := evt.Get("clientY").Float() - box.Get("top").Float() + 16

	// Keep it inside the field: past the right edge it would be clipped or
	// widen the widget, so it flips to the pointer's other side.
	if w := t.tip.Get("offsetWidth").Float(); w > 0 {
		if max := box.Get("width").Float() - w - 4; x > max {
			x = max
		}
	}
	if x < 0 {
		x = 0
	}
	// Assigning to style PROPERTIES rather than interpolating a style
	// attribute: the browser parses each as one value, so no assignment
	// here can turn into a second declaration.
	style := t.tip.Get("style")
	style.Set("left", strconv.FormatFloat(x, 'f', 1, 64)+"px")
	style.Set("top", strconv.FormatFloat(y, 'f', 1, 64)+"px")
}
