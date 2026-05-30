//go:build js && wasm

package currencydisplay

import (
	_ "embed"

	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/wi18n"
)

//go:embed currencydisplay.i18n.html
var htmlContent string

const cssContent = `.currency-display p { margin: 4px 0; }
.currency-display strong { color: var(--wings-primary, #063); font-variant-numeric: tabular-nums; }`

type CurrencyDisplay struct{}

func init() {
	wings.Register(
		"currency-display",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &CurrencyDisplay{} },
	)
}

func (w *CurrencyDisplay) InitData() map[string]any {
	return map[string]any{
		"price": wi18n.Currency{Amount: 1299, Code: "USD"},
	}
}

func (w *CurrencyDisplay) Render(obj *wings.PranaObj) {}
