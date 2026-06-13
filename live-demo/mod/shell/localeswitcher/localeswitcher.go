//go:build js && wasm

package localeswitcher

import (
	_ "embed"
	"syscall/js"

	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
	"github.com/luisfurquim/wings/wi18n"
)

//go:embed localeswitcher.i18n.html
var htmlContent string

//go:embed localeswitcher.css
var cssContent string

type LocaleSwitcher struct{}

func init() {
	wings.Register(
		"locale-switcher",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &LocaleSwitcher{} },
	)
}

func (w *LocaleSwitcher) InitData() map[string]any {
	return map[string]any{}
}

func (w *LocaleSwitcher) Render(obj *wings.PranaObj) {
	sels := dom.Query(obj.Dom, "#lsw-sel")
	if len(sels) == 0 {
		return
	}
	sel := sels[0]
	sel.Set("value", wings.Locale)
	dom.AddEvent(sel, "change",
		func(this js.Value, args []js.Value) any {
			next := sel.Get("value").String()
			wi18n.SetLang(next, func(err error) {
				if err != nil {
					// The console log always fires; the @error trigger is
					// optional wiring for the hosting app (dialog, toast, …).
					wings.G.Logf(1, "locale-switcher: SetLang(%q) error: %v\n", next, err)
					sel.Set("value", wings.Locale) // the active locale did not change
					obj.Trigger("error", err)
				}
			})
			return nil
		}, false, false)
}
