//go:build js && wasm

// Package darkforest provides the "darkforest" wprana skin.
package darkforest

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed skin.css
var css string

func init() {
	wprana.RegisterSkin("darkforest", css)
}
