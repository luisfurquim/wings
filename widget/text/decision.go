//go:build js && wasm

package text

import (
	"errors"
	"syscall/js"

	"github.com/luisfurquim/wings/dom"
	"github.com/luisfurquim/wings/wtext"
)

// The widget's half of the PendingDecision contract. A plugin action that
// needs the user's answer returns one instead of finishing (it cannot open
// a dialog: it never touches the DOM); the widget asks the question here,
// with the app's own widgets and the app's own catalog, and calls Resume
// with what the user picked.
//
// "Don't ask again" stores the PICKED OPTION, not merely the silence: the
// remembered thing is a policy the widget can apply by itself next time.
// What comes back out of localStorage is INPUT, not state — the store is
// user-writable — so it only counts when it names one of the options the
// plugin declared.

// actionErr is the single exit of every plugin action the widget invokes.
// A PendingDecision is a question, not a failure: it gets asked. Anything
// else is logged.
func (t *toolbar) actionErr(id string, err error) {
	if err == nil {
		return
	}
	var pd *wtext.PendingDecision
	if errors.As(err, &pd) {
		t.decide(pd)
		return
	}
	G.Logf(1, "w-text: %q failed: %v\n", id, err)
}

// decide answers the question from memory when the user already settled
// it, and asks otherwise.
func (t *toolbar) decide(pd *wtext.PendingDecision) {
	if !pd.Valid() {
		G.Logf(1, "w-text: unanswerable decision %q ignored\n", pd.Title)
		return
	}
	if choice, ok := t.remembered(pd); ok {
		t.resume(pd, choice)
		return
	}
	t.openDecision(pd)
}

// resume finishes the action with the user's answer. A decision raised
// from INSIDE Resume is not chained — it is logged and dropped: a plugin
// that could keep asking forever would be a dialog loop with no exit.
func (t *toolbar) resume(pd *wtext.PendingDecision, choice string) {
	if err := pd.Resume(t.editor, choice); err != nil {
		G.Logf(1, "w-text: decision %q (%s) failed: %v\n", pd.Title, choice, err)
	}
	t.afterAction()
}

// openDecision builds the question as a w-dialog: the message, what is at
// stake, one button per option, and the "don't ask again" checkbox when
// the decision is one the plugin lets us remember. Mounted at body like
// the help and settings dialogs (an ancestor's backdrop-filter would trap
// a fixed-position descendant inside the tab panel), so its buttons are
// bound directly in the shadow instead of through wings' triggers.
func (t *toolbar) openDecision(pd *wtext.PendingDecision) {
	if t.decDlg.Truthy() {
		return
	}
	dlg := t.doc().Call("createElement", "w-dialog")
	dlg.Call("setAttribute", "buttons", "cancel")
	dlg.Call("setAttribute", "title", t.resolveLabel(pd.Title))

	if msg := t.resolveLabel(pd.Message); msg != "" {
		p := t.doc().Call("createElement", "p")
		p.Set("textContent", msg)
		dlg.Call("appendChild", p)
	}
	if len(pd.Detail) > 0 {
		// The dialog's body is LIGHT DOM, out of this widget's shadow
		// stylesheet's reach (same as the settings rows), so the little
		// layout it needs is set right here.
		list := t.doc().Call("createElement", "ul")
		lst := list.Get("style")
		lst.Set("margin", "0.4rem 0")
		lst.Set("paddingLeft", "1.2rem")
		lst.Set("maxHeight", "12rem")
		lst.Set("overflowY", "auto")
		for _, d := range pd.Detail {
			li := t.doc().Call("createElement", "li")
			li.Set("textContent", d) // user data (style names): shown as-is
			list.Call("appendChild", li)
		}
		dlg.Call("appendChild", list)
	}

	// The checkbox is only offered when the plugin named a key to remember
	// the answer under.
	var check js.Value
	if pd.Remember != "" {
		row := t.doc().Call("createElement", "label")
		st := row.Get("style")
		st.Set("display", "flex")
		st.Set("alignItems", "center")
		st.Set("gap", "0.4rem")
		st.Set("margin", "0.8rem 0")
		check = t.doc().Call("createElement", "input")
		check.Call("setAttribute", "type", "checkbox")
		row.Call("appendChild", check)
		span := t.doc().Call("createElement", "span")
		span.Set("textContent", t.resolveLabel("wtext-remember"))
		row.Call("appendChild", span)
		dlg.Call("appendChild", row)
	}

	buttons := t.doc().Call("createElement", "div")
	bst := buttons.Get("style")
	bst.Set("display", "flex")
	bst.Set("gap", "0.5rem")
	bst.Set("flexWrap", "wrap")
	for i, opt := range pd.Options {
		btn := t.doc().Call("createElement", "w-button")
		btn.Call("setAttribute", "type", "button")
		btn.Call("setAttribute", "size", "sm")
		if i > 0 {
			btn.Call("setAttribute", "variant", "ghost")
		}
		btn.Set("textContent", t.resolveLabel(opt.Label))
		value := opt.Value
		dom.AddEvent(btn, "click", func(_ js.Value, _ []js.Value) any {
			if check.Truthy() && check.Get("checked").Bool() {
				t.remember(pd.Remember, value)
			}
			t.closeDecision()
			t.resume(pd, value)
			return nil
		}, false, false)
		buttons.Call("appendChild", btn)
	}
	dlg.Call("appendChild", buttons)

	// Cancel: the question goes unanswered and the action simply does not
	// finish — never a default answer picked on the user's behalf.
	if shadow := dlg.Get("shadowRoot"); shadow.Truthy() {
		if els := dom.Query(shadow, "#dlg-cancel"); len(els) > 0 {
			dom.AddEvent(els[0], "click", func(_ js.Value, _ []js.Value) any {
				t.closeDecision()
				return nil
			}, false, false)
		}
	}

	t.decDlg = dlg
	t.doc().Get("body").Call("appendChild", dlg)
}

// closeDecision removes the open decision dialog, if any. Idempotent.
func (t *toolbar) closeDecision() {
	if !t.decDlg.Truthy() {
		return
	}
	t.decDlg.Call("remove")
	t.decDlg = js.Undefined()
}

// remembered reads the answer the user asked us to remember. It is
// hostile input — anything can write localStorage — so it only counts
// when it names one of the options this decision actually declared.
func (t *toolbar) remembered(pd *wtext.PendingDecision) (string, bool) {
	if pd.Remember == "" {
		return "", false
	}
	store := localStore()
	if !store.Truthy() {
		return "", false
	}
	var choice string
	if !guard("localStorage.getItem", func() {
		if v := store.Call("getItem", pd.Remember); v.Type() == js.TypeString {
			choice = v.String()
		}
	}) {
		return "", false
	}
	if choice == "" {
		return "", false
	}
	if !pd.Allows(choice) {
		G.Logf(1, "w-text: stored answer %q for %q is not an option; asking again\n", choice, pd.Remember)
		guard("localStorage.removeItem", func() { store.Call("removeItem", pd.Remember) })
		return "", false
	}
	return choice, true
}

// remember stores the user's answer under the plugin's key.
func (t *toolbar) remember(key, choice string) {
	store := localStore()
	if !store.Truthy() {
		return
	}
	guard("localStorage.setItem", func() { store.Call("setItem", key, choice) })
}

// localStore returns window.localStorage, or undefined where it cannot be
// reached — merely READING the property throws in a browser with storage
// disabled (Safari's private mode, a third-party frame with cookies
// blocked), and an exception through syscall/js is a panic: total loss.
func localStore() (store js.Value) {
	store = js.Undefined()
	guard("localStorage", func() { store = js.Global().Get("localStorage") })
	return store
}

// guard runs fn, converting a JS exception into a logged false — the
// house rule of this codebase: a panic in a wasm frontend takes the whole
// app down, so nothing that crosses the syscall/js boundary may panic.
func guard(what string, fn func()) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			G.Logf(1, "w-text: %s unavailable: %v\n", what, r)
			ok = false
		}
	}()
	fn()
	return true
}
