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
			case wtext.MenuUpload:
				add(it.Group, it)
			}
		}
	}

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
			case wtext.MenuUpload:
				tab.Call("appendChild", t.menuUpload(it))
			}
		}
		tabs.Call("appendChild", tab)
	}
	// The standard settings group (ConfigPlugins) closes the menu.
	t.renderConfigGroup(tabs)
	if !tabs.Get("firstChild").Truthy() {
		return // no groups at all: the column stays empty and collapses
	}

	// Hamburger: collapses the column down to itself, freeing the width
	// for the editor. The state attribute lives on the nav — outside the
	// innerHTML wipe above — so it survives a retranslation re-render.
	t.menu.Call("appendChild", t.menuToggle())

	body := t.doc().Call("createElement", "div")
	body.Call("setAttribute", "class", "wt-menu-body")
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
		t.actionErr(it.ID, it.Do(t.editor))
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
			t.actionErr(it.ID, it.Do(t.editor, val))
			t.refresh()
		})

	dom.AddEvent(btn, "mousedown", func(_ js.Value, _ []js.Value) any { return nil }, true, false)
	dom.AddEvent(btn, "click", func(_ js.Value, _ []js.Value) any {
		toggle()
		return nil
	}, false, false)
	return wrap
}

// menuUpload renders a MenuUpload: the item's button plus the hidden file
// input it clicks. The item never sees the input — the widget owns the
// picker, reads the file and hands the bytes to Do, which treats them as
// the hostile input they are.
func (t *toolbar) menuUpload(it wtext.MenuUpload) js.Value {
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

	input := t.doc().Call("createElement", "input")
	input.Call("setAttribute", "type", "file")
	input.Call("setAttribute", "hidden", "")
	if it.Accept != "" {
		input.Call("setAttribute", "accept", it.Accept)
	}
	wrap.Call("appendChild", input)

	limit := it.MaxLen
	if limit <= 0 {
		limit = wtext.DefaultUploadLen
	}

	dom.AddEvent(input, "change", func(_ js.Value, _ []js.Value) any {
		files := input.Get("files")
		if !files.Truthy() || files.Get("length").Int() == 0 {
			return nil
		}
		file := files.Index(0)
		// Clear the input right away: picking the SAME file again must
		// fire change again, and the input only fires when its value
		// differs from the last one.
		input.Set("value", "")
		if size := file.Get("size").Int(); size > limit {
			G.Logf(1, "w-text: %q: file of %d bytes over the %d limit\n", it.ID, size, limit)
			return nil
		}
		t.readFile(file, func(data []byte) {
			if it.Do == nil {
				return
			}
			t.actionErr(it.ID, it.Do(t.editor, data))
			t.refresh()
		})
		return nil
	}, false, false)

	dom.AddEvent(btn, "mousedown", func(_ js.Value, _ []js.Value) any { return nil }, true, false)
	dom.AddEvent(btn, "click", func(_ js.Value, _ []js.Value) any {
		input.Call("click")
		return nil
	}, false, false)
	return wrap
}

// readFile reads a File's bytes through its arrayBuffer() promise and
// hands them to done. The promise settles on the JS thread, so the
// callbacks only hand the result to a waiting goroutine — blocking the
// event handler itself would deadlock the single JS thread — and the
// js.Funcs are released there, once, after either outcome.
func (t *toolbar) readFile(file js.Value, done func([]byte)) {
	type result struct {
		data []byte
		err  bool
	}
	ch := make(chan result, 1)
	var thenFn, catchFn js.Func

	thenFn = js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			ch <- result{err: true}
			return nil
		}
		buf := args[0]
		data := make([]byte, buf.Get("byteLength").Int())
		js.CopyBytesToGo(data, js.Global().Get("Uint8Array").New(buf))
		ch <- result{data: data}
		return nil
	})
	catchFn = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		ch <- result{err: true}
		return nil
	})

	go func() {
		res := <-ch
		thenFn.Release()
		catchFn.Release()
		if res.err {
			G.Logf(1, "w-text: could not read the picked file\n")
			return
		}
		done(res.data)
	}()

	file.Call("arrayBuffer").Call("then", thenFn).Call("catch", catchFn)
}
