//go:build js && wasm

package main

import (
	"github.com/luisfurquim/wprana"

	// Side-effect i18n: init() detects browser language, fetches the CSV,
	// and overrides wprana.Printer so TextNodes get translated. Remove this
	// import to revert to the raw (untranslated) build.
	_ "github.com/luisfurquim/wprana/wi18n"

	// Side-effect imports: each init() registers the module via wprana.Register()
	_ "live-demo/mod/mywidget"
)

func main() {
	wprana.Main()
}
