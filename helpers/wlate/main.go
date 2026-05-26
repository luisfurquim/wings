//go:build js && wasm

package main

import (
	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/wi18n"

	// Identity skins
	_ "github.com/luisfurquim/wprana/skins/autumn"
	_ "github.com/luisfurquim/wprana/skins/dark"
	_ "github.com/luisfurquim/wprana/skins/darkblueberry"
	_ "github.com/luisfurquim/wprana/skins/darkforest"
	_ "github.com/luisfurquim/wprana/skins/light"
	_ "github.com/luisfurquim/wprana/skins/lightblueberry"
	_ "github.com/luisfurquim/wprana/skins/mushroom"
	_ "github.com/luisfurquim/wprana/skins/vividforest"

	// Geometry skins
	_ "github.com/luisfurquim/wprana/skins/classic"
	_ "github.com/luisfurquim/wprana/skins/sharp"
	_ "github.com/luisfurquim/wprana/skins/soft"

	// Depth skins
	_ "github.com/luisfurquim/wprana/skins/flat"
	_ "github.com/luisfurquim/wprana/skins/floating"
	_ "github.com/luisfurquim/wprana/skins/lifted"

	// Motion skins
	_ "github.com/luisfurquim/wprana/skins/brisk"
	_ "github.com/luisfurquim/wprana/skins/calm"
	_ "github.com/luisfurquim/wprana/skins/gentle"

	// Atmosphere
	_ "github.com/luisfurquim/wprana/skins/glass"

	_ "github.com/luisfurquim/wprana/widget/combobox"
	_ "github.com/luisfurquim/wprana/widget/dialog"
	_ "github.com/luisfurquim/wprana/widget/navbar"
	_ "github.com/luisfurquim/wprana/widget/skinswitcher"

	_ "wlate/mod/flexeditor"
	_ "wlate/mod/texteditor"
	_ "wlate/mod/wlate"
)

func init() {
	wi18n.SetBasePath("wlate-i18n/")
}

func main() {
	for _, name := range []string{"light", "classic", "lifted", "calm"} {
		if err := wprana.ApplySkin(name); err != nil {
			panic(err)
		}
	}
	wprana.Main()
}
