//go:build js && wasm

// Package wlate provides the wp-wlate custom element: the shell of the wlate
// translation editor.
//
// wp-wlate owns the editing *session* — the loaded record lists, the language
// pair, navigation position, the filter, the dirty flag, the single Save (which
// writes both the text and inflection catalogs), the unsaved-changes dialog and
// the keyboard shortcuts. It does NOT render or harvest individual records:
// that work lives in two sibling custom elements, wl-text-editor and
// wl-flex-editor (see packages texteditor and flexeditor), one per tab.
//
// Each editor registers itself with the shell at Render time via the @register
// trigger (the editorReg handshake below). Thereafter the shell drives whichever
// tab is active through the wldata.TextEditor / wldata.FlexEditor contract:
// Display(left,right) to show a record, Harvest(&right) to pull edits back, and
// Clear() when there is nothing to show.
package wlate

import (
	_ "embed"
	"encoding/json"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/dom"
	"github.com/luisfurquim/wprana/wi18n"
	_ "github.com/luisfurquim/wprana/widget/tab"
	_ "github.com/luisfurquim/wprana/widget/tabbutton"
	_ "github.com/luisfurquim/wprana/widget/tabs"

	"wlate/mod/wldata"
)

// G is this package's goose alert. SetConfig (called from init() with
// wprana.json bytes) propagates the project-wide debugLevel here via
// wi18n.ConfigureGoose.
var G goose.Alert

const elementTag = "wp-wlate"

//go:embed wlate.i18n.html
var htmlContent string

//go:embed vars.css
var varsCSS string

//go:embed design.css
var designCSS string

func init() {
	wprana.Register(
		elementTag,
		htmlContent,
		varsCSS+"\n"+designCSS,
		func() wprana.PranaMod { return &Wlate{} },
	)
}

// ── Default keybindings ────────────────────────────────────────────────────

var defaultKeys = map[string]string{
	"prev":    "Alt+ArrowLeft",
	"next":    "Alt+ArrowRight",
	"prev10":  "Alt+PageUp",
	"next10":  "Alt+PageDown",
	"first":   "Alt+Home",
	"last":    "Alt+End",
	"save":    "Ctrl+S",
	"revised": "Alt+R",
}

// ── Editor registration handshake ──────────────────────────────────────────

// editorReg collects the two tab editors as they register from their Render.
// The shell's init goroutine blocks on ready until both are present, so the
// first displayRecord always has live editors to drive.
type editorReg struct {
	text  wldata.TextEditor
	flex  wldata.FlexEditor
	ready chan struct{}
	done  bool
}

// register routes a registering editor to the right slot by which contract it
// satisfies (the two method sets are disjoint, so the type switch is exact),
// and closes ready once both have arrived. All goroutines share one JS thread,
// so no locking is needed.
func (r *editorReg) register(args ...any) {
	if len(args) == 0 {
		return
	}
	switch e := args[0].(type) {
	case wldata.TextEditor:
		r.text = e
	case wldata.FlexEditor:
		r.flex = e
	default:
		return
	}
	if !r.done && r.text != nil && r.flex != nil {
		r.done = true
		close(r.ready)
	}
}

// ── Component ──────────────────────────────────────────────────────────────

type Wlate struct {
	reg *editorReg
}

func (w *Wlate) InitData() map[string]any {
	w.reg = &editorReg{ready: make(chan struct{})}
	return map[string]any{
		// Config
		"default_lang": "",
		"lang_options": "[]",

		// Current state
		"left_lang":    "",
		"right_lang":   "",
		"tab":          "text",
		"filter_unrev": false,

		// Status bar
		"pending_count": 0,
		"total_count":   0,

		// Shared context bar (shown for both tabs)
		"context":        "",
		"ctxdetail_text": "",
		"nav_input":      "1",

		// Dialog
		"show_dialog": false,
		"fnSave":      wprana.TriggerHandler(nil),
		"fnDiscard":   wprana.TriggerHandler(nil),
		"fnCancel":    wprana.TriggerHandler(nil),

		// Navbar trigger handlers (set in wireEvents).
		"navFirst":  wprana.TriggerHandler(nil),
		"navPrev10": wprana.TriggerHandler(nil),
		"navPrev":   wprana.TriggerHandler(nil),
		"navNext":   wprana.TriggerHandler(nil),
		"navNext10": wprana.TriggerHandler(nil),
		"navLast":   wprana.TriggerHandler(nil),
		"navJump":   wprana.TriggerHandler(nil),

		// Combobox + editor trigger handlers (set in wireEvents).
		"on_left_lang":  wprana.TriggerHandler(nil),
		"on_right_lang": wprana.TriggerHandler(nil),
		"toggleRevised": wprana.TriggerHandler(nil),
		"markDirty":     wprana.TriggerHandler(nil),

		// register fires from the editors' Render, which runs after this map is
		// already bound — so the handler must be live here, not in wireEvents.
		"registerEditor": wprana.TriggerHandler(func(args ...any) { w.reg.register(args...) }),
	}
}

// ── Internal state ─────────────────────────────────────────────────────────

type wlateCtx struct {
	obj              *wprana.PranaObj
	reg              *editorReg
	config           wldata.Config
	keys             map[string]string
	leftLang         string
	rightLang        string
	leftText         []wldata.TextRecord
	rightText        []wldata.TextRecord
	leftInflections  []wldata.InflectionRecord
	rightInflections []wldata.InflectionRecord
	filteredIdx      []int
	currentPos       int // position in filteredIdx
	dirty            bool
	pendingLang      string
	pendingSide      string
	filterUnrev      bool
	activeTab        string // "text" or "inflection"
}

func (wc *wlateCtx) currentDataLen() int {
	if wc.activeTab == "inflection" {
		return len(wc.rightInflections)
	}
	return len(wc.rightText)
}

func (wc *wlateCtx) isRecordRevised(idx int) bool {
	if wc.activeTab == "inflection" {
		if idx >= 0 && idx < len(wc.rightInflections) {
			return wc.rightInflections[idx].Revised
		}
	} else {
		if idx >= 0 && idx < len(wc.rightText) {
			return wc.rightText[idx].Revised
		}
	}
	return false
}

// buildFilteredIdx rebuilds the filtered index based on the current filter state.
func (wc *wlateCtx) buildFilteredIdx() {
	total := wc.currentDataLen()
	wc.filteredIdx = wc.filteredIdx[:0]
	for i := 0; i < total; i++ {
		if wc.filterUnrev && wc.isRecordRevised(i) {
			continue
		}
		wc.filteredIdx = append(wc.filteredIdx, i)
	}
}

// actualIdx returns the actual data index for the current position in filteredIdx.
func (wc *wlateCtx) actualIdx() int {
	if wc.currentPos < 0 || wc.currentPos >= len(wc.filteredIdx) {
		return -1
	}
	return wc.filteredIdx[wc.currentPos]
}

// countPending counts records with revised=false.
func (wc *wlateCtx) countPending() int {
	count := 0
	if wc.activeTab == "inflection" {
		for i := range wc.rightInflections {
			if !wc.rightInflections[i].Revised {
				count++
			}
		}
	} else {
		for i := range wc.rightText {
			if !wc.rightText[i].Revised {
				count++
			}
		}
	}
	return count
}

// updateStatusBar updates pending/total counts.
func (wc *wlateCtx) updateStatusBar() {
	wc.obj.This.Set("pending_count", wc.countPending())
	wc.obj.This.Set("total_count", wc.currentDataLen())
}

// displayRecord shows the record at the current position: it sets the shared
// chrome (record number, context bar) and delegates the panel rendering to the
// active editor.
func (wc *wlateCtx) displayRecord() {
	idx := wc.actualIdx()
	if idx < 0 {
		if wc.activeTab == "inflection" {
			if wc.reg.flex != nil {
				wc.reg.flex.Clear()
			}
		} else {
			if wc.reg.text != nil {
				wc.reg.text.Clear()
			}
		}
		wc.obj.This.Set("context", "")
		wc.obj.This.Set("ctxdetail_text", "")
		wc.obj.This.Set("nav_input", "0")
		return
	}

	wc.obj.This.Set("nav_input", strconv.Itoa(idx+1))

	ctx, detail := wc.recordContext(idx)
	wc.obj.This.Set("context", ctx)
	wc.obj.This.Set("ctxdetail_text", detail)

	if wc.activeTab == "inflection" {
		var left, right wldata.InflectionRecord
		if idx < len(wc.leftInflections) {
			left = wc.leftInflections[idx]
		}
		if idx < len(wc.rightInflections) {
			right = wc.rightInflections[idx]
		}
		if wc.reg.flex != nil {
			wc.reg.flex.Display(left, right)
		}
	} else {
		var left, right wldata.TextRecord
		if idx < len(wc.leftText) {
			left = wc.leftText[idx]
		}
		if idx < len(wc.rightText) {
			right = wc.rightText[idx]
		}
		if wc.reg.text != nil {
			wc.reg.text.Display(left, right)
		}
	}
	wc.updateStatusBar()
}

// recordContext returns the shared context bar strings for the record at idx.
// Text prefers the right (target) side's context, falling back to the left
// (reference); inflection prefers the left, matching the original behaviour.
func (wc *wlateCtx) recordContext(idx int) (ctx, detail string) {
	if wc.activeTab == "inflection" {
		if idx < len(wc.leftInflections) {
			ctx = wc.leftInflections[idx].Context
			detail = wldata.HumanizeCtx(wc.leftInflections[idx].Ctxdetail)
		}
		if idx < len(wc.rightInflections) {
			if ctx == "" {
				ctx = wc.rightInflections[idx].Context
			}
			if detail == "" {
				detail = wldata.HumanizeCtx(wc.rightInflections[idx].Ctxdetail)
			}
		}
		return
	}

	var lctx, ldetail string
	if idx < len(wc.leftText) {
		lctx = wc.leftText[idx].Context
		ldetail = wldata.HumanizeCtx(wc.leftText[idx].Ctxdetail)
	}
	if idx < len(wc.rightText) {
		ctx = wc.rightText[idx].Context
		detail = wldata.HumanizeCtx(wc.rightText[idx].Ctxdetail)
	}
	if ctx == "" {
		ctx = lctx
	}
	if detail == "" {
		detail = ldetail
	}
	return
}

// navigate moves to a new position in filteredIdx.
func (wc *wlateCtx) navigate(newPos int) {
	// Save current edits BEFORE changing position
	wc.syncRightContent()

	if len(wc.filteredIdx) == 0 {
		wc.currentPos = 0
		wc.displayRecord()
		return
	}
	if newPos < 0 {
		newPos = 0
	}
	if newPos >= len(wc.filteredIdx) {
		newPos = len(wc.filteredIdx) - 1
	}
	wc.currentPos = newPos
	wc.displayRecord()
}

// syncRightContent harvests the active editor's edits into the current record,
// marking the session dirty if anything changed.
func (wc *wlateCtx) syncRightContent() {
	idx := wc.actualIdx()
	if idx < 0 {
		return
	}
	if wc.activeTab == "text" {
		if idx < len(wc.rightText) && wc.reg.text != nil {
			if wc.reg.text.Harvest(&wc.rightText[idx]) {
				wc.dirty = true
			}
		}
		return
	}
	if idx < len(wc.rightInflections) && wc.reg.flex != nil {
		if wc.reg.flex.Harvest(&wc.rightInflections[idx]) {
			wc.dirty = true
		}
	}
}

// findNextUnrevised finds the position of the next unrevised record after currentPos.
// Returns -1 if none found.
func (wc *wlateCtx) findNextUnrevised() int {
	total := wc.currentDataLen()
	startIdx := wc.actualIdx() + 1
	for i := startIdx; i < total; i++ {
		if !wc.isRecordRevised(i) {
			for pos, fi := range wc.filteredIdx {
				if fi == i {
					return pos
				}
			}
			return -1
		}
	}
	// Wrap around from the beginning
	for i := 0; i < startIdx && i < total; i++ {
		if !wc.isRecordRevised(i) {
			for pos, fi := range wc.filteredIdx {
				if fi == i {
					return pos
				}
			}
			return -1
		}
	}
	return -1
}

// ── Key matching ───────────────────────────────────────────────────────────

// matchKey checks if a keyboard event matches a key binding string like "Alt+ArrowLeft".
func matchKey(event js.Value, binding string) bool {
	parts := strings.Split(binding, "+")
	if len(parts) == 0 {
		return false
	}
	key := parts[len(parts)-1]
	needAlt := false
	needCtrl := false
	needShift := false
	for _, mod := range parts[:len(parts)-1] {
		switch strings.ToLower(mod) {
		case "alt":
			needAlt = true
		case "ctrl", "control":
			needCtrl = true
		case "shift":
			needShift = true
		}
	}

	if event.Get("key").String() != key {
		if len(key) == 1 && strings.EqualFold(event.Get("key").String(), key) {
			// case-insensitive single-letter match — ok
		} else {
			return false
		}
	}
	if event.Get("altKey").Bool() != needAlt {
		return false
	}
	if event.Get("ctrlKey").Bool() != needCtrl {
		return false
	}
	if event.Get("shiftKey").Bool() != needShift {
		return false
	}
	return true
}

// getKey returns the keybinding for an action, using config override or default.
func (wc *wlateCtx) getKey(action string) string {
	if k, ok := wc.keys[action]; ok && k != "" {
		return k
	}
	if k, ok := defaultKeys[action]; ok {
		return k
	}
	return ""
}

// ── Render ─────────────────────────────────────────────────────────────────

func (w *Wlate) Render(obj *wprana.PranaObj) {
	wc := &wlateCtx{
		obj:         obj,
		reg:         w.reg,
		activeTab:   "text",
		filteredIdx: make([]int, 0, 256),
		keys:        map[string]string{},
	}

	go wc.init()
}

func (wc *wlateCtx) init() {
	// Load config
	body, err := wldata.FetchText("wprana.json")
	if err != nil {
		G.Logf(1, "wlate: failed to load wprana.json: %v", err)
		return
	}
	// Apply project-wide settings (debugLevel, traceOn, measure overrides)
	// before parsing wlate-specific keys, so subsequent G.Logf calls observe
	// the configured level.
	if err := wi18n.SetConfig([]byte(body)); err != nil {
		G.Logf(1, "wlate: failed to apply wprana.json: %v", err)
		return
	}
	wi18n.ConfigureGoose(&G)
	if err := json.Unmarshal([]byte(body), &wc.config); err != nil {
		G.Logf(1, "wlate: failed to parse wprana.json: %v", err)
		return
	}

	// Merge keybindings
	for k, v := range defaultKeys {
		wc.keys[k] = v
	}
	for k, v := range wc.config.Wlate.Keys {
		wc.keys[k] = v
	}

	wc.obj.This.Set("default_lang", wc.config.DefaultLang)

	// Build language options JSON for combobox
	opts := make([]map[string]string, len(wc.config.Languages))
	for i, lang := range wc.config.Languages {
		opts[i] = map[string]string{"label": lang, "value": lang}
	}
	optsJSON, _ := json.Marshal(opts)
	wc.obj.This.Set("lang_options", string(optsJSON))

	// Load default language on the left
	wc.leftLang = wc.config.DefaultLang
	wc.obj.This.Set("left_lang", wc.leftLang)
	wc.loadLeftData()

	// Pick first non-default language for the right, or default if only one
	wc.rightLang = wc.config.DefaultLang
	for _, lang := range wc.config.Languages {
		if lang != wc.config.DefaultLang {
			wc.rightLang = lang
			break
		}
	}
	wc.obj.This.Set("right_lang", wc.rightLang)
	wc.loadRightData()

	// Wait for both editor panes to register from their Render before the first
	// paint, so displayRecord has live editors to drive. This receive yields,
	// letting the editor goroutines run if they have not already.
	<-wc.reg.ready

	// Build initial filter and display
	wc.buildFilteredIdx()
	wc.displayRecord()
	wc.updateStatusBar()

	// Wire up event handlers
	wc.wireEvents()
}

func (wc *wlateCtx) loadLeftData() {
	wc.leftText = wldata.LoadText(wc.leftLang)
	wc.leftInflections = wldata.LoadFlex(wc.leftLang)
}

func (wc *wlateCtx) loadRightData() {
	wc.rightText = wldata.LoadText(wc.rightLang)
	wc.rightInflections = wldata.LoadFlex(wc.rightLang)
	wc.dirty = false

	// If right-side files don't exist, bootstrap from left-side shape so
	// translators see the full list to fill in.
	if wc.rightText == nil && len(wc.leftText) > 0 {
		wc.rightText = make([]wldata.TextRecord, len(wc.leftText))
		for i, lt := range wc.leftText {
			wc.rightText[i] = wldata.TextRecord{EntryMeta: lt.EntryMeta}
		}
	}

	if wc.rightInflections == nil && len(wc.leftInflections) > 0 {
		wc.rightInflections = make([]wldata.InflectionRecord, len(wc.leftInflections))
		for i, li := range wc.leftInflections {
			cells := make(map[string]string, len(li.Cells))
			for key := range li.Cells {
				cells[key] = ""
			}
			wc.rightInflections[i] = wldata.InflectionRecord{
				FlexEntryData: wi18n.FlexEntryData{
					Label: li.Label,
					Cells: cells,
				},
				FlexEntryMeta: li.FlexEntryMeta,
			}
		}
	}
}

func (wc *wlateCtx) wireEvents() {
	// Tab switching. <w-tabs> here is headless (no w-tabbutton): it is driven
	// by the bound `active="{{tab}}"` attribute and imposes no chrome. The tab
	// buttons are plain <button>s we wire by hand — switchTab sets the `tab`
	// data, which flows back into <w-tabs> and selects the matching panel.
	for _, m := range []struct{ id, tab string }{
		{"#wl-tab-text", "text"},
		{"#wl-tab-inflection", "inflection"},
	} {
		btns := dom.Query(wc.obj.Dom, m.id)
		if len(btns) == 0 {
			continue
		}
		tab := m.tab
		dom.AddEvent(btns[0], "click", func(_ js.Value, _ []js.Value) any {
			wc.switchTab(tab)
			return nil
		}, false, false)
	}
	wc.highlightTabButtons("text")

	// Navigation — wired to w-navbar widget triggers.
	wc.obj.This.M["navFirst"] = wprana.TriggerHandler(func(_ ...any) {
		wc.navigate(0)
	})
	wc.obj.This.M["navPrev10"] = wprana.TriggerHandler(func(_ ...any) {
		wc.navigate(wc.currentPos - 10)
	})
	wc.obj.This.M["navPrev"] = wprana.TriggerHandler(func(_ ...any) {
		wc.navigate(wc.currentPos - 1)
	})
	wc.obj.This.M["navNext"] = wprana.TriggerHandler(func(_ ...any) {
		wc.navigate(wc.currentPos + 1)
	})
	wc.obj.This.M["navNext10"] = wprana.TriggerHandler(func(_ ...any) {
		wc.navigate(wc.currentPos + 10)
	})
	wc.obj.This.M["navLast"] = wprana.TriggerHandler(func(_ ...any) {
		wc.navigate(len(wc.filteredIdx) - 1)
	})
	wc.obj.This.M["navJump"] = wprana.TriggerHandler(func(args ...any) {
		if len(args) == 0 {
			return
		}
		val, ok := args[0].(string)
		if !ok {
			return
		}
		num, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil || num < 1 {
			return
		}
		// num is 1-based actual record number; find its position in filteredIdx.
		targetIdx := num - 1
		for pos, fi := range wc.filteredIdx {
			if fi == targetIdx {
				wc.navigate(pos)
				return
			}
		}
		wc.navigate(0)
	})

	// Revised toggle — fired by either editor's @revised trigger (the clickable
	// border indicator lives inside the editor's shadow tree now).
	wc.obj.This.M["toggleRevised"] = wprana.TriggerHandler(func(_ ...any) {
		wc.toggleRevised()
	})

	// Dirty signal — fired by wl-text-editor's @input on every keystroke, so the
	// beforeunload guard works even before the edit is harvested.
	wc.obj.This.M["markDirty"] = wprana.TriggerHandler(func(_ ...any) {
		wc.dirty = true
	})

	// Filter toggle
	filterToggles := dom.Query(wc.obj.Dom, "#wl-filter-toggle")
	if len(filterToggles) > 0 {
		dom.AddEvent(filterToggles[0], "click", func(_ js.Value, _ []js.Value) any {
			wc.filterUnrev = !wc.filterUnrev
			wc.obj.This.Set("filter_unrev", wc.filterUnrev)
			// Toggle CSS class on the track
			tracks := dom.Query(wc.obj.Dom, "#wl-toggle-track")
			if len(tracks) > 0 {
				cls := tracks[0].Get("classList")
				if wc.filterUnrev {
					cls.Call("add", "wl-toggle-on")
				} else {
					cls.Call("remove", "wl-toggle-on")
				}
			}
			wc.syncRightContent()
			wc.buildFilteredIdx()
			wc.currentPos = 0
			wc.displayRecord()
			return nil
		}, false, false)
	}

	// Save button
	saveBtns := dom.Query(wc.obj.Dom, "#btn-save")
	if len(saveBtns) > 0 {
		dom.AddEvent(saveBtns[0], "click", func(_ js.Value, _ []js.Value) any {
			go wc.save()
			return nil
		}, false, false)
	}

	// Dialog button handlers — fired by w-dialog's @save/@discard/@cancel triggers.
	// save()/loadXxxData() use synchronous fetch via syscall/js, which deadlocks
	// the main thread when invoked from a click callback; run them off-loop.
	wc.obj.This.M["fnSave"] = wprana.TriggerHandler(func(_ ...any) {
		go func() {
			wc.save()
			wc.displayRecord()
			wc.obj.This.Set("show_dialog", false)
			wc.doLangSwitch()
		}()
	})
	wc.obj.This.M["fnDiscard"] = wprana.TriggerHandler(func(_ ...any) {
		wc.dirty = false
		wc.obj.This.Set("show_dialog", false)
		go wc.doLangSwitch()
	})
	wc.obj.This.M["fnCancel"] = wprana.TriggerHandler(func(_ ...any) {
		wc.obj.This.Set("show_dialog", false)
	})

	// Keyboard shortcuts (on document)
	doc := js.Global().Get("document")
	dom.AddEvent(doc, "keydown", func(_ js.Value, args []js.Value) any {
		event := args[0]
		wc.handleKeyboard(event)
		return nil
	}, false, false)

	// beforeunload guard
	js.Global().Call("addEventListener", "beforeunload",
		js.FuncOf(func(this js.Value, args []js.Value) any {
			if wc.dirty {
				args[0].Call("preventDefault")
				args[0].Set("returnValue", "")
			}
			return nil
		}),
	)

	// Combobox @change handlers are set via wprana trigger system
	wc.obj.This.M["on_left_lang"] = wprana.TriggerHandler(func(args ...any) {
		if len(args) == 0 {
			return
		}
		selected, ok := args[0].([]any)
		if !ok || len(selected) == 0 {
			return
		}
		item, ok := selected[0].(map[string]any)
		if !ok {
			return
		}
		lang, ok := item["value"].(string)
		if !ok || lang == wc.leftLang {
			return
		}
		wc.syncRightContent()
		if wc.dirty {
			wc.pendingLang = lang
			wc.pendingSide = "left"
			wc.obj.This.Set("show_dialog", true)
			return
		}
		wc.leftLang = lang
		wc.obj.This.Set("left_lang", wc.leftLang)
		go func() {
			wc.loadLeftData()
			wc.displayRecord()
		}()
	})

	wc.obj.This.M["on_right_lang"] = wprana.TriggerHandler(func(args ...any) {
		if len(args) == 0 {
			return
		}
		selected, ok := args[0].([]any)
		if !ok || len(selected) == 0 {
			return
		}
		item, ok := selected[0].(map[string]any)
		if !ok {
			return
		}
		lang, ok := item["value"].(string)
		if !ok || lang == wc.rightLang {
			return
		}
		wc.syncRightContent()
		if wc.dirty {
			wc.pendingLang = lang
			wc.pendingSide = "right"
			wc.obj.This.Set("show_dialog", true)
			return
		}
		wc.rightLang = lang
		wc.obj.This.Set("right_lang", wc.rightLang)
		go func() {
			wc.loadRightData()
			wc.buildFilteredIdx()
			wc.currentPos = 0
			wc.displayRecord()
			wc.updateStatusBar()
		}()
	})
	wc.obj.This.Sync()
}

// highlightTabButtons mirrors the active tab onto the plain <button> tab
// controls (the panels themselves are driven by <w-tabs> via `active`).
func (wc *wlateCtx) highlightTabButtons(tab string) {
	for _, m := range []struct{ id, tab string }{
		{"#wl-tab-text", "text"},
		{"#wl-tab-inflection", "inflection"},
	} {
		btns := dom.Query(wc.obj.Dom, m.id)
		if len(btns) == 0 {
			continue
		}
		cl := btns[0].Get("classList")
		if m.tab == tab {
			cl.Call("add", "wl-tab-active")
		} else {
			cl.Call("remove", "wl-tab-active")
		}
	}
}

func (wc *wlateCtx) switchTab(tab string) {
	if tab == wc.activeTab {
		return
	}
	wc.syncRightContent()
	wc.activeTab = tab
	// Setting `tab` flows into <w-tabs active="{{tab}}">, which selects the
	// matching panel. The buttons are plain <button>s, so we highlight them.
	wc.obj.This.Set("tab", tab)
	wc.highlightTabButtons(tab)

	wc.buildFilteredIdx()
	wc.currentPos = 0
	wc.displayRecord()
	wc.updateStatusBar()
}

func (wc *wlateCtx) toggleRevised() {
	idx := wc.actualIdx()
	if idx < 0 {
		return
	}

	var wasRevised bool
	if wc.activeTab == "inflection" {
		if idx < len(wc.rightInflections) {
			wasRevised = wc.rightInflections[idx].Revised
			wc.rightInflections[idx].Revised = !wasRevised
		}
	} else {
		if idx < len(wc.rightText) {
			wasRevised = wc.rightText[idx].Revised
			wc.rightText[idx].Revised = !wasRevised
		}
	}
	wc.dirty = true

	if !wasRevised {
		// Marked as revised → auto-advance to next unrevised
		wc.updateStatusBar()
		nextPos := wc.findNextUnrevised()
		if nextPos >= 0 {
			wc.navigate(nextPos)
		} else {
			wc.displayRecord()
		}
	} else {
		// Unmarked → stay on current record
		wc.displayRecord()
	}
}

func (wc *wlateCtx) save() {
	wc.syncRightContent()

	// Save text: project to data-only (meta is build-server territory).
	if len(wc.rightText) > 0 {
		datas := make([]wi18n.EntryData, len(wc.rightText))
		for i, r := range wc.rightText {
			datas[i] = r.EntryData
		}
		data, err := json.MarshalIndent(datas, "", "  ")
		if err != nil {
			G.Logf(1, "wlate: failed to marshal text data: %v", err)
			return
		}
		if err := wldata.PostJSON("/save?file=i18n/"+wc.rightLang+".json", data); err != nil {
			G.Logf(1, "wlate: failed to save text data: %v", err)
			return
		}
	}

	// Save inflections: project to data-only.
	if len(wc.rightInflections) > 0 {
		datas := make([]wi18n.FlexEntryData, len(wc.rightInflections))
		for i, r := range wc.rightInflections {
			datas[i] = r.FlexEntryData
		}
		data, err := json.MarshalIndent(datas, "", "  ")
		if err != nil {
			G.Logf(1, "wlate: failed to marshal inflection data: %v", err)
			return
		}
		if err := wldata.PostJSON("/save?file=i18n/"+wc.rightLang+".inflections.json", data); err != nil {
			G.Logf(1, "wlate: failed to save inflection data: %v", err)
			return
		}
	}

	wc.dirty = false
	G.Logf(2, "wlate: saved %s (%d text, %d inflections)",
		wc.rightLang, len(wc.rightText), len(wc.rightInflections))
}

func (wc *wlateCtx) doLangSwitch() {
	if wc.pendingSide == "left" {
		wc.leftLang = wc.pendingLang
		wc.obj.This.Set("left_lang", wc.leftLang)
		wc.loadLeftData()
	} else {
		wc.rightLang = wc.pendingLang
		wc.obj.This.Set("right_lang", wc.rightLang)
		wc.loadRightData()
		wc.buildFilteredIdx()
		wc.currentPos = 0
		wc.updateStatusBar()
	}
	wc.displayRecord()
}

func (wc *wlateCtx) handleKeyboard(event js.Value) {
	switch {
	case matchKey(event, wc.getKey("prev")):
		event.Call("preventDefault")
		wc.navigate(wc.currentPos - 1)
	case matchKey(event, wc.getKey("next")):
		event.Call("preventDefault")
		wc.navigate(wc.currentPos + 1)
	case matchKey(event, wc.getKey("prev10")):
		event.Call("preventDefault")
		wc.navigate(wc.currentPos - 10)
	case matchKey(event, wc.getKey("next10")):
		event.Call("preventDefault")
		wc.navigate(wc.currentPos + 10)
	case matchKey(event, wc.getKey("first")):
		event.Call("preventDefault")
		wc.navigate(0)
	case matchKey(event, wc.getKey("last")):
		event.Call("preventDefault")
		wc.navigate(len(wc.filteredIdx) - 1)
	case matchKey(event, wc.getKey("save")):
		event.Call("preventDefault")
		go wc.save()
	case matchKey(event, wc.getKey("revised")):
		event.Call("preventDefault")
		wc.toggleRevised()
	}
}
