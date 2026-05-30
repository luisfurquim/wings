//go:build js && wasm

package modetoggle

import (
	_ "embed"
	"syscall/js"

	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
)

//go:embed modetoggle.i18n.html
var htmlContent string

const cssContent = `.mode-toggle .row { display: flex; gap: 8px; align-items: center; margin: 4px 0; }
.mode-toggle button { padding: 4px 12px; cursor: pointer; background: var(--wings-btn-bg, #fff); color: var(--wings-text, #222); border: 1px solid var(--wings-border, #ccc); border-radius: 4px; transition: background 0.15s; }
.mode-toggle button:hover { background: var(--wings-btn-hover-bg, #f5f5f5); color: var(--wings-btn-hover-color, #222); box-shadow: var(--wings-btn-hover-shadow, 0 2px 4px rgba(0,0,0,.1)); }
.mode-toggle .extra-box { background: var(--wings-primary-pale, #eef9ff); padding: 6px 10px; border-radius: 4px; margin: 6px 0; }
.mode-toggle .extra-hint { color: var(--wings-text-light, #888); font-style: italic; margin: 6px 0; }
.mode-toggle .cond { color: var(--wings-text, #444); font-size: 0.9rem; margin: 2px 0; }
.mode-toggle p { margin: 0; }`

type ModeToggle struct{}

func init() {
	wings.Register(
		"mode-toggle",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &ModeToggle{} },
	)
}

func (w *ModeToggle) InitData() map[string]any {
	return map[string]any{
		"mode":       "edit",
		"show_extra": false,
	}
}

func (w *ModeToggle) Render(obj *wings.PranaObj) {
	if btns := dom.Query(obj.Dom, "#mt-mode"); len(btns) > 0 {
		dom.AddEvent(btns[0], "click",
			func(this js.Value, args []js.Value) any {
				mode, _ := obj.This.Get("mode").(string)
				if mode == "edit" {
					obj.This.Set("mode", "readonly")
				} else {
					obj.This.Set("mode", "edit")
				}
				return nil
			}, false, false)
	}
	if btns := dom.Query(obj.Dom, "#mt-extra"); len(btns) > 0 {
		dom.AddEvent(btns[0], "click",
			func(this js.Value, args []js.Value) any {
				show, _ := obj.This.Get("show_extra").(bool)
				obj.This.Set("show_extra", !show)
				return nil
			}, false, false)
	}
}
