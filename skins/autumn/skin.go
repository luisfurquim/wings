//go:build js && wasm

// Package autumn provides the "autumn" wprana skin.
package autumn

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed skin.css
var css string

func init() {
	wprana.RegisterSkin("autumn", css)
}
