//go:build js && wasm

package main

import (
	"github.com/luisfurquim/wings"

	_ "github.com/luisfurquim/wings/wi18n"

	// Identity skins (mutually exclusive)
	_ "github.com/luisfurquim/wings/skins/autumn"
	_ "github.com/luisfurquim/wings/skins/dark"
	_ "github.com/luisfurquim/wings/skins/darkblueberry"
	_ "github.com/luisfurquim/wings/skins/darkforest"
	_ "github.com/luisfurquim/wings/skins/light"
	_ "github.com/luisfurquim/wings/skins/lightblueberry"
	_ "github.com/luisfurquim/wings/skins/mushroom"
	_ "github.com/luisfurquim/wings/skins/vividforest"

	// Geometry skins (radius/border/padding/gap)
	_ "github.com/luisfurquim/wings/skins/classic"
	_ "github.com/luisfurquim/wings/skins/sharp"
	_ "github.com/luisfurquim/wings/skins/soft"

	// Depth skins (shadow shapes)
	_ "github.com/luisfurquim/wings/skins/flat"
	_ "github.com/luisfurquim/wings/skins/floating"
	_ "github.com/luisfurquim/wings/skins/lifted"

	// Motion skins (transitions / hover-lift / active-scale)
	_ "github.com/luisfurquim/wings/skins/brisk"
	_ "github.com/luisfurquim/wings/skins/calm"
	_ "github.com/luisfurquim/wings/skins/gentle"

	// Atmosphere skins (composes with all of the above)
	_ "github.com/luisfurquim/wings/skins/glass"

	_ "github.com/luisfurquim/wings/widget/combobox"
	_ "github.com/luisfurquim/wings/widget/skinswitcher"
	_ "github.com/luisfurquim/wings/widget/tab"
	_ "github.com/luisfurquim/wings/widget/tabbutton"
	_ "github.com/luisfurquim/wings/widget/tabs"

	_ "live-demo/mod/shell/appshell"
	_ "live-demo/mod/shell/localeswitcher"

	_ "live-demo/mod/tabs/basics"
	_ "live-demo/mod/tabs/basics/counterpair"
	_ "live-demo/mod/tabs/basics/itemlist"
	_ "live-demo/mod/tabs/basics/modetoggle"

	_ "live-demo/mod/tabs/flex"
	_ "live-demo/mod/tabs/flex/countinput"
	_ "live-demo/mod/tabs/flex/genderpicker"

	_ "live-demo/mod/tabs/fmt"
	_ "live-demo/mod/tabs/fmt/currencydisplay"
	_ "live-demo/mod/tabs/fmt/currencylist"
	_ "live-demo/mod/tabs/fmt/datedisplay"
	_ "live-demo/mod/tabs/fmt/numberdisplay"

	_ "live-demo/mod/tabs/i18n"

	_ "live-demo/mod/tabs/measures"
	_ "live-demo/mod/tabs/measures/lengthdisplay"
	_ "live-demo/mod/tabs/measures/speeddisplay"
	_ "live-demo/mod/tabs/measures/tempdisplay"

	_ "live-demo/mod/tabs/tabsdemo"
)

func main() {
	// One skin per orthogonal family. Categories are disjoint so all four
	// can stack: Identity (light) + Geometry/Spacing (classic) +
	// Depth (lifted) + Motion (calm).
	for _, name := range []string{"light", "classic", "lifted", "calm"} {
		if err := wings.ApplySkin(name); err != nil {
			panic(err)
		}
	}
	wings.Main()
}
