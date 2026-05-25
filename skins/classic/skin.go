//go:build js && wasm

// Package classic provides the default Geometry/Spacing skin: balanced
// corner radius and inner spacing, matching the values previously
// hard-coded in the eight identity themes.
package classic

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed skin.css
var css string

func init() {
	wprana.RegisterSkin("classic", wprana.GeometrySkinCategories, css)
}
