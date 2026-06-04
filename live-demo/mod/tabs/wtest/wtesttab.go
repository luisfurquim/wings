//go:build js && wasm

package wtest

import (
	_ "embed"
	"fmt"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wings"
)

// G is the logger for this demo module.
var G goose.Alert

//go:embed wtesttab.i18n.html
var htmlContent string

const cssContent = `.wtest-tab section { margin: 12px 0; }
.wtest-tab w-test { display: block; margin: 12px 0; }`

type WTestTab struct{}

func init() {
	G.Set(2)

	// countChanged passes once the wrapped <count-input> has fired a
	// countchange event; the detail shows the most recent value.
	wings.RegisterCheck("countChanged", func(ctx wings.CheckCtx) (bool, string) {
		for i := len(ctx.Events) - 1; i >= 0; i-- {
			if ctx.Events[i].Name != "countchange" {
				continue
			}
			val := ""
			if len(ctx.Events[i].Args) > 0 {
				val = fmt.Sprintf("%v", ctx.Events[i].Args[0])
			}
			return true, "último valor: " + val
		}
		return false, "altere o número para disparar countchange"
	})

	wings.Register(
		"wtest-tab",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &WTestTab{} },
	)
}

func (w *WTestTab) InitData() map[string]any {
	return map[string]any{"on_report": wings.TriggerHandler(nil)}
}

func (w *WTestTab) Render(obj *wings.PranaObj) {
	// The app receives the report and decides what to do with it. The widget
	// already shows the JSON; here we just log that it arrived to show the
	// @report channel working — a real app would POST it, save it, diff it…
	obj.This.Set("on_report", func(args ...any) {
		size := 0
		if len(args) > 0 {
			size = len(fmt.Sprintf("%v", args[0]))
		}
		G.Logf(2, "wtest-tab: got test report (%d bytes)\n", size)
	})
}
