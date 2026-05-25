//go:build js && wasm

// Package tab provides the w-tab custom element for wprana.
//
// w-tab is a content panel with a per-form shadow template chosen by the
// `mode` attribute the parent <w-tabs> stamps on it (see tab.html). In the
// panel/detached/menu forms it is a transparent <slot> whose visibility is
// governed by the boolean `active` attribute — hidden via
// :host(:not([active]):not([mode=accordion])) { display:none } so the DOM (and
// any state inside it — input values, scroll position, embedded widgets) is
// preserved across switches. In the accordion form it renders a native
// <details>; <w-tabs> moves the matching <w-tabbutton> in as the <summary>, and
// after the first paint open/close is the platform's job. The `active` flag
// seeds the INITIAL open state: a w-tab marked `active` starts expanded (Render
// mirrors `active`→`<details open>`); thereafter the user's clicks drive the
// native disclosure and `active` is left alone.
//
// # Attributes
//
//   - tid     (optional) — identifier for programmatic/deep-link activation.
//     Button↔panel pairing is by DOM adjacency, so tid is not required.
//   - active  (boolean)  — non-accordion: present when this panel is visible
//     (managed by <w-tabs>). accordion: the author may write it to choose which
//     section starts open.
//   - mode    (managed)  — panel/detached/menu/accordion, stamped by <w-tabs>
//     to select the per-form template and CSS. Webdevs do not write it.
//
// # Sizing
//
// The host element is `width: 100%; height: 100%` so the webdev's outer
// container determines the panel's size.
//
// # Example (co-located with its button, direct children of <w-tabs>)
//
//	<w-tabbutton>Overview</w-tabbutton>
//	<w-tab>
//	    <h2>Overview</h2>
//	    <p>…</p>
//	</w-tab>
package tab

import (
	_ "embed"
	"strings"
	"syscall/js"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wprana"
)

const elementTag = "w-tab"

// G is the logger for this module.
var G goose.Alert

//go:embed tab.html
var htmlContent string

//go:embed vars.css
var varsCSS string

//go:embed design.css
var designCSS string

var cssParts = []wprana.CSSPart{
	{Name: "Vars", Content: ""},
	{Name: "Design", Content: ""},
}

func buildCSS() string {
	var sb strings.Builder
	for _, p := range cssParts {
		sb.WriteString(p.Content)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// New creates a new Tab instance.
func New() *Tab {
	return &Tab{}
}

func init() {
	G.Set(3)
	cssParts[0].Content = varsCSS
	cssParts[1].Content = designCSS
	wprana.Register(
		elementTag,
		htmlContent,
		buildCSS(),
		func() wprana.PranaMod { return &Tab{} },
		"tid", "active", "mode",
	)
	G.Logf(3, "w-tab: module registered\n")
}

// Tab implements wprana.PranaMod and wprana.Customizable for the w-tab
// custom element.
type Tab struct{}

var _ wprana.Customizable = (*Tab)(nil)

func (t *Tab) ListCSS() []wprana.CSSPart {
	result := make([]wprana.CSSPart, len(cssParts))
	copy(result, cssParts)
	return result
}

func (t *Tab) ReplaceCSS(key string, content string) {
	for i := range cssParts {
		if cssParts[i].Name == key {
			cssParts[i].Content = content
			wprana.Update(elementTag, buildCSS())
			return
		}
	}
	G.Logf(1, "ReplaceCSS: key %q not found\n", key)
}

func (t *Tab) InitData() map[string]any {
	return map[string]any{
		"tid":    "",
		"active": false,
		// Default form. <w-tabs> stamps the real mode; the ?mode conditional
		// in tab.html resolves against this var, so it must start defined.
		"mode": "panel",
	}
}

// Render wires the one behaviour the CSS/conditional cannot express on its own:
// in accordion mode, seed the native <details>'s initial open state from the
// host's `active` attribute. (HTML's `open` is boolean — present means open — so
// it cannot be data-bound; and the wt template already spends its single ?cond
// on the mode switch.) Everything else — which form to render, panel
// visibility — stays CSS/conditional-driven.
func (t *Tab) Render(obj *wprana.PranaObj) {
	host := obj.Element

	// Mirror active→<details open>. No-op unless mode is accordion AND the
	// shadow <details> already exists. Both directions so a later active
	// change (programmatic) re-seeds; a user's manual toggle is not touched
	// because it changes the shadow <details>, not the host's `active`.
	syncOpen := func() {
		if m := host.Call("getAttribute", "mode"); !m.Truthy() || m.String() != "accordion" {
			return
		}
		sr := host.Get("shadowRoot")
		if !sr.Truthy() {
			return
		}
		det := sr.Call("querySelector", "details")
		if !det.Truthy() {
			return
		}
		if host.Call("hasAttribute", "active").Bool() {
			det.Call("setAttribute", "open", "")
		} else {
			det.Call("removeAttribute", "open")
		}
	}

	// Initial pass: covers the case where <w-tabs> stamped mode=accordion
	// before this element upgraded (the <details> already exists here).
	syncOpen()

	// Later passes: when <w-tabs> stamps mode=accordion after upgrade, its
	// setAttribute fires this element's (synchronous) attributeChangedCallback,
	// which creates the <details>; this observer's microtask then runs with the
	// <details> present. Also re-seeds on programmatic `active` changes.
	mo := js.Global().Get("MutationObserver").New(js.FuncOf(func(_ js.Value, _ []js.Value) any {
		syncOpen()
		return nil
	}))
	mo.Call("observe", host, map[string]any{
		"attributes":      true,
		"attributeFilter": []any{"mode", "active"},
	})
}
