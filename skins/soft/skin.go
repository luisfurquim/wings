//go:build js && wasm

// Package soft provides a focused wprana skin covering only Geometry
// and Spacing: generous corner radius, airy padding. Composes with any
// Identity / Depth / Motion skin.
package soft

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed skin.css
var css string

func init() {
	wprana.RegisterSkin("soft", wprana.GeometrySkinCategories, css)
}
