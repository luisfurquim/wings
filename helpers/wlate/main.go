//go:build js && wasm

package main

import (
	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/wi18n"

	// Identity skins
	_ "github.com/luisfurquim/wings/skins/autumn"
	_ "github.com/luisfurquim/wings/skins/dark"
	_ "github.com/luisfurquim/wings/skins/darkblueberry"
	_ "github.com/luisfurquim/wings/skins/darkforest"
	_ "github.com/luisfurquim/wings/skins/light"
	_ "github.com/luisfurquim/wings/skins/lightblueberry"
	_ "github.com/luisfurquim/wings/skins/mushroom"
	_ "github.com/luisfurquim/wings/skins/vividforest"

	// Geometry skins
	_ "github.com/luisfurquim/wings/skins/classic"
	_ "github.com/luisfurquim/wings/skins/sharp"
	_ "github.com/luisfurquim/wings/skins/soft"

	// Depth skins
	_ "github.com/luisfurquim/wings/skins/flat"
	_ "github.com/luisfurquim/wings/skins/floating"
	_ "github.com/luisfurquim/wings/skins/lifted"

	// Motion skins
	_ "github.com/luisfurquim/wings/skins/brisk"
	_ "github.com/luisfurquim/wings/skins/calm"
	_ "github.com/luisfurquim/wings/skins/gentle"

	// Atmosphere
	_ "github.com/luisfurquim/wings/skins/glass"

	_ "github.com/luisfurquim/wings/widget/combobox"
	_ "github.com/luisfurquim/wings/widget/dialog"
	_ "github.com/luisfurquim/wings/widget/navbar"
	_ "github.com/luisfurquim/wings/widget/skinswitcher"

	_ "wlate/mod/flexeditor"
	_ "wlate/mod/texteditor"
	_ "wlate/mod/wlate"
)

func init() {
	wi18n.SetBasePath("wlate-i18n/")
}

func main() {
	for _, name := range []string{"light", "classic", "lifted", "calm"} {
		if err := wings.ApplySkin(name); err != nil {
			panic(err)
		}
	}
	wings.Main()
}
