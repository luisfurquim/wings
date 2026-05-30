//go:build js && wasm

package tempdisplay

import (
	_ "embed"

	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/wi18n/temperature"
)

//go:embed tempdisplay.i18n.html
var htmlContent string

const cssContent = `.temp-display table { border-collapse: collapse; margin: 4px 0; }
.temp-display td { padding: 2px 12px 2px 0; }
.temp-display td:first-child { color: var(--wings-text-light, #666); font-size: 0.9em; font-family: monospace; min-width: 180px; }
.temp-display strong { color: var(--wings-primary, #036); font-variant-numeric: tabular-nums; }`

type TempDisplay struct{}

func init() {
	wings.Register(
		"temperature-display",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &TempDisplay{} },
	)
}

func (w *TempDisplay) InitData() map[string]any {
	return map[string]any{
		"temp": temperature.Temperature{Kelvin: 300},
	}
}

func (w *TempDisplay) Render(obj *wings.PranaObj) {}
