//go:build js && wasm

package tempdisplay

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/wi18n/temperature"
)

//go:embed tempdisplay.i18n.html
var htmlContent string

const cssContent = `.temp-display table { border-collapse: collapse; margin: 4px 0; }
.temp-display td { padding: 2px 12px 2px 0; }
.temp-display td:first-child { color: #666; font-size: 0.9em; font-family: monospace; min-width: 180px; }
.temp-display strong { color: #036; font-variant-numeric: tabular-nums; }`

type TempDisplay struct{}

func init() {
	wprana.Register(
		"temperature-display",
		htmlContent,
		cssContent,
		func() wprana.PranaMod { return &TempDisplay{} },
	)
}

func (w *TempDisplay) InitData() map[string]any {
	return map[string]any{
		"temp": temperature.Temperature{Kelvin: 300},
	}
}

func (w *TempDisplay) Render(obj *wprana.PranaObj) {}
