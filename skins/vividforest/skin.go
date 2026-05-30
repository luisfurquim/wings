//go:build js && wasm

// Package vividforest provides the "vividforest" wprana skin.
package vividforest

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed skin.css
var css string

func init() {
	wings.RegisterSkin("vividforest", wings.IdentitySkinCategories, css)
}
