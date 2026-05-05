//go:build js && wasm

// Package dialog provides a w-dialog custom element for wprana.
//
// Features:
//   - Fixed button definitions with configurable order via buttons attribute
//   - i18n support via data-i18n attributes
//   - CSS-based reordering via flexbox order property
//   - Parent receives clicks via @save / @discard / @cancel triggers
//
// # Usage in parent template
//
//	<w-dialog ?show_dialog
//	          title="Confirmar ação"
//	          buttons="save,discard,cancel"
//	          @save="fnSave" @discard="fnDiscard" @cancel="fnCancel">
//	    Deseja salvar antes de continuar?
//	</w-dialog>
//
// The buttons attribute specifies which buttons to show and in what order.
// Supported button IDs: save, discard, cancel.
//
// Visibility is controlled by the parent through a conditional render
// (e.g. `?show_dialog`). The widget itself has no internal hidden flag.
//
// # Events fired to parent
//
//	@save    — Save button clicked
//	@discard — Discard button clicked
//	@cancel  — Cancel button clicked
//
// # CSS Customization
//
// Dialog implements wprana.Customizable. CSS is split into two parts:
//   - "Vars"   — CSS custom properties (colors, shadows).
//   - "Design" — Layout and structure rules.
package dialog

import (
	_ "embed"
	"strings"
	"syscall/js"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/dom"
)

const elementTag = "w-dialog"

// G is the logger for this module.
var G goose.Alert

//go:embed dialog.html
var htmlContent string

//go:embed vars.css
var varsCSS string

//go:embed design.css
var designCSS string

// cssParts holds the CSS sections; shared by all instances.
var cssParts = []wprana.CSSPart{
	{Name: "Vars", Content: ""},
	{Name: "Design", Content: ""},
}

// buildCSS concatenates all CSS parts in the defined order.
func buildCSS() string {
	var sb strings.Builder
	for _, p := range cssParts {
		sb.WriteString(p.Content)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// New creates a new Dialog instance.
func New() *Dialog {
	return &Dialog{}
}

func init() {
	G.Set(3)
	cssParts[0].Content = varsCSS
	cssParts[1].Content = designCSS
	wprana.Register(
		elementTag,
		htmlContent,
		buildCSS(),
		func() wprana.PranaMod { return &Dialog{} },
		"buttons", "title",
	)
	G.Logf(3, "w-dialog: module registered\n")
}

// Dialog implements wprana.PranaMod and wprana.Customizable
// for the w-dialog custom element.
type Dialog struct {
	obj *wprana.PranaObj
}

// Compile-time interface check.
var _ wprana.Customizable = (*Dialog)(nil)

// ListCSS returns the named CSS parts in order.
func (d *Dialog) ListCSS() []wprana.CSSPart {
	result := make([]wprana.CSSPart, len(cssParts))
	copy(result, cssParts)
	return result
}

// ReplaceCSS replaces the CSS part identified by key and updates
// all live instances via wprana.Update.
func (d *Dialog) ReplaceCSS(key string, content string) {
	for i := range cssParts {
		if cssParts[i].Name == key {
			cssParts[i].Content = content
			wprana.Update(elementTag, buildCSS())
			return
		}
	}
	G.Logf(1, "ReplaceCSS: key %q not found\n", key)
}

func (d *Dialog) InitData() map[string]any {
	return map[string]any{
		"buttons":         "save,discard,cancel",
		"title":           "",
		"btnSaveOrder":    1,
		"btnDiscardOrder": 2,
		"btnCancelOrder":  3,
	}
}

func (d *Dialog) parseButtons() {
	buttonsAttr, ok := d.obj.This.Get("buttons").(string)
	if !ok || buttonsAttr == "" {
		return
	}

	// Default order
	buttonOrder := map[string]int{
		"save":    1,
		"discard": 2,
		"cancel":  3,
	}

	// Parse buttons attribute and reorder
	buttons := strings.Split(buttonsAttr, ",")
	for i, btn := range buttons {
		btn = strings.TrimSpace(btn)
		buttonOrder[btn] = i + 1
	}

	// Update the order variables (data binding will sync to DOM)
	d.obj.This.Set("btnSaveOrder", buttonOrder["save"])
	d.obj.This.Set("btnDiscardOrder", buttonOrder["discard"])
	d.obj.This.Set("btnCancelOrder", buttonOrder["cancel"])
}

func (d *Dialog) Render(obj *wprana.PranaObj) {
	d.obj = obj
	d.parseButtons()
	d.bindButton("#dlg-save", "save")
	d.bindButton("#dlg-discard", "discard")
	d.bindButton("#dlg-cancel", "cancel")
}

func (d *Dialog) bindButton(selector, event string) {
	btns := dom.Query(d.obj.Dom, selector)
	if len(btns) == 0 {
		return
	}
	dom.AddEvent(btns[0], "click", func(_ js.Value, _ []js.Value) any {
		d.obj.Trigger(event)
		return nil
	}, false, false)
}
