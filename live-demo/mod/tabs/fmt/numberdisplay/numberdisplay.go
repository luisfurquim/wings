//go:build js && wasm

package numberdisplay

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed numberdisplay.i18n.html
var htmlContent string

const cssContent = `.number-display p { margin: 4px 0; }
.number-display strong { color: var(--wings-primary, #036); font-variant-numeric: tabular-nums; }`

type NumberDisplay struct{}

func init() {
	wings.Register(
		"number-display",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &NumberDisplay{} },
	)
}

func (w *NumberDisplay) InitData() map[string]any {
	return map[string]any{
		"intVal":   1234567,
		"floatVal": 1234.56789,
	}
}

func (w *NumberDisplay) Render(obj *wings.PranaObj) {}
