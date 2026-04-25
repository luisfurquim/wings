//go:build js && wasm

package currencydisplay

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/wi18n"
)

//go:embed currencydisplay.i18n.html
var htmlContent string

const cssContent = `.currency-display p { margin: 4px 0; }
.currency-display strong { color: #063; font-variant-numeric: tabular-nums; }`

type CurrencyDisplay struct{}

func init() {
	wprana.Register(
		"currency-display",
		htmlContent,
		cssContent,
		func() wprana.PranaMod { return &CurrencyDisplay{} },
	)
}

func (w *CurrencyDisplay) InitData() map[string]any {
	return map[string]any{
		"price": wi18n.Currency{Amount: 1299, Code: "USD"},
	}
}

func (w *CurrencyDisplay) Render(obj *wprana.PranaObj) {}
