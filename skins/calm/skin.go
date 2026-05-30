//go:build js && wasm

// Package calm is the default Motion skin: balanced transitions and
// hover lift, matching the values previously hard-coded in the eight
// identity themes.
package calm

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed skin.css
var css string

func init() {
	wings.RegisterSkin("calm", wings.MotionSkinCategories, css)
}
