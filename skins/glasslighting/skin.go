//go:build js && wasm

// Package glasslighting provides the Lighting complement for glass-morphism:
// a subtle diagonal surface gradient and a diffused drop shadow.
//
// It is designed to compose with the [glass] Atmosphere skin.  The
// [glassmorphism] bundle applies both automatically; import this package
// directly only when you want the gradient/glow without the backdrop-filter
// blur (e.g. browsers or environments that do not support it).
package glasslighting

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed skin.css
var css string

// CSS is the raw payload of this skin, exported for use by bundles.
var CSS string

// Categories is the bitmask declared by this skin, exported for use by bundles.
var Categories = wings.CategoryLighting

func init() {
	CSS = css
	wings.RegisterSkin("glasslighting", Categories, CSS)
}
