//go:build js && wasm

package i18ntab

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed i18ntab.i18n.html
var htmlContent string

const cssContent = `.i18n-tab p { color: var(--wings-text, #444); line-height: 1.4; }
.i18n-tab table { border-collapse: collapse; width: 100%; margin-top: 8px; }
.i18n-tab th, .i18n-tab td { border: 1px solid var(--wings-border, #ddd); padding: 6px 10px; text-align: left; vertical-align: top; }
.i18n-tab th { background: var(--wings-surface, #f0f0f0); font-size: 0.9rem; }
.i18n-tab td.src { color: var(--wings-text-light, #666); font-style: italic; }
.i18n-tab td.cur { color: var(--wings-primary, #036); font-weight: 600; }`

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
func (w *I18nTab) Render(obj *wprana.PranaObj)  {}
