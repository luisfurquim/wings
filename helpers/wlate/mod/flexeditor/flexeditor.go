//go:build js && wasm

// Package flexeditor provides the wl-flex-editor custom element: the
// inflection translation pane of wlate (the "Inflexões" tab). It owns the
// side-by-side gender×CLDR grids and the logic to build them from a record and
// harvest the per-cell edits. As with wl-text-editor, the editing session
// (record list, navigation, save, dirty, dialog) lives in the wp-wlate shell;
// this element registers with the shell (the @register trigger) and is driven
// through the wldata.FlexEditor contract.
//
// One trigger flows back up to the shell:
//   - @revised — the clickable left-border indicator was clicked.
//
// Unlike the text pane there is no per-keystroke @input: the inflection cells
// mark the record dirty only when harvested (matching the original wlate).
package flexeditor

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"

	"wlate/mod/wldata"
)

// G is the logger for this module.
var G goose.Alert

const elementTag = "wl-flex-editor"

//go:embed flexeditor.i18n.html
var htmlContent string

//go:embed vars.css
var varsCSS string

//go:embed design.css
var designCSS string

func init() {
	G.Set(3)
	wings.Register(
		elementTag,
		htmlContent,
		varsCSS+"\n"+designCSS,
		func() wings.PranaMod { return &Editor{} },
	)
	G.Logf(3, "wl-flex-editor: module registered\n")
}

// Editor implements wings.PranaMod and wldata.FlexEditor. curGenders and
// curCategories are the column/row keys of the grid currently on screen; they
// are computed in Display and consumed by Harvest to map a cell's data-ci/gi
// back to its "gender.category" key.
type Editor struct {
	obj           *wings.PranaObj
	curGenders    []string
	curCategories []string
	// contentMode is true while the current record is a programmable flex rule
	// (a per-locale Content phrase) rather than a gender×CLDR cells grid. It is
	// set in Display and read in Harvest to pick the right pane to read back.
	contentMode bool
}

var _ wldata.FlexEditor = (*Editor)(nil)

func (e *Editor) InitData() map[string]any {
	return map[string]any{
		"inflection_expr":   "",
		"flex_mode":         "cells",
		"left_content":      "",
		"right_content":     "",
		"confirm_overwrite": false,
		// Dialog button handlers: nil placeholders so the child <w-dialog>'s
		// @overwrite/@cancel bindings resolve at bind time; the real handlers
		// are installed in Render (same pattern as the shell's fnSave et al.).
		"dlg_overwrite":    wings.TriggerHandler(nil),
		"dlg_cancel":       wings.TriggerHandler(nil),
		"genders":          []any{},
		"categories":       []any{},
		"gender_headers":   []any{},
		"category_headers": []any{},
		"left_cells":       []any{},
		"right_cells":      []any{},
		"right_srcs":       []any{},
	}
}

func (e *Editor) Render(obj *wings.PranaObj) {
	e.obj = obj

	if inds := dom.Query(obj.Dom, "#wl-revised-toggle-inflection"); len(inds) > 0 {
		dom.AddEvent(inds[0], "click", func(_ js.Value, _ []js.Value) any {
			obj.Trigger("revised")
			return nil
		}, false, false)
	}

	// Copy-to-right (content mode): copy the reference phrase into the editable
	// textarea verbatim, so the translator keeps the fragile sigils ($var,
	// ~$var, %count with paths) intact and only edits the words. When the
	// textarea already has content, ask before overwriting.
	if btns := dom.Query(obj.Dom, "#wl-copy-to-right"); len(btns) > 0 {
		dom.AddEvent(btns[0], "click", func(_ js.Value, _ []js.Value) any {
			if e.rightPhrase() == "" {
				e.copyToRight()
			} else {
				e.obj.This.Set("confirm_overwrite", true)
			}
			return nil
		}, false, false)
	}

	// Real dialog handlers (the InitData placeholders were nil).
	obj.This.M["dlg_overwrite"] = wings.TriggerHandler(func(_ ...any) {
		e.obj.This.Set("confirm_overwrite", false)
		e.copyToRight()
	})
	obj.This.M["dlg_cancel"] = wings.TriggerHandler(func(_ ...any) {
		e.obj.This.Set("confirm_overwrite", false)
	})

	obj.Trigger("register", e)
}

// rightPhrase returns the current value of the editable phrase textarea, or ""
// when it is not present (cells mode).
func (e *Editor) rightPhrase() string {
	tas := dom.Query(e.obj.Dom, ".wl-phrase-edit")
	if len(tas) == 0 {
		return ""
	}
	return tas[0].Get("value").String()
}

// copyToRight copies the reference (left) phrase into the editable textarea and
// signals the shell to mark the session dirty (the flex editor otherwise only
// goes dirty at Harvest time; this is an explicit edit before any keystroke).
func (e *Editor) copyToRight() {
	left, _ := e.obj.This.M["left_content"].(string)
	e.obj.This.Set("right_content", left)
	e.obj.Trigger("input")
}

// Display builds both grids from the union of the left (reference) and right
// (editable) records' form keys, so every language shows the full CLDR×gender
// matrix even when one axis is degenerate.
func (e *Editor) Display(left, right wldata.InflectionRecord) {
	leftForms := left.Cells
	rightForms := right.Cells

	expr := left.Label
	if expr == "" {
		expr = right.Label
	}
	e.obj.This.Set("inflection_expr", expr)
	e.setRevisedBorder(right.Revised)

	// Programmable rule: a per-locale Content phrase the translator edits as
	// free text (reordering words, keeping the sigils) instead of the
	// gender×CLDR grid. Detected by a non-empty Content on either side.
	// (project_customflex_design)
	if left.Content != "" || right.Content != "" {
		e.contentMode = true
		e.curGenders = nil
		e.curCategories = nil
		e.obj.This.Set("flex_mode", "content")
		e.obj.This.Set("left_content", left.Content)
		e.obj.This.Set("right_content", right.Content)
		e.clearGridData()
		return
	}

	e.contentMode = false
	e.obj.This.Set("flex_mode", "cells")
	e.obj.This.Set("left_content", "")
	e.obj.This.Set("right_content", "")

	// Axes = union of both sides' "gender.category" keys.
	genderSet := map[string]bool{}
	catSet := map[string]bool{}
	for _, forms := range []map[string]string{leftForms, rightForms} {
		for key := range forms {
			parts := strings.SplitN(key, ".", 2)
			if len(parts) == 2 {
				genderSet[parts[0]] = true
				catSet[parts[1]] = true
			}
		}
	}
	genders := wldata.SortedKeys(genderSet)
	categories := wldata.SortedCLDR(catSet)
	e.curGenders = genders
	e.curCategories = categories

	// Translator-facing headers. Degenerate gender (empty string) renders
	// visually blank with an ARIA label for screen readers.
	genderHeaders := make([]any, len(genders))
	for i, g := range genders {
		genderHeaders[i] = map[string]any{
			"label": wldata.GenderHeaderLabel(g),
			"aria":  wldata.GenderAriaLabel(g),
		}
	}
	categoryHeaders := make([]any, len(categories))
	for i, c := range categories {
		categoryHeaders[i] = map[string]any{
			"label": c,
			"aria":  wldata.CategoryAriaLabel(c),
		}
	}

	// Cell arrays: left_cells[ci].cols[gi], right_cells[ci].cols[gi],
	// right_srcs[ci].cols[gi] (avatar URL or "" for no badge).
	rightSources := right.Sources
	leftCells := make([]any, len(categories))
	rightCells := make([]any, len(categories))
	rightSrcs := make([]any, len(categories))
	for ci, cat := range categories {
		lcols := make([]any, len(genders))
		rcols := make([]any, len(genders))
		srccols := make([]any, len(genders))
		for gi, gen := range genders {
			key := gen + "." + cat
			if leftForms != nil {
				lcols[gi] = leftForms[key]
			} else {
				lcols[gi] = ""
			}
			if rightForms != nil {
				rcols[gi] = rightForms[key]
			} else {
				rcols[gi] = ""
			}
			src := ""
			if rightSources != nil {
				src = wldata.SourceToAvatarURL(rightSources[key])
			}
			srccols[gi] = src
		}
		leftCells[ci] = map[string]any{"cols": lcols}
		rightCells[ci] = map[string]any{"cols": rcols}
		rightSrcs[ci] = map[string]any{"cols": srccols}
	}

	e.obj.This.Set("genders", toAny(genders))
	e.obj.This.Set("categories", toAny(categories))
	e.obj.This.Set("gender_headers", genderHeaders)
	e.obj.This.Set("category_headers", categoryHeaders)
	e.obj.This.Set("left_cells", leftCells)
	e.obj.This.Set("right_cells", rightCells)
	e.obj.This.Set("right_srcs", rightSrcs)

	// Set CSS grid columns dynamically: 1 row-header + N gender columns.
	gridCols := fmt.Sprintf("auto repeat(%d, 1fr)", len(genders))
	for _, gridID := range []string{"#wl-grid-left", "#wl-grid-right"} {
		for _, g := range dom.Query(e.obj.Dom, gridID) {
			g.Get("style").Set("gridTemplateColumns", gridCols)
		}
	}
}

// Harvest reads the editable cell inputs back into right, marking changed cells
// manual and the record revised. Returns true if anything changed.
func (e *Editor) Harvest(right *wldata.InflectionRecord) bool {
	if e.contentMode {
		tas := dom.Query(e.obj.Dom, ".wl-phrase-edit")
		if len(tas) == 0 {
			return false
		}
		content := tas[0].Get("value").String()
		if content == right.Content {
			return false
		}
		right.Content = content
		right.Revised = true
		// Mirror back so a reactive sync (e.g. the unsaved-changes dialog
		// closing) re-renders the freshly harvested phrase, not the old one.
		e.obj.This.Set("right_content", content)
		return true
	}

	inputs := dom.Query(e.obj.Dom, ".wl-cell-input")
	changed := false
	for _, inp := range inputs {
		ci := inp.Get("dataset").Get("ci").String()
		gi := inp.Get("dataset").Get("gi").String()
		ciN, err1 := strconv.Atoi(ci)
		giN, err2 := strconv.Atoi(gi)
		if err1 != nil || err2 != nil {
			continue
		}
		if ciN >= len(e.curCategories) || giN >= len(e.curGenders) {
			continue
		}
		key := e.curGenders[giN] + "." + e.curCategories[ciN]
		val := inp.Get("value").String()
		if right.Cells[key] != val {
			if right.Cells == nil {
				right.Cells = make(map[string]string)
			}
			right.Cells[key] = val
			if right.Sources == nil {
				right.Sources = make(map[string]string)
			}
			right.Sources[key] = "manual"
			// Mirror into M["right_cells"] so a reactive sync triggered by the
			// shell (e.g. closing the unsaved-changes dialog) renders the new
			// value, not the old one.
			if rc, ok := e.obj.This.M["right_cells"].([]any); ok && ciN < len(rc) {
				if row, ok := rc[ciN].(map[string]any); ok {
					if cols, ok := row["cols"].([]any); ok && giN < len(cols) {
						cols[giN] = val
					}
				}
			}
			changed = true
		}
	}
	if changed {
		right.Revised = true
	}
	return changed
}

// Clear blanks both panes when there is no current record.
func (e *Editor) Clear() {
	e.contentMode = false
	e.curGenders = nil
	e.curCategories = nil
	e.obj.This.Set("inflection_expr", "")
	e.obj.This.Set("flex_mode", "cells")
	e.obj.This.Set("left_content", "")
	e.obj.This.Set("right_content", "")
	e.clearGridData()
	e.setRevisedBorder(false)
}

// clearGridData empties the gender×CLDR grid's reactive arrays. Shared by
// Clear and by Display when switching to a programmable (content) record.
func (e *Editor) clearGridData() {
	e.obj.This.Set("genders", []any{})
	e.obj.This.Set("categories", []any{})
	e.obj.This.Set("gender_headers", []any{})
	e.obj.This.Set("category_headers", []any{})
	e.obj.This.Set("left_cells", []any{})
	e.obj.This.Set("right_cells", []any{})
	e.obj.This.Set("right_srcs", []any{})
}

func (e *Editor) setRevisedBorder(revised bool) {
	panels := dom.Query(e.obj.Dom, "#wl-right-inflection-panel")
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

func toAny(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}
