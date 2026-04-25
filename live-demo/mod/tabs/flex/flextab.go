//go:build js && wasm

package flex

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed flextab.i18n.html
var htmlContent string

const cssContent = `.tab-stub { padding: 8px 0; color: #444; }
.tab-stub h2 { margin: 4px 0 8px; font-size: 1.1rem; }
.tab-stub p { margin: 0; font-style: italic; }`

type FlexTab struct{}

func init() {
	wprana.Register(
		"flex-tab",
		htmlContent,
		cssContent,
		func() wprana.PranaMod { return &FlexTab{} },
	)
}

func (w *FlexTab) InitData() map[string]any { return map[string]any{} }
func (w *FlexTab) Render(obj *wprana.PranaObj) {}
