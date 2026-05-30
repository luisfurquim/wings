//go:build js && wasm

// Package light provides the baseline "light" wings skin.
//
// The skin defines every --wings-* token documented in skins/tokens.md
// at :root, using values extracted from the current widget defaults so
// existing UIs retain their look once the global skin is in effect.
//
// # Activation
//
//	import (
//	    "github.com/luisfurquim/wings"
//	    _ "github.com/luisfurquim/wings/skins/light"
//	)
//
//	func main() {
//	    wings.ApplySkin("light")
//	    wings.Main()
//	}
//
// The blank import drives init() to register the skin; ApplySkin then
// injects the CSS into <style id="wings-skin"> in document.head.
package light

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed skin.css
var css string

func init() {
	wings.RegisterSkin("light", wings.IdentitySkinCategories, css)
}
