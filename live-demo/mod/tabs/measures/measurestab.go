//go:build js && wasm

package measures

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed measurestab.i18n.html
var htmlContent string

const cssContent = `.measures-tab section { margin: 12px 0; }
.measures-tab h3 { margin: 0 0 6px; font-size: 1rem; color: var(--wings-text, #333); }`

type MeasuresTab struct{}

func init() {
	wings.Register(
		"measures-tab",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &MeasuresTab{} },
	)
}

func (w *MeasuresTab) InitData() map[string]any   { return map[string]any{} }
func (w *MeasuresTab) Render(obj *wings.PranaObj) {}
