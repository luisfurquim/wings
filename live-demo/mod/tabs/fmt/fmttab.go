//go:build js && wasm

package fmt

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed fmttab.i18n.html
var htmlContent string

const cssContent = `.tab-stub { padding: 8px 0; color: #444; }
.tab-stub h2 { margin: 4px 0 8px; font-size: 1.1rem; }
.tab-stub p { margin: 0; font-style: italic; }`

type FmtTab struct{}

func init() {
	wprana.Register(
		"fmt-tab",
		htmlContent,
		cssContent,
		func() wprana.PranaMod { return &FmtTab{} },
	)
}

func (w *FmtTab) InitData() map[string]any { return map[string]any{} }
func (w *FmtTab) Render(obj *wprana.PranaObj) {}
