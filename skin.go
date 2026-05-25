//go:build js && wasm

package wprana

import (
	"syscall/js"
)

// ── Global skin registry (multi-skin composition) ───────────────────────────
//
// A "skin" is a CSS payload — a set of `--wings-*` custom property
// definitions at `:root` — together with a SkinCategory bitmask declaring
// which design dimensions it touches (Identity, Geometry, Depth, Motion,
// Interaction, Typography, Spacing, Lighting, Atmosphere).
//
// Multiple skins can be active simultaneously **provided their bitmasks
// are disjoint**. This lets a complete theme (e.g. mushroom — cores +
// geometria + profundidade + …) compose with a focused skin (e.g. glass
// — apenas atmosfera). Two themes that both declare CategoryIdentity
// will not coexist; ApplySkin returns *SkinConflictError* in that case.
//
// Each active skin owns its own `<style id="wprana-skin-NAME"
// data-wprana-skin="NAME">` element in document.head. The DOM order
// equals the activation order; later activations cascade over earlier
// ones in case the same property is set in two skins (which the bitmask
// gate already prevents — but the cascade still acts as a safety net).
//
// Widgets reference tokens via fallbacks so they remain functional with
// no skin active, e.g. `color: var(--wings-text, #222);`.

const skinStylePrefix = "wprana-skin-"

type skinEntry struct {
	name       string
	categories SkinCategory
	css        string
}

// SkinInfo is the public, immutable view of a registered skin.
type SkinInfo struct {
	Name       string
	Categories SkinCategory
}

var (
	skins             = map[string]*skinEntry{}
	activeSkins       []string // ordered, FIFO of activations
	skinChangeHooks   []func()
)

// RegisterSkin registers a named skin with its category bitmask and CSS
// payload. Call from init() in the skin package.
//
// Re-registering an existing name overwrites the previous entry and
// logs a level-1 warning.
func RegisterSkin(name string, categories SkinCategory, css string) {
	if _, exists := skins[name]; exists {
		G.Logf(1, "RegisterSkin: skin %q already registered, overwriting\n", name)
	}
	skins[name] = &skinEntry{
		name:       name,
		categories: categories,
		css:        css,
	}
}

// ListSkins returns the registered skin names (unsorted).
func ListSkins() []string {
	names := make([]string, 0, len(skins))
	for k := range skins {
		names = append(names, k)
	}
	return names
}

// ListSkinInfos returns every registered skin with its declared
// categories. Unsorted; the caller may sort by Name as needed.
func ListSkinInfos() []SkinInfo {
	out := make([]SkinInfo, 0, len(skins))
	for _, e := range skins {
		out = append(out, SkinInfo{Name: e.name, Categories: e.categories})
	}
	return out
}

// SkinCategoriesOf returns the bitmask declared by name, or
// (CategoryNone, false) if the skin is not registered.
func SkinCategoriesOf(name string) (SkinCategory, bool) {
	e, ok := skins[name]
	if !ok {
		return CategoryNone, false
	}
	return e.categories, true
}

// ActiveSkins returns the names of currently active skins, in
// activation order (oldest first).
func ActiveSkins() []string {
	out := make([]string, len(activeSkins))
	copy(out, activeSkins)
	return out
}

// ActiveSkin returns the most recently activated skin, or "" when no
// skin is active. Kept for compatibility with callers that assume a
// single active skin (e.g. simple combobox UIs).
func ActiveSkin() string {
	if len(activeSkins) == 0 {
		return ""
	}
	return activeSkins[len(activeSkins)-1]
}

// ActiveCategories returns the OR of every active skin's bitmask.
func ActiveCategories() SkinCategory {
	var c SkinCategory
	for _, n := range activeSkins {
		if e, ok := skins[n]; ok {
			c |= e.categories
		}
	}
	return c
}

// ConflictsWith returns the names of active skins whose categories
// overlap categories (i.e. activating a new skin with categories would
// fail because of these). The list is in activation order.
func ConflictsWith(categories SkinCategory) []string {
	var out []string
	for _, n := range activeSkins {
		e, ok := skins[n]
		if !ok {
			continue
		}
		if e.categories.Conflicts(categories) {
			out = append(out, n)
		}
	}
	return out
}

// ApplySkin activates name. If a skin with the same name is already
// active, ApplySkin is a no-op and returns nil.
//
// Returns *SkinNotRegisteredError if name is unknown, or
// *SkinConflictError if any active skin shares a category bit with
// name's declared categories.
func ApplySkin(name string) error {
	entry, ok := skins[name]
	if !ok {
		G.Logf(1, "ApplySkin: skin %q not registered\n", name)
		return &SkinNotRegisteredError{Name: name}
	}
	// Idempotent: re-applying an active skin is a success.
	for _, n := range activeSkins {
		if n == name {
			return nil
		}
	}
	// Conflict check: any active skin sharing bits?
	var conflicts []string
	var conflictingBits SkinCategory
	for _, n := range activeSkins {
		ae := skins[n]
		if ae.categories.Conflicts(entry.categories) {
			conflicts = append(conflicts, n)
			conflictingBits |= ae.categories & entry.categories
		}
	}
	if len(conflicts) > 0 {
		err := &SkinConflictError{
			Name:                  name,
			Categories:            entry.categories,
			Conflicts:             conflicts,
			ConflictingCategories: conflictingBits,
		}
		G.Logf(1, "ApplySkin: %s\n", err.Error())
		return err
	}
	if !injectSkinCSS(name, entry.css) {
		return nil // document not available — nothing to do
	}
	activeSkins = append(activeSkins, name)
	G.Logf(3, "ApplySkin: %q activated (categories=%s, %d bytes)\n",
		name, entry.categories, len(entry.css))
	notifySkinChange()
	return nil
}

// DeactivateSkin removes name from the set of active skins. Returns
// nil whether or not name was active. Returns *SkinNotRegisteredError
// only when name is not a registered skin name.
func DeactivateSkin(name string) error {
	if _, ok := skins[name]; !ok {
		return &SkinNotRegisteredError{Name: name}
	}
	idx := -1
	for i, n := range activeSkins {
		if n == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}
	removeSkinStyle(name)
	activeSkins = append(activeSkins[:idx], activeSkins[idx+1:]...)
	G.Logf(3, "DeactivateSkin: %q deactivated\n", name)
	notifySkinChange()
	return nil
}

// ClearSkins deactivates every active skin.
func ClearSkins() {
	if len(activeSkins) == 0 {
		return
	}
	// Iterate over a copy because removeSkinStyle does not touch the slice.
	for _, name := range append([]string(nil), activeSkins...) {
		removeSkinStyle(name)
	}
	activeSkins = activeSkins[:0]
	G.Logf(3, "ClearSkins: all deactivated\n")
	notifySkinChange()
}

// ClearSkin is a deprecated alias for ClearSkins, kept so legacy
// single-skin callers do not break.
//
// Deprecated: use ClearSkins.
func ClearSkin() { ClearSkins() }

// OnSkinChange registers fn to be invoked after every successful
// ApplySkin / DeactivateSkin / ClearSkins. Used by the skinswitcher
// widget to keep its UI in sync with programmatic activations.
//
// The callback runs synchronously on the JS goroutine; do not block.
func OnSkinChange(fn func()) {
	if fn == nil {
		return
	}
	skinChangeHooks = append(skinChangeHooks, fn)
}

func notifySkinChange() {
	for _, fn := range skinChangeHooks {
		fn()
	}
}

// ── DOM helpers ─────────────────────────────────────────────────────────────

// injectSkinCSS appends a <style> element scoped to name, holding css.
// Returns false when document is not available (e.g. during unit tests).
func injectSkinCSS(name, css string) bool {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return false
	}
	id := skinStylePrefix + name
	style := doc.Call("getElementById", id)
	if !style.Truthy() {
		style = doc.Call("createElement", "style")
		style.Set("id", id)
		style.Call("setAttribute", "data-wprana-skin", name)
		doc.Get("head").Call("appendChild", style)
	}
	style.Set("textContent", css)
	return true
}

// removeSkinStyle drops the <style id="wprana-skin-NAME"> element.
func removeSkinStyle(name string) {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	style := doc.Call("getElementById", skinStylePrefix+name)
	if style.Truthy() {
		style.Call("remove")
	}
}
