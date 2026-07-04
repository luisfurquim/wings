//go:build js && wasm

// Package filled provides a wings Material skin that applies the filled form
// to fields: tinted background, bottom border only, rounded top corners.
// Composes with any Identity, Geometry, Depth, Motion, or Atmosphere skin.
package filled

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
	wings.RegisterSkin("filled", Categories, CSS)
}
