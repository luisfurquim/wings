//go:build js && wasm

// Package darkblueberry provides the "darkblueberry" wprana skin.
package darkblueberry

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed skin.css
var css string

func init() {
	wprana.RegisterSkin("darkblueberry", wprana.IdentitySkinCategories, css)
}
