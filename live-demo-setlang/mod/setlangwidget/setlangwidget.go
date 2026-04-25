//go:build js && wasm

package setlangwidget

import (
	_ "embed"
	"syscall/js"

	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/dom"
	"github.com/luisfurquim/wprana/wi18n"
)

//go:embed setlangwidget.i18n.html
var htmlContent string

//go:embed setlangwidget.css
var cssContent string

type SetLangWidget struct{}

func init() {
	wprana.Register(
		"setlang-widget",
		htmlContent,
		cssContent,
		func() wprana.PranaMod { return &SetLangWidget{} },
	)
}

func (w *SetLangWidget) InitData() map[string]any {
	return map[string]any{
		"input_val": "",
		"items":     []any{},
		"count":     0,
		"locale":    wprana.Locale,
	}
}

func (w *SetLangWidget) Render(obj *wprana.PranaObj) {
	if inputs := dom.Query(obj.Dom, "#inp"); len(inputs) > 0 {
		dom.AddEvent(inputs[0], "input",
			func(this js.Value, args []js.Value) any {
				obj.This.Set("input_val", inputs[0].Get("value").String())
				return nil
			}, false, false)
	}

	if addBtn := dom.Query(obj.Dom, "#btn-add"); len(addBtn) > 0 {
		dom.AddEvent(addBtn[0], "click",
			func(this js.Value, args []js.Value) any {
				val, _ := obj.This.Get("input_val").(string)
				if val == "" {
					return nil
				}
				obj.This.Append("items", val)
				obj.This.Set("input_val", "")
				items, _ := obj.This.Get("items").([]any)
				obj.This.Set("count", len(items))
				return nil
			}, false, false)
	}

	if switchBtn := dom.Query(obj.Dom, "#btn-switch"); len(switchBtn) > 0 {
		dom.AddEvent(switchBtn[0], "click",
			func(this js.Value, args []js.Value) any {
				next := "en-US"
				if wprana.Locale == "en-US" {
					next = "pt-BR"
				}
				wi18n.SetLang(next, func(err error) {
					if err != nil {
						wprana.G.Logf(1, "setlangwidget: SetLang error: %v\n", err)
						return
					}
					obj.This.Set("locale", wprana.Locale)
				})
				return nil
			}, false, false)
	}
}
