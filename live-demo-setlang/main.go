//go:build js && wasm

package main

import (
	"github.com/luisfurquim/wprana"

	_ "github.com/luisfurquim/wprana/wi18n"

	_ "live-demo-setlang/mod/setlangwidget"
)

func main() {
	wprana.Main()
}
