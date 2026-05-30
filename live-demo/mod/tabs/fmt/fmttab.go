//go:build js && wasm

package fmt

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed fmttab.i18n.html
var htmlContent string

const cssContent = `.fmt-tab section { margin: 12px 0; }
.fmt-tab h3 { margin: 0 0 6px; font-size: 1rem; color: var(--wings-text, #333); }`

type FmtTab struct{}

func init() {
	wings.Register(
		"fmt-tab",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &FmtTab{} },
	)
}

func (w *FmtTab) InitData() map[string]any { return map[string]any{} }
func (w *FmtTab) Render(obj *wings.PranaObj)  {}
