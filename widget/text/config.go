//go:build js && wasm

package text

import (
	"syscall/js"

	"github.com/luisfurquim/wings/dom"
	"github.com/luisfurquim/wings/widget/dialog"
	"github.com/luisfurquim/wings/wtext"
)

// The settings UI: one menu entry per registered ConfigSection, each
// opening a w-dialog ANCHORED over this w-text instance (covering it
// minus --wings-dialog-anchor-inset, scrim over the widget only) — the
// visual statement of which editor is being configured on a page with
// several. Mounted at document.body like the help dialog (an ancestor's
// backdrop-filter would trap absolutely-positioned descendants too), so
// buttons are bound directly in the dialog's shadow, never via triggers.
//
// ConfigChoice renders as a native <select> for now: a w-combobox
// delivers its selection through the @change trigger, which resolves by
// walking DOM ancestors — unreachable from a body-mounted dialog.

// cfgControl pairs a field's store key with the getter that reads its
// control's current value at save time.
type cfgControl struct {
	key string
	get func() string
}

// renderConfigGroup appends the standard settings group to the menu when
// the profile registers ConfigPlugins: one item per section.
func (t *toolbar) renderConfigGroup(tabs js.Value) {
	var sections []wtext.ConfigSection
	for _, plug := range t.profile.Config {
		sections = append(sections, plug.ConfigSections()...)
	}
	if len(sections) == 0 {
		return
	}
	btn := t.doc().Call("createElement", "w-tabbutton")
	btn.Set("textContent", t.resolveLabel("wtext-config"))
	tabs.Call("appendChild", btn)
	tab := t.doc().Call("createElement", "w-tab")
	for _, s := range sections {
		tab.Call("appendChild", t.configItem(s))
	}
	tabs.Call("appendChild", tab)
}

// configItem renders one section's menu button.
func (t *toolbar) configItem(s wtext.ConfigSection) js.Value {
	btn := t.doc().Call("createElement", "w-button")
	btn.Call("setAttribute", "type", "button")
	btn.Call("setAttribute", "variant", "ghost")
	btn.Call("setAttribute", "size", "sm")
	btn.Call("setAttribute", "data-item", "cfg-"+s.ID)
	label := t.resolveLabel(s.Label)
	btn.Call("setAttribute", "aria-label", label)
	btn.Call("setAttribute", "title", label)
	btn.Set("textContent", label)
	dom.AddEvent(btn, "mousedown", func(_ js.Value, _ []js.Value) any { return nil }, true, false)
	dom.AddEvent(btn, "click", func(_ js.Value, _ []js.Value) any {
		t.openConfig(s)
		return nil
	}, false, false)
	return btn
}

// openConfig builds and anchors the section's settings dialog; a no-op
// while one is already open.
func (t *toolbar) openConfig(s wtext.ConfigSection) {
	if t.cfgDlg.Truthy() {
		return
	}
	dlg := t.doc().Call("createElement", "w-dialog")
	dlg.Call("setAttribute", "buttons", "save,cancel")
	dlg.Call("setAttribute", "title", t.resolveLabel(s.Label))

	var controls []cfgControl
	for _, f := range s.Fields {
		switch fd := f.(type) {
		case wtext.ConfigText:
			key := wtext.ConfigKey(s.ID, fd.ID)
			row, get := t.cfgTextRow(t.resolveLabel(fd.Label), t.editor.Config(key))
			dlg.Call("appendChild", row)
			controls = append(controls, cfgControl{key: key, get: get})
		case wtext.ConfigChoice:
			key := wtext.ConfigKey(s.ID, fd.ID)
			row, get := t.cfgChoiceRow(t.resolveLabel(fd.Label), t.editor.Config(key), fd.Options)
			dlg.Call("appendChild", row)
			controls = append(controls, cfgControl{key: key, get: get})
		}
	}

	if shadow := dlg.Get("shadowRoot"); shadow.Truthy() {
		if els := dom.Query(shadow, "#dlg-save"); len(els) > 0 {
			dom.AddEvent(els[0], "click", func(_ js.Value, _ []js.Value) any {
				for _, c := range controls {
					if err := t.editor.SetConfig(c.key, c.get()); err != nil {
						G.Logf(1, "w-text: config %q rejected: %v\n", c.key, err)
					}
				}
				t.closeConfig()
				t.afterAction() // properties persist through Content(): mark the value dirty
				return nil
			}, false, false)
		}
		if els := dom.Query(shadow, "#dlg-cancel"); len(els) > 0 {
			dom.AddEvent(els[0], "click", func(_ js.Value, _ []js.Value) any {
				t.closeConfig()
				return nil
			}, false, false)
		}
	}

	t.cfgDlg = dlg
	t.doc().Get("body").Call("appendChild", dlg)
	t.cfgRelease = dialog.AnchorTo(dlg, t.host)
}

// closeConfig removes the open settings dialog, if any. Idempotent.
func (t *toolbar) closeConfig() {
	if !t.cfgDlg.Truthy() {
		return
	}
	if t.cfgRelease != nil {
		t.cfgRelease()
		t.cfgRelease = nil
	}
	t.cfgDlg.Call("remove")
	t.cfgDlg = js.Undefined()
}

// cfgTextRow renders a labelled w-input row; the getter reads the inner
// input (the host's value property lags programmatic seeding).
func (t *toolbar) cfgTextRow(label, value string) (js.Value, func() string) {
	row := t.cfgRow(label)
	inp := t.doc().Call("createElement", "w-input")
	inp.Call("setAttribute", "type", "text")
	inp.Call("setAttribute", "aria-label", label)
	// Seeding goes through the value ATTRIBUTE: it lands in the widget's
	// model before its async Render, which would wipe a value written
	// straight into the inner input (the render re-materializes the
	// template). The immediate inner write below just covers the gap
	// until that render runs.
	inp.Call("setAttribute", "value", value)
	row.Call("appendChild", inp)
	if shadow := inp.Get("shadowRoot"); shadow.Truthy() {
		if els := dom.Query(shadow, "input"); len(els) > 0 {
			els[0].Set("value", value)
		}
	}
	get := func() string {
		if shadow := inp.Get("shadowRoot"); shadow.Truthy() {
			if els := dom.Query(shadow, "input"); len(els) > 0 {
				return els[0].Get("value").String()
			}
		}
		if v := inp.Get("value"); v.Type() == js.TypeString {
			return v.String()
		}
		return ""
	}
	return row, get
}

// cfgChoiceRow renders a labelled native <select> row.
func (t *toolbar) cfgChoiceRow(label, value string, opts []wtext.Option) (js.Value, func() string) {
	row := t.cfgRow(label)
	sel := t.doc().Call("createElement", "select")
	sel.Call("setAttribute", "class", "wt-cfg-select")
	sel.Call("setAttribute", "aria-label", label)
	for _, o := range opts {
		opt := t.doc().Call("createElement", "option")
		opt.Set("value", o.Value)
		opt.Set("textContent", t.resolveLabel(o.Label))
		if o.Value == value {
			opt.Set("selected", true)
		}
		sel.Call("appendChild", opt)
	}
	row.Call("appendChild", sel)
	return row, func() string { return sel.Get("value").String() }
}

// cfgRow builds a row with a block label. The rows live in the dialog's
// LIGHT DOM, out of this widget's shadow stylesheet's reach, so the
// little layout they need is set right here.
func (t *toolbar) cfgRow(label string) js.Value {
	row := t.doc().Call("createElement", "p")
	row.Get("style").Set("margin", "0 0 0.8rem")
	lbl := t.doc().Call("createElement", "label")
	lbl.Set("textContent", label)
	st := lbl.Get("style")
	st.Set("display", "block")
	st.Set("fontWeight", "600")
	st.Set("marginBottom", "0.25rem")
	row.Call("appendChild", lbl)
	return row
}
