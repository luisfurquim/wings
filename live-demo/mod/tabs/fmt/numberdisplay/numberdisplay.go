//go:build js && wasm

package numberdisplay

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed numberdisplay.i18n.html
var htmlContent string

const cssContent = `.number-display p { margin: 4px 0; }
.number-display strong { color: #036; font-variant-numeric: tabular-nums; }`

type NumberDisplay struct{}

func init() {
	wprana.Register(
		"number-display",
		htmlContent,
		cssContent,
		func() wprana.PranaMod { return &NumberDisplay{} },
	)
}

func (w *NumberDisplay) InitData() map[string]any {
	return map[string]any{
		"intVal":   1234567,
		"floatVal": 1234.56789,
	}
}

func (w *NumberDisplay) Render(obj *wprana.PranaObj) {}
