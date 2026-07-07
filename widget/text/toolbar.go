//go:build js && wasm

package text

import (
	"encoding/json"
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
	el       js.Value
	current  func(wtext.EditorCore) string
	options  func(wtext.EditorCore) []wtext.Option
	lastOpts string // JSON last written to the options attribute
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
	case wtext.InputItem:
		t.inputControl(it)
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
	optsJSON := t.optionsJSON(it.Options(t.editor))
	cb.Call("setAttribute", "options", optsJSON)

	key := "wt_sel_" + it.ID
	t.obj.This.Set(key, func(args ...any) {
		val := firstSelectedValue(args)
		if val == "" {
			return
		}
		if err := it.Pick(t.editor, val); err != nil {
			G.Logf(1, "w-text: block pick %q failed: %v\n", val, err)
		}
		// Focus-taking control: hand focus and the selection back to the
		// editor now that the pick landed.
		t.editor.RestoreSel()
		t.afterAction()
	})
	cb.Call("setAttribute", "@change", key)

	// Unlike the buttons, the combobox MUST take focus — its dropdown opens
	// on the inner input's focus and the user may type to filter. The
	// editor's remembered selection keeps the pick anchored meanwhile.
	t.container.Call("appendChild", cb)
	if it.Current != nil || it.Options != nil {
		t.selects = append(t.selects, trackedSelect{
			el: cb, current: it.Current, options: it.Options, lastOpts: optsJSON,
		})
	}
}

// cbOption is one combobox option in its JSON wire shape. A struct, not a
// map: marshaling must be byte-stable, because refresh only rewrites the
// options attribute when the JSON differs — a spurious rewrite makes the
// combobox reload and re-filter its open dropdown.
type cbOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// optionsJSON renders a SelectItem's options as the combobox JSON, with
// labels resolved through the catalog (a label absent from it — a style
// name — displays as itself).
func (t *toolbar) optionsJSON(opts []wtext.Option) string {
	arr := make([]cbOption, 0, len(opts))
	for _, o := range opts {
		arr = append(arr, cbOption{Value: o.Value, Label: t.resolveLabel(o.Label)})
	}
	b, err := json.Marshal(arr)
	if err != nil {
		G.Logf(1, "w-text: options marshal: %v\n", err)
		return "[]"
	}
	return string(b)
}

// inputControl renders an InputItem: a w-button that opens a small
// popover holding a w-input plus confirm/cancel buttons. The opener is
// focus-transparent like the other buttons; the w-input takes focus while
// the editor's remembered selection keeps the action anchored (the same
// contract as the combobox), and RestoreSel returns focus and selection
// after either exit.
func (t *toolbar) inputControl(it wtext.InputItem) {
	wrap := t.doc().Call("createElement", "span")
	wrap.Call("setAttribute", "class", "wt-inputitem")
	t.container.Call("appendChild", wrap)

	label := t.resolveLabel(it.Label)
	btn := t.doc().Call("createElement", "w-button")
	btn.Call("setAttribute", "type", "button")
	btn.Call("setAttribute", "variant", "ghost")
	btn.Call("setAttribute", "size", "sm")
	btn.Call("setAttribute", "data-item", it.ID)
	btn.Call("setAttribute", "aria-label", label)
	btn.Call("setAttribute", "title", label)
	if it.Icon != "" {
		btn.Set("textContent", iconGlyph(it.Icon, label))
	} else {
		btn.Set("textContent", label)
	}
	wrap.Call("appendChild", btn)

	pop := t.doc().Call("createElement", "div")
	pop.Call("setAttribute", "class", "wt-popover")
	pop.Call("setAttribute", "hidden", "")
	placeholder := t.resolveLabel(it.Placeholder)
	inp := t.doc().Call("createElement", "w-input")
	inp.Call("setAttribute", "type", "text")
	inp.Call("setAttribute", "placeholder", placeholder)
	inp.Call("setAttribute", "aria-label", placeholder)
	pop.Call("appendChild", inp)
	okBtn := t.doc().Call("createElement", "w-button")
	okBtn.Call("setAttribute", "type", "button")
	okBtn.Call("setAttribute", "size", "sm")
	okBtn.Set("textContent", t.resolveLabel("wtext-ok"))
	pop.Call("appendChild", okBtn)
	cancelBtn := t.doc().Call("createElement", "w-button")
	cancelBtn.Call("setAttribute", "type", "button")
	cancelBtn.Call("setAttribute", "variant", "ghost")
	cancelBtn.Call("setAttribute", "size", "sm")
	cancelBtn.Set("textContent", t.resolveLabel("wtext-cancel"))
	pop.Call("appendChild", cancelBtn)
	wrap.Call("appendChild", pop)

	setValue := func(v string) {
		inp.Set("value", v)
		if shadow := inp.Get("shadowRoot"); shadow.Truthy() {
			if els := dom.Query(shadow, "input"); len(els) > 0 {
				els[0].Set("value", v)
			}
		}
	}
	dismiss := func() {
		pop.Call("setAttribute", "hidden", "")
		t.editor.RestoreSel()
	}
	confirm := func() {
		val := ""
		if v := inp.Get("value"); v.Type() == js.TypeString {
			val = strings.TrimSpace(v.String())
		}
		dismiss()
		if val == "" {
			return
		}
		if err := it.Do(t.editor, val); err != nil {
			G.Logf(1, "w-text: toolbar input %q failed: %v\n", it.ID, err)
			return
		}
		t.afterAction()
	}
	open := func() {
		setValue("")
		pop.Call("removeAttribute", "hidden")
		if shadow := inp.Get("shadowRoot"); shadow.Truthy() {
			if els := dom.Query(shadow, "input"); len(els) > 0 {
				els[0].Call("focus")
				return
			}
		}
		inp.Call("focus")
	}

	// Decision 8.2: the opener never steals the selection; the w-input
	// inside the popover does take focus once open.
	dom.AddEvent(btn, "mousedown", func(_ js.Value, _ []js.Value) any { return nil }, true, false)
	dom.AddEvent(btn, "click", func(_ js.Value, _ []js.Value) any {
		if pop.Call("hasAttribute", "hidden").Bool() {
			open()
		} else {
			dismiss()
		}
		return nil
	}, false, false)
	dom.AddEvent(okBtn, "click", func(_ js.Value, _ []js.Value) any {
		confirm()
		return nil
	}, false, false)
	dom.AddEvent(cancelBtn, "click", func(_ js.Value, _ []js.Value) any {
		dismiss()
		return nil
	}, false, false)
	// Enter confirms, Escape cancels (keydown composes across the w-input
	// shadow boundary, so listening on the host sees the inner input).
	dom.AddEvent(inp, "keydown", func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		switch args[0].Get("key").String() {
		case "Enter":
			args[0].Call("preventDefault")
			confirm()
		case "Escape":
			args[0].Call("preventDefault")
			dismiss()
		}
		return nil
	}, false, false)
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
// controls, without rebuilding the DOM. Dynamic option lists (the style
// picker grows as styles are created) are re-written only when they
// changed; the combobox reloads the attribute when its dropdown opens.
func (t *toolbar) refresh() {
	for _, tg := range t.toggles {
		if tg.active(t.editor) {
			tg.el.Call("setAttribute", "data-active", "")
		} else {
			tg.el.Call("removeAttribute", "data-active")
		}
	}
	for i := range t.selects {
		sl := &t.selects[i]
		if sl.options != nil {
			if j := t.optionsJSON(sl.options(t.editor)); j != sl.lastOpts {
				sl.lastOpts = j
				sl.el.Call("setAttribute", "options", j)
			}
		}
		if sl.current != nil {
			if cur := sl.current(t.editor); cur != "" {
				sl.el.Set("value", cur)
			}
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
	"wtext-block":       "Block",
	"wtext-block-p":     "Paragraph",
	"wtext-block-h1":    "Heading 1",
	"wtext-block-h2":    "Heading 2",
	"wtext-block-h3":    "Heading 3",
	"wtext-block-h4":    "Heading 4",
	"wtext-block-h5":    "Heading 5",
	"wtext-block-h6":    "Heading 6",
	"wtext-block-quote": "Quote",
	"wtext-block-pre":   "Code block",

	"wtext-font":          "Font",
	"wtext-font-default":  "Default font",
	"wtext-font-serif":    "Serif",
	"wtext-font-sans":     "Sans-serif",
	"wtext-font-mono":     "Monospace",
	"wtext-font-cursive":  "Cursive",
	"wtext-size":          "Font size",
	"wtext-size-default":  "Default size",
	"wtext-align-left":    "Align left",
	"wtext-align-center":  "Center",
	"wtext-align-right":   "Align right",
	"wtext-align-justify": "Justify",

	"wtext-style":      "Style",
	"wtext-style-new":  "Create style from selection",
	"wtext-style-name": "Style name",
	"wtext-style-none": "(no style)",
	"wtext-ok":         "Apply",
	"wtext-cancel":     "Cancel",
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
	case "format_align_left":
		return "⇤"
	case "format_align_center":
		return "↔"
	case "format_align_right":
		return "⇥"
	case "format_align_justify":
		return "≣"
	case "style_new":
		return "✎"
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
