//go:build js && wasm

// Package input provides a w-input custom element for wings.
//
// Features:
//   - Full label/field/feedback anatomy with independent ::part() surfaces
//   - Three layout variants: outlined (default), filled, underlined
//   - Three sizes: sm, md, lg
//   - Named slots: label (rich label), prefix (left icon/text), suffix (right icon/text),
//     helper (rich helper text), error (rich error content)
//   - Clearable mode: shows a × button when the field has a value
//   - Required indicator (*) via required="true"
//   - Character count via maxlength attribute
//   - Host state attributes for CSS hooks: [data-focused], [data-has-value],
//     [data-empty], [data-invalid], [disabled], [variant], [size]
//   - Fires @change on every input event; @clear when the × button is clicked
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
//   - variant     — outlined (default) | filled | underlined
//   - size        — sm | md (default) | lg
//   - required    — "true" to show the required mark (*)
//   - clearable   — "true" to show the × button when field has a value
//   - disabled    — standard HTML (reflected to inner <input>)
//
// # Events fired to parent
//
//	@change  — fires on every keystroke; args[0] = current string value
//	@clear   — fires when the × clear button is clicked
//
// # CSS Customisation
//
// Input implements wings.Customizable. CSS is split into two parts:
//   - "Vars"   — CSS custom properties (empty by default).
//   - "Design" — Layout and structure rules.
//
// Key tokens consumed: --wings-input-bg, --wings-input-bg-filled,
// --wings-input-color, --wings-input-placeholder-color,
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
	wings.Register(
		elementTag,
		htmlContent,
		buildCSS(),
		func() wings.PranaMod { return &Input{} },
		"type", "label", "placeholder", "value",
		"helper", "error", "maxlength",
		"variant", "size", "required", "clearable",
	)
	G.Logf(3, "w-input: module registered\n")
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
		"variant":     "outlined",
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

	// Watch for dynamic disabled/readonly/required changes on the host.
	onHostMutation := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		reflectInputAttrs(obj.Element, inp)
		return nil
	})
	mo := js.Global().Get("MutationObserver").New(onHostMutation)
	mo.Call("observe", obj.Element, map[string]any{
		"attributes":      true,
		"attributeFilter": []any{"disabled", "readonly", "required"},
	})

	// Set initial host state attributes.
	initVal, _ := obj.This.Get("value").(string)
	setValueHostAttrs(obj.Element, initVal)

	// Set initial error host attribute.
	if errVal, _ := obj.This.Get("error").(string); errVal != "" {
		obj.Element.Call("setAttribute", "data-invalid", "")
	}

	// Recompute derived state from the initial value.
	updateDerived(obj, initVal)

	// ── Event listeners (freed by dom.RmEventsUnder on disconnect) ──────────

	dom.AddEvent(inp, "focus", func(_ js.Value, _ []js.Value) any {
		obj.Element.Call("setAttribute", "data-focused", "")
		return nil
	}, false, false)

	dom.AddEvent(inp, "blur", func(_ js.Value, _ []js.Value) any {
		obj.Element.Call("removeAttribute", "data-focused")
		return nil
	}, false, false)

	dom.AddEvent(inp, "input", func(_ js.Value, args []js.Value) any {
		val := inp.Get("value").String()
		setValueHostAttrs(obj.Element, val)
		updateDerived(obj, val)
		obj.Trigger("change", val)
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
			obj.Trigger("clear")
			inp.Call("focus")
			return nil
		}, false, false)
	}

	// Slot visibility for prefix/suffix.
	setupSlotWrapper(obj.Dom, "slot[name='prefix']", ".inp-prefix")
	setupSlotWrapper(obj.Dom, "slot[name='suffix']", ".inp-suffix")
}

// reflectInputAttrs copies disabled, readonly, required from the host to the inner <input>.
func reflectInputAttrs(host, inp js.Value) {
	for _, attr := range []string{"disabled", "readonly", "required"} {
		if host.Call("hasAttribute", attr).Bool() {
			inp.Call("setAttribute", attr, "")
		} else {
			inp.Call("removeAttribute", attr)
		}
	}
}

// setValueHostAttrs sets data-has-value / data-empty on the host element.
func setValueHostAttrs(host js.Value, val string) {
	if val != "" {
		host.Call("setAttribute", "data-has-value", "")
		host.Call("removeAttribute", "data-empty")
	} else {
		host.Call("removeAttribute", "data-has-value")
		host.Call("setAttribute", "data-empty", "")
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
