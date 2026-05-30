//go:build js && wasm

package countinput

import (
	_ "embed"
	"strconv"
	"syscall/js"

	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
)

//go:embed countinput.i18n.html
var htmlContent string

const cssContent = `.count-input { display: inline-flex; gap: 6px; align-items: center; }
.count-input label { font-size: 0.9rem; color: var(--wings-text-muted, #555); }
.count-input input { width: 80px; padding: 2px 4px; }`

type CountInput struct{}

func init() {
	wings.Register(
		"count-input",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &CountInput{} },
	)
}

func (w *CountInput) InitData() map[string]any { return map[string]any{} }

func (w *CountInput) Render(obj *wings.PranaObj) {
	inputs := dom.Query(obj.Dom, "#ci-inp")
	if len(inputs) == 0 {
		return
	}
	inp := inputs[0]
	dom.AddEvent(inp, "input",
		func(this js.Value, args []js.Value) any {
			n, err := strconv.Atoi(inp.Get("value").String())
			if err != nil {
				n = 0
			}
			obj.Trigger("countchange", n)
			return nil
		}, false, false)
}
