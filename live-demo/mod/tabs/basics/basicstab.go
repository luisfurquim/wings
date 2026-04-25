//go:build js && wasm

package basics

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed basicstab.i18n.html
var htmlContent string

const cssContent = `.tab-stub { padding: 8px 0; color: #444; }
.tab-stub h2 { margin: 4px 0 8px; font-size: 1.1rem; }
.tab-stub p { margin: 0; font-style: italic; }`

type BasicsTab struct{}

func init() {
	wprana.Register(
		"basics-tab",
		htmlContent,
		cssContent,
		func() wprana.PranaMod { return &BasicsTab{} },
	)
}

func (w *BasicsTab) InitData() map[string]any { return map[string]any{} }
func (w *BasicsTab) Render(obj *wprana.PranaObj) {}
