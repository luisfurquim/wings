//go:build js && wasm

// Package classic provides the default Geometry/Spacing skin: balanced
// corner radius and inner spacing, matching the values previously
// hard-coded in the eight identity themes.
package classic

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed skin.css
var css string

func init() {
	wings.RegisterSkin("classic", wings.GeometrySkinCategories, css)
}
