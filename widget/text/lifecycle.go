//go:build js && wasm

package text

import (
	"syscall/js"

	"github.com/luisfurquim/wings/dom"
)

// Form lifecycle and teardown for w-text, mirroring w-input.

// formReset restores the mount-time content and clears undo history on
// form.reset(). Restoring content wholesale (SetContent) is right here:
// unlike a text field, an editor's default is a document, and its undo
// stack must not survive a reset.
func formReset(host js.Value) {
	b, ok := bindingFor(host)
	if !ok {
		return
	}
	if err := b.editor.SetContent(b.initial); err != nil {
		G.Logf(1, "w-text: reset content rejected: %v\n", err)
	}
	content := b.editor.Content()
	host.Set("value", content)
	setFormValue(host, content)
	setEmptyState(host, b.editor)
	host.Call("dispatchEvent", js.Global().Get("Event").New("change"))
}

// formDisabled reflects an ancestor <fieldset disabled> to the editing
// surface without clobbering the author's own disabled attribute.
func formDisabled(host js.Value, disabled bool) {
	edit := editorEl(host)
	if !edit.Truthy() {
		return
	}
	on := disabled || host.Call("hasAttribute", "disabled").Bool()
	applyDisabled(host, edit, on)
}

// applyDisabled toggles the surface's editability and a CSS host hook.
func applyDisabled(host, edit js.Value, on bool) {
	if on {
		edit.Call("setAttribute", "contenteditable", "false")
		host.Call("setAttribute", "data-form-disabled", "")
	} else {
		edit.Call("setAttribute", "contenteditable", "true")
		host.Call("removeAttribute", "data-form-disabled")
	}
}

// retranslate re-renders the toolbar after a SetLang switch so its buttons
// re-resolve their labels against the now-translated slotted <span
// slot="labels"> nodes. RetranslateAll runs this only after every instance
// re-rendered, so those spans already carry the new language.
func retranslate(host js.Value) {
	if b, ok := bindingFor(host); ok && b.tb != nil {
		b.tb.render()
	}
}

// disconnect tears down the editor when the instance leaves the DOM: its
// document-level selectionchange listener and hand-built MutationObserver
// are not reachable by the runtime's subtree auto-release. The toolbar's
// help dialog needs the same explicit teardown for a different reason: it
// is deliberately mounted at document.body (see toolbar.openHelp), so it
// is not a descendant of host and the runtime's subtree auto-release
// would never find it either.
func disconnect(host js.Value) {
	b, ok := bindingFor(host)
	if !ok {
		return
	}
	if b.tb != nil {
		b.tb.closeHelp()
		b.tb.closeConfig()
	}
	b.editor.Detach()
	if id, ok := nodeKey(host); ok {
		delete(editors, id)
	}
}

// editorEl returns the shadow .wt-editor for a host.
func editorEl(host js.Value) js.Value {
	shadow := host.Get("shadowRoot")
	if !shadow.Truthy() {
		return js.Undefined()
	}
	els := dom.Query(shadow, ".wt-editor")
	if len(els) == 0 {
		return js.Undefined()
	}
	return els[0]
}
