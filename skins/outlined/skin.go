//go:build js && wasm

// Package outlined provides a wings Material skin that explicitly selects the
// default outlined form for fields: full border on all four sides, rounded
// corners, opaque background. This is the widget default when no Material skin
// is active; registering it is useful to lock the form and prevent a
// conflicting Material skin (filled, underlined) from being applied.
package outlined

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
	wings.RegisterSkin("outlined", Categories, CSS)
}
