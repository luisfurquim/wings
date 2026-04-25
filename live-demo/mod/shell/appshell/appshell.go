//go:build js && wasm

package appshell

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed appshell.i18n.html
var htmlContent string

//go:embed appshell.css
var cssContent string

type AppShell struct{}

func init() {
	wprana.Register(
		"app-shell",
		htmlContent,
		cssContent,
		func() wprana.PranaMod { return &AppShell{} },
	)
}

func (w *AppShell) InitData() map[string]any {
	return map[string]any{}
}

func (w *AppShell) Render(obj *wprana.PranaObj) {}
