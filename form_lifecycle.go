//go:build js && wasm

package wings

import "syscall/js"

// Form-associated lifecycle hooks. A form-associated custom element (registered
// with ComponentOpts.FormAssociated) receives browser callbacks when its owning
// <form> is reset or when an ancestor <fieldset disabled> toggles. PranaMod has
// no per-instance callback surface for these, so widgets register a per-tag hook
// here — mirroring OnRetranslate — and the JS lifecycle callbacks (prana_helper.js)
// dispatch to it through elementFormReset / elementFormDisabled.

var (
	formResetHooks    = map[string]func(js.Value){}
	formDisabledHooks = map[string]func(js.Value, bool){}
)

// OnFormReset registers fn to run when a form-associated element of tag receives
// formResetCallback — i.e. its owning <form> was reset. w-input uses it to
// restore its default value and clear validation state. A later call for the
// same tag replaces the previous hook.
func OnFormReset(tag string, fn func(js.Value)) {
	formResetHooks[tag] = fn
}

// OnFormDisabled registers fn to run when a form-associated element's
// formDisabledCallback fires — e.g. an ancestor <fieldset disabled> was toggled.
// The browser does not set the element's `disabled` attribute in this case, so
// the widget must reflect the state to its inner control itself. A later call
// for the same tag replaces the previous hook.
func OnFormDisabled(tag string, fn func(js.Value, bool)) {
	formDisabledHooks[tag] = fn
}

// elementFormReset dispatches a browser formResetCallback to the tag's hook.
func elementFormReset(self js.Value) {
	if fn := formResetHooks[self.Get("_pranaTag").String()]; fn != nil {
		fn(self)
	}
}

// elementFormDisabled dispatches a browser formDisabledCallback to the tag's hook.
func elementFormDisabled(self js.Value, disabled bool) {
	if fn := formDisabledHooks[self.Get("_pranaTag").String()]; fn != nil {
		fn(self, disabled)
	}
}
