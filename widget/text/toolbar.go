//go:build js && wasm

package text

import (
	"encoding/json"
	"fmt"
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
	menu      js.Value // the side-menu container (.wt-menu), or undefined
	editor    *wtext.Editor
	profile   wtext.Profile
	toggles   []trackedToggle
	selects   []trackedSelect
	statuses  []trackedStatus
	helpDlg   js.Value // the open help dialog, or js.Undefined() when none is open

	// CSS inspector (inspect.go): the mode flag survives a toolbar
	// rebuild, the listeners and the tooltip do not.
	inspect    bool
	inspectBtn js.Value // the toggle, or js.Undefined()
	inspectIDs []int64  // pointer listeners held while the mode is on
	tip        js.Value // the tooltip element, or js.Undefined()
	tipFor     js.Value // the element the tooltip currently describes

	cfgDlg     js.Value // the open settings dialog, or js.Undefined()
	cfgRelease func()   // releases the settings dialog's anchor listener
	decDlg     js.Value // the open decision dialog, or js.Undefined()
	unsubFonts func()   // unsubscribes the webfont-registry watcher
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

type trackedStatus struct {
	el     js.Value
	format string // resolved fmt template (re-resolved on every render)
	args   func(wtext.EditorCore) []any
	last   string // text last written, to skip no-op DOM writes
}

func newToolbar(obj *wings.PranaObj, container, menu js.Value, editor *wtext.Editor, p wtext.Profile) *toolbar {
	return &toolbar{
		obj: obj, host: obj.Element, container: container, menu: menu,
		editor: editor, profile: p,
		helpDlg: js.Undefined(), cfgDlg: js.Undefined(), decDlg: js.Undefined(),
		inspectBtn: js.Undefined(), tip: js.Undefined(), tipFor: js.Undefined(),
	}
}

// render draws every item of every toolbar plugin into the container. It is
// also the re-translation path: an OnRetranslate re-render rebuilds the
// controls with freshly resolved labels, so the tracked slices are reset
// first to avoid accumulating stale element references. A help dialog left
// open from before the rebuild is closed too — its "?" button is about to
// be torn down and recreated, and the dialog holds no state worth keeping
// across a rebuild.
func (t *toolbar) render() {
	t.closeHelp()
	t.closeConfig()
	t.closeDecision()
	t.toggles = nil
	t.selects = nil
	t.statuses = nil
	// The inspector's listeners point at elements this rebuild is about to
	// discard. The MODE flag survives (renderInspect re-arms it).
	t.disarmInspect()
	t.container.Set("innerHTML", "") // static container; safe empty string
	for _, plug := range t.profile.Toolbar {
		for _, item := range plug.Items() {
			t.renderItem(item)
		}
	}
	t.renderInspect()
	t.renderHelp()
	t.renderMenu()
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
	case wtext.StatusItem:
		t.statusControl(it)
	case wtext.Separator:
		sep := t.doc().Call("createElement", "div")
		sep.Call("setAttribute", "class", "wt-sep")
		t.container.Call("appendChild", sep)
	}
}

// helpEntry is one resolved row of the toolbar's composed help dialog:
// a control's label and its plugin-supplied explanation, already run
// through resolveLabel.
type helpEntry struct{ label, help string }

// helpEntries walks every plugin's declared items and collects the ones
// that opted into the help dialog (a non-empty Help id) — the mechanism
// by which a ToolbarPlugin "delivers its help" without the widget knowing
// anything about what any particular plugin does. The switch mirrors
// renderItem's: the sealed ToolbarItem set makes it exhaustive, and
// Separator (and any future help-less kind) is simply skipped.
func (t *toolbar) helpEntries() []helpEntry {
	var out []helpEntry
	for _, plug := range t.profile.Toolbar {
		for _, item := range plug.Items() {
			var labelID, helpID string
			switch it := item.(type) {
			case wtext.ToggleItem:
				labelID, helpID = it.Label, it.Help
			case wtext.ButtonItem:
				labelID, helpID = it.Label, it.Help
			case wtext.SelectItem:
				labelID, helpID = it.Label, it.Help
			case wtext.InputItem:
				labelID, helpID = it.Label, it.Help
			case wtext.StatusItem:
				labelID, helpID = it.Label, it.Help
			default:
				continue
			}
			if helpID == "" {
				continue
			}
			out = append(out, helpEntry{label: t.resolveLabel(labelID), help: t.resolveLabel(helpID)})
		}
	}
	for _, plug := range t.profile.Menu {
		for _, item := range plug.MenuItems() {
			var groupID, labelID, helpID string
			switch it := item.(type) {
			case wtext.MenuAction:
				groupID, labelID, helpID = it.Group, it.Label, it.Help
			case wtext.MenuInput:
				groupID, labelID, helpID = it.Group, it.Label, it.Help
			case wtext.MenuUpload:
				groupID, labelID, helpID = it.Group, it.Label, it.Help
			default:
				continue
			}
			if helpID == "" {
				continue
			}
			out = append(out, helpEntry{
				label: t.resolveLabel(groupID) + " › " + t.resolveLabel(labelID),
				help:  t.resolveLabel(helpID),
			})
		}
	}
	// The inspector is the widget's own control rather than a plugin's, so
	// it has no declared item to walk — but a composed help that omits a
	// button sitting in the toolbar is a help that lies. Last, after the
	// plugins, which is also where its button is.
	if t.inspectEnabled() {
		if help := t.resolveLabel("wtext-inspect-help"); help != "" {
			out = append(out, helpEntry{label: t.resolveLabel("wtext-inspect"), help: help})
		}
	}
	return out
}

// renderHelp draws the toolbar's trailing "?" button, when at least one
// item across the active plugins opted into the help dialog. The
// entries are resolved once here (fresh on every render, including an
// OnRetranslate) and closed over by the click handler, which builds and
// opens a fresh <w-dialog> via openHelp — closed again by closeHelp.
func (t *toolbar) renderHelp() {
	entries := t.helpEntries()
	if len(entries) == 0 {
		return
	}
	sep := t.doc().Call("createElement", "div")
	sep.Call("setAttribute", "class", "wt-sep")
	t.container.Call("appendChild", sep)

	label := t.resolveLabel("wtext-help")
	btn := t.doc().Call("createElement", "w-button")
	btn.Call("setAttribute", "type", "button")
	btn.Call("setAttribute", "variant", "ghost")
	btn.Call("setAttribute", "size", "sm")
	btn.Call("setAttribute", "data-item", "help")
	btn.Call("setAttribute", "aria-label", label)
	btn.Call("setAttribute", "title", label)
	btn.Set("textContent", "?")
	dom.AddEvent(btn, "mousedown", func(_ js.Value, _ []js.Value) any { return nil }, true, false)
	dom.AddEvent(btn, "click", func(_ js.Value, _ []js.Value) any {
		t.openHelp(entries)
		return nil
	}, false, false)
	t.container.Call("appendChild", btn)
}

// openHelp builds one entry per control as a definition list inside a
// w-dialog — the composed help of every plugin's items, in toolbar
// order — and mounts it at document.body, a no-op if one is already
// open. It is mounted at body rather than inside the toolbar's own
// container deliberately: <w-dialog>'s overlay is position:fixed, meant
// to cover the viewport, but a fixed-position element is instead
// positioned relative to the nearest ANCESTOR that establishes a new
// containing block — and w-tab's own :host rule sets a non-"none"
// backdrop-filter unconditionally (its "atmosphere opt-in" pattern reads
// blur(var(--wings-surface-blur, 0)), and blur(0) still counts as
// non-"none" per the CSS spec, even though it visually does nothing).
// Nested inside a tab panel, the dialog rendered off-screen, scrolled
// with the panel's own content instead of centering over the viewport —
// exactly the "dark box, no visible content" the mounting-inside-
// container version produced. Mounting at body escapes that (and any
// future ancestor with the same property) entirely.
//
// This also means wings' own @cancel/Trigger plumbing — which resolves
// a named handler by walking UP the DOM ancestor chain from the firing
// element to find a prana ancestor carrying it — can no longer reach
// back into this toolbar's w-text host once the dialog sits outside its
// tree. So the close button is wired directly instead: the shadow DOM
// dialog.html builds (including #dlg-cancel) is present synchronously
// once the element is constructed — elementConstructor calls
// bindElement, which evaluates ?btn_cancel_show off InitData's own
// default (true) before Render (the async half) ever runs — so the
// button can be found and bound the moment the element is created, no
// wait needed.
func (t *toolbar) openHelp(entries []helpEntry) {
	if t.helpDlg.Truthy() {
		return
	}
	dlg := t.doc().Call("createElement", "w-dialog")
	dlg.Call("setAttribute", "buttons", "cancel")
	dlg.Call("setAttribute", "title", t.resolveLabel("wtext-help-title"))

	list := t.doc().Call("createElement", "dl")
	list.Call("setAttribute", "class", "wt-help-list")
	for _, e := range entries {
		dt := t.doc().Call("createElement", "dt")
		dt.Set("textContent", e.label)
		list.Call("appendChild", dt)
		dd := t.doc().Call("createElement", "dd")
		dd.Set("textContent", e.help)
		list.Call("appendChild", dd)
	}
	dlg.Call("appendChild", list)

	if shadow := dlg.Get("shadowRoot"); shadow.Truthy() {
		if els := dom.Query(shadow, "#dlg-cancel"); len(els) > 0 {
			dom.AddEvent(els[0], "click", func(_ js.Value, _ []js.Value) any {
				t.closeHelp()
				return nil
			}, false, false)
		}
	}

	t.helpDlg = dlg
	t.doc().Get("body").Call("appendChild", dlg)
}

// closeHelp removes the open help dialog, if any. Idempotent.
func (t *toolbar) closeHelp() {
	if !t.helpDlg.Truthy() {
		return
	}
	t.helpDlg.Call("remove")
	t.helpDlg = js.Undefined()
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
		t.actionErr(id, do())
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
	label := t.resolveLabel(it.Label)
	cb.Call("setAttribute", "aria-label", label)
	// A filterable dropdown's generic "Type to filter..." placeholder gives
	// no clue what the control is for; the item's own label doubles as one.
	cb.Call("setAttribute", "placeholder", label)
	optsJSON := t.optionsJSON(it.Options(t.editor))
	cb.Call("setAttribute", "options", optsJSON)

	key := "wt_sel_" + it.ID
	t.obj.This.Set(key, func(args ...any) {
		val, ok := firstSelectedValue(args)
		if !ok {
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

	// The not-in-list door (Enter on unmatched text): the face picker
	// uses it to accept a dropped webfont store URL.
	if it.NotInList != nil {
		nk := "wt_nil_" + it.ID
		t.obj.This.Set(nk, func(args ...any) {
			text := ""
			for _, a := range args {
				if s, ok := a.(string); ok && s != "" {
					text = s
					break
				}
			}
			if text == "" {
				return
			}
			if err := it.NotInList(t.editor, strings.TrimSpace(text)); err != nil {
				G.Logf(1, "w-text: %q not-in-list: %v\n", it.ID, err)
			}
			t.editor.RestoreSel()
			t.afterAction()
		})
		cb.Call("setAttribute", "@notinlist", nk)
	}

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
	Font  string `json:"font,omitempty"`
}

// optionsJSON renders a SelectItem's options as the combobox JSON, with
// labels resolved through the catalog (a label absent from it — a style
// name — displays as itself).
func (t *toolbar) optionsJSON(opts []wtext.Option) string {
	arr := make([]cbOption, 0, len(opts))
	for _, o := range opts {
		arr = append(arr, cbOption{Value: o.Value, Label: t.resolveLabel(o.Label), Font: o.Font})
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

	toggle := t.inputPopover(wrap, t.resolveLabel(it.Placeholder),
		func() string { return "" },
		func(val string) {
			t.actionErr(it.ID, it.Do(t.editor, val))
			t.afterAction()
		})

	// Decision 8.2: the opener never steals the selection; the w-input
	// inside the popover does take focus once open.
	dom.AddEvent(btn, "mousedown", func(_ js.Value, _ []js.Value) any { return nil }, true, false)
	dom.AddEvent(btn, "click", func(_ js.Value, _ []js.Value) any {
		toggle()
		return nil
	}, false, false)
}

// inputPopover builds the typed-value prompt shared by toolbar InputItems
// and menu MenuInputs — a .wt-popover with a w-input plus OK/Cancel —
// appended to wrap, returning the open/close toggle. prefill seeds the
// input on EVERY open, selected: Enter keeps the suggestion, typing
// replaces it. confirm receives the trimmed value; empty confirmations
// are discarded here, never delivered.
func (t *toolbar) inputPopover(wrap js.Value, placeholder string, prefill func() string, confirm func(string)) (toggle func()) {
	pop := t.doc().Call("createElement", "div")
	pop.Call("setAttribute", "class", "wt-popover")
	pop.Call("setAttribute", "hidden", "")
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
	doConfirm := func() {
		val := ""
		if v := inp.Get("value"); v.Type() == js.TypeString {
			val = strings.TrimSpace(v.String())
		}
		dismiss()
		if val == "" {
			return
		}
		confirm(val)
	}
	open := func() {
		setValue(prefill())
		pop.Call("removeAttribute", "hidden")
		if shadow := inp.Get("shadowRoot"); shadow.Truthy() {
			if els := dom.Query(shadow, "input"); len(els) > 0 {
				els[0].Call("focus")
				els[0].Call("select")
				return
			}
		}
		inp.Call("focus")
	}

	dom.AddEvent(okBtn, "click", func(_ js.Value, _ []js.Value) any {
		doConfirm()
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
			doConfirm()
		case "Escape":
			args[0].Call("preventDefault")
			dismiss()
		}
		return nil
	}, false, false)

	return func() {
		if pop.Call("hasAttribute", "hidden").Bool() {
			open()
		} else {
			dismiss()
		}
	}
}

// statusControl renders a StatusItem: a passive text span refreshed in
// the same pass as toggle/select state. The item's name travels as title
// only — an aria-label would override the visible numbers for assistive
// tech, and aria-live would announce every keystroke.
func (t *toolbar) statusControl(it wtext.StatusItem) {
	el := t.doc().Call("createElement", "span")
	el.Call("setAttribute", "class", "wt-status")
	el.Call("setAttribute", "data-item", it.ID)
	el.Call("setAttribute", "title", t.resolveLabel(it.Label))
	t.container.Call("appendChild", el)
	if it.Args != nil {
		t.statuses = append(t.statuses, trackedStatus{
			el: el, format: t.resolveLabel(it.Format), args: it.Args,
		})
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
			// The "value" ATTRIBUTE (not a bare JS property) is what the
			// combobox's own reactive state actually observes; setting the
			// property here would land on an inert ad hoc field nothing
			// reads. Written unconditionally — including "" — so moving the
			// caret into unformatted text resyncs the display back to the
			// picker's "Default"/"None" option instead of going stale.
			sl.el.Call("setAttribute", "value", sl.current(t.editor))
		}
	}
	for i := range t.statuses {
		st := &t.statuses[i]
		text := fmt.Sprintf(st.format, st.args(t.editor)...)
		if text != st.last {
			st.last = text
			st.el.Set("textContent", text)
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
	"wtext-underline":   "Underline",
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

	"wtext-help":       "Help",
	"wtext-help-title": "Toolbar help",

	"wtext-menu":     "Menu",
	"wtext-export":   "Export",
	"wtext-import":   "Import",
	"wtext-config":   "Settings",
	"wtext-remember": "Don't ask again",
	"wtext-choose":   "Choose",

	"wtext-stylelib-export":       "Styles",
	"wtext-stylelib-export-help":  "Save this document's named styles to a file, to reuse them in other documents.",
	"wtext-stylelib-import":       "Styles",
	"wtext-stylelib-import-help":  "Load styles from a saved file. They join the style picker; nothing is applied to the text.",
	"wtext-stylelib-name":         "File name",
	"wtext-stylelib-conflict":     "Styles with the same name",
	"wtext-stylelib-conflict-msg": "This document already has styles with these names. Replace them with the ones from the file, or keep the ones you have?",
	"wtext-stylelib-overwrite":    "Replace",
	"wtext-stylelib-skip":         "Keep mine",

	"wtext-counter-label": "Counter",
	"wtext-counter":       "Chars: %d · Letters: %d · Words: %d",
	"wtext-counter-help":  "Live count of the document's characters (spaces included, line breaks not), letters and words.",

	"wtext-bold-help":          "Make the selected text bold.",
	"wtext-italic-help":        "Make the selected text italic.",
	"wtext-underline-help":     "Underline the selected text.",
	"wtext-code-help":          "Mark the selected text as code.",
	"wtext-block-help":         "Change the paragraph's block type (heading, quote, code block...).",
	"wtext-font-help":          "Change the font family of the selected text.",
	"wtext-size-help":          "Change the font size of the selected text.",
	"wtext-align-left-help":    "Align the paragraph to the left.",
	"wtext-align-center-help":  "Center the paragraph.",
	"wtext-align-right-help":   "Align the paragraph to the right.",
	"wtext-align-justify-help": "Justify the paragraph (align both edges).",
	"wtext-style-new-help":     "Name and save the selection's formatting as a reusable style.",
	"wtext-style-help":         "Apply a saved style to the selection, or clear it with \"(no style)\".",
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
	case "format_underlined":
		return "U"
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
// map[string]any{"label","value"}; single mode carries 0 or 1). ok is false
// only when nothing was picked — a legitimate pick of the empty-valued
// "None"/"Default" option (v == "", ok == true) must reach the caller, so
// FontToolbar/StyleToolbar's clear options are actually reachable by click.
func firstSelectedValue(args []any) (v string, ok bool) {
	if len(args) == 0 {
		return "", false
	}
	arr, isArr := args[0].([]any)
	if !isArr || len(arr) == 0 {
		return "", false
	}
	m, isMap := arr[0].(map[string]any)
	if !isMap {
		return "", false
	}
	v, _ = m["value"].(string)
	return v, true
}
