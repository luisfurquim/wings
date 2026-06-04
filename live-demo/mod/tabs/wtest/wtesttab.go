//go:build js && wasm

package wtest

import (
	_ "embed"
	"fmt"

	"github.com/luisfurquim/wings"
)

//go:embed wtesttab.i18n.html
var htmlContent string

const cssContent = `.wtest-tab section { margin: 12px 0; }
.wtest-tab w-test { display: block; margin: 12px 0; }`

type WTestTab struct{}

func init() {
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

func (w *WTestTab) InitData() map[string]any   { return map[string]any{} }
func (w *WTestTab) Render(obj *wings.PranaObj) {}
