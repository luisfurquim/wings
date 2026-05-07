//go:build js && wasm

package flex

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed flextab.i18n.html
var htmlContent string

const cssContent = `.flex-tab .controls { display: flex; gap: 18px; flex-wrap: wrap; margin: 8px 0 12px; }
.flex-tab .result { background: var(--wings-surface, #f7f7f7); padding: 10px 14px; border-radius: 4px; font-size: 1rem; }`

type FlexTab struct{}

func init() {
	wprana.Register(
		"flex-tab",
		htmlContent,
		cssContent,
		func() wprana.PranaMod { return &FlexTab{} },
	)
}

func (w *FlexTab) InitData() map[string]any {
	return map[string]any{
		"gender":    "m",
		"qt":        1,
		"setgender": wprana.TriggerHandler(nil),
		"setcount":  wprana.TriggerHandler(nil),
	}
}

func (w *FlexTab) Render(obj *wprana.PranaObj) {
	obj.This.M["setgender"] = wprana.TriggerHandler(func(args ...any) {
		if len(args) == 0 {
			return
		}
		if s, ok := args[0].(string); ok {
			obj.This.Set("gender", s)
		}
	})
	obj.This.M["setcount"] = wprana.TriggerHandler(func(args ...any) {
		if len(args) == 0 {
			return
		}
		if n, ok := args[0].(int); ok {
			obj.This.Set("qt", n)
		}
	})
}
