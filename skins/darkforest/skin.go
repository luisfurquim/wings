//go:build js && wasm

// Package darkforest provides the "darkforest" wings skin.
package darkforest

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed skin.css
var css string

func init() {
	wings.RegisterSkin("darkforest", wings.IdentitySkinCategories, css)
}
