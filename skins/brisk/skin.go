//go:build js && wasm

// Package brisk provides a fast, expressive Motion skin: short
// transitions, pronounced hover lift, firm click feedback. Pairs well
// with `sharp + flat` for an industrial/utility tone.
package brisk

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed skin.css
var css string

func init() {
	wprana.RegisterSkin("brisk", wprana.MotionSkinCategories, css)
}
