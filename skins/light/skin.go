//go:build js && wasm

// Package light provides the baseline "light" wprana skin.
//
// The skin defines every --wings-* token documented in skins/tokens.md
// at :root, using values extracted from the current widget defaults so
// existing UIs retain their look once the global skin is in effect.
//
// # Activation
//
//	import (
//	    "github.com/luisfurquim/wprana"
//	    _ "github.com/luisfurquim/wprana/skins/light"
//	)
//
//	func main() {
//	    wprana.ApplySkin("light")
//	    wprana.Main()
//	}
//
// The blank import drives init() to register the skin; ApplySkin then
// injects the CSS into <style id="wprana-skin"> in document.head.
package light

import (
	_ "embed"

	"github.com/luisfurquim/wprana"
)

//go:embed skin.css
var css string

func init() {
	wprana.RegisterSkin("light", wprana.IdentitySkinCategories, css)
}
