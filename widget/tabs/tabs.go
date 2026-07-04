//go:build js && wasm

// Package tabs provides the w-tabs custom element for wings.
//
// w-tabs is a controlled tab container. The single source of truth for which
// panel is visible is the host's `active` attribute — a w-tab tid (or a
// positional index). Authors write w-tab panels as children; the w-tabbutton
// buttons are OPTIONAL sugar.
//
// # Two usage shapes
//
// 1. With buttons (simple tabs): co-locate each button with its panel as direct
// children — button,panel,button,panel. w-tabs routes the buttons into a strip
// and a click sets `active` for you:
//
//	<w-tabs mode="panel">
//	    <w-tabbutton active>Overview</w-tabbutton>
//	    <w-tab>…</w-tab>
//	    <w-tabbutton>Details</w-tabbutton>
//	    <w-tab>…</w-tab>
//	</w-tabs>
//
// 2. Headless (controlled): provide NO w-tabbutton. w-tabs imposes no chrome
// and renders its content transparently (passthrough), so the consumer's own
// layout reaches the panels. The consumer positions its own controls anywhere
// and drives selection by binding the `active` attribute:
//
//	<w-tabs mode="detached" active="{{current}}">
//	    <header>… your buttons call a handler that sets `current` …</header>
//	    <w-tab tid="a">…</w-tab>
//	    <w-tab tid="b">…</w-tab>
//	</w-tabs>
//
// Button↔panel pairing (shape 1) is positional; selection (shape 2) is by tid.
//
// # Modes
//
//   - panel (default) — button strip on top (scrolls when it overflows), panel
//     below.
//   - detached — same shape, chip-like buttons, transparent panels.
//   - menu — button column on the left, panel on the right.
//   - accordion — w-tabs MOVES each w-tabbutton into its w-tab so it becomes
//     the native <summary>; open/close is the platform's job.
//
// # The accordion move
//
// Moving a node fires disconnectedCallback, which the framework treats
// destructively (it deletes reactive state). Safe ONLY because w-tabbutton is a
// stateless, CSS-only leaf — see widget/tabbutton.
//
// # Events to parent
//
//	@change — fired after a user-driven (click/keyboard) activation; args[0] is
//	          the selected tid (or positional index as a string). Not fired at
//	          init or for programmatic `active` changes.
package tabs

import (
	_ "embed"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
)

const elementTag = "w-tabs"

// G is the logger for this module.
var G goose.Alert

//go:embed tabs.html
var htmlContent string

//go:embed vars.css
var varsCSS string

//go:embed design.css
var designCSS string

var cssParts = []wings.CSSPart{
	{Name: "Vars", Content: ""},
	{Name: "Design", Content: ""},
}

func buildCSS() string {
	var sb strings.Builder
	for _, p := range cssParts {
		sb.WriteString(p.Content)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// New creates a new Tabs instance.
func New() *Tabs {
	return &Tabs{}
}

func init() {
	G.Set(3)
	cssParts[0].Content = varsCSS
	cssParts[1].Content = designCSS
	wings.Register(
		elementTag,
		htmlContent,
		buildCSS(),
		func() wings.PranaMod { return &Tabs{} },
	)
	G.Logf(3, "w-tabs: module registered\n")
}

// Tabs implements wings.PranaMod and wings.Customizable for the w-tabs custom element.
type Tabs struct{}

var _ wings.Customizable = (*Tabs)(nil)

func (t *Tabs) ListCSS() []wings.CSSPart {
	result := make([]wings.CSSPart, len(cssParts))
	copy(result, cssParts)
	return result
}

func (t *Tabs) ReplaceCSS(key string, content string) {
	for i := range cssParts {
		if cssParts[i].Name == key {
			cssParts[i].Content = content
			wings.Update(elementTag, buildCSS())
			return
		}
	}
	G.Logf(1, "ReplaceCSS: key %q not found\n", key)
}

func (t *Tabs) InitData() map[string]any {
	// No template bindings: the controller reads mode/active straight off the
	// host element and arranges the light DOM imperatively.
	return map[string]any{}
}

// ── small DOM helpers ───────────────────────────────────────────────────────

func query(root js.Value, tag string) []js.Value { return dom.Query(root, tag) }

func attr(el js.Value, name string) string {
	v := el.Call("getAttribute", name)
	if v.IsNull() || v.IsUndefined() {
		return ""
	}
	return v.String()
}

func hasAttr(el js.Value, name string) bool {
	return el.Call("hasAttribute", name).Bool()
}

func setActive(el js.Value, on bool) {
	if on {
		el.Call("setAttribute", "active", "")
	} else {
		el.Call("removeAttribute", "active")
	}
}

func tabMode(host js.Value) string {
	if m := attr(host, "mode"); m != "" {
		return m
	}
	return "panel"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// closestButton walks up from target to find the w-tabbutton, stopping at host.
func closestButton(target, host js.Value) js.Value {
	el := target
	for !el.IsNull() && !el.IsUndefined() {
		if el.Equal(host) {
			break
		}
		tag := el.Get("tagName")
		if !tag.IsUndefined() && !tag.IsNull() && strings.EqualFold(tag.String(), "w-tabbutton") {
			return el
		}
		el = el.Get("parentElement")
	}
	return js.Null()
}

// ── arrangement ─────────────────────────────────────────────────────────────

// sync stamps the current mode on every child and, when buttons are present,
// routes each one: in accordion it is moved into its paired panel (becoming the
// <summary>); otherwise it is a direct child of the host (routed to the strip).
// With no buttons the host goes "headless" — no strip, transparent passthrough.
func (t *Tabs) sync(host js.Value) {
	mode := tabMode(host)
	buttons := query(host, "w-tabbutton")
	panels := query(host, "w-tab")
	for _, p := range panels {
		p.Call("setAttribute", "mode", mode)
	}
	for _, b := range buttons {
		b.Call("setAttribute", "mode", mode)
	}

	if len(buttons) == 0 {
		// Headless: selection is driven entirely by the `active` attribute.
		host.Call("setAttribute", "headless", "")
		return
	}
	host.Call("removeAttribute", "headless")

	n := minInt(len(buttons), len(panels))
	for i := 0; i < n; i++ {
		b, p := buttons[i], panels[i]
		b.Call("setAttribute", "slot", "button")
		if mode == "accordion" {
			if !b.Get("parentElement").Equal(p) {
				p.Call("insertBefore", b, p.Get("firstChild"))
			}
			b.Call("removeAttribute", "tabindex")
		} else {
			if !b.Get("parentElement").Equal(host) {
				host.Call("insertBefore", b, p) // restore: right before its panel
			}
			b.Call("setAttribute", "tabindex", "0")
		}
	}
}

// ── selection (controlled by the `active` attribute) ────────────────────────

func (t *Tabs) pairIndex(host, button js.Value) int {
	buttons := query(host, "w-tabbutton")
	for i := range buttons {
		if buttons[i].Equal(button) {
			return i
		}
	}
	return -1
}

// panelValue returns the canonical `active` value for the i-th panel: its tid
// if set, otherwise the positional index as a string.
func (t *Tabs) panelValue(host js.Value, idx int) string {
	panels := query(host, "w-tab")
	if idx >= 0 && idx < len(panels) {
		if tid := attr(panels[idx], "tid"); tid != "" {
			return tid
		}
	}
	return strconv.Itoa(idx)
}

// resolveIndex maps the host's `active` value to a panel index (tid match, then
// numeric index; default 0).
func (t *Tabs) resolveIndex(host js.Value, panels []js.Value) int {
	want := attr(host, "active")
	if want == "" {
		return 0
	}
	for i := range panels {
		if attr(panels[i], "tid") == want {
			return i
		}
	}
	if n, err := strconv.Atoi(want); err == nil && n >= 0 && n < len(panels) {
		return n
	}
	return 0
}

// showActive renders the panel selected by the `active` attribute and mirrors
// the selection onto the buttons. No-op in accordion (native <details> owns it).
func (t *Tabs) showActive(host js.Value) {
	if tabMode(host) == "accordion" {
		return
	}
	panels := query(host, "w-tab")
	buttons := query(host, "w-tabbutton")
	idx := t.resolveIndex(host, panels)
	for i := range panels {
		setActive(panels[i], i == idx)
	}
	for i := range buttons {
		setActive(buttons[i], i == idx)
	}
}

// initActive sets the initial `active` value (honouring an existing attribute or
// a markup-level active flag, else the first panel) and renders it — silently.
func (t *Tabs) initActive(host js.Value) {
	if tabMode(host) == "accordion" {
		return
	}
	if attr(host, "active") != "" {
		t.showActive(host) // respect a value already set (e.g. data-bound)
		return
	}
	idx := 0
	buttons := query(host, "w-tabbutton")
	panels := query(host, "w-tab")
	found := false
	for i := range buttons {
		if hasAttr(buttons[i], "active") {
			idx, found = i, true
			break
		}
	}
	if !found {
		for i := range panels {
			if hasAttr(panels[i], "active") {
				idx = i
				break
			}
		}
	}
	host.Call("setAttribute", "active", t.panelValue(host, idx))
	t.showActive(host)
}

// activate is the user-driven path: set `active` to the i-th panel's value,
// render, and fire @change.
func (t *Tabs) activate(obj *wings.PranaObj, idx int) {
	host := obj.Element
	val := t.panelValue(host, idx)
	host.Call("setAttribute", "active", val)
	t.showActive(host)
	obj.Trigger("change", val)
}

// ── lifecycle ───────────────────────────────────────────────────────────────

func (t *Tabs) Render(obj *wings.PranaObj) {
	host := obj.Element

	// Initial arrangement + selection. Silent: the parent's TriggerHandler may
	// not be wired yet, so firing @change now could hit a nil handler.
	t.sync(host)
	t.initActive(host)

	// Click delegation — one listener catches clicks from any button.
	dom.AddEvent(host, "click", func(_ js.Value, args []js.Value) any {
		if len(args) == 0 || tabMode(host) == "accordion" {
			return nil // accordion: native <details> toggle handles it
		}
		btn := closestButton(args[0].Get("target"), host)
		if !btn.IsNull() && !btn.IsUndefined() {
			if idx := t.pairIndex(host, btn); idx >= 0 {
				t.activate(obj, idx)
			}
		}
		return nil
	}, false, false)

	// Keyboard — buttons are <span>s, so Enter/Space activation is on us; focus
	// rides a tabindex on the host.
	dom.AddEvent(host, "keydown", func(_ js.Value, args []js.Value) any {
		if len(args) == 0 || tabMode(host) == "accordion" {
			return nil
		}
		ev := args[0]
		switch ev.Get("key").String() {
		case "Enter", " ", "Spacebar":
		default:
			return nil
		}
		btn := closestButton(ev.Get("target"), host)
		if !btn.IsNull() && !btn.IsUndefined() {
			if idx := t.pairIndex(host, btn); idx >= 0 {
				ev.Call("preventDefault")
				t.activate(obj, idx)
			}
		}
		return nil
	}, false, false)

	// React to runtime changes of the host's own mode/active attributes (the
	// latter is how a headless consumer drives selection via data binding).
	// dom.Observe registers the observer for auto-release on disconnect.
	dom.Observe(host, map[string]any{
		"attributes":      true,
		"attributeFilter": []any{"mode", "active"},
	}, func(_ js.Value, margs []js.Value) any {
		if len(margs) == 0 {
			return nil
		}
		muts := margs[0]
		for i := 0; i < muts.Get("length").Int(); i++ {
			switch muts.Index(i).Get("attributeName").String() {
			case "mode":
				t.sync(host)
				t.initActive(host)
			case "active":
				t.showActive(host)
			}
		}
		return nil
	})
}
