//go:build js && wasm

// Package autumn provides the "autumn" wprana skin.
package autumn

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed skin.css
var css string

func init() {
	wings.RegisterSkin("autumn", wings.IdentitySkinCategories, css)
}
