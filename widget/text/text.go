//go:build js && wasm

// Package text provides a w-text rich-text editor custom element for wings.
//
// w-text is a pluggable contenteditable editor whose every write passes
// through the epubhtml content policy (package epubhtml), so forbidden
// markup cannot exist in its content by construction. The editing engine
// lives in package wtext; this package is the widget shell: the w-input
// family anatomy (label/field/feedback, sizes, form participation), a
// toolbar rendered from the active profile with wings' own widgets, and
// the i18n/a11y wiring.
//
// # Dependencies
//
// The toolbar renders w-button and w-combobox elements, so an app using
// w-text must also register those widgets (widget/button, widget/combobox).
// A profile with InputItem controls (StyleToolbar, for one) also renders
// w-input (widget/input) inside the item's popover.
//
// # Usage in parent template
//
//	<w-text label="Biography" profile="basic"
//	        helper="Tell readers about yourself."
//	        &value="{{bio}}">
//	</w-text>
//
// value binds a wings.FieldCodec (e.g. field.NewText()); the codec's
// String seeds the editor and Get reads it back on blur. A plain string
// value works too (read-only-ish: replaced wholesale on external change).
//
// # Attributes
//
//   - label       — label text (or slot="label" for rich content)
//   - profile     — registered wtext profile name (default: "basic")
//   - placeholder — shown while the editor is empty
//   - helper      — helper text below the field (or slot="helper")
//   - error        — error message (or slot="error")
//   - required    — "true" shows the required mark
//   - disabled    — standard HTML; makes the surface non-editable
//
// # Events fired to parent
//
//	@change — fires on blur after the two-way write-back, so a bound
//	          FieldCodec/Validator has already run; args[0] = epub-html string
//	@input  — fires (coalesced) as content changes; args[0] = epub-html string
//
// # Native form participation
//
// w-text is form-associated: its serialized content is submitted under the
// host name, a bound Validator gates submission, form.reset() restores the
// initial content and clears undo history, and an ancestor
// <fieldset disabled> disables the surface.
package text

import (
	_ "embed"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
	"github.com/luisfurquim/wings/wtext"
)

const elementTag = "w-text"

// G is the logger for this module.
var G goose.Alert

//go:embed text.html
var htmlContent string

//go:embed vars.css
var varsCSS string

//go:embed design.css
var designCSS string

var cssParts = []wings.CSSPart{
	{Name: "Vars", Content: ""},
	{Name: "Design", Content: ""},
}

func buildCSS() string {
	var sb strings.Builder
	for _, p := range cssParts {
		sb.WriteString(p.Content)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// New creates a new Text instance.
func New() *Text { return &Text{} }

func init() {
	G.Set(3)
	cssParts[0].Content = varsCSS
	cssParts[1].Content = designCSS
	// Ensure a "basic" profile exists even if the app did not register one,
	// so <w-text> works out of the box.
	if _, ok := wtext.ProfileFor("basic"); !ok {
		wtext.RegisterProfile("basic", wtext.Profile{
			Toolbar: []wtext.ToolbarPlugin{wtext.BasicToolbar{}},
		})
	}
	wings.RegisterWithOpts(
		elementTag,
		htmlContent,
		buildCSS(),
		wings.ComponentOpts{FormAssociated: true},
		func() wings.PranaMod { return &Text{} },
		"label", "profile", "placeholder", "helper", "error", "required", "disabled",
	)
	wings.OnFormReset(elementTag, formReset)
	wings.OnFormDisabled(elementTag, formDisabled)
	// On a SetLang switch the slotted toolbar labels are re-translated in
	// place; re-render the toolbar so the buttons pick up the new language.
	wings.OnRetranslate(elementTag, retranslate)
	// Release the editor's document-level listener and own MutationObserver
	// when the instance leaves the DOM (dom auto-release cannot reach them).
	wings.OnDisconnect(elementTag, disconnect)
	G.Logf(3, "w-text: module registered\n")
}

// Text implements wings.PranaMod and wings.Customizable for w-text.
type Text struct{}

var _ wings.Customizable = (*Text)(nil)

// ListCSS returns the named CSS parts in order.
func (t *Text) ListCSS() []wings.CSSPart {
	result := make([]wings.CSSPart, len(cssParts))
	copy(result, cssParts)
	return result
}

// ReplaceCSS replaces a CSS part and updates live instances.
func (t *Text) ReplaceCSS(key, content string) {
	for i := range cssParts {
		if cssParts[i].Name == key {
			cssParts[i].Content = content
			wings.Update(elementTag, buildCSS())
			return
		}
	}
	G.Logf(1, "ReplaceCSS: key %q not found\n", key)
}

// InitData seeds the template bindings.
func (t *Text) InitData() map[string]any {
	return map[string]any{
		"label":       "",
		"profile":     "basic",
		"placeholder": "",
		"helper":      "",
		"error":       "",
		"required":    false,
		"value":       "",
	}
}

// editors maps a host element's node identity to its live Editor, so
// lifecycle hooks (reset/disabled/disconnect) reach the right instance.
// Keyed by the host's _pranaNodeID string.
var editors = map[string]*binding{}

// binding ties a host element to its Editor, toolbar and initial content.
type binding struct {
	host    js.Value
	editor  *wtext.Editor
	tb      *toolbar // nil when the profile renders no toolbar
	initial string   // mount-time content, the form-reset default
}

func (t *Text) Render(obj *wings.PranaObj) {
	editES := dom.Query(obj.Dom, ".wt-editor")
	toolbarES := dom.Query(obj.Dom, ".wt-toolbar")
	if len(editES) == 0 {
		return
	}
	editEl := editES[0]

	if lbl, _ := obj.This.Get("label").(string); lbl != "" {
		obj.Element.Call("setAttribute", "data-has-label", "")
	}
	if ph, _ := obj.This.Get("placeholder").(string); ph != "" {
		editEl.Call("setAttribute", "data-placeholder", ph)
	}

	profName, _ := obj.This.Get("profile").(string)
	if profName == "" {
		profName = "basic"
	}
	prof, ok := wtext.ProfileFor(profName)
	if !ok {
		G.Logf(1, "w-text: unknown profile %q; falling back to basic\n", profName)
		prof, _ = wtext.ProfileFor("basic")
	}

	editor, err := wtext.New(editEl, prof)
	if err != nil {
		G.Logf(1, "w-text: editor init failed: %v\n", err)
		return
	}

	// Plugin setup runs before content loads: the utility classes a
	// toolbar defines (wtext.InitPlugin) must exist when a stored document
	// meets the class filter, or its class attributes would be stripped.
	runPluginInits(prof, editor)

	// Seed content from the bound value (a FieldCodec's String or a plain
	// string). Hostile input like any other: SetContent runs it through the
	// filter, so a stored-XSS value cannot execute.
	initial := valueString(obj)
	if initial != "" {
		if err := editor.SetContent(initial); err != nil {
			G.Logf(1, "w-text: initial content rejected: %v\n", err)
		}
	}

	b := &binding{host: obj.Element, editor: editor, initial: editor.Content()}
	if id, ok := nodeKey(obj.Element); ok {
		editors[id] = b
	}
	setFormValue(obj.Element, editor.Content())

	// Reflect disabled to the editing surface.
	applyDisabled(obj.Element, editEl, obj.Element.Call("hasAttribute", "disabled").Bool())

	// Render the toolbar (and the side menu, when the profile declares
	// menu items) and keep state fresh on selection changes.
	if len(toolbarES) > 0 {
		menu := js.Undefined()
		if menuES := dom.Query(obj.Dom, ".wt-menu"); len(menuES) > 0 {
			menu = menuES[0]
		}
		tb := newToolbar(obj, toolbarES[0], menu, editor, prof)
		b.tb = tb
		editor.OnSelectionChange(tb.refresh)
		tb.render()
	}

	wireEditorEvents(obj, editEl, editor)
}

// runPluginInits runs the wtext.InitPlugin hook of every plugin in the
// profile, whatever its category. A failed init is logged and the plugin
// runs degraded (its classes missing), never killing the widget.
func runPluginInits(prof wtext.Profile, editor *wtext.Editor) {
	initOne := func(p any) {
		if ip, ok := p.(wtext.InitPlugin); ok {
			if err := ip.Init(editor); err != nil {
				G.Logf(1, "w-text: plugin init failed: %v\n", err)
			}
		}
	}
	for _, p := range prof.Toolbar {
		initOne(p)
	}
	for _, p := range prof.Edition {
		initOne(p)
	}
	for _, p := range prof.Clipboard {
		initOne(p)
	}
}

// wireEditorEvents connects focus/blur/input to host state, form value and
// the parent-facing @change/@input triggers.
func wireEditorEvents(obj *wings.PranaObj, editEl js.Value, editor *wtext.Editor) {
	dom.AddEvent(editEl, "focus", func(_ js.Value, _ []js.Value) any {
		obj.Element.Call("setAttribute", "data-focused", "")
		return nil
	}, false, false)

	dom.AddEvent(editEl, "blur", func(_ js.Value, _ []js.Value) any {
		obj.Element.Call("removeAttribute", "data-focused")
		content := editor.Content()
		// Read-on-blur: expose the value and fire change so the parent's
		// two-way &value binding writes back (the moment a bound
		// FieldCodec/Validator runs), then the widget-facing @change.
		obj.Element.Set("value", content)
		setFormValue(obj.Element, content)
		obj.Element.Call("dispatchEvent", js.Global().Get("Event").New("change"))
		obj.Trigger("change", content)
		return nil
	}, false, false)

	// Coalesced input signal for reactive parents (not the &value write-back).
	dom.AddEvent(editEl, "input", func(_ js.Value, _ []js.Value) any {
		content := editor.Content()
		setFormValue(obj.Element, content)
		setEmptyState(obj.Element, editor)
		obj.Trigger("input", content)
		return nil
	}, false, false)

	setEmptyState(obj.Element, editor)
}

// valueString extracts the initial content string from the bound value: a
// fmt.Stringer (FieldCodec) or a plain string. An empty binding falls
// through to the host `value` property, which carries the content when
// the element is created programmatically (or on re-render).
func valueString(obj *wings.PranaObj) string {
	switch s := obj.This.Get("value").(type) {
	case interface{ String() string }:
		return s.String()
	case string:
		if s != "" {
			return s
		}
	}
	if p := obj.Element.Get("value"); p.Type() == js.TypeString {
		return p.String()
	}
	return ""
}

// setEmptyState mirrors emptiness to host attributes for CSS hooks.
func setEmptyState(host js.Value, editor *wtext.Editor) {
	if editor.IsEmpty() {
		host.Call("setAttribute", "data-empty", "")
		host.Call("removeAttribute", "data-has-value")
	} else {
		host.Call("removeAttribute", "data-empty")
		host.Call("setAttribute", "data-has-value", "")
	}
}

// setFormValue keeps the submitted form value current.
func setFormValue(host js.Value, content string) {
	if internals := host.Get("_internals"); internals.Truthy() {
		internals.Call("setFormValue", content)
	}
}

// nodeKey returns the host's stable prana node id (_pranaId, assigned by
// the runtime when the instance is set up), used to key the editor
// registry across the lifecycle hooks. _pranaId is a JS number: read it as
// an int (Value.Call on a number panics — a panic is total loss here).
func nodeKey(host js.Value) (string, bool) {
	if v := host.Get("_pranaId"); v.Type() == js.TypeNumber {
		return strconv.Itoa(v.Int()), true
	}
	return "", false
}

// bindingFor returns the binding registered for host.
func bindingFor(host js.Value) (*binding, bool) {
	id, ok := nodeKey(host)
	if !ok {
		return nil, false
	}
	b, ok := editors[id]
	return b, ok
}
