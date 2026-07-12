//go:build js && wasm

// Package dialog provides a w-dialog custom element for wings.
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
// The buttons attribute is the authoritative button set: only the listed IDs
// are shown, in listed order; any unlisted ID is hidden. Supported button IDs:
// save, discard, cancel, overwrite. Omitting the attribute keeps the default
// save,discard,cancel.
//
// Dialog visibility (the whole element) is controlled by the parent through a
// conditional render (e.g. `?show_dialog`). The widget itself has no internal
// hidden flag.
//
// # Events fired to parent
//
//	@save      — Save button clicked
//	@discard   — Discard button clicked
//	@cancel    — Cancel button clicked
//	@overwrite — Overwrite button clicked
//
// # CSS Customization
//
// Dialog implements wings.Customizable. CSS is split into two parts:
//   - "Vars"   — CSS custom properties (colors, shadows).
//   - "Design" — Layout and structure rules.
package dialog

import (
	_ "embed"
	"fmt"
	"strings"
	"syscall/js"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
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
var cssParts = []wings.CSSPart{
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
	wings.Register(
		elementTag,
		htmlContent,
		buildCSS(),
		func() wings.PranaMod { return &Dialog{} },
		"buttons", "title",
	)
	G.Logf(3, "w-dialog: module registered\n")
}

// Dialog implements wings.PranaMod and wings.Customizable
// for the w-dialog custom element.
type Dialog struct {
	obj *wings.PranaObj
}

// Compile-time interface check.
var _ wings.Customizable = (*Dialog)(nil)

// ListCSS returns the named CSS parts in order.
func (d *Dialog) ListCSS() []wings.CSSPart {
	result := make([]wings.CSSPart, len(cssParts))
	copy(result, cssParts)
	return result
}

// ReplaceCSS replaces the CSS part identified by key and updates
// all live instances via wings.Update.
func (d *Dialog) ReplaceCSS(key string, content string) {
	for i := range cssParts {
		if cssParts[i].Name == key {
			cssParts[i].Content = content
			wings.Update(elementTag, buildCSS())
			return
		}
	}
	G.Logf(1, "ReplaceCSS: key %q not found\n", key)
}

func (d *Dialog) InitData() map[string]any {
	return map[string]any{
		"buttons":           "save,discard,cancel",
		"title":             "",
		"btnSaveOrder":      1,
		"btnDiscardOrder":   2,
		"btnCancelOrder":    3,
		"btnOverwriteOrder": 0,
		// Show flags live in `?…` attribute NAMES, which the HTML parser
		// lowercases — so they MUST be snake_case (not camelCase) to match the
		// data key at cond-eval time. (Value bindings like {{btnSaveOrder}} are
		// immune; only attribute-NAME identifiers are lowercased.)
		"btn_save_show":      true,
		"btn_discard_show":   true,
		"btn_cancel_show":    true,
		"btn_overwrite_show": false,
	}
}

func (d *Dialog) parseButtons() {
	buttonsAttr, ok := d.obj.This.Get("buttons").(string)
	if !ok || buttonsAttr == "" {
		return // no attribute → keep the InitData defaults
	}

	// The attribute is the authoritative set: a button is shown only if listed,
	// in listed order. Unlisted buttons get show=false (and order 0, unused).
	show := map[string]bool{}
	order := map[string]int{}
	for i, btn := range strings.Split(buttonsAttr, ",") {
		btn = strings.TrimSpace(btn)
		show[btn] = true
		order[btn] = i + 1
	}

	// Push the show flags and order into the bound data (synced to the DOM).
	// Show flags use snake_case keys to survive HTML attribute-name lowercasing
	// (they back `?btn_*_show` conditionals); order keys stay camelCase because
	// they are read from attribute VALUES ({{btnSaveOrder}}), which are not.
	d.obj.This.Set("btn_save_show", show["save"])
	d.obj.This.Set("btn_discard_show", show["discard"])
	d.obj.This.Set("btn_cancel_show", show["cancel"])
	d.obj.This.Set("btn_overwrite_show", show["overwrite"])
	d.obj.This.Set("btnSaveOrder", order["save"])
	d.obj.This.Set("btnDiscardOrder", order["discard"])
	d.obj.This.Set("btnCancelOrder", order["cancel"])
	d.obj.This.Set("btnOverwriteOrder", order["overwrite"])
}

func (d *Dialog) Render(obj *wings.PranaObj) {
	d.obj = obj
	d.parseButtons()
	d.bindButton("#dlg-save", "save")
	d.bindButton("#dlg-discard", "discard")
	d.bindButton("#dlg-overwrite", "overwrite")
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

// AnchorTo switches dlg (a <w-dialog> element, mounted anywhere in the
// document) to anchored mode: positioned over anchor's document
// rectangle, its overlay dimming only that rectangle and its box filling
// it minus --wings-dialog-anchor-inset (default 1% per edge) — the
// visual statement that THIS element is what the dialog configures.
// Absolute document coordinates make it scroll with the page; the
// returned release removes the window resize listener that keeps it
// glued across layout changes, and must be called when the dialog goes
// away (it is not under the anchor's subtree, so the runtime's
// auto-release cannot find it).
func AnchorTo(dlg, anchor js.Value) (release func()) {
	place := func() {
		rect := anchor.Call("getBoundingClientRect")
		win := js.Global()
		top := rect.Get("top").Float() + win.Get("scrollY").Float()
		left := rect.Get("left").Float() + win.Get("scrollX").Float()
		style := dlg.Get("style")
		style.Set("top", fmt.Sprintf("%.0fpx", top))
		style.Set("left", fmt.Sprintf("%.0fpx", left))
		style.Set("width", fmt.Sprintf("%.0fpx", rect.Get("width").Float()))
		style.Set("height", fmt.Sprintf("%.0fpx", rect.Get("height").Float()))
	}
	dlg.Call("setAttribute", "anchored", "")
	place()
	id := dom.AddEvent(js.Global(), "resize", func(_ js.Value, _ []js.Value) any {
		place()
		return nil
	}, false, false)
	return func() { dom.RmEvent(id) }
}
