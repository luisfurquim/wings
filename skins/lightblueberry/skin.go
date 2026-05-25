//go:build js && wasm

// Package lightblueberry provides the "lightblueberry" wprana skin.
package lightblueberry

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed skin.css
var css string

func init() {
	wprana.RegisterSkin("lightblueberry", wprana.IdentitySkinCategories, css)
}
