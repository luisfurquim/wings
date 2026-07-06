//go:build js && wasm

package text

import (
	"strings"
	"syscall/js"

	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
	"github.com/luisfurquim/wings/wtext"
)

// The core RENDERS the toolbar; a plugin only DECLARES items. Rendering
// uses wings' own widgets (dogfooding): toggles/buttons become w-button,
// selects become w-combobox. State refresh (active toggles, current
// block) is the core's job too — it re-queries the tracked closures on
// selectionchange and updates attributes without rebuilding the DOM.

// toolbar renders one editor's items and tracks the controls that need
// live state refresh.
type toolbar struct {
	obj       *wings.PranaObj
	host      js.Value
	container js.Value
	editor    *wtext.Editor
	profile   wtext.Profile
	toggles   []trackedToggle
	selects   []trackedSelect
}

type trackedToggle struct {
	el     js.Value
	active func(wtext.EditorCore) bool
}

type trackedSelect struct {
	el      js.Value
	current func(wtext.EditorCore) string
}

func newToolbar(obj *wings.PranaObj, container js.Value, editor *wtext.Editor, p wtext.Profile) *toolbar {
	return &toolbar{obj: obj, host: obj.Element, container: container, editor: editor, profile: p}
}

// render draws every item of every toolbar plugin into the container. It is
// also the re-translation path: an OnRetranslate re-render rebuilds the
// controls with freshly resolved labels, so the tracked slices are reset
// first to avoid accumulating stale element references.
func (t *toolbar) render() {
	t.toggles = nil
	t.selects = nil
	t.container.Set("innerHTML", "") // static container; safe empty string
	for _, plug := range t.profile.Toolbar {
		for _, item := range plug.Items() {
			t.renderItem(item)
		}
	}
	t.refresh()
}

// renderItem draws one item by kind. The sealed ToolbarItem set means
// this type switch is total: only this package's kinds can appear.
func (t *toolbar) renderItem(item wtext.ToolbarItem) {
	switch it := item.(type) {
	case wtext.ToggleItem:
		t.button(it.ID, it.Label, it.Icon, func() error { return it.Do(t.editor) })
		if it.Active != nil {
			t.trackToggle(it.Active)
		}
	case wtext.ButtonItem:
		el := t.button(it.ID, it.Label, it.Icon, func() error { return it.Do(t.editor) })
		if it.Enabled != nil && !it.Enabled(t.editor) {
			el.Call("setAttribute", "disabled", "")
		}
	case wtext.SelectItem:
		t.selectControl(it)
	case wtext.Separator:
		sep := t.doc().Call("createElement", "div")
		sep.Call("setAttribute", "class", "wt-sep")
		t.container.Call("appendChild", sep)
	}
}

// button creates a w-button for a toggle/action item. Label text goes in
// as a resolved i18n string; an icon (Material symbol name) becomes the
// accessible-name-backed glyph. mousedown is prevented so clicking the
// button never collapses the editor's selection before the action runs.
func (t *toolbar) button(id, labelID, icon string, do func() error) js.Value {
	btn := t.doc().Call("createElement", "w-button")
	btn.Call("setAttribute", "type", "button")
	btn.Call("setAttribute", "variant", "ghost")
	btn.Call("setAttribute", "size", "sm")
	btn.Call("setAttribute", "data-item", id)
	label := t.resolveLabel(labelID)
	btn.Call("setAttribute", "aria-label", label)
	btn.Call("setAttribute", "title", label)
	if icon != "" {
		btn.Set("textContent", iconGlyph(icon, label))
	} else {
		btn.Set("textContent", label)
	}
	// Decision 8.2: keep the selection alive across the click.
	dom.AddEvent(btn, "mousedown", func(_ js.Value, _ []js.Value) any { return nil }, true, false)
	dom.AddEvent(btn, "click", func(_ js.Value, _ []js.Value) any {
		if err := do(); err != nil {
			G.Logf(1, "w-text: toolbar action %q failed: %v\n", id, err)
		}
		t.afterAction()
		return nil
	}, false, false)
	t.container.Call("appendChild", btn)
	return btn
}

// selectControl creates a w-combobox for a SelectItem. The combobox
// reports selection through the wings @change trigger (it fires no native
// DOM event), so we register a per-item handler in the host's data map and
// point the combobox's @change attribute at it — the same wiring a parent
// template would write by hand.
func (t *toolbar) selectControl(it wtext.SelectItem) {
	cb := t.doc().Call("createElement", "w-combobox")
	cb.Call("setAttribute", "mode", "single")
	cb.Call("setAttribute", "data-item", it.ID)
	cb.Call("setAttribute", "aria-label", t.resolveLabel(it.Label))
	opts := it.Options(t.editor)
	arr := make([]any, 0, len(opts))
	for _, o := range opts {
		arr = append(arr, map[string]any{"value": o.Value, "label": t.resolveLabel(o.Label)})
	}
	cb.Call("setAttribute", "options", jsonStringify(arr))

	key := "wt_sel_" + it.ID
	t.obj.This.Set(key, func(args ...any) {
		val := firstSelectedValue(args)
		if val == "" {
			return
		}
		if err := it.Pick(t.editor, val); err != nil {
			G.Logf(1, "w-text: block pick %q failed: %v\n", val, err)
		}
		t.afterAction()
	})
	cb.Call("setAttribute", "@change", key)

	dom.AddEvent(cb, "mousedown", func(_ js.Value, _ []js.Value) any { return nil }, true, false)
	t.container.Call("appendChild", cb)
	if it.Current != nil {
		t.selects = append(t.selects, trackedSelect{el: cb, current: it.Current})
	}
}

func (t *toolbar) trackToggle(active func(wtext.EditorCore) bool) {
	// The most recently appended button is this toggle's element.
	kids := t.container.Get("children")
	if n := kids.Get("length").Int(); n > 0 {
		t.toggles = append(t.toggles, trackedToggle{el: kids.Index(n - 1), active: active})
	}
}

// afterAction re-checks toolbar state and notifies the parent that
// content changed (programmatic edits do not fire a native input event).
func (t *toolbar) afterAction() {
	t.refresh()
	if editEl := t.editorEl(); editEl.Truthy() {
		editEl.Call("dispatchEvent",
			js.Global().Get("Event").New("input", map[string]any{"bubbles": true}))
	}
}

// refresh re-queries the tracked closures and reflects state to the
// controls, without rebuilding the DOM.
func (t *toolbar) refresh() {
	for _, tg := range t.toggles {
		if tg.active(t.editor) {
			tg.el.Call("setAttribute", "data-active", "")
		} else {
			tg.el.Call("removeAttribute", "data-active")
		}
	}
	for _, sl := range t.selects {
		cur := sl.current(t.editor)
		if cur != "" {
			sl.el.Set("value", cur)
		}
	}
}

// ── helpers ─────────────────────────────────────────────────────────────

func (t *toolbar) doc() js.Value { return t.host.Get("ownerDocument") }

// editorEl returns the shadow .wt-editor of this instance.
func (t *toolbar) editorEl() js.Value {
	shadow := t.host.Get("shadowRoot")
	if !shadow.Truthy() {
		return js.Undefined()
	}
	els := dom.Query(shadow, ".wt-editor")
	if len(els) == 0 {
		return js.Undefined()
	}
	return els[0]
}

// resolveLabel maps a toolbar label id to display text: a translated
// slotted <span slot="labels" id="..."> in the host light DOM (swept by
// gen_i18n), then a document element with that id, then a built-in
// default, then the id itself. Same escalation as w-input error messages.
func (t *toolbar) resolveLabel(id string) string {
	if id == "" {
		return ""
	}
	esc := js.Global().Get("CSS").Call("escape", id).String()
	sel := "[slot=\"labels\"][id=\"" + esc + "\"]"
	if el := t.host.Call("querySelector", sel); el.Truthy() {
		if txt := strings.TrimSpace(el.Get("textContent").String()); txt != "" {
			return txt
		}
	}
	if el := t.doc().Call("getElementById", id); el.Truthy() {
		if txt := strings.TrimSpace(el.Get("textContent").String()); txt != "" {
			return txt
		}
	}
	if def, ok := defaultLabels[id]; ok {
		return def
	}
	return id
}

// defaultLabels is the built-in English fallback for the stock toolbar,
// so w-text is usable before an app supplies a translated catalog.
var defaultLabels = map[string]string{
	"wtext-bold":        "Bold",
	"wtext-italic":      "Italic",
	"wtext-code":        "Code",
	"wtext-block":       "Style",
	"wtext-block-p":     "Paragraph",
	"wtext-block-h1":    "Heading 1",
	"wtext-block-h2":    "Heading 2",
	"wtext-block-h3":    "Heading 3",
	"wtext-block-h4":    "Heading 4",
	"wtext-block-h5":    "Heading 5",
	"wtext-block-h6":    "Heading 6",
	"wtext-block-quote": "Quote",
	"wtext-block-pre":   "Code block",
}

// iconGlyph maps a small set of Material symbol names to a unicode glyph,
// so the stock toolbar needs no icon font. Unknown names fall back to the
// accessible label's first rune.
func iconGlyph(name, label string) string {
	switch name {
	case "format_bold":
		return "B"
	case "format_italic":
		return "I"
	case "code":
		return "</>"
	}
	if label != "" {
		return string([]rune(label)[:1])
	}
	return "?"
}

// firstSelectedValue pulls the value out of a w-combobox @change payload
// delivered through the wings trigger (args[0] = []any of
// map[string]any{"label","value"}; single mode carries 0 or 1).
func firstSelectedValue(args []any) string {
	if len(args) == 0 {
		return ""
	}
	arr, ok := args[0].([]any)
	if !ok || len(arr) == 0 {
		return ""
	}
	m, ok := arr[0].(map[string]any)
	if !ok {
		return ""
	}
	v, _ := m["value"].(string)
	return v
}

// jsonStringify serializes a Go value to a JSON string via the JS engine
// (the value is built from strings we control — option labels/values).
func jsonStringify(v any) string {
	return js.Global().Get("JSON").Call("stringify", v).String()
}
