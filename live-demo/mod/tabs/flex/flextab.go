//go:build js && wasm

package flex

import (
	_ "embed"

	"github.com/luisfurquim/wings"
)

//go:embed flextab.i18n.html
var htmlContent string

const cssContent = `.flex-tab .controls { display: flex; gap: 18px; flex-wrap: wrap; margin: 8px 0 12px; }
.flex-tab .result { background: var(--wings-surface, #f7f7f7); padding: 10px 14px; border-radius: 4px; font-size: 1rem; }
.flex-tab .hint { font-size: 0.85rem; color: var(--wings-text-muted, #555); margin: 16px 0 4px; }
.flex-tab .hint-code { font-family: monospace; background: var(--wings-surface, #eee); padding: 4px 8px; border-radius: 3px; display: inline-block; margin-top: 0; }`

type FlexTab struct {
	flexer *RemoteFlexer
}

func init() {
	wings.Register(
		"flex-tab",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &FlexTab{} },
	)
}

func (w *FlexTab) InitData() map[string]any {
	w.flexer = NewRemoteFlexer()
	return map[string]any{
		"gender":     "m",
		"qt":         1,
		"produto":    "maçã",
		"flexer":     w.flexer,
		"setgender":  wings.TriggerHandler(nil),
		"setcount":   wings.TriggerHandler(nil),
		"setproduto": wings.TriggerHandler(nil),
	}
}

func (w *FlexTab) Render(obj *wings.PranaObj) {
	obj.This.M["setgender"] = wings.TriggerHandler(func(args ...any) {
		if len(args) == 0 {
			return
		}
		if s, ok := args[0].(string); ok {
			obj.This.Set("gender", s)
		}
	})
	obj.This.M["setcount"] = wings.TriggerHandler(func(args ...any) {
		if len(args) == 0 {
			return
		}
		if n, ok := args[0].(int); ok {
			obj.This.Set("qt", n)
		}
	})
	obj.This.M["setproduto"] = wings.TriggerHandler(func(args ...any) {
		if len(args) == 0 {
			return
		}
		if s, ok := args[0].(string); ok {
			obj.This.Set("produto", s)
		}
	})

	// Re-sync trigger for the async engine: re-set `produto` to its current
	// value, which re-runs the {{… *flexer ~$produto}} block (now a cache hit).
	// This is exactly the obj.This.Set() pattern an app developer would use.
	w.flexer.SetNotify(func() {
		obj.This.Set("produto", obj.This.M["produto"])
	})
}
