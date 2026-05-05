//go:build js && wasm

package main

import (
	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/wi18n"

	_ "github.com/luisfurquim/wprana/skins/dark"
	_ "github.com/luisfurquim/wprana/skins/light"

	_ "github.com/luisfurquim/wprana/widget/combobox"
	_ "github.com/luisfurquim/wprana/widget/dialog"
	_ "github.com/luisfurquim/wprana/widget/navbar"
	_ "github.com/luisfurquim/wprana/widget/skinswitcher"

	_ "wlate/mod/wlate"
)

func init() {
	wi18n.SetBasePath("wlate-i18n/")
}

func main() {
	wprana.ApplySkin("light")
	wprana.Main()
}
