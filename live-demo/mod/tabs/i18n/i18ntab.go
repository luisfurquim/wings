//go:build js && wasm

package i18ntab

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed i18ntab.i18n.html
var htmlContent string

const cssContent = `.tab-stub { padding: 8px 0; color: #444; }
.tab-stub h2 { margin: 4px 0 8px; font-size: 1.1rem; }
.tab-stub p { margin: 0; font-style: italic; }`

type I18nTab struct{}

func init() {
	wprana.Register(
		"i18n-tab",
		htmlContent,
		cssContent,
		func() wprana.PranaMod { return &I18nTab{} },
	)
}

func (w *I18nTab) InitData() map[string]any { return map[string]any{} }
func (w *I18nTab) Render(obj *wprana.PranaObj) {}
