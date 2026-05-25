//go:build js && wasm

// Package tabbutton provides the w-tabbutton custom element for wprana.
//
// w-tabbutton is a strictly passive leaf — and that passivity is load-bearing.
// In accordion mode the parent <w-tabs> physically MOVES this element into the
// matching <w-tab> (so it becomes the native <summary>). A DOM move fires
// disconnectedCallback, which the framework treats destructively (it deletes
// the node's reactive state — see prana.go elementDisconnected). Because this
// widget owns no JS-side state, runs no Render logic, and drives its visuals
// purely through CSS attribute selectors (:host([active]) / :host([mode=…])),
// it keeps working after the move: the shadow root and CSS persist on the
// element, and clicks still bubble to <w-tabs> via event delegation. Do NOT
// add data binding or Render behaviour here without revisiting the move.
//
// The inner element is a <span>, not a <button>: a real button inside a
// <summary> swallows the activation and blocks the native <details> toggle.
// <w-tabs> supplies keyboard activation and a roving tabindex on the host.
//
// # Attributes
//
//   - tid     (optional) — string identifier for programmatic/deep-link
//     activation. Pairing of button↔panel is by DOM adjacency (the next
//     <w-tab> sibling), so tid is no longer required for basic operation.
//   - active  (boolean)  — present when this is the active tab. Set/cleared
//     by the parent <w-tabs>; the webdev may write it on the initial markup
//     to choose which tab starts active.
//   - mode    (managed)  — stamped by <w-tabs> (panel/menu/detached/accordion)
//     to select the per-mode CSS. Webdevs do not write it.
//
// # Example (co-located with its panel, direct children of <w-tabs>)
//
//	<w-tabbutton active>Overview</w-tabbutton>
//	<w-tab>…</w-tab>
package tabbutton

import (
	_ "embed"
	"strings"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wprana"
)

const elementTag = "w-tabbutton"

// G is the logger for this module.
var G goose.Alert

//go:embed tabbutton.html
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

// New creates a new TabButton instance.
func New() *TabButton {
	return &TabButton{}
}

func init() {
	G.Set(3)
	cssParts[0].Content = varsCSS
	cssParts[1].Content = designCSS
	wprana.Register(
		elementTag,
		htmlContent,
		buildCSS(),
		func() wprana.PranaMod { return &TabButton{} },
		"tid", "active",
	)
	G.Logf(3, "w-tabbutton: module registered\n")
}

// TabButton implements wprana.PranaMod and wprana.Customizable for the
// w-tabbutton custom element.
type TabButton struct{}

var _ wprana.Customizable = (*TabButton)(nil)

func (b *TabButton) ListCSS() []wprana.CSSPart {
	result := make([]wprana.CSSPart, len(cssParts))
	copy(result, cssParts)
	return result
}

func (b *TabButton) ReplaceCSS(key string, content string) {
	for i := range cssParts {
		if cssParts[i].Name == key {
			cssParts[i].Content = content
			wprana.Update(elementTag, buildCSS())
			return
		}
	}
	G.Logf(1, "ReplaceCSS: key %q not found\n", key)
}

func (b *TabButton) InitData() map[string]any {
	return map[string]any{
		"tid":    "",
		"active": false,
	}
}

// Render is intentionally empty — the parent <w-tabs> drives the active
// state via setAttribute, and the click event bubbles naturally.
func (b *TabButton) Render(obj *wprana.PranaObj) {}
