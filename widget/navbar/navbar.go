//go:build js && wasm

// Package navbar provides a w-navbar custom element for wings.
//
// It renders a record-navigation toolbar — first / prev-many / prev /
// position input / next / next-many / last — and forwards user actions
// to the parent module as triggers. The widget keeps no internal state:
// the current position and total count are owned by the parent through
// the bound data fields.
//
// # Usage in parent template
//
//	<w-navbar
//	    nav_input="{{cur_record}}"
//	    total_count="{{record_count}}"
//	    @first="goFirst"
//	    @prevmany="goPrevPage"
//	    @prev="goPrev"
//	    @next="goNext"
//	    @nextmany="goNextPage"
//	    @last="goLast"
//	    @change="onPositionEdited">
//	</w-navbar>
//
// The input uses wprana two-way binding (&value), so typing into it
// updates nav_input on the widget; the @change trigger then fires with
// the new value so the parent can react (e.g. seek to that record).
// Pressing Enter while focused on the input also fires @change without
// waiting for blur.
//
// # Events fired to parent
//
//	@first    — jump to first record
//	@prevmany — page back (e.g. -10)
//	@prev     — previous record
//	@next     — next record
//	@nextmany — page forward (e.g. +10)
//	@last     — jump to last record
//	@change   — position input edited; args[0] = new nav_input value
//
// # CSS Customization
//
// NavBar implements wings.Customizable. CSS is split into two parts:
//   - "Vars"   — CSS custom properties (colors, shadows).
//   - "Design" — Layout and structure rules.
package navbar

import (
	_ "embed"
	"strings"
	"syscall/js"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
)

const elementTag = "w-navbar"

// G is the logger for this module.
var G goose.Alert

//go:embed navbar.html
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

// New creates a new NavBar instance.
func New() *NavBar {
	return &NavBar{}
}

func init() {
	G.Set(3)
	cssParts[0].Content = varsCSS
	cssParts[1].Content = designCSS
	wings.Register(
		elementTag,
		htmlContent,
		buildCSS(),
		func() wings.PranaMod { return &NavBar{} },
		"nav_input", "total_count",
	)
	G.Logf(3, "w-navbar: module registered")
}

// NavBar implements wings.PranaMod and wings.Customizable
// for the w-navbar custom element.
type NavBar struct {
	obj *wings.PranaObj
}

// Compile-time interface check.
var _ wings.Customizable = (*NavBar)(nil)

// ListCSS returns the named CSS parts in order.
func (nb *NavBar) ListCSS() []wings.CSSPart {
	result := make([]wings.CSSPart, len(cssParts))
	copy(result, cssParts)
	return result
}

// ReplaceCSS replaces the CSS part identified by key and updates
// all live instances via wings.Update.
func (nb *NavBar) ReplaceCSS(key string, content string) {
	for i := range cssParts {
		if cssParts[i].Name == key {
			cssParts[i].Content = content
			wings.Update(elementTag, buildCSS())
			return
		}
	}
	G.Logf(1, "ReplaceCSS: key %q not found\n", key)
}

func (nb *NavBar) InitData() map[string]any {
	return map[string]any{
		"nav_input":   "",
		"total_count": 0,
	}
}

func (nb *NavBar) Render(obj *wings.PranaObj) {
	nb.obj = obj
	for _, button := range []string{"first", "prevmany", "prev", "next", "nextmany", "last"} {
		nb.bindButton("#nav-"+button, button)
	}

	inp := dom.Query(nb.obj.Dom, "#nav-input")
	if len(inp) == 0 {
		return
	}
	dom.AddEvent(inp[0], "change", func(_ js.Value, _ []js.Value) any {
		nb.obj.Trigger("change", nb.obj.This.Get("nav_input"))
		return nil
	}, false, false)
	// Enter fires @change immediately, without waiting for blur.
	dom.AddEvent(inp[0], "keydown", func(_ js.Value, args []js.Value) any {
		if len(args) == 0 || args[0].Get("key").String() != "Enter" {
			return nil
		}
		args[0].Call("preventDefault")
		// Sync the typed value into the bound data before triggering, in
		// case the &value binding has not yet observed it (Enter precedes
		// the native change event).
		nb.obj.This.Set("nav_input", inp[0].Get("value").String())
		nb.obj.Trigger("change", nb.obj.This.Get("nav_input"))
		return nil
	}, false, false)
}

func (nb *NavBar) bindButton(selector, event string) {
	btns := dom.Query(nb.obj.Dom, selector)
	if len(btns) == 0 {
		return
	}
	dom.AddEvent(btns[0], "click", func(_ js.Value, _ []js.Value) any {
		nb.obj.Trigger(event)
		return nil
	}, false, false)
}
