//go:build js && wasm

package basics

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed basicstab.i18n.html
var htmlContent string

const cssContent = `.basics-tab section { margin: 12px 0; }
.basics-tab h3 { margin: 0 0 6px; font-size: 1rem; color: var(--wings-text, #333); }`

type BasicsTab struct{}

func init() {
	wings.Register(
		"basics-tab",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &BasicsTab{} },
	)
}

func (w *BasicsTab) InitData() map[string]any   { return map[string]any{} }
func (w *BasicsTab) Render(obj *wings.PranaObj) {}
