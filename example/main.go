//go:build js && wasm

// Package main is the WASM binary entry point.
// It imports all modules (side-effect imports to trigger their init())
// and then calls wings.Main() which defines the custom elements and blocks.
package main

import (
	"github.com/luisfurquim/wings"

	// Side-effect imports: each init() registers the module via wings.Register()
	_ "github.com/luisfurquim/wings/example/mywidget"
)

func main() {
	// Define all registered custom elements and keep the WASM alive.
	wings.Main()
}
