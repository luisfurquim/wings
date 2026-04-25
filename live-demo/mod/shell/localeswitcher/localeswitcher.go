//go:build js && wasm

package localeswitcher

import (
	_ "embed"
	"syscall/js"

	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/dom"
	"github.com/luisfurquim/wprana/wi18n"
)

//go:embed localeswitcher.i18n.html
var htmlContent string

//go:embed localeswitcher.css
var cssContent string

type LocaleSwitcher struct{}

func init() {
	wprana.Register(
		"locale-switcher",
		htmlContent,
		cssContent,
		func() wprana.PranaMod { return &LocaleSwitcher{} },
	)
}

func (w *LocaleSwitcher) InitData() map[string]any {
	return map[string]any{}
}

func (w *LocaleSwitcher) Render(obj *wprana.PranaObj) {
	sels := dom.Query(obj.Dom, "#lsw-sel")
	if len(sels) == 0 {
		return
	}
	sel := sels[0]
	sel.Set("value", wprana.Locale)
	dom.AddEvent(sel, "change",
		func(this js.Value, args []js.Value) any {
			next := sel.Get("value").String()
			wi18n.SetLang(next, func(err error) {
				if err != nil {
					wprana.G.Logf(1, "locale-switcher: SetLang(%q) error: %v\n", next, err)
				}
			})
			return nil
		}, false, false)
}
