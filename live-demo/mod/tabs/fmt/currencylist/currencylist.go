//go:build js && wasm

package currencylist

import (
	_ "embed"

	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/wi18n"
)

//go:embed currencylist.i18n.html
var htmlContent string

const cssContent = `.currency-list ul { margin: 0; padding-left: 18px; }
.currency-list li { margin: 2px 0; }
.currency-list strong { color: var(--wings-primary, #063); font-variant-numeric: tabular-nums; }`

type CurrencyList struct{}

func init() {
	wings.Register(
		"currency-list",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &CurrencyList{} },
	)
}

func (w *CurrencyList) InitData() map[string]any {
	return map[string]any{
		"cart": []any{
			wi18n.Currency{Amount: 4990, Code: "BRL"},
			wi18n.Currency{Amount: 1299, Code: "USD"},
			wi18n.Currency{Amount: 25000, Code: "ARS"},
			wi18n.Currency{Amount: 8750, Code: "EUR"},
		},
	}
}

func (w *CurrencyList) Render(obj *wings.PranaObj) {}
