//go:build js && wasm

package main

import (
	"github.com/luisfurquim/wprana"

	// Side-effect i18n: init() detects browser language, fetches JSON
	// catalogs (text + inflections), and installs Printer + SynPrinter.
	_ "github.com/luisfurquim/wprana/wi18n"

	// Side-effect: registers the <flex-widget> custom element.
	_ "live-demo-flex/mod/flexwidget"
)

func main() {
	wprana.Main()
}
