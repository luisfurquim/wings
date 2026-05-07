//go:build js && wasm

package lengthdisplay

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/wi18n/length"
)

//go:embed lengthdisplay.i18n.html
var htmlContent string

const cssContent = `.length-display table { border-collapse: collapse; margin: 4px 0; }
.length-display td { padding: 2px 12px 2px 0; }
.length-display td:first-child { color: var(--wings-text-light, #666); font-size: 0.9em; font-family: monospace; min-width: 180px; }
.length-display strong { color: var(--wings-primary, #036); font-variant-numeric: tabular-nums; }`

type LengthDisplay struct{}

func init() {
	wprana.Register(
		"length-display",
		htmlContent,
		cssContent,
		func() wprana.PranaMod { return &LengthDisplay{} },
	)
}

func (w *LengthDisplay) InitData() map[string]any {
	return map[string]any{
		"dist": length.Length{Meters: 1500},
	}
}

func (w *LengthDisplay) Render(obj *wprana.PranaObj) {}
