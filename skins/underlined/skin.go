//go:build js && wasm

// Package underlined provides a wings Material skin that applies the line-only
// form to fields: bottom border only, transparent background, no horizontal
// padding.  Composes with any Identity, Geometry, Depth, Motion, or Atmosphere
// skin.
package underlined

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed skin.css
var css string

// CSS is the raw payload of this skin, exported for use by bundles.
var CSS string

// Categories is the bitmask declared by this skin, exported for use by bundles.
var Categories = wings.MaterialSkinCategories

func init() {
	CSS = css
	wings.RegisterSkin("underlined", Categories, CSS)
}
