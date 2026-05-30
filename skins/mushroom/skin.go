//go:build js && wasm

// Package mushroom provides the "mushroom" wprana skin.
package mushroom

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed skin.css
var css string

func init() {
	wings.RegisterSkin("mushroom", wings.IdentitySkinCategories, css)
}
