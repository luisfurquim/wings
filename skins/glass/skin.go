//go:build js && wasm

// Package glass provides a focused wings skin covering only the
// CategoryAtmosphere dimension — glass-morphism via backdrop-filter blur
// and translucent surface alpha.
//
// Unlike the chromatic theme skins (which all declare
// IdentitySkinCategories and are mutually exclusive), glass touches only
// Atmosphere, so it composes with any of them:
//
//	_ = wings.ApplySkin("mushroom") // colors + geometry + …
//	_ = wings.ApplySkin("glass")    // adds atmospheric blur on top
//
// Widgets opt into the effect by using the documented tokens with a
// `0` fallback, so the skin is invisible until a widget references it.
//
// Currently consumed by `w-dialog` and `w-combobox` (dropdown panel).
package glass

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed skin.css
var css string

// CSS is the raw payload of this skin, exported for use by bundles.
var CSS string

// Categories is the bitmask declared by this skin, exported for use by bundles.
var Categories = wings.CategoryAtmosphere

func init() {
	CSS = css
	wings.RegisterSkin("glass", Categories, CSS)
}
