//go:build js && wasm

// Package darkblueberry provides the "darkblueberry" wings skin.
package darkblueberry

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed skin.css
var css string

func init() {
	wings.RegisterSkin("darkblueberry", wings.IdentitySkinCategories, css)
}
