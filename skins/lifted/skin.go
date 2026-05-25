//go:build js && wasm

// Package lifted is the default Depth skin: balanced shadow geometry
// matching the values previously hard-coded in the eight identity
// themes. Composes with any Identity skin's shadow colours.
package lifted

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed skin.css
var css string

func init() {
	wprana.RegisterSkin("lifted", wprana.DepthSkinCategories, css)
}
