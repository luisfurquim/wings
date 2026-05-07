//go:build js && wasm

// Package mushroom provides the "mushroom" wprana skin.
package mushroom

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed skin.css
var css string

func init() {
	wprana.RegisterSkin("mushroom", css)
}
