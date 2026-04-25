//go:build js && wasm

package itemlist

import (
	_ "embed"
	"syscall/js"

	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/dom"
)

//go:embed itemlist.i18n.html
var htmlContent string

const cssContent = `.item-list form { display: flex; gap: 6px; align-items: center; margin-bottom: 8px; }
.item-list label { font-size: 0.9rem; color: #555; }
.item-list input { flex: 1; padding: 4px; }
.item-list button { padding: 4px 12px; cursor: pointer; }
.item-list ul { margin: 0; padding-left: 20px; }`

type ItemList struct{}

func init() {
	wprana.Register(
		"item-list",
		htmlContent,
		cssContent,
		func() wprana.PranaMod { return &ItemList{} },
	)
}

func (w *ItemList) InitData() map[string]any {
	return map[string]any{
		"input_val": "",
		"items":     []any{},
	}
}

func (w *ItemList) Render(obj *wprana.PranaObj) {
	forms := dom.Query(obj.Dom, "#il-form")
	if len(forms) == 0 {
		return
	}
	dom.AddEvent(forms[0], "submit",
		func(this js.Value, args []js.Value) any {
			val, _ := obj.This.Get("input_val").(string)
			if val == "" {
				return nil
			}
			obj.This.Append("items", val)
			obj.This.Set("input_val", "")
			return nil
		}, true, false)
}
