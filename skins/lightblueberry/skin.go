//go:build js && wasm

// Package lightblueberry provides the "lightblueberry" wings skin.
package lightblueberry

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed skin.css
var css string

func init() {
	wings.RegisterSkin("lightblueberry", wings.IdentitySkinCategories, css)
}
