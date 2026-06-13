//go:build js && wasm

package appshell

import (
	_ "embed"
	"errors"

	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/wi18n"
)

//go:embed appshell.i18n.html
var htmlContent string

//go:embed appshell.css
var cssContent string

type AppShell struct{}

func init() {
	wings.Register(
		"app-shell",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &AppShell{} },
	)
}

func (w *AppShell) InitData() map[string]any {
	return map[string]any{
		"show_locale_error":  false,
		"locale_error_sig":   false,
		"locale_error_other": false,
		"fnLocaleError":      wings.TriggerHandler(nil),
		"fnLocaleErrorClose": wings.TriggerHandler(nil),
	}
}

func (w *AppShell) Render(obj *wings.PranaObj) {
	obj.This.Set("fnLocaleError", wings.TriggerHandler(func(args ...any) {
		// Branch the dialog message on the typed signature error; everything
		// else (missing catalog, parse failure) gets the generic one.
		var sigErr *wi18n.CatalogSignatureError
		isSig := false
		if len(args) > 0 {
			if err, ok := args[0].(error); ok {
				isSig = errors.As(err, &sigErr)
			}
		}
		obj.This.Set("locale_error_sig", isSig)
		obj.This.Set("locale_error_other", !isSig)
		obj.This.Set("show_locale_error", true)
	}))
	obj.This.Set("fnLocaleErrorClose", wings.TriggerHandler(func(args ...any) {
		obj.This.Set("show_locale_error", false)
	}))
}
