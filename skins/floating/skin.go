//go:build js && wasm

// Package floating provides a focused wprana Depth skin with diffuse,
// generous shadow geometry — pairs naturally with `soft` for an
// "elevated cards" aesthetic.
package floating

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed skin.css
var css string

func init() {
	wings.RegisterSkin("floating", wings.DepthSkinCategories, css)
}
