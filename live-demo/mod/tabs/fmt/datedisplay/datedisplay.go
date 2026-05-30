//go:build js && wasm

package datedisplay

import (
	_ "embed"
	"time"

	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/timer"
)

//go:embed datedisplay.i18n.html
var htmlContent string

const cssContent = `.date-display p { margin: 4px 0; }
.date-display strong { color: var(--wings-primary, #503); }`

type DateDisplay struct{}

func init() {
	wings.Register(
		"date-display",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &DateDisplay{} },
	)
}

func (w *DateDisplay) InitData() map[string]any {
	return map[string]any{
		"now": time.Now(),
	}
}

func (w *DateDisplay) Render(obj *wings.PranaObj) {
	go func() {
		tk := timer.NewTicker(1000)
		defer tk.Stop()
		for range tk.Tick {
			obj.This.Set("now", time.Now())
		}
	}()
}
