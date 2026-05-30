//go:build js && wasm

package tabsdemo

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed tabsdemo.i18n.html
var htmlContent string

const cssContent = `
.tabs-demo section { margin: 16px 0 24px; }
.tabs-demo h3 { margin: 0 0 8px; font-size: 1rem; color: var(--wings-text, #333); }
.tabs-demo .demo-frame { padding: 12px; border-radius: var(--wings-radius-lg, 10px); background: var(--wings-bg, #f5f5f5); }
.tabs-demo .bar { display: flex; gap: 2px; padding: 0 4px; border-bottom: var(--wings-border-width, 1px) var(--wings-border-style, solid) var(--wings-border, #ccc); }
.tabs-demo .content { min-height: 180px; }
.tabs-demo h4 { margin: 0 0 8px; }
.tabs-demo code { background: var(--wings-primary-pale, #eef); padding: 1px 4px; border-radius: var(--wings-radius-xs, 2px); }
.tabs-demo nav { display: flex; flex-direction: column; gap: 2px; }
`

type TabsDemo struct{}

func init() {
	wings.Register(
		"tabs-demo",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &TabsDemo{} },
	)
}

func (w *TabsDemo) InitData() map[string]any   { return map[string]any{} }
func (w *TabsDemo) Render(obj *wings.PranaObj) {}
