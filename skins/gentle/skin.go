//go:build js && wasm

// Package gentle provides a slow, restrained Motion skin: long
// transitions and minimal hover/active feedback. Useful when the UI
// must feel calm, or for users who prefer reduced motion.
package gentle

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed skin.css
var css string

func init() {
	wprana.RegisterSkin("gentle", wprana.MotionSkinCategories, css)
}
