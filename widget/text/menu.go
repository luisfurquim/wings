//go:build js && wasm

package text

import (
	"syscall/js"

	"github.com/luisfurquim/wings/dom"
	"github.com/luisfurquim/wings/wtext"
)

// renderMenu draws the side menu: one accordion section (w-tabs
// mode=accordion — native <details> under the hood) per distinct group
// declared by the profile's MenuPlugins, in first-seen order. It runs in
// render()'s pass, so an OnRetranslate rebuild refreshes menu labels too.
// No items → the container stays empty and design.css collapses the
// column; no menu container in the template → no-op.
//
// The w-tabs family is registered by the app, not by this widget (a
// profile without menu plugins shouldn't pay for it): a profile using
// MenuPlugins needs
// import _ ".../widget/tabs", ".../widget/tabbutton" and ".../widget/tab".
func (t *toolbar) renderMenu() {
	if !t.menu.Truthy() {
		return
	}
	t.menu.Set("innerHTML", "") // static container; safe empty string
	var groups []string
	items := map[string][]wtext.MenuItem{}
	add := func(group string, item wtext.MenuItem) {
		if _, ok := items[group]; !ok {
			groups = append(groups, group)
		}
		items[group] = append(items[group], item)
	}
	for _, plug := range t.profile.Menu {
		for _, item := range plug.MenuItems() {
			switch it := item.(type) {
			case wtext.MenuAction:
				add(it.Group, it)
			case wtext.MenuInput:
				add(it.Group, it)
			}
		}
	}
	if len(groups) == 0 {
		return
	}

	// Hamburger: collapses the column down to itself, freeing the width
	// for the editor. The state attribute lives on the nav — outside the
	// innerHTML wipe above — so it survives a retranslation re-render.
	t.menu.Call("appendChild", t.menuToggle())

	body := t.doc().Call("createElement", "div")
	body.Call("setAttribute", "class", "wt-menu-body")
	tabs := t.doc().Call("createElement", "w-tabs")
	tabs.Call("setAttribute", "mode", "accordion")
	for _, g := range groups {
		btn := t.doc().Call("createElement", "w-tabbutton")
		btn.Set("textContent", t.resolveLabel(g))
		tabs.Call("appendChild", btn)
		tab := t.doc().Call("createElement", "w-tab")
		for _, item := range items[g] {
			switch it := item.(type) {
			case wtext.MenuAction:
				tab.Call("appendChild", t.menuAction(it))
			case wtext.MenuInput:
				tab.Call("appendChild", t.menuInput(it))
			}
		}
		tabs.Call("appendChild", tab)
	}
	body.Call("appendChild", tabs)
	t.menu.Call("appendChild", body)
}

// menuToggle renders the hamburger button reflecting the nav's current
// collapsed state.
func (t *toolbar) menuToggle() js.Value {
	btn := t.doc().Call("createElement", "w-button")
	btn.Call("setAttribute", "type", "button")
	btn.Call("setAttribute", "variant", "ghost")
	btn.Call("setAttribute", "size", "sm")
	btn.Call("setAttribute", "data-item", "menu-toggle")
	label := t.resolveLabel("wtext-menu")
	btn.Call("setAttribute", "aria-label", label)
	btn.Call("setAttribute", "title", label)
	btn.Set("textContent", "☰")
	expanded := "true"
	if t.menu.Call("hasAttribute", "data-collapsed").Bool() {
		expanded = "false"
	}
	btn.Call("setAttribute", "aria-expanded", expanded)
	dom.AddEvent(btn, "mousedown", func(_ js.Value, _ []js.Value) any { return nil }, true, false)
	dom.AddEvent(btn, "click", func(_ js.Value, _ []js.Value) any {
		if t.menu.Call("toggleAttribute", "data-collapsed").Bool() {
			btn.Call("setAttribute", "aria-expanded", "false")
		} else {
			btn.Call("setAttribute", "aria-expanded", "true")
		}
		return nil
	}, false, false)
	return btn
}

// menuAction renders one action item, under the toolbar buttons' focus
// contract: mousedown never steals the editor's selection.
func (t *toolbar) menuAction(it wtext.MenuAction) js.Value {
	btn := t.doc().Call("createElement", "w-button")
	btn.Call("setAttribute", "type", "button")
	btn.Call("setAttribute", "variant", "ghost")
	btn.Call("setAttribute", "size", "sm")
	btn.Call("setAttribute", "data-item", it.ID)
	label := t.resolveLabel(it.Label)
	btn.Call("setAttribute", "aria-label", label)
	btn.Call("setAttribute", "title", label)
	btn.Set("textContent", label)
	if it.Enabled != nil && !it.Enabled(t.editor) {
		btn.Call("setAttribute", "disabled", "")
	}
	dom.AddEvent(btn, "mousedown", func(_ js.Value, _ []js.Value) any { return nil }, true, false)
	dom.AddEvent(btn, "click", func(_ js.Value, _ []js.Value) any {
		if it.Do == nil {
			return nil
		}
		if err := it.Do(t.editor); err != nil {
			G.Logf(1, "w-text: menu action %q failed: %v\n", it.ID, err)
		}
		t.refresh()
		return nil
	}, false, false)
	return btn
}

// menuInput renders a MenuInput: the item button plus the shared prompt
// popover (inputPopover), seeded by the item's Value on every open.
func (t *toolbar) menuInput(it wtext.MenuInput) js.Value {
	wrap := t.doc().Call("createElement", "span")
	wrap.Call("setAttribute", "class", "wt-inputitem wt-menu-inputitem")

	btn := t.doc().Call("createElement", "w-button")
	btn.Call("setAttribute", "type", "button")
	btn.Call("setAttribute", "variant", "ghost")
	btn.Call("setAttribute", "size", "sm")
	btn.Call("setAttribute", "data-item", it.ID)
	label := t.resolveLabel(it.Label)
	btn.Call("setAttribute", "aria-label", label)
	btn.Call("setAttribute", "title", label)
	btn.Set("textContent", label)
	wrap.Call("appendChild", btn)

	prefill := func() string { return "" }
	if it.Value != nil {
		prefill = func() string { return it.Value(t.editor) }
	}
	toggle := t.inputPopover(wrap, t.resolveLabel(it.Placeholder), prefill,
		func(val string) {
			if it.Do == nil {
				return
			}
			if err := it.Do(t.editor, val); err != nil {
				G.Logf(1, "w-text: menu input %q failed: %v\n", it.ID, err)
			}
			t.refresh()
		})

	dom.AddEvent(btn, "mousedown", func(_ js.Value, _ []js.Value) any { return nil }, true, false)
	dom.AddEvent(btn, "click", func(_ js.Value, _ []js.Value) any {
		toggle()
		return nil
	}, false, false)
	return wrap
}
