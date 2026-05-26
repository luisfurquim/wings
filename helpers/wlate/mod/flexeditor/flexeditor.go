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
	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/dom"

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
	wprana.Register(
		elementTag,
		htmlContent,
		varsCSS+"\n"+designCSS,
		func() wprana.PranaMod { return &Editor{} },
	)
	G.Logf(3, "wl-flex-editor: module registered\n")
}

// Editor implements wprana.PranaMod and wldata.FlexEditor. curGenders and
// curCategories are the column/row keys of the grid currently on screen; they
// are computed in Display and consumed by Harvest to map a cell's data-ci/gi
// back to its "gender.category" key.
type Editor struct {
	obj           *wprana.PranaObj
	curGenders    []string
	curCategories []string
}

var _ wldata.FlexEditor = (*Editor)(nil)

func (e *Editor) InitData() map[string]any {
	return map[string]any{
		"inflection_expr":  "",
		"genders":          []any{},
		"categories":       []any{},
		"gender_headers":   []any{},
		"category_headers": []any{},
		"left_cells":       []any{},
		"right_cells":      []any{},
		"right_srcs":       []any{},
	}
}

func (e *Editor) Render(obj *wprana.PranaObj) {
	e.obj = obj

	if inds := dom.Query(obj.Dom, "#wl-revised-toggle-inflection"); len(inds) > 0 {
		dom.AddEvent(inds[0], "click", func(_ js.Value, _ []js.Value) any {
			obj.Trigger("revised")
			return nil
		}, false, false)
	}

	obj.Trigger("register", e)
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

// Clear blanks the grids when there is no current record.
func (e *Editor) Clear() {
	e.curGenders = nil
	e.curCategories = nil
	e.obj.This.Set("inflection_expr", "")
	e.obj.This.Set("genders", []any{})
	e.obj.This.Set("categories", []any{})
	e.obj.This.Set("gender_headers", []any{})
	e.obj.This.Set("category_headers", []any{})
	e.obj.This.Set("left_cells", []any{})
	e.obj.This.Set("right_cells", []any{})
	e.obj.This.Set("right_srcs", []any{})
	e.setRevisedBorder(false)
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
