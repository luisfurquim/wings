//go:build js && wasm

// Package sharp provides a focused wprana skin covering only Geometry
// and Spacing: minimal radius, tight padding. Composes with any
// Identity / Depth / Motion skin.
package sharp

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed skin.css
var css string

func init() {
	wprana.RegisterSkin("sharp", wprana.GeometrySkinCategories, css)
}
