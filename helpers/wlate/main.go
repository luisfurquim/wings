//go:build js && wasm

package main

import (
	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/wi18n"

	_ "github.com/luisfurquim/wprana/widget/combobox"

	_ "wlate/mod/wlate"
)

func init() {
	wi18n.SetBasePath("wlate-i18n/")
}

func main() {
	wprana.Main()
}
