//go:build js && wasm

// Package button provides a w-button custom element for wings.
//
// Features:
//   - Semantic variants via the variant attribute (secondary/primary/ghost/danger/success)
//   - Three sizes (sm/md/lg) and three shapes (default/pill/square)
//   - Optional loading spinner controlled by loading="true"
//   - Named slots: prefix (left icon), default (label), suffix (right icon)
//   - Full ::part() surface for external CSS customisation (root/prefix/label/suffix/spinner)
//   - Host state attributes for CSS hooks: [variant], [size], [shape], [disabled], [loading]
//   - Disabled reflected to inner <button> for keyboard and a11y correctness
//
// # Usage in parent template
//
//	<w-button variant="primary" @click="handler">Save</w-button>
//	<w-button variant="danger" size="sm" shape="pill">Delete</w-button>
//	<w-button loading="true">Saving…</w-button>
//
//	<!-- icon + label via slots -->
//	<w-button variant="primary">
//	  <svg slot="prefix">…</svg>
//	  Submit
//	</w-button>
//
// # Attributes
//
//   - variant   — secondary (default) | primary | ghost | danger | success
//   - size      — sm | md (default) | lg
//   - shape     — default | pill | square
//   - type      — like native <button>: inside a <form>, a click submits it
//     (via requestSubmit, so constraint validation runs first); type="button"
//     opts out. Outside a form the attribute is irrelevant.
//   - loading   — "true" to show spinner and keep button non-interactive
//     (also suppresses form submission)
//   - disabled  — standard HTML (reflected to inner <button>; CSS handles visual)
//
// # Events fired to parent
//
//	Clicks bubble naturally from the inner <button> through the open shadow
//	root and retarget to <w-button>. Use @click on the host as normal.
//
// # CSS Customisation
//
// Button implements wings.Customizable. CSS is split into two parts:
//   - "Vars"   — CSS custom properties (empty by default; defaults live in
//     "Design" as var(--wings-X, fallback)).
//   - "Design" — Layout and structure rules.
//
// Key tokens consumed: --wings-button-bg/hover-bg/active-bg/border/border-hover,
// --wings-button-padding, --wings-button-gap, --wings-button-font-weight,
// --wings-button-disabled-opacity, --wings-primary/primary-color/primary-hover-bg,
// --wings-primary-pale, --wings-danger/danger-hover, --wings-success/success-hover,
// --wings-radius-md/pill/xs/sm, --wings-text, --wings-focus-ring,
// --wings-transition-fast, --wings-hover-lift, --wings-active-scale,
// --wings-btn-hover-color, --wings-btn-hover-shadow.
//
// Key parts exposed: ::part(root), ::part(prefix), ::part(label),
// ::part(suffix), ::part(spinner).
package button

import (
	_ "embed"
	"strings"
	"syscall/js"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
)

const elementTag = "w-button"

// G is the logger for this module.
var G goose.Alert

//go:embed button.html
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

// New creates a new Button instance.
func New() *Button {
	return &Button{}
}

func init() {
	G.Set(3)
	cssParts[0].Content = varsCSS
	cssParts[1].Content = designCSS
	wings.RegisterWithOpts(
		elementTag,
		htmlContent,
		buildCSS(),
		// Form-associated so internals.form resolves the owning <form>: like a
		// native <button>, w-button submits it on click (type="button" opts out).
		wings.ComponentOpts{FormAssociated: true},
		func() wings.PranaMod { return &Button{} },
		"variant", "size", "shape", "loading",
	)
	G.Logf(3, "w-button: module registered\n")
}

// Button implements wings.PranaMod and wings.Customizable
// for the w-button custom element.
type Button struct{}

// Compile-time interface check.
var _ wings.Customizable = (*Button)(nil)

// ListCSS returns the named CSS parts in order.
func (b *Button) ListCSS() []wings.CSSPart {
	result := make([]wings.CSSPart, len(cssParts))
	copy(result, cssParts)
	return result
}

// ReplaceCSS replaces the CSS part identified by key and updates
// all live instances via wings.Update.
func (b *Button) ReplaceCSS(key string, content string) {
	for i := range cssParts {
		if cssParts[i].Name == key {
			cssParts[i].Content = content
			wings.Update(elementTag, buildCSS())
			return
		}
	}
	G.Logf(1, "ReplaceCSS: key %q not found\n", key)
}

func (b *Button) InitData() map[string]any {
	return map[string]any{
		"variant": "secondary",
		"size":    "md",
		"shape":   "default",
		"loading": false,
	}
}

func (b *Button) Render(obj *wings.PranaObj) {
	btns := dom.Query(obj.Dom, ".btn-root")
	if len(btns) == 0 {
		return
	}
	btn := btns[0]

	// Reflect initial disabled state to the inner <button>.
	reflectDisabled(obj.Element, btn)

	// Watch for dynamic disabled changes on the host element.
	// dom.Observe registers the observer for auto-release on disconnect.
	dom.Observe(obj.Element, map[string]any{
		"attributes":      true,
		"attributeFilter": []any{"disabled"},
	}, func(_ js.Value, _ []js.Value) any {
		reflectDisabled(obj.Element, btn)
		return nil
	})

	// Native-like form submission: the inner <button> lives in the shadow DOM,
	// so its click cannot submit a light-DOM <form> by itself. Mirror the
	// platform default — a button inside a form submits it unless
	// type="button" — via the form-associated internals. requestSubmit runs
	// constraint validation first, so invalid form-associated fields (w-input)
	// block it and get the native bubble.
	dom.AddEvent(btn, "click", func(_ js.Value, _ []js.Value) any {
		host := obj.Element
		typ := host.Call("getAttribute", "type")
		if (typ.Type() == js.TypeString && typ.String() == "button") ||
			host.Call("hasAttribute", "disabled").Bool() ||
			host.Call("hasAttribute", "loading").Bool() {
			return nil
		}
		internals := host.Get("_internals")
		if !internals.Truthy() {
			return nil
		}
		if form := internals.Get("form"); form.Truthy() {
			form.Call("requestSubmit")
		}
		return nil
	}, false, false)

	// Show prefix/suffix slot wrappers only when slot content is assigned.
	setupSlotWrapper(obj.Dom, "slot[name='prefix']", ".btn-prefix")
	setupSlotWrapper(obj.Dom, "slot[name='suffix']", ".btn-suffix")
}

// reflectDisabled copies the host disabled attribute to the inner <button>.
func reflectDisabled(host, btn js.Value) {
	if host.Call("hasAttribute", "disabled").Bool() {
		btn.Call("setAttribute", "disabled", "")
	} else {
		btn.Call("removeAttribute", "disabled")
	}
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
