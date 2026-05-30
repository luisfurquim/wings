//go:build js && wasm

// Package brisk provides a fast, expressive Motion skin: short
// transitions, pronounced hover lift, firm click feedback. Pairs well
// with `sharp + flat` for an industrial/utility tone.
package brisk

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed skin.css
var css string

func init() {
	wings.RegisterSkin("brisk", wings.MotionSkinCategories, css)
}
