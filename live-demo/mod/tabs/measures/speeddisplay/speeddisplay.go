//go:build js && wasm

package speeddisplay

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/wi18n/speed"
)

//go:embed speeddisplay.i18n.html
var htmlContent string

const cssContent = `.speed-display table { border-collapse: collapse; margin: 4px 0; }
.speed-display td { padding: 2px 12px 2px 0; }
.speed-display td:first-child { color: var(--wings-text-light, #666); font-size: 0.9em; font-family: monospace; min-width: 180px; }
.speed-display strong { color: var(--wings-primary, #036); font-variant-numeric: tabular-nums; }`

type SpeedDisplay struct{}

func init() {
	wprana.Register(
		"speed-display",
		htmlContent,
		cssContent,
		func() wprana.PranaMod { return &SpeedDisplay{} },
	)
}

func (w *SpeedDisplay) InitData() map[string]any {
	return map[string]any{
		"vel": speed.Speed{MetersPerSecond: 30},
	}
}

func (w *SpeedDisplay) Render(obj *wprana.PranaObj) {}
