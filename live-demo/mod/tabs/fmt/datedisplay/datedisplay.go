//go:build js && wasm

package datedisplay

import (
	_ "embed"
	"time"

	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/timer"
)

//go:embed datedisplay.i18n.html
var htmlContent string

const cssContent = `.date-display p { margin: 4px 0; }
.date-display strong { color: var(--wings-primary, #503); }`

type DateDisplay struct{}

func init() {
	wprana.Register(
		"date-display",
		htmlContent,
		cssContent,
		func() wprana.PranaMod { return &DateDisplay{} },
	)
}

func (w *DateDisplay) InitData() map[string]any {
	return map[string]any{
		"now": time.Now(),
	}
}

func (w *DateDisplay) Render(obj *wprana.PranaObj) {
	go func() {
		tk := timer.NewTicker(1000)
		defer tk.Stop()
		for range tk.Tick {
			obj.This.Set("now", time.Now())
		}
	}()
}
