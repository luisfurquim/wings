//go:build js && wasm

// Package floating provides a focused wprana Depth skin with diffuse,
// generous shadow geometry — pairs naturally with `soft` for an
// "elevated cards" aesthetic.
package floating

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed skin.css
var css string

func init() {
	wprana.RegisterSkin("floating", wprana.DepthSkinCategories, css)
}
