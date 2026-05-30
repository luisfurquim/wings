//go:build js && wasm

// Package soft provides a focused wings skin covering only Geometry
// and Spacing: generous corner radius, airy padding. Composes with any
// Identity / Depth / Motion skin.
package soft

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed skin.css
var css string

func init() {
	wings.RegisterSkin("soft", wings.GeometrySkinCategories, css)
}
