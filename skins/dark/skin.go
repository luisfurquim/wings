//go:build js && wasm

// Package dark provides a dark wprana skin.
//
// Mirrors every --wings-* token documented in skins/tokens.md, swapping
// the surface and text values for dark-mode-friendly palette while
// keeping the same accent hues (blue primary, violet combobox).
//
// # Activation
//
//	import (
//	    "github.com/luisfurquim/wprana"
//	    _ "github.com/luisfurquim/wprana/skins/dark"
//	)
//
//	func main() {
//	    wprana.ApplySkin("dark")
//	    wprana.Main()
//	}
//
// Skins can be swapped at runtime with another ApplySkin call; the
// global <style id="wprana-skin"> element is replaced in place.
package dark

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed skin.css
var css string

func init() {
	wprana.RegisterSkin("dark", wprana.IdentitySkinCategories, css)
}
