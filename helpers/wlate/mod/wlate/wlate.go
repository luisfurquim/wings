//go:build js && wasm

package wlate

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/dom"
)

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

// ── Data types ─────────────────────────────────────────────────────────────

type Config struct {
	DefaultLang string      `json:"defaultLang"`
	Languages   []string    `json:"languages"`
	Wlate       WlateConfig `json:"wlate"`
}

type WlateConfig struct {
	Keys map[string]string `json:"keys"`
}

type TextRecord struct {
	Content   string `json:"content"`
	Revised   bool   `json:"revised"`
	Context   string `json:"context"`
	CtxDetail string `json:"ctxdetail,omitempty"`
}

type InflectionRecord struct {
	Expr      string            `json:"expr"`
	Context   string            `json:"context"`
	CtxDetail string            `json:"ctxdetail,omitempty"`
	Revised   bool              `json:"revised"`
	Forms     map[string]string `json:"forms"`
}

// ── Context detail humanization ────────────────────────────────────────────

var ctxLabels = map[string]string{
	"title":      "Título da página",
	"header":     "Cabeçalho",
	"footer":     "Rodapé",
	"caption":    "Legenda de tabela",
	"button":     "Botão",
	"label":      "Rótulo de campo",
	"th":         "Cabeçalho de coluna",
	"nav":        "Área de navegação",
	"legend":     "Legenda de formulário",
	"figcaption": "Legenda de figura",
	"summary":    "Resumo expansível",
	"a":          "Texto de link",
	"option":     "Opção de seleção",
	"abbr":       "Abreviação",
	"dt":         "Termo de definição",
}

func humanizeCtx(detail string) string {
	if label, ok := ctxLabels[detail]; ok {
		return label
	}
	return detail
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

// ── Component ──────────────────────────────────────────────────────────────

type Wlate struct{}

func (w *Wlate) InitData() map[string]any {
	return map[string]any{
		// Config
		"default_lang": "",
		"lang_options": "[]",

		// Current state
		"left_lang":  "",
		"right_lang": "",
		"tab":        "text",
		"filter_unrev": false,

		// Status bar
		"pending_count": 0,
		"total_count":   0,

		// Current record display
		"left_content":   "",
		"right_content":  "",
		"context":        "",
		"ctxdetail_text": "",
		"is_revised":     false,
		"nav_input":      "1",

		// Inflection tab
		"inflection_expr": "",
		"genders":         []any{},
		"categories":      []any{},
		"left_cells":      []any{},
		"right_cells":     []any{},

		// Dialog
		"show_dialog": false,
	}
}

// ── Internal state ─────────────────────────────────────────────────────────

type wlateCtx struct {
	obj              *wprana.PranaObj
	config           Config
	keys             map[string]string
	leftLang         string
	rightLang        string
	leftText         []TextRecord
	rightText        []TextRecord
	leftInflections  []InflectionRecord
	rightInflections []InflectionRecord
	filteredIdx      []int
	currentPos       int // position in filteredIdx
	dirty            bool
	pendingLang      string
	pendingSide      string
	filterUnrev      bool
	activeTab        string // "text" or "inflection"
	curGenders       []string // current inflection grid column keys
	curCategories    []string // current inflection grid row keys
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

// displayRecord shows the record at the current position.
func (wc *wlateCtx) displayRecord() {
	idx := wc.actualIdx()
	if idx < 0 {
		wc.obj.This.Set("left_content", "")
		wc.obj.This.Set("right_content", "")
		wc.obj.This.Set("context", "")
		wc.obj.This.Set("ctxdetail_text", "")
		wc.obj.This.Set("is_revised", false)
		wc.obj.This.Set("nav_input", "0")
		wc.updateRevisedBorder(false)
		return
	}

	wc.obj.This.Set("nav_input", strconv.Itoa(idx+1))

	if wc.activeTab == "inflection" {
		wc.displayInflectionRecord(idx)
	} else {
		wc.displayTextRecord(idx)
	}
	wc.updateStatusBar()
}

func (wc *wlateCtx) displayTextRecord(idx int) {
	var leftCtx, leftDetail, leftContent string
	if idx < len(wc.leftText) {
		leftContent = wc.leftText[idx].Content
		leftCtx = wc.leftText[idx].Context
		leftDetail = humanizeCtx(wc.leftText[idx].CtxDetail)
	}

	var rightContent string
	var revised bool
	var rightCtx, rightDetail string
	if idx < len(wc.rightText) {
		rightContent = wc.rightText[idx].Content
		revised = wc.rightText[idx].Revised
		rightCtx = wc.rightText[idx].Context
		rightDetail = humanizeCtx(wc.rightText[idx].CtxDetail)
	}

	wc.obj.This.Set("left_content", leftContent)
	wc.obj.This.Set("right_content", rightContent)
	// Use left context if right is empty (context is shared)
	ctx := rightCtx
	if ctx == "" {
		ctx = leftCtx
	}
	detail := rightDetail
	if detail == "" {
		detail = leftDetail
	}
	wc.obj.This.Set("context", ctx)
	wc.obj.This.Set("ctxdetail_text", detail)
	wc.obj.This.Set("is_revised", revised)
	wc.updateRevisedBorder(revised)
}

func (wc *wlateCtx) displayInflectionRecord(idx int) {
	var leftForms map[string]string
	var expr, ctx, detail string

	if idx < len(wc.leftInflections) {
		leftForms = wc.leftInflections[idx].Forms
		expr = wc.leftInflections[idx].Expr
		ctx = wc.leftInflections[idx].Context
		detail = humanizeCtx(wc.leftInflections[idx].CtxDetail)
	}

	var rightForms map[string]string
	var revised bool
	if idx < len(wc.rightInflections) {
		rightForms = wc.rightInflections[idx].Forms
		revised = wc.rightInflections[idx].Revised
		if expr == "" {
			expr = wc.rightInflections[idx].Expr
		}
		if ctx == "" {
			ctx = wc.rightInflections[idx].Context
		}
		if detail == "" {
			detail = humanizeCtx(wc.rightInflections[idx].CtxDetail)
		}
	}

	wc.obj.This.Set("inflection_expr", expr)
	wc.obj.This.Set("context", ctx)
	wc.obj.This.Set("ctxdetail_text", detail)
	wc.obj.This.Set("is_revised", revised)
	wc.updateRevisedBorder(revised)

	// Extract genders and categories from the union of both sides' form keys
	genderSet := map[string]bool{}
	catSet := map[string]bool{}
	allForms := []map[string]string{leftForms, rightForms}
	for _, forms := range allForms {
		for key := range forms {
			parts := strings.SplitN(key, ".", 2)
			if len(parts) == 2 {
				genderSet[parts[0]] = true
				catSet[parts[1]] = true
			}
		}
	}

	genders := sortedKeys(genderSet)
	categories := sortedCLDR(catSet)

	wc.curGenders = genders
	wc.curCategories = categories

	gendersAny := make([]any, len(genders))
	for i, g := range genders {
		gendersAny[i] = g
	}
	categoriesAny := make([]any, len(categories))
	for i, c := range categories {
		categoriesAny[i] = c
	}

	// Build cell arrays: left_cells[ci].cols[gi] and right_cells[ci].cols[gi]
	leftCells := make([]any, len(categories))
	rightCells := make([]any, len(categories))
	for ci, cat := range categories {
		lcols := make([]any, len(genders))
		rcols := make([]any, len(genders))
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
		}
		leftCells[ci] = map[string]any{"cols": lcols}
		rightCells[ci] = map[string]any{"cols": rcols}
	}

	wc.obj.This.Set("genders", gendersAny)
	wc.obj.This.Set("categories", categoriesAny)
	wc.obj.This.Set("left_cells", leftCells)
	wc.obj.This.Set("right_cells", rightCells)

	// Set CSS grid columns dynamically: 1 row-header + N gender columns
	gridCols := fmt.Sprintf("auto repeat(%d, 1fr)", len(genders))
	for _, gridID := range []string{"#wl-grid-left", "#wl-grid-right"} {
		grids := dom.Query(wc.obj.Dom, gridID)
		for _, g := range grids {
			g.Get("style").Set("gridTemplateColumns", gridCols)
		}
	}
}

// updateRevisedBorder sets/removes the wl-revised class on the right panel.
func (wc *wlateCtx) updateRevisedBorder(revised bool) {
	var panelID string
	if wc.activeTab == "inflection" {
		panelID = "#wl-right-inflection-panel"
	} else {
		panelID = "#wl-right-panel"
	}
	panels := dom.Query(wc.obj.Dom, panelID)
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

// syncRightContent writes the current textarea content back to the right data.
// Reads directly from the DOM textarea since there is no &value two-way binding.
func (wc *wlateCtx) syncRightContent() {
	idx := wc.actualIdx()
	if idx < 0 {
		return
	}
	if wc.activeTab == "text" && idx < len(wc.rightText) {
		textareas := dom.Query(wc.obj.Dom, ".wl-content-edit")
		if len(textareas) == 0 {
			return
		}
		content := textareas[0].Get("value").String()
		if content != wc.rightText[idx].Content {
			wc.rightText[idx].Content = content
			wc.rightText[idx].Revised = true
			wc.dirty = true
		}
	}
	if wc.activeTab == "inflection" && idx < len(wc.rightInflections) {
		inputs := dom.Query(wc.obj.Dom, ".wl-cell-input")
		changed := false
		for _, inp := range inputs {
			ci := inp.Get("dataset").Get("ci").String()
			gi := inp.Get("dataset").Get("gi").String()
			ciN, err1 := strconv.Atoi(ci)
			giN, err2 := strconv.Atoi(gi)
			if err1 != nil || err2 != nil {
				continue
			}
			if ciN >= len(wc.curCategories) || giN >= len(wc.curGenders) {
				continue
			}
			key := wc.curGenders[giN] + "." + wc.curCategories[ciN]
			val := inp.Get("value").String()
			if wc.rightInflections[idx].Forms[key] != val {
				wc.rightInflections[idx].Forms[key] = val
				changed = true
			}
		}
		if changed {
			wc.rightInflections[idx].Revised = true
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
			// Find position of this index in filteredIdx
			for pos, fi := range wc.filteredIdx {
				if fi == i {
					return pos
				}
			}
			// Not in filteredIdx (shouldn't happen unless filtered to only revised)
			// Rebuild filter to include it
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

// ── Sort helpers ───────────────────────────────────────────────────────────

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// sortedCLDR sorts CLDR categories in canonical order.
var cldrOrder = map[string]int{
	"zero": 0, "one": 1, "two": 2, "few": 3, "many": 4, "other": 5,
}

func sortedCLDR(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		oi, oki := cldrOrder[out[i]]
		oj, okj := cldrOrder[out[j]]
		if oki && okj {
			return oi < oj
		}
		if oki {
			return true
		}
		if okj {
			return false
		}
		return out[i] < out[j]
	})
	return out
}

// ── Fetch helpers ──────────────────────────────────────────────────────────

type fetchResult struct {
	body string
	err  error
}

func fetchText(url string) (string, error) {
	ch := make(chan fetchResult, 1)

	var thenFn, catchFn, textThen, textCatch js.Func

	textThen = js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- fetchResult{body: args[0].String()}
		return nil
	})
	textCatch = js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- fetchResult{err: fmt.Errorf("fetch text error: %s", jsErrMsg(args))}
		return nil
	})
	thenFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		resp := args[0]
		if !resp.Get("ok").Bool() {
			ch <- fetchResult{err: fmt.Errorf("fetch %s: status %d", url, resp.Get("status").Int())}
			return nil
		}
		resp.Call("text").Call("then", textThen).Call("catch", textCatch)
		return nil
	})
	catchFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- fetchResult{err: fmt.Errorf("fetch %s: %s", url, jsErrMsg(args))}
		return nil
	})

	js.Global().Call("fetch", url).Call("then", thenFn).Call("catch", catchFn)
	r := <-ch
	thenFn.Release()
	catchFn.Release()
	textThen.Release()
	textCatch.Release()
	return r.body, r.err
}

func postJSON(url string, data []byte) error {
	ch := make(chan error, 1)

	headers := js.Global().Get("Object").New()
	headers.Set("Content-Type", "application/json")

	opts := js.Global().Get("Object").New()
	opts.Set("method", "POST")
	opts.Set("headers", headers)

	// Create Uint8Array from Go bytes
	jsBody := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(jsBody, data)
	opts.Set("body", jsBody)

	var thenFn, catchFn js.Func
	thenFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		resp := args[0]
		if !resp.Get("ok").Bool() {
			ch <- fmt.Errorf("save failed: status %d", resp.Get("status").Int())
			return nil
		}
		ch <- nil
		return nil
	})
	catchFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- fmt.Errorf("save error: %s", jsErrMsg(args))
		return nil
	})

	js.Global().Call("fetch", url, opts).Call("then", thenFn).Call("catch", catchFn)
	err := <-ch
	thenFn.Release()
	catchFn.Release()
	return err
}

func jsErrMsg(args []js.Value) string {
	if len(args) == 0 {
		return "unknown error"
	}
	v := args[0]
	if v.IsUndefined() || v.IsNull() {
		return "unknown error"
	}
	msg := v.Get("message")
	if msg.IsUndefined() || msg.IsNull() {
		return v.String()
	}
	return msg.String()
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
		// Try case-insensitive for single letters
		if len(key) == 1 && strings.EqualFold(event.Get("key").String(), key) {
			// ok
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
		activeTab:   "text",
		filteredIdx: make([]int, 0, 256),
		keys:        map[string]string{},
	}

	go wc.init()
}

func (wc *wlateCtx) init() {
	// Load config
	body, err := fetchText("wprana.json")
	if err != nil {
		fmt.Println("wlate: failed to load wprana.json:", err)
		return
	}
	if err := json.Unmarshal([]byte(body), &wc.config); err != nil {
		fmt.Println("wlate: failed to parse wprana.json:", err)
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

	// Build initial filter and display
	wc.buildFilteredIdx()
	wc.displayRecord()
	wc.updateStatusBar()

	// Wire up event handlers
	wc.wireEvents()
}

func (wc *wlateCtx) loadLeftData() {
	wc.leftText = nil
	wc.leftInflections = nil

	body, err := fetchText("i18n/" + wc.leftLang + ".json")
	if err == nil {
		json.Unmarshal([]byte(body), &wc.leftText)
	}

	body, err = fetchText("i18n/" + wc.leftLang + ".inflections.json")
	if err == nil {
		json.Unmarshal([]byte(body), &wc.leftInflections)
	}
}

func (wc *wlateCtx) loadRightData() {
	wc.rightText = nil
	wc.rightInflections = nil
	wc.dirty = false

	body, err := fetchText("i18n/" + wc.rightLang + ".json")
	if err == nil {
		json.Unmarshal([]byte(body), &wc.rightText)
	}
	// If file doesn't exist, create empty structure mirroring left side
	if wc.rightText == nil && len(wc.leftText) > 0 {
		wc.rightText = make([]TextRecord, len(wc.leftText))
		for i, lt := range wc.leftText {
			wc.rightText[i] = TextRecord{
				Context:   lt.Context,
				CtxDetail: lt.CtxDetail,
			}
		}
	}

	body, err = fetchText("i18n/" + wc.rightLang + ".inflections.json")
	if err == nil {
		json.Unmarshal([]byte(body), &wc.rightInflections)
	}
	// If file doesn't exist, create empty structure mirroring left side
	if wc.rightInflections == nil && len(wc.leftInflections) > 0 {
		wc.rightInflections = make([]InflectionRecord, len(wc.leftInflections))
		for i, li := range wc.leftInflections {
			forms := make(map[string]string, len(li.Forms))
			for key := range li.Forms {
				forms[key] = ""
			}
			wc.rightInflections[i] = InflectionRecord{
				Expr:      li.Expr,
				Context:   li.Context,
				CtxDetail: li.CtxDetail,
				Forms:     forms,
			}
		}
	}
}

func (wc *wlateCtx) wireEvents() {
	// Tab buttons
	tabText := dom.Query(wc.obj.Dom, "#tab-text")
	tabInflection := dom.Query(wc.obj.Dom, "#tab-inflection")
	if len(tabText) > 0 {
		dom.AddEvent(tabText[0], "click", func(_ js.Value, _ []js.Value) any {
			wc.switchTab("text")
			return nil
		}, false, false)
	}
	if len(tabInflection) > 0 {
		dom.AddEvent(tabInflection[0], "click", func(_ js.Value, _ []js.Value) any {
			wc.switchTab("inflection")
			return nil
		}, false, false)
	}

	// Navigation buttons
	navHandlers := map[string]func(){
		"#nav-first":  func() { wc.navigate(0) },
		"#nav-prev10": func() { wc.navigate(wc.currentPos - 10) },
		"#nav-prev":   func() { wc.navigate(wc.currentPos - 1) },
		"#nav-next":   func() { wc.navigate(wc.currentPos + 1) },
		"#nav-next10": func() { wc.navigate(wc.currentPos + 10) },
		"#nav-last":   func() { wc.navigate(len(wc.filteredIdx) - 1) },
	}
	for sel, handler := range navHandlers {
		h := handler // capture
		btns := dom.Query(wc.obj.Dom, sel)
		if len(btns) > 0 {
			dom.AddEvent(btns[0], "click", func(_ js.Value, _ []js.Value) any {
				h()
				return nil
			}, false, false)
		}
	}

	// Nav input — jump to record on Enter
	navInputs := dom.Query(wc.obj.Dom, "#nav-input")
	if len(navInputs) > 0 {
		dom.AddEvent(navInputs[0], "keydown", func(_ js.Value, args []js.Value) any {
			if args[0].Get("key").String() == "Enter" {
				val := navInputs[0].Get("value").String()
				num, err := strconv.Atoi(strings.TrimSpace(val))
				if err != nil || num < 1 {
					return nil
				}
				// num is 1-based actual record number, find its position in filteredIdx
				targetIdx := num - 1
				for pos, fi := range wc.filteredIdx {
					if fi == targetIdx {
						wc.navigate(pos)
						return nil
					}
				}
				// If not in filtered list, navigate to closest
				wc.navigate(0)
			}
			return nil
		}, false, false)
	}

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

	// Revised toggle (text tab)
	revisedToggles := dom.Query(wc.obj.Dom, "#wl-revised-toggle")
	if len(revisedToggles) > 0 {
		dom.AddEvent(revisedToggles[0], "click", func(_ js.Value, _ []js.Value) any {
			wc.toggleRevised()
			return nil
		}, false, false)
	}

	// Revised toggle (inflection tab)
	revisedTogglesInfl := dom.Query(wc.obj.Dom, "#wl-revised-toggle-inflection")
	if len(revisedTogglesInfl) > 0 {
		dom.AddEvent(revisedTogglesInfl[0], "click", func(_ js.Value, _ []js.Value) any {
			wc.toggleRevised()
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

	// Dialog buttons
	dlgSave := dom.Query(wc.obj.Dom, "#dlg-save")
	dlgDiscard := dom.Query(wc.obj.Dom, "#dlg-discard")
	dlgCancel := dom.Query(wc.obj.Dom, "#dlg-cancel")
	if len(dlgSave) > 0 {
		dom.AddEvent(dlgSave[0], "click", func(_ js.Value, _ []js.Value) any {
			wc.obj.This.Set("show_dialog", false)
			go func() {
				wc.save()
				wc.doLangSwitch()
			}()
			return nil
		}, false, false)
	}
	if len(dlgDiscard) > 0 {
		dom.AddEvent(dlgDiscard[0], "click", func(_ js.Value, _ []js.Value) any {
			wc.obj.This.Set("show_dialog", false)
			wc.dirty = false
			wc.doLangSwitch()
			return nil
		}, false, false)
	}
	if len(dlgCancel) > 0 {
		dom.AddEvent(dlgCancel[0], "click", func(_ js.Value, _ []js.Value) any {
			wc.obj.This.Set("show_dialog", false)
			return nil
		}, false, false)
	}

	// Textarea input tracking for dirty state
	textareas := dom.Query(wc.obj.Dom, ".wl-content-edit")
	if len(textareas) > 0 {
		dom.AddEvent(textareas[0], "input", func(_ js.Value, _ []js.Value) any {
			wc.dirty = true
			return nil
		}, false, false)
	}

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

func (wc *wlateCtx) switchTab(tab string) {
	if tab == wc.activeTab {
		return
	}
	wc.syncRightContent()
	wc.activeTab = tab
	wc.obj.This.Set("tab", tab)

	// Update tab button styles
	tabTexts := dom.Query(wc.obj.Dom, "#tab-text")
	tabInflections := dom.Query(wc.obj.Dom, "#tab-inflection")
	if len(tabTexts) > 0 && len(tabInflections) > 0 {
		if tab == "text" {
			tabTexts[0].Get("classList").Call("add", "wl-tab-active")
			tabInflections[0].Get("classList").Call("remove", "wl-tab-active")
		} else {
			tabInflections[0].Get("classList").Call("add", "wl-tab-active")
			tabTexts[0].Get("classList").Call("remove", "wl-tab-active")
		}
	}

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

	// Save text data
	if len(wc.rightText) > 0 {
		data, err := json.MarshalIndent(wc.rightText, "", "  ")
		if err != nil {
			fmt.Println("wlate: failed to marshal text data:", err)
			return
		}
		err = postJSON("/save?file=i18n/"+wc.rightLang+".json", data)
		if err != nil {
			fmt.Println("wlate: failed to save text data:", err)
			return
		}
	}

	// Save inflection data
	if len(wc.rightInflections) > 0 {
		data, err := json.MarshalIndent(wc.rightInflections, "", "  ")
		if err != nil {
			fmt.Println("wlate: failed to marshal inflection data:", err)
			return
		}
		err = postJSON("/save?file=i18n/"+wc.rightLang+".inflections.json", data)
		if err != nil {
			fmt.Println("wlate: failed to save inflection data:", err)
			return
		}
	}

	wc.dirty = false
	fmt.Printf("wlate: saved %s (%d text, %d inflections)\n",
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
