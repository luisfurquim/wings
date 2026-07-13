//go:build js && wasm

// Package combobox provides a w-combobox custom element for wings.
//
// Features:
//   - Multi-select (default) with tag display, OR single-select that
//     behaves like a native <select> — picked via the mode attribute.
//   - Typing filters the dropdown list (case-insensitive, substring match)
//   - Enter with text not in the list fires the @notinlist event to the parent
//   - Enter with text matching an option selects that option
//   - Escape clears the input and closes the dropdown
//   - Click outside the widget closes the dropdown
//
// # Usage in parent template
//
//	<w-combobox
//	    options='["Alpha","Beta","Gamma"]'
//	    placeholder="Type to filter..."
//	    mode="multi"
//	    @notinlist="on_notinlist"
//	    @change="on_change">
//	</w-combobox>
//
// The options attribute accepts either:
//   - JSON array of strings:  ["A","B","C"]
//   - JSON array of objects:  [{"label":"A","value":"a"},...]
//
// The mode attribute accepts:
//   - "multi"  (default) — multiple selections shown as removable tags;
//     selecting clears the input and adds a tag.
//   - "single"           — exactly zero or one selection; selecting an
//     option replaces any previous selection and the chosen label is
//     shown directly in the input. Tag display is hidden via CSS.
//
// # Events fired to parent (all lowercase — HTML spec lowercases attribute names)
//
//	@notinlist  — Enter pressed with text absent from the option list
//	              args[0] = the typed string
//	@change     — selection changed (add or remove)
//	              args[0] = []any of currently selected {label, value} maps
//	              In single mode the slice has 0 or 1 element.
//
// # CSS Customization
//
// Combobox implements wings.Customizable. CSS is split into two parts:
//   - "Vars"   — CSS custom properties (colors, shadows). Replace this to
//     change the color scheme without affecting layout.
//   - "Design" — Layout and structure rules using var() references.
//
// Example:
//
//	mod := combobox.New()  // or any instance from the factory
//	mod.ReplaceCSS("Vars", myDarkThemeVars)
package combobox

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
)

const elementTag = "w-combobox"

// G is the logger for this module.
var G goose.Alert

//go:embed combobox.html
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

// New creates a new Combobox instance.
// Exported so that applications can call ListCSS/ReplaceCSS
// without waiting for the factory.
func New() *Combobox {
	return &Combobox{}
}

func init() {
	G.Set(3)
	cssParts[0].Content = varsCSS
	cssParts[1].Content = designCSS
	wings.Register(
		elementTag,
		htmlContent,
		buildCSS(),
		func() wings.PranaMod { return &Combobox{} },
		"options", "placeholder", "value", "mode",
	)
	G.Logf(3, "w-combobox: module registered\n")
}

// Combobox implements wings.PranaMod and wings.Customizable
// for the w-combobox custom element.
type Combobox struct{}

// Compile-time interface check.
var _ wings.Customizable = (*Combobox)(nil)

// ListCSS returns the named CSS parts in order.
// Modifying the returned slice does not affect the component.
func (c *Combobox) ListCSS() []wings.CSSPart {
	result := make([]wings.CSSPart, len(cssParts))
	copy(result, cssParts)
	return result
}

// ReplaceCSS replaces the CSS part identified by key and updates
// all live instances via wings.Update.
func (c *Combobox) ReplaceCSS(key string, content string) {
	for i := range cssParts {
		if cssParts[i].Name == key {
			cssParts[i].Content = content
			wings.Update(elementTag, buildCSS())
			return
		}
	}
	G.Logf(1, "ReplaceCSS: key %q not found\n", key)
}

func (c *Combobox) InitData() map[string]any {
	return map[string]any{
		// "options" is populated by the observed attribute of the same name.
		// It holds a JSON string parsed by loadOptions().
		"options":          "",
		"all_options":      []any{},
		"filtered_options": []any{},
		"selected_items":   []any{},
		"input_val":        "",
		"placeholder":      "Type to filter...",
		"mode":             "multi",
	}
}

// isSingle reports whether the widget is in single-select mode.
func (cb *cbCtx) isSingle() bool {
	m, _ := cb.obj.This.Get("mode").(string)
	return m == "single"
}

// parseOptions converts the JSON string from the options attribute into a
// normalised []any where every element is map[string]any{"label":…,"value":…}.
// Accepts either []string or []{"label":string,"value":string}. An optional
// "font" key rides along: the dropdown previews that option in its own
// typeface (see paintFonts).
func parseOptions(raw string) []any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []any{}
	}

	// Try a plain string array first.
	var strs []string
	if json.Unmarshal([]byte(raw), &strs) == nil {
		result := make([]any, len(strs))
		for i, s := range strs {
			result[i] = map[string]any{"label": s, "value": s}
		}
		return result
	}

	// Try an object array.
	var objs []map[string]any
	if json.Unmarshal([]byte(raw), &objs) == nil {
		result := make([]any, len(objs))
		for i, o := range objs {
			var label, value string
			if l, ok := o["label"].(string); ok {
				label = l
			}
			// A present "value" key wins even when its string is empty — a
			// single-mode picker may declare a real, selectable "None"/
			// "Default" option with value "". Only an ABSENT key derives
			// value from the label (the plain-string-array convenience).
			// `v != ""` here would silently coerce an explicit empty value
			// into the label text, making that option unreachable by its
			// real identity.
			if raw, present := o["value"]; present {
				if v, ok := raw.(string); ok {
					value = v
				} else {
					value = label
				}
			} else {
				value = label
			}
			norm := map[string]any{"label": label, "value": value}
			if f, ok := o["font"].(string); ok && f != "" {
				norm["font"] = f
			}
			result[i] = norm
		}
		return result
	}

	return []any{}
}

// cbCtx holds the runtime state shared across all event handlers
// of a single combobox instance.
type cbCtx struct {
	obj          *wings.PranaObj
	inp          js.Value
	dropWrap     js.Value
	selectedVals map[string]bool
	lastRaw      string
	lastValue    string
}

// showDrop opens the dropdown. On browsers with the Popover API the list
// is promoted to the top layer: any ancestor with overflow (a tab panel,
// the editor's own box) clips a merely position:absolute dropdown, which
// used to cut its tail off near a container's bottom edge — no maxHeight
// clamp can fix that, because the clip happens outside the widget. A
// top-layer popover is fixed-positioned, so it is anchored to the field's
// viewport rect, and its height is clamped to the space below (with a
// floor). Browsers without the API keep the absolute in-flow dropdown.
func (cb *cbCtx) showDrop() {
	style := cb.dropWrap.Get("style")
	if cb.dropWrap.Get("showPopover").Type() == js.TypeFunction {
		// All geometry derives from the ANCHOR's rect (the .cb-root): the
		// dropdown's own rect is useless here — a closed [popover] is
		// display:none !important per the UA sheet, so measuring it before
		// showPopover() yields zeros (which once inflated the height clamp
		// past the viewport).
		anchor := cb.dropWrap.Get("parentElement").Call("getBoundingClientRect")
		top := anchor.Get("bottom").Float() + 4
		style.Set("position", "fixed")
		style.Set("inset", "auto")
		style.Set("left", fmt.Sprintf("%.0fpx", anchor.Get("left").Float()))
		style.Set("top", fmt.Sprintf("%.0fpx", top))
		style.Set("width", fmt.Sprintf("%.0fpx", anchor.Get("width").Float()))
		style.Set("margin", "0")
		style.Set("maxHeight", fmt.Sprintf("%.0fpx", clampDropHeight(js.Global().Get("innerHeight").Float()-top-8)))
		style.Set("overflowY", "auto")
		style.Set("display", "block")
		if !cb.dropWrap.Call("matches", ":popover-open").Bool() {
			cb.dropWrap.Call("showPopover")
		}
		return
	}
	// No Popover API: the absolute in-flow dropdown, clamped to the space
	// below it in the viewport (ancestors with overflow may still clip it).
	style.Set("display", "block")
	rect := cb.dropWrap.Call("getBoundingClientRect")
	style.Set("maxHeight", fmt.Sprintf("%.0fpx", clampDropHeight(js.Global().Get("innerHeight").Float()-rect.Get("top").Float()-8)))
	style.Set("overflowY", "auto")
}

// clampDropHeight bounds the open list's height: a floor keeps it usable
// when the field sits near the viewport bottom, a ceiling keeps a long
// option list from swallowing the screen.
func clampDropHeight(free float64) float64 {
	if free < 96 {
		return 96
	}
	if free > 320 {
		return 320
	}
	return free
}

func (cb *cbCtx) hideDrop() {
	if cb.dropWrap.Get("hidePopover").Type() == js.TypeFunction &&
		cb.dropWrap.Call("matches", ":popover-open").Bool() {
		cb.dropWrap.Call("hidePopover")
	}
	cb.dropWrap.Get("style").Set("display", "none")
}

// inputFocused reports whether the inner input is its shadow root's
// active element — i.e., the user is in the field right now.
func (cb *cbCtx) inputFocused() bool {
	root := cb.inp.Call("getRootNode")
	if !root.Truthy() {
		return false
	}
	return root.Get("activeElement").Equal(cb.inp)
}

// applyFilter rebuilds filtered_options from all_options, excluding
// already-selected values and applying a case-insensitive substring filter.
//
// In single mode the current selection is NOT excluded from the dropdown,
// matching native <select> semantics where every option is reachable.
func (cb *cbCtx) applyFilter(query string) {
	query = strings.ToLower(strings.TrimSpace(query))
	single := cb.isSingle()
	var allOpts []any
	if v, ok := cb.obj.This.Get("all_options").([]any); ok {
		allOpts = v
	}
	filtered := make([]any, 0, len(allOpts))
	for _, opt := range allOpts {
		m, ok := opt.(map[string]any)
		if !ok {
			continue
		}
		val, ok := m["value"].(string)
		if !ok {
			continue
		}
		if !single && cb.selectedVals[val] {
			continue
		}
		if query == "" {
			filtered = append(filtered, m)
			continue
		}
		label, ok := m["label"].(string)
		if !ok {
			continue
		}
		if strings.Contains(strings.ToLower(label), query) {
			filtered = append(filtered, m)
		}
	}
	cb.obj.This.Set("filtered_options", filtered)
	cb.paintFonts(filtered)
}

// paintFonts previews each rendered option in its own typeface, when its
// entry carries a "font". Deliberately a PROPERTY assignment
// (style.fontFamily), never a style-attribute interpolation: the browser
// parses a property value as exactly one value, so a hostile string
// cannot smuggle extra declarations (url(...) exfiltration and friends)
// past it — it just fails to parse and the option renders in the default
// face. Runs right after filtered_options lands: the reactive loop has
// already rebuilt the .cb-opt elements by the time Set returns.
func (cb *cbCtx) paintFonts(filtered []any) {
	opts := dom.Query(cb.obj.Dom, ".cb-opt")
	for _, el := range opts {
		fi := el.Call("getAttribute", "data-fi")
		if !fi.Truthy() {
			continue
		}
		i, err := strconv.Atoi(fi.String())
		if err != nil || i < 0 || i >= len(filtered) {
			continue
		}
		font := ""
		if m, ok := filtered[i].(map[string]any); ok {
			font, _ = m["font"].(string)
		}
		el.Get("style").Set("fontFamily", font)
	}
}

// loadOptions parses the options attribute (JSON) into all_options.
// It is a no-op when nothing relevant changed since the last call.
//
// In single mode the external `value` attribute is authoritative: if it
// disagrees with the current internal selection (e.g., the parent reverted
// after the user cancelled a pending switch in a confirmation dialog, or a
// continuously-driven picker reflects a live cursor position) the selection
// is silently re-synced to match. This path does not fire @change. An empty
// val participates too — a parent may register an option with value ""
// (a "None"/"Default" choice) and drive the display back to it; when no
// such option exists resyncSingleValue simply finds no match and is a no-op,
// so plain pickers without an empty-valued option are unaffected.
func (cb *cbCtx) loadOptions() {
	var raw string
	if v, ok := cb.obj.This.Get("options").(string); ok {
		raw = v
	}
	val, _ := cb.obj.This.Get("value").(string)

	rawChanged := raw != cb.lastRaw
	valChanged := val != cb.lastValue
	singleDrift := cb.isSingle() && !cb.selectedVals[val]

	if !rawChanged && !valChanged && !singleDrift {
		return
	}
	if rawChanged {
		cb.lastRaw = raw
		cb.obj.This.Set("all_options", parseOptions(raw))
		cb.applyFilter(cb.inputVal())
	}
	cb.lastValue = val

	if singleDrift {
		cb.resyncSingleValue(val)
		return
	}
	cb.applyValuePreset(val)
}

// resyncSingleValue replaces the single-mode selection with the option whose
// value matches val. Silent — does not fire @change — so a parent revert via
// the bound `value` attribute can recover the combobox without re-entering
// the change handler that opened the confirmation dialog in the first place.
func (cb *cbCtx) resyncSingleValue(val string) {
	opts, ok := cb.obj.This.Get("all_options").([]any)
	if !ok {
		return
	}
	for _, o := range opts {
		m, ok := o.(map[string]any)
		if !ok {
			continue
		}
		if v, _ := m["value"].(string); v == val {
			cb.selectedVals = map[string]bool{val: true}
			cb.obj.This.Set("selected_items", []any{m})
			label, _ := m["label"].(string)
			cb.obj.This.Set("input_val", label)
			cb.applyFilter("")
			// This resync often lands asynchronously (the sentinel's
			// MutationObserver) a frame AFTER the field was focused —
			// the repaint above rewrites the input and silently kills
			// onFocus's select-all, so the user's next paste APPENDS to
			// the label instead of replacing it. Re-select to keep the
			// type-to-replace affordance alive.
			if cb.inputFocused() {
				cb.inp.Call("select")
			}
			return
		}
	}
}

// applyValuePreset silently pre-selects the option whose value matches val,
// but only when no item is currently selected. Does not fire @change so that
// programmatic initialisation does not trigger parent reload handlers.
// In single mode the matched label is shown in the input.
func (cb *cbCtx) applyValuePreset(val string) {
	if val == "" || len(cb.selectedVals) > 0 {
		return
	}
	opts, ok := cb.obj.This.Get("all_options").([]any)
	if !ok || len(opts) == 0 {
		return
	}
	for _, o := range opts {
		m, ok := o.(map[string]any)
		if !ok {
			continue
		}
		if v, ok := m["value"].(string); ok && v == val {
			cb.selectedVals[val] = true
			existing, _ := cb.obj.This.Get("selected_items").([]any)
			cb.obj.This.Set("selected_items", append(existing, m))
			if cb.isSingle() {
				label, _ := m["label"].(string)
				cb.obj.This.Set("input_val", label)
			} else {
				cb.obj.This.Set("input_val", "")
			}
			cb.applyFilter("")
			// Same select-all preservation as resyncSingleValue: never
			// leave a focused field deselected after a silent rewrite.
			if cb.isSingle() && cb.inputFocused() {
				cb.inp.Call("select")
			}
			return
		}
	}
}

// inputVal reads the current input_val from the reactive data.
func (cb *cbCtx) inputVal() string {
	if v, ok := cb.obj.This.Get("input_val").(string); ok {
		return v
	}
	return ""
}

// selectItem commits an option as the selection.
//
// In multi mode it appends to selected_items and clears the input.
// In single mode it replaces any previous selection and writes the
// chosen label into the input so it is shown like a native <select>.
// Either way @change is fired with the resulting selected_items slice.
func (cb *cbCtx) selectItem(m map[string]any) {
	val, ok := m["value"].(string)
	if !ok {
		return
	}
	if cb.isSingle() {
		// Re-selecting the same value is a no-op (avoids spurious @change).
		if cb.selectedVals[val] && len(cb.selectedVals) == 1 {
			cb.hideDrop()
			return
		}
		// Replace previous selection.
		cb.selectedVals = map[string]bool{val: true}
		cb.obj.This.Set("selected_items", []any{m})
		label, _ := m["label"].(string)
		cb.obj.This.Set("input_val", label)
		cb.hideDrop()
		cb.applyFilter("")
		cb.obj.Trigger("change", cb.obj.This.Get("selected_items"))
		return
	}
	if cb.selectedVals[val] {
		return
	}
	cb.selectedVals[val] = true
	cb.obj.This.Append("selected_items", m)
	cb.obj.This.Set("input_val", "")
	cb.hideDrop()
	cb.applyFilter("")
	cb.obj.Trigger("change", cb.obj.This.Get("selected_items"))
}

// removeItem removes the selected item at index si and fires @change.
func (cb *cbCtx) removeItem(si int) {
	selected, ok := cb.obj.This.Get("selected_items").([]any)
	if !ok {
		return
	}
	if si < 0 || si >= len(selected) {
		return
	}
	m, ok := selected[si].(map[string]any)
	if !ok {
		return
	}
	if val, ok := m["value"].(string); ok {
		delete(cb.selectedVals, val)
	}
	cb.obj.This.DeleteAt("selected_items", si)
	if cb.isSingle() {
		// Removing the (only) tag in single mode clears the input too.
		cb.obj.This.Set("input_val", "")
	}
	cb.applyFilter(cb.inputVal())
	cb.obj.Trigger("change", cb.obj.This.Get("selected_items"))
}

// onFocus reloads options and opens the dropdown. In single mode the
// input text (the current selection's label) is select-all'd so the
// user can type to replace it without manually clearing first.
func (cb *cbCtx) onFocus(_ js.Value, _ []js.Value) any {
	cb.loadOptions()
	if cb.isSingle() {
		cb.applyFilter("")
	} else {
		cb.applyFilter(cb.inputVal())
	}
	cb.showDrop()
	if cb.isSingle() && cb.inp.Get("value").String() != "" {
		cb.inp.Call("select")
	}
	return nil
}

// onInput filters the dropdown as the user types. The typed value is
// synced into the model FIRST: the &value two-way binding only syncs
// DOM→data on `change` (blur), and applyFilter's own Set re-renders the
// template — which repaints this very input from the model. With a stale
// model that repaint ERASED each keystroke as it was typed.
func (cb *cbCtx) onInput(_ js.Value, _ []js.Value) any {
	val := cb.inp.Get("value").String()
	cb.obj.This.Set("input_val", val)
	cb.applyFilter(val)
	cb.showDrop()
	return nil
}

// onKeydown handles Enter (select or notinlist) and Escape (clear and close).
func (cb *cbCtx) onKeydown(_ js.Value, args []js.Value) any {
	key := args[0].Get("key").String()
	switch key {
	case "Enter":
		// The @notinlist/@change handlers may move focus (w-text's
		// RestoreSel returns it to the editor) DURING this dispatch; the
		// browser then delivers the key's default action to the newly
		// focused element — a contenteditable receives a paragraph break
		// out of nowhere. Kill the default before triggering anything.
		args[0].Call("preventDefault")
		val := strings.TrimSpace(cb.inp.Get("value").String())
		if val == "" {
			return nil
		}
		valLower := strings.ToLower(val)
		var filtered []any
		if v, ok := cb.obj.This.Get("filtered_options").([]any); ok {
			filtered = v
		}
		for _, opt := range filtered {
			m, ok := opt.(map[string]any)
			if !ok {
				continue
			}
			label, ok := m["label"].(string)
			if !ok {
				continue
			}
			if strings.ToLower(label) == valLower {
				cb.selectItem(m)
				return nil
			}
		}
		// No exact match — clear input, close dropdown, notify parent.
		cb.obj.This.Set("input_val", "")
		cb.hideDrop()
		cb.obj.Trigger("notinlist", val)

	case "Escape":
		// In single mode keep the selected label visible after Esc; in
		// multi mode the input is a transient filter, so clear it.
		if cb.isSingle() {
			selected, _ := cb.obj.This.Get("selected_items").([]any)
			label := ""
			if len(selected) > 0 {
				if m, ok := selected[0].(map[string]any); ok {
					label, _ = m["label"].(string)
				}
			}
			cb.obj.This.Set("input_val", label)
			cb.hideDrop()
			cb.applyFilter(label)
			return nil
		}
		cb.obj.This.Set("input_val", "")
		cb.hideDrop()
		cb.applyFilter("")
	}
	return nil
}

// onRootClick is a delegated click handler covering both option selection
// (.cb-opt) and tag removal (.cb-rm).
func (cb *cbCtx) onRootClick(_ js.Value, args []js.Value) any {
	event := args[0]
	// Prevent clicks inside the combobox from reaching the document handler
	// that would close the dropdown.
	event.Call("stopPropagation")
	el := event.Get("target")
	for !el.IsNull() && !el.IsUndefined() {
		cls := el.Get("className").String()

		if strings.Contains(cls, "cb-opt") {
			fi, err := strconv.Atoi(el.Get("dataset").Get("fi").String())
			if err != nil {
				return nil
			}
			var filtered []any
			if v, ok := cb.obj.This.Get("filtered_options").([]any); ok {
				filtered = v
			}
			if fi >= 0 && fi < len(filtered) {
				if m, ok := filtered[fi].(map[string]any); ok {
					cb.selectItem(m)
				}
			}
			return nil
		}

		if strings.Contains(cls, "cb-rm") {
			si, err := strconv.Atoi(el.Get("dataset").Get("si").String())
			if err != nil {
				return nil
			}
			cb.removeItem(si)
			return nil
		}

		el = el.Get("parentElement")
	}
	return nil
}

// onDocClick closes the dropdown when clicking outside the component.
func (cb *cbCtx) onDocClick(_ js.Value, _ []js.Value) any {
	cb.hideDrop()
	return nil
}

func (c *Combobox) Render(obj *wings.PranaObj) {
	// Query stable elements — none of them are guarded by a ? conditional,
	// so they are always present in the shadow DOM at Render time.
	inps := dom.Query(obj.Dom, ".cb-input")
	roots := dom.Query(obj.Dom, ".cb-root")
	dropWraps := dom.Query(obj.Dom, ".cb-drop-wrap")
	sentinels := dom.Query(obj.Dom, ".cb-sentinel")
	if len(inps) == 0 || len(roots) == 0 || len(dropWraps) == 0 {
		return
	}

	cb := &cbCtx{
		obj:          obj,
		inp:          inps[0],
		dropWrap:     dropWraps[0],
		selectedVals: map[string]bool{},
	}
	// Top-layer dropdown (see showDrop). "manual" keeps light-dismiss off:
	// open/close stays fully owned by this widget's own handlers.
	cb.dropWrap.Call("setAttribute", "popover", "manual")

	// Parse options that may already be present via the attribute.
	cb.loadOptions()

	// Watch the sentinel for reactive updates to options/value that arrive
	// after Render() — parent typically sets these after connectedCallback.
	if len(sentinels) > 0 {
		onAttrChange := js.FuncOf(func(_ js.Value, _ []js.Value) any {
			cb.loadOptions()
			return nil
		})
		mo := js.Global().Get("MutationObserver").New(onAttrChange)
		mo.Call("observe", sentinels[0], map[string]any{"attributes": true})
	}

	// Register event handlers.
	dom.AddEvent(cb.inp, "focus", cb.onFocus, false, false)
	dom.AddEvent(cb.inp, "input", cb.onInput, false, false)
	dom.AddEvent(cb.inp, "keydown", cb.onKeydown, false, false)
	dom.AddEvent(roots[0], "click", cb.onRootClick, false, false)

	// Document: close dropdown when clicking outside the component.
	// This handler persists on the document even if the component is later
	// removed from the DOM.  In that scenario it becomes a harmless no-op because
	// wings stops syncing disconnected components.
	doc := js.Global().Get("document")
	dom.AddEvent(doc, "click", cb.onDocClick, false, false)
}
