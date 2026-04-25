//go:build js && wasm

package main

import (
	"github.com/luisfurquim/wprana"

	_ "github.com/luisfurquim/wprana/wi18n"

	_ "live-demo/mod/shell/appshell"
	_ "live-demo/mod/shell/localeswitcher"
	_ "live-demo/mod/tabs/basics"
	_ "live-demo/mod/tabs/flex"
	_ "live-demo/mod/tabs/fmt"
	_ "live-demo/mod/tabs/i18n"
)

func main() {
	wprana.Main()
}
