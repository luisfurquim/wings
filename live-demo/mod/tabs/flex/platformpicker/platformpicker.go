//go:build js && wasm

package platformpicker

import (
	_ "embed"
	"syscall/js"

	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
)

//go:embed platformpicker.i18n.html
var htmlContent string

const cssContent = `.platform-picker { display: inline-flex; gap: 6px; align-items: center; }
.platform-picker label { font-size: 0.9rem; color: var(--wings-text-muted, #555); }`

// PlatformPicker is a tiny <select> that lets the user choose the platform fed
// to the contextual-selection demo. It emits @platformchange with the value.
type PlatformPicker struct{}

func init() {
	wings.Register(
		"platform-picker",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &PlatformPicker{} },
	)
}

func (w *PlatformPicker) InitData() map[string]any { return map[string]any{} }

func (w *PlatformPicker) Render(obj *wings.PranaObj) {
	sels := dom.Query(obj.Dom, "#plat-sel")
	if len(sels) == 0 {
		return
	}
	sel := sels[0]
	dom.AddEvent(sel, "change",
		func(this js.Value, args []js.Value) any {
			obj.Trigger("platformchange", sel.Get("value").String())
			return nil
		}, false, false)
}
