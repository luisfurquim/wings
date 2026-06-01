//go:build js && wasm

package productpicker

import (
	_ "embed"
	"syscall/js"

	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
)

//go:embed productpicker.i18n.html
var htmlContent string

const cssContent = `.product-picker { display: inline-flex; gap: 6px; align-items: center; }
.product-picker label { font-size: 0.9rem; color: var(--wings-text-muted, #555); }`

// ProductPicker is a tiny <select> that lets the user choose which lemma feeds
// the runtime-inflection demo. It emits @productchange with the selected lemma.
type ProductPicker struct{}

func init() {
	wings.Register(
		"product-picker",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &ProductPicker{} },
	)
}

func (w *ProductPicker) InitData() map[string]any { return map[string]any{} }

func (w *ProductPicker) Render(obj *wings.PranaObj) {
	sels := dom.Query(obj.Dom, "#pp-sel")
	if len(sels) == 0 {
		return
	}
	sel := sels[0]
	dom.AddEvent(sel, "change",
		func(this js.Value, args []js.Value) any {
			obj.Trigger("productchange", sel.Get("value").String())
			return nil
		}, false, false)
}
