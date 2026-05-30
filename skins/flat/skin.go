//go:build js && wasm

// Package flat provides a focused wprana Depth skin: minimal,
// near-flush shadow geometry. Combines with any Identity skin's
// shadow colour to produce a discreet elevation feel.
package flat

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed skin.css
var css string

func init() {
	wings.RegisterSkin("flat", wings.DepthSkinCategories, css)
}
