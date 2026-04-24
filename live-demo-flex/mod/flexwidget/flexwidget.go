//go:build js && wasm

package flexwidget

import (
	_ "embed"
	"strconv"
	"syscall/js"

	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/dom"
)

//go:embed flexwidget.i18n.html
var htmlContent string

//go:embed flexwidget.css
var cssContent string

type FlexWidget struct{}

func init() {
	wprana.Register(
		"flex-widget",
		htmlContent,
		cssContent,
		func() wprana.PranaMod { return &FlexWidget{} },
	)
}

func (w *FlexWidget) InitData() map[string]any {
	return map[string]any{
		"genero": "m",
		"qt":   1,
	}
}

func (w *FlexWidget) Render(obj *wprana.PranaObj) {
	sels := dom.Query(obj.Dom, "#sel-genero")
	if len(sels) > 0 {
		dom.AddEvent(sels[0], "change",
			func(this js.Value, args []js.Value) any {
				obj.This.Set("genero", sels[0].Get("value").String())
				return nil
			}, false, false)
	}

	inputs := dom.Query(obj.Dom, "#inp-qt")
	if len(inputs) > 0 {
		dom.AddEvent(inputs[0], "input",
			func(this js.Value, args []js.Value) any {
				n, err := strconv.Atoi(inputs[0].Get("value").String())
				if err != nil {
					n = 0
				}
				obj.This.Set("qt", n)
				return nil
			}, false, false)
	}
}
