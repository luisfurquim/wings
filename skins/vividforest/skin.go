//go:build js && wasm

// Package vividforest provides the "vividforest" wprana skin.
package vividforest

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed skin.css
var css string

func init() {
	wprana.RegisterSkin("vividforest", wprana.IdentitySkinCategories, css)
}
