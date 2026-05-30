//go:build js && wasm

package genderpicker

import (
	_ "embed"
	"syscall/js"

	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
)

//go:embed genderpicker.i18n.html
var htmlContent string

const cssContent = `.gender-picker { display: inline-flex; gap: 6px; align-items: center; }
.gender-picker label { font-size: 0.9rem; color: var(--wings-text-muted, #555); }`

type GenderPicker struct{}

func init() {
	wings.Register(
		"gender-picker",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &GenderPicker{} },
	)
}

func (w *GenderPicker) InitData() map[string]any { return map[string]any{} }

func (w *GenderPicker) Render(obj *wings.PranaObj) {
	sels := dom.Query(obj.Dom, "#gp-sel")
	if len(sels) == 0 {
		return
	}
	sel := sels[0]
	dom.AddEvent(sel, "change",
		func(this js.Value, args []js.Value) any {
			obj.Trigger("genderchange", sel.Get("value").String())
			return nil
		}, false, false)
}
