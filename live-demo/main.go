//go:build js && wasm

package main

import (
	_ "embed"

	"github.com/luisfurquim/wings"

	"github.com/luisfurquim/wings/wi18n"

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
	_ "github.com/luisfurquim/wings/widget/test"
	_ "github.com/luisfurquim/wings/widget/testreport"

	_ "live-demo/mod/shell/appshell"
	_ "live-demo/mod/shell/localeswitcher"

	_ "live-demo/mod/tabs/basics"
	_ "live-demo/mod/tabs/basics/counterpair"
	_ "live-demo/mod/tabs/basics/itemlist"
	_ "live-demo/mod/tabs/basics/modetoggle"

	_ "live-demo/mod/tabs/flex"
	_ "live-demo/mod/tabs/flex/countinput"
	_ "live-demo/mod/tabs/flex/genderpicker"
	_ "live-demo/mod/tabs/flex/platformpicker"
	_ "live-demo/mod/tabs/flex/productpicker"

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

	_ "live-demo/mod/tabs/wtest"
)

// catalogPubKeyPEM is the ed25519 public key matching the keypair that
// build.sh uses to sign the published catalogs (see gen_i18n.ed25519.key).
// With a key configured, every <lang>.json must carry a valid .sig sidecar or
// SetLang rejects it — this dogfoods catalog signature verification on the
// static GitHub Pages deployment.
//
//go:embed gen_i18n.ed25519.pub
var catalogPubKeyPEM []byte

func main() {
	if err := wi18n.SetCatalogPublicKey(catalogPubKeyPEM); err != nil {
		panic(err)
	}

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
