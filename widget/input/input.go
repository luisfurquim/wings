//go:build js && wasm

// Package input provides a w-input custom element for wings.
//
// Features:
//   - Full label/field/feedback anatomy with independent ::part() surfaces
//   - Surface form controlled by the active Material skin (outlined by default)
//   - Three sizes: sm, md, lg
//   - Named slots: label (rich label), prefix (left icon/text), suffix (right icon/text),
//     helper (rich helper text), error (rich error content)
//   - Clearable mode: shows a × button when the field has a value
//   - Required indicator (*) via required="true"
//   - Character count via maxlength attribute
//   - Host state attributes for CSS hooks: [data-focused], [data-has-value],
//     [data-empty], [data-invalid], [disabled], [size]
//   - Fires @input on every keystroke; @change when the field is left (native
//     semantics); @clear when the × button is clicked
//   - Form-associated: participates in a wrapping native <form> — validity
//     (required/type/maxlength + bound Validator) gates form.checkValidity()
//     and submission, and the value is submitted under the host's name
//     attribute
//
// # Usage in parent template
//
//	<w-input label="Email" type="email" placeholder="you@example.com"
//	         helper="We'll never share your email."
//	         required="true" clearable="true"
//	         @change="on_email_change">
//	</w-input>
//
//	<!-- icon prefix via slot -->
//	<w-input label="Search">
//	  <svg slot="prefix">…</svg>
//	</w-input>
//
//	<!-- mark invalid from parent -->
//	<w-input error="Invalid email format" *invalid="{{form_invalid}}">
//	</w-input>
//
// # Attributes (all observed; re-syncs template on change)
//
//   - type        — text (default) | email | password | search | number | tel | url
//   - label       — label text (also activates the label zone); use slot="label" for HTML
//   - placeholder — placeholder text
//   - value       — initial value; kept in sync via two-way binding
//   - helper      — helper text below field; use slot="helper" for HTML
//   - error        — error message below field; use slot="error" for HTML
//   - maxlength   — max characters; enables character count display
//   - size        — sm | md (default) | lg
//   - required    — "true" to show the required mark (*)
//   - clearable   — "true" to show the × button when field has a value
//   - disabled    — standard HTML (reflected to inner <input>)
//
// # Events fired to parent
//
//	@input   — fires on every keystroke; args[0] = current string value
//	@change  — fires when the user leaves the field (blur), after the two-way
//	           write-back, so a bound FieldCodec/Validator has already run;
//	           args[0] = current string value
//	@clear   — fires when the × clear button is clicked
//
// # Native form participation
//
// w-input is a form-associated custom element. Inside a <form>, an invalid
// field (native constraints or a bound wings.Validator) blocks
// form.checkValidity()/reportValidity() and submission; give the host a
// name attribute and the value is included in the submitted form data.
// form.reset() restores the field to its default value (the initial `value`
// attribute) and clears validation, and an ancestor <fieldset disabled>
// disables the field (via formReset/formDisabled lifecycle callbacks).
//
// # CSS Customisation
//
// Input implements wings.Customizable. CSS is split into two parts:
//   - "Vars"   — CSS custom properties (empty by default).
//   - "Design" — Layout and structure rules.
//
// Key tokens consumed: --wings-input-bg, --wings-input-color,
// --wings-input-placeholder-color,
// --wings-input-border, --wings-input-border-focus, --wings-input-border-error,
// --wings-input-label-color, --wings-input-label-color-focus,
// --wings-input-label-color-error, --wings-input-helper-color,
// --wings-input-error-color, --wings-input-count-color,
// --wings-input-clear-color, --wings-input-prefix-color,
// --wings-input-disabled-opacity, --wings-remover-hover-bg,
// --wings-remover-hover-color, --wings-radius-md, --wings-border,
// --wings-border-focus, --wings-surface, --wings-text, --wings-text-muted,
// --wings-text-light, --wings-focus-ring, --wings-transition-fast.
//
// Material tokens (set by the active Material skin):
// --wings-input-material-border-top/right/bottom/left,
// --wings-input-material-radius, --wings-input-material-bg,
// --wings-input-material-padding-x,
// --wings-input-material-focus-shadow, --wings-input-material-focus-shadow-error.
//
// Key parts exposed: ::part(root), ::part(label-wrap), ::part(label),
// ::part(required-mark), ::part(field), ::part(prefix), ::part(input),
// ::part(suffix), ::part(clear-btn), ::part(feedback), ::part(helper),
// ::part(error), ::part(count).
//
// Host hooks (set by this widget for external CSS):
//
//	[data-focused]   — present while the inner <input> has focus
//	[data-has-value] — present while value is non-empty
//	[data-empty]     — present while value is empty
//	[data-invalid]   — present when the error attribute is non-empty
//	[size]           — "sm" | "md" | "lg" (default "md")
//
// Floating label example (external CSS only, no Go changes needed):
//
//	w-input[data-focused]::part(label),
//	w-input[data-has-value]::part(label) {
//	  transform: translateY(-1.5em) scale(0.8);
//	  color: var(--wings-primary);
//	}
package input

import (
	_ "embed"
	"strings"
	"syscall/js"
	"unicode/utf8"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
)

const elementTag = "w-input"

// G is the logger for this module.
var G goose.Alert

//go:embed input.html
var htmlContent string

//go:embed vars.css
var varsCSS string

//go:embed design.css
var designCSS string

// cssParts holds the CSS sections; shared by all instances.
var cssParts = []wings.CSSPart{
	{Name: "Vars", Content: ""},
	{Name: "Design", Content: ""},
}

// buildCSS concatenates all CSS parts in the defined order.
func buildCSS() string {
	var sb strings.Builder
	for _, p := range cssParts {
		sb.WriteString(p.Content)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// New creates a new Input instance.
func New() *Input {
	return &Input{}
}

func init() {
	G.Set(3)
	cssParts[0].Content = varsCSS
	cssParts[1].Content = designCSS
	wings.RegisterWithOpts(
		elementTag,
		htmlContent,
		buildCSS(),
		// Form-associated: the host participates in a wrapping native <form>.
		// syncValidity mirrors the inner <input>'s ValidityState into
		// ElementInternals, so form.checkValidity()/reportValidity() see the
		// field, and setFormValue puts the value in the submitted data.
		wings.ComponentOpts{FormAssociated: true},
		func() wings.PranaMod { return &Input{} },
		"type", "label", "placeholder", "value",
		"helper", "error", "maxlength",
		"size", "required", "clearable",
	)
	// On a SetLang language switch the slotted error messages are re-translated;
	// refresh the native validity message for any currently-invalid instance by
	// re-resolving its id and re-setting the error attribute (the host
	// MutationObserver mirrors it to setCustomValidity).
	wings.OnRetranslate(elementTag, func(el js.Value) {
		if !el.Call("hasAttribute", "data-invalid-ref").Bool() {
			return
		}
		id := el.Call("getAttribute", "data-invalid-ref").String()
		if id == "" {
			return
		}
		el.Call("setAttribute", "error", resolveErrorMessage(el, id))
	})
	// Native <form> reset: restore the default value (the initial `value`
	// attribute, like a native input's defaultValue) and re-run the widget's
	// own input pipeline + a bound &value validation.
	wings.OnFormReset(elementTag, formReset)
	// Ancestor <fieldset disabled> toggled: the browser does not touch our
	// `disabled` attribute, so reflect the state to the inner input ourselves.
	wings.OnFormDisabled(elementTag, formDisabled)
	G.Logf(3, "w-input: module registered\n")
}

// innerInput returns the shadow <input> of a host w-input, or an invalid value.
func innerInput(host js.Value) js.Value {
	shadow := host.Get("shadowRoot")
	if !shadow.Truthy() {
		return js.Undefined()
	}
	inps := dom.Query(shadow, ".inp-input")
	if len(inps) == 0 {
		return js.Undefined()
	}
	return inps[0]
}

// formReset restores the field to its default value on form.reset(). It sets the
// inner input to the initial `value` attribute and replays the widget's own
// input + change events, so the reactive value, host state, character count and
// a bound FieldCodec/Validator all reconcile through the existing handlers.
func formReset(host js.Value) {
	inp := innerInput(host)
	if !inp.Truthy() {
		return
	}
	// The reset default is the mount-time snapshot (data-default-value), not the
	// `value` attribute — the latter is reflected to the live value by the
	// two-way binding, so it would restore the field to its current text.
	def := ""
	if host.Call("hasAttribute", "data-default-value").Bool() {
		def = host.Call("getAttribute", "data-default-value").String()
	}
	inp.Set("value", def)
	// input event → the Render-time listener updates reactive value/state/count.
	inp.Call("dispatchEvent",
		js.Global().Get("Event").New("input", map[string]any{"bubbles": true}))
	// change on the host → a bound &value re-reads and re-validates the default.
	host.Set("value", def)
	host.Call("dispatchEvent", js.Global().Get("Event").New("change"))
}

// formDisabled reflects an ancestor fieldset's disabled state to the inner
// input and to a data-form-disabled host hook (for CSS), without clobbering the
// author's own `disabled` attribute. When the fieldset re-enables, the inner
// input's disabled state falls back to that author attribute.
func formDisabled(host js.Value, disabled bool) {
	inp := innerInput(host)
	if !inp.Truthy() {
		return
	}
	if disabled || host.Call("hasAttribute", "disabled").Bool() {
		inp.Call("setAttribute", "disabled", "")
		host.Call("setAttribute", "data-form-disabled", "")
	} else {
		inp.Call("removeAttribute", "disabled")
		host.Call("removeAttribute", "data-form-disabled")
	}
}

// Input implements wings.PranaMod and wings.Customizable
// for the w-input custom element.
type Input struct{}

// Compile-time interface check.
var _ wings.Customizable = (*Input)(nil)

// ListCSS returns the named CSS parts in order.
func (in *Input) ListCSS() []wings.CSSPart {
	result := make([]wings.CSSPart, len(cssParts))
	copy(result, cssParts)
	return result
}

// ReplaceCSS replaces the CSS part identified by key and updates
// all live instances via wings.Update.
func (in *Input) ReplaceCSS(key string, content string) {
	for i := range cssParts {
		if cssParts[i].Name == key {
			cssParts[i].Content = content
			wings.Update(elementTag, buildCSS())
			return
		}
	}
	G.Logf(1, "ReplaceCSS: key %q not found\n", key)
}

func (in *Input) InitData() map[string]any {
	return map[string]any{
		// Attribute-backed keys
		"type":        "text",
		"label":       "",
		"placeholder": "",
		"value":       "",
		"helper":      "",
		"error":       "",
		"maxlength":   "",
		"size":        "md",
		// Boolean attrs — use "true" value from HTML: required="true" clearable="true"
		"required":  false,
		"clearable": false,
		// Derived state (updated in Render event handlers)
		"clearable_show": false,
		"value_len":      0,
	}
}

func (in *Input) Render(obj *wings.PranaObj) {
	inps := dom.Query(obj.Dom, ".inp-input")
	if len(inps) == 0 {
		return
	}
	inp := inps[0]

	// Reflect initial boolean attributes to the inner <input>.
	reflectInputAttrs(obj.Element, inp)

	// Watch host attribute changes: disabled/readonly/required reflect to the
	// inner input; data-invalid-ref (set by the two-way binding when the bound
	// value validates) and error drive native validity and the visible message.
	// dom.Observe registers the observer for auto-release on disconnect.
	dom.Observe(obj.Element, map[string]any{
		"attributes":      true,
		"attributeFilter": []any{"disabled", "readonly", "required", "aria-label", "data-invalid-ref", "error"},
	}, func(_ js.Value, _ []js.Value) any {
		reflectInputAttrs(obj.Element, inp)
		syncValidity(obj.Element, inp)
		return nil
	})

	// Set initial host state attributes.
	initVal, _ := obj.This.Get("value").(string)
	obj.Element.Set("value", initVal)
	setValueHostAttrs(obj.Element, initVal)
	// Snapshot the mount-time value as the reset default. The `value` attribute
	// can't serve this: the two-way &value binding reflects the live value back
	// onto it, so by reset time it holds the current text, not the default.
	if !obj.Element.Call("hasAttribute", "data-default-value").Bool() {
		obj.Element.Call("setAttribute", "data-default-value", initVal)
	}

	// Reconcile initial validity/error state from current host attributes.
	syncValidity(obj.Element, inp)

	// Recompute derived state from the initial value.
	updateDerived(obj, initVal)

	// ── Event listeners (freed by dom.RmEventsUnder on disconnect) ──────────

	dom.AddEvent(inp, "focus", func(_ js.Value, _ []js.Value) any {
		obj.Element.Call("setAttribute", "data-focused", "")
		return nil
	}, false, false)

	dom.AddEvent(inp, "blur", func(_ js.Value, _ []js.Value) any {
		obj.Element.Call("removeAttribute", "data-focused")
		// Expose the current value as a host property and fire a native change
		// so the parent's two-way `&value` binding (enabled for custom elements)
		// writes back to the parent data map on blur — the moment a bound
		// FieldCodec/Validator runs.
		obj.Element.Set("value", inp.Get("value"))
		obj.Element.Call("dispatchEvent", js.Global().Get("Event").New("change"))
		// Native semantics: @change fires when the field is left, not per
		// keystroke (that is @input). Fired after the two-way write-back, so a
		// parent handler already sees the parsed/validated value in its map.
		obj.Trigger("change", inp.Get("value").String())
		return nil
	}, false, false)

	dom.AddEvent(inp, "input", func(_ js.Value, args []js.Value) any {
		val := inp.Get("value").String()
		// &value uses the 'change' event (blur), not 'input'. Writing directly
		// to the map (without triggering a sync) ensures that when updateDerived
		// calls obj.This.Set(...) and sync runs, it sees the current value and
		// does not overwrite the input with the stale reactive value.
		obj.This.M["value"] = val
		// Keep the host's `value` property current so the parent two-way binding
		// reads the live value when change fires on blur.
		obj.Element.Set("value", val)
		setValueHostAttrs(obj.Element, val)
		updateDerived(obj, val)
		// Native constraints (required/type/maxlength) change as the user
		// types; refresh the form-facing validity mirror.
		syncValidity(obj.Element, inp)
		obj.Trigger("input", val)
		return nil
	}, false, false)

	// Clear button.
	clears := dom.Query(obj.Dom, ".inp-clear")
	if len(clears) > 0 {
		dom.AddEvent(clears[0], "click", func(_ js.Value, _ []js.Value) any {
			obj.This.Set("value", "")
			obj.This.Set("clearable_show", false)
			obj.This.Set("value_len", 0)
			setValueHostAttrs(obj.Element, "")
			syncValidity(obj.Element, inp)
			obj.Trigger("clear")
			inp.Call("focus")
			return nil
		}, false, false)
	}

	// Slot visibility for prefix/suffix.
	setupSlotWrapper(obj.Dom, "slot[name='prefix']", ".inp-prefix")
	setupSlotWrapper(obj.Dom, "slot[name='suffix']", ".inp-suffix")
}

// syncValidity reconciles the host's validation state with the inner input.
//
// When data-invalid-ref is present (set by the two-way binding for a
// Validator-bound value), it resolves the message id to translated text and
// writes it to the error attribute — an empty id means valid, so the error is
// cleared. It then mirrors the error attribute to the data-invalid host hook and
// to the inner input's native setCustomValidity (empty message = valid).
//
// A manual error attribute (set by the webdev, no data-invalid-ref) is honoured
// too: it gets the same native validity and data-invalid treatment.
func syncValidity(host, inp js.Value) {
	if host.Call("hasAttribute", "data-invalid-ref").Bool() {
		id := host.Call("getAttribute", "data-invalid-ref").String()
		text := ""
		if id != "" {
			text = resolveErrorMessage(host, id)
		}
		switch {
		case text == "" && host.Call("hasAttribute", "error").Bool():
			host.Call("removeAttribute", "error") // re-enters the observer; resolves below
		case text != "" && attrOrEmpty(host, "error") != text:
			host.Call("setAttribute", "error", text)
		}
	}

	errMsg := attrOrEmpty(host, "error")
	invalid := host.Call("hasAttribute", "data-invalid").Bool()
	if errMsg != "" && !invalid {
		host.Call("setAttribute", "data-invalid", "")
	} else if errMsg == "" && invalid {
		host.Call("removeAttribute", "data-invalid")
	}
	inp.Call("setCustomValidity", errMsg)
	// Mirror validity to ARIA so assistive tech announces the state; the
	// visible message is wired via aria-describedby + role="alert" in the
	// template.
	if errMsg != "" {
		inp.Call("setAttribute", "aria-invalid", "true")
	} else {
		inp.Call("removeAttribute", "aria-invalid")
	}
	mirrorValidity(host, inp)
}

// validityFlags are the ValidityState members mirrored from the inner <input>
// into the host's ElementInternals.
var validityFlags = []string{
	"valueMissing", "typeMismatch", "patternMismatch", "tooLong", "tooShort",
	"rangeUnderflow", "rangeOverflow", "stepMismatch", "badInput", "customError",
}

// mirrorValidity relays the inner <input>'s ValidityState — native constraints
// (required/type/maxlength, already reflected onto it) plus the customError
// set from a bound Validator — into the host's ElementInternals, so a wrapping
// native <form> sees the field: checkValidity() gates submission and
// reportValidity() anchors the browser bubble on the inner input, with the
// browser's own localized message for native constraints. No-op when the page
// runs an old prana_helper.js without form-associated support.
func mirrorValidity(host, inp js.Value) {
	internals := host.Get("_internals")
	if !internals.Truthy() {
		return
	}
	validity := inp.Get("validity")
	if validity.Get("valid").Bool() {
		internals.Call("setValidity", map[string]any{})
		return
	}
	flags := map[string]any{}
	for _, f := range validityFlags {
		if validity.Get(f).Bool() {
			flags[f] = true
		}
	}
	internals.Call("setValidity", flags, inp.Get("validationMessage"), inp)
}

// resolveErrorMessage maps a validation message id to its translated text.
// Lookup order: a slotted `<span slot="errors" id="...">` in the host's light
// DOM (translated in place by gen_i18n), then a document-level element with
// that id (a shared message table), then the id itself as a last resort.
//
// The id comes from a Validator implementation — app data, not wings data — so
// it is CSS-escaped before entering the selector: an unescaped quote or
// bracket would make querySelector throw, and a thrown JS exception panics the
// whole WASM app.
func resolveErrorMessage(host js.Value, id string) string {
	esc := js.Global().Get("CSS").Call("escape", id).String()
	sel := "[slot=\"errors\"][id=\"" + esc + "\"]"
	if el := host.Call("querySelector", sel); el.Truthy() {
		return strings.TrimSpace(el.Get("textContent").String())
	}
	if el := js.Global().Get("document").Call("getElementById", id); el.Truthy() {
		return strings.TrimSpace(el.Get("textContent").String())
	}
	return id
}

// attrOrEmpty returns the value of attribute name, or "" if absent.
func attrOrEmpty(el js.Value, name string) string {
	if el.Call("hasAttribute", name).Bool() {
		return el.Call("getAttribute", name).String()
	}
	return ""
}

// reflectInputAttrs copies disabled, readonly, required (presence) and
// aria-label (value — a label-less host, e.g. a grid cell, names the real
// control this way) from the host to the inner <input>.
func reflectInputAttrs(host, inp js.Value) {
	for _, attr := range []string{"disabled", "readonly", "required"} {
		if host.Call("hasAttribute", attr).Bool() {
			inp.Call("setAttribute", attr, "")
		} else {
			inp.Call("removeAttribute", attr)
		}
	}
	if host.Call("hasAttribute", "aria-label").Bool() {
		inp.Call("setAttribute", "aria-label", host.Call("getAttribute", "aria-label").String())
	} else {
		inp.Call("removeAttribute", "aria-label")
	}
}

// setValueHostAttrs sets data-has-value / data-empty on the host element and
// keeps the form submission value (ElementInternals.setFormValue) current.
func setValueHostAttrs(host js.Value, val string) {
	if val != "" {
		host.Call("setAttribute", "data-has-value", "")
		host.Call("removeAttribute", "data-empty")
	} else {
		host.Call("removeAttribute", "data-has-value")
		host.Call("setAttribute", "data-empty", "")
	}
	if internals := host.Get("_internals"); internals.Truthy() {
		internals.Call("setFormValue", val)
	}
}

// updateDerived recomputes clearable_show and value_len from the current value.
func updateDerived(obj *wings.PranaObj, val string) {
	clearable, _ := obj.This.Get("clearable").(bool)
	obj.This.Set("clearable_show", clearable && val != "")
	obj.This.Set("value_len", utf8.RuneCountInString(val))
}

// setupSlotWrapper registers a slotchange listener that shows or hides the
// wrapper element depending on whether the named slot has assigned nodes.
func setupSlotWrapper(shadow js.Value, slotSel, wrapSel string) {
	wraps := dom.Query(shadow, wrapSel)
	slots := dom.Query(shadow, slotSel)
	if len(wraps) == 0 || len(slots) == 0 {
		return
	}
	w, s := wraps[0], slots[0]
	dom.AddEvent(s, "slotchange", func(_ js.Value, args []js.Value) any {
		nodes := args[0].Get("target").Call("assignedNodes")
		if nodes.Length() > 0 {
			w.Get("style").Set("display", "inline-flex")
		} else {
			w.Get("style").Set("display", "none")
		}
		return nil
	}, false, false)
}
