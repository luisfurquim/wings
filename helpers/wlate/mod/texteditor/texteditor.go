//go:build js && wasm

// Package texteditor provides the wl-text-editor custom element: the text
// translation pane of wlate (the "Texto" tab). It owns the side-by-side
// reference/editable panels and the logic to render a record and harvest the
// translator's edits — but not the editing session itself. The wp-wlate shell
// owns the record list, navigation, save, dirty flag and dialog; this element
// registers with the shell (the @register trigger) and the shell drives it
// through the wldata.TextEditor contract.
//
// Two triggers flow back up to the shell:
//   - @revised — the clickable left-border indicator was clicked.
//   - @input   — the textarea received input (so the shell can mark dirty
//     even before the edit is harvested, for the beforeunload guard).
package texteditor

import (
	_ "embed"
	"syscall/js"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/dom"

	"wlate/mod/wldata"
)

// G is the logger for this module.
var G goose.Alert

const elementTag = "wl-text-editor"

//go:embed texteditor.i18n.html
var htmlContent string

//go:embed vars.css
var varsCSS string

//go:embed design.css
var designCSS string

func init() {
	G.Set(3)
	wprana.Register(
		elementTag,
		htmlContent,
		varsCSS+"\n"+designCSS,
		func() wprana.PranaMod { return &Editor{} },
	)
	G.Logf(3, "wl-text-editor: module registered\n")
}

// Editor implements wprana.PranaMod and wldata.TextEditor.
type Editor struct {
	obj *wprana.PranaObj
}

var _ wldata.TextEditor = (*Editor)(nil)

func (e *Editor) InitData() map[string]any {
	return map[string]any{
		"left_content":   "",
		"right_content":  "",
		"right_text_src": "",
	}
}

func (e *Editor) Render(obj *wprana.PranaObj) {
	e.obj = obj

	// The revised indicator is a click target inside this shadow tree; bubble
	// the intent up so the shell (which owns the record's revised flag and the
	// auto-advance) can act on it.
	if inds := dom.Query(obj.Dom, "#wl-revised-toggle"); len(inds) > 0 {
		dom.AddEvent(inds[0], "click", func(_ js.Value, _ []js.Value) any {
			obj.Trigger("revised")
			return nil
		}, false, false)
	}

	// Per-keystroke dirty signal so the shell's beforeunload guard fires even
	// if the user closes the tab before navigating (which is what harvests).
	if tas := dom.Query(obj.Dom, ".wl-content-edit"); len(tas) > 0 {
		dom.AddEvent(tas[0], "input", func(_ js.Value, _ []js.Value) any {
			obj.Trigger("input")
			return nil
		}, false, false)
	}

	// Hand ourselves to the shell. Safe at Render time: the shell installs the
	// registerEditor handler in InitData, before its template (and thus this
	// element) is created.
	obj.Trigger("register", e)
}

// Display renders the reference (left) and editable (right) text records.
func (e *Editor) Display(left, right wldata.TextRecord) {
	e.obj.This.Set("left_content", left.Content)
	e.obj.This.Set("right_content", right.Content)
	e.obj.This.Set("right_text_src", wldata.SourceToAvatarURL(right.Source))
	e.setRevisedBorder(right.Revised)
}

// Harvest reads the textarea back into right. A change marks the record manual
// and revised, clears the provenance badge, and returns true.
func (e *Editor) Harvest(right *wldata.TextRecord) bool {
	tas := dom.Query(e.obj.Dom, ".wl-content-edit")
	if len(tas) == 0 {
		return false
	}
	content := tas[0].Get("value").String()
	if content == right.Content {
		return false
	}
	right.Content = content
	right.Source = "manual"
	right.Revised = true
	e.obj.This.Set("right_content", content)
	e.obj.This.Set("right_text_src", "")
	return true
}

// Clear blanks the editor when there is no current record.
func (e *Editor) Clear() {
	e.obj.This.Set("left_content", "")
	e.obj.This.Set("right_content", "")
	e.obj.This.Set("right_text_src", "")
	e.setRevisedBorder(false)
}

// setRevisedBorder toggles the wl-revised class on the right panel, which
// drives the left-border colour (unrevised accent vs. quiet revised tone).
func (e *Editor) setRevisedBorder(revised bool) {
	panels := dom.Query(e.obj.Dom, "#wl-right-panel")
	if len(panels) == 0 {
		return
	}
	cls := panels[0].Get("classList")
	if revised {
		cls.Call("add", "wl-revised")
	} else {
		cls.Call("remove", "wl-revised")
	}
}
