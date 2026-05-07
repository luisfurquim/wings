//go:build js && wasm

package counterpair

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/timer"
)

//go:embed counterpair.i18n.html
var htmlContent string

const cssContent = `.counter-pair p { margin: 4px 0; }
.counter-pair strong { color: var(--wings-primary, #0a5); font-variant-numeric: tabular-nums; }`

type CounterPair struct{}

func init() {
	wprana.Register(
		"counter-pair",
		htmlContent,
		cssContent,
		func() wprana.PranaMod { return &CounterPair{} },
	)
}

func (w *CounterPair) InitData() map[string]any {
	return map[string]any{
		"count":  0,
		"count2": 0,
	}
}

func (w *CounterPair) Render(obj *wprana.PranaObj) {
	go func() {
		tk := timer.NewTicker(2000)
		defer tk.Stop()
		n := 0
		for range tk.Tick {
			n++
			obj.This.Set("count", n)
		}
	}()
	go func() {
		tk := timer.NewTicker(5000)
		defer tk.Stop()
		n := 0
		for range tk.Tick {
			n++
			obj.This.Set("count2", n)
		}
	}()
}
