//go:build js && wasm

// Package mushroom provides the "mushroom" wings skin.
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
