//go:build js && wasm

package widgets

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed widgetstab.html
var htmlContent string

const cssContent = `
.widgets-tab { display: flex; flex-direction: column; gap: 24px; padding: 8px 0; }
.widgets-tab section { display: flex; flex-direction: column; gap: 12px; }
.widgets-tab h3 {
  margin: 0 0 4px;
  font-size: 0.85rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--wings-text-muted, #666);
  border-bottom: 1px solid var(--wings-border, #e0e0e0);
  padding-bottom: 6px;
}
.widget-row { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; }
`

type WidgetsTab struct{}

func init() {
	wings.Register(
		"widgets-tab",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &WidgetsTab{} },
	)
}

func (w *WidgetsTab) InitData() map[string]any   { return map[string]any{} }
func (w *WidgetsTab) Render(obj *wings.PranaObj) {}
