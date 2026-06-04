//go:build js && wasm && wings_test

package countinput

import (
	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
)

// Testable declares count-input's own integration self-tests. It is compiled
// only under the wings_test build tag, so a normal build ships no test code.
// <w-test-report> discovers it per mounted instance and runs these checks.
func (w *CountInput) Testable() map[string]wings.CheckFunc {
	return map[string]wings.CheckFunc{
		"renders-number-input": func(ctx wings.CheckCtx) (bool, string) {
			root := ctx.Subject.Get("shadowRoot")
			if !root.Truthy() {
				return false, "no shadow root"
			}
			inputs := dom.Query(root, "#ci-inp")
			if len(inputs) == 0 {
				return false, "no #ci-inp rendered"
			}
			if got := inputs[0].Call("getAttribute", "type").String(); got != "number" {
				return false, "input type is " + got + ", want number"
			}
			return true, "number input present"
		},
	}
}
