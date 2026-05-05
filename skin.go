//go:build js && wasm

package wprana

import (
	"syscall/js"
)

// ── Global skin registry ────────────────────────────────────────────────────
//
// A "skin" is a single CSS string that defines the global wprana design
// tokens (the --wings-* custom properties documented in skins/tokens.md).
// Skin packages register themselves at init() time via RegisterSkin; the
// host application then activates one with ApplySkin.
//
// Activation injects (or replaces) a single <style id="wprana-skin"> element
// in document.head. Because CSS custom properties cascade through the shadow
// boundary from the host element down, defining tokens at :root makes them
// visible inside every wprana custom element — provided the widget does NOT
// shadow them with its own :host { --wings-X: ... } rule.
//
// Widgets should reference tokens with a fallback so they remain functional
// when no skin is active:
//
//	color: var(--wings-text, #222);
//
// Per-widget defaults belong in those fallback positions, not in :host blocks.

const skinStyleID = "wprana-skin"

var (
	skins      = map[string]string{}
	activeSkin string
)

// RegisterSkin registers a named skin — a CSS payload that defines the
// global --wings-* tokens. Call from init() in the skin package.
//
//	name - identifier used by ApplySkin (e.g. "light", "dark")
//	css  - CSS source; typically a single :root { ... } block
//
// A second registration under the same name overwrites the first and logs
// a level-1 warning; this allows tests/demos to swap a skin's contents
// without restarting, but flags accidental collisions.
func RegisterSkin(name, css string) {
	if _, exists := skins[name]; exists {
		G.Logf(1, "RegisterSkin: skin %q already registered, overwriting\n", name)
	}
	skins[name] = css
}

// ListSkins returns the registered skin names (unsorted).
func ListSkins() []string {
	names := make([]string, 0, len(skins))
	for k := range skins {
		names = append(names, k)
	}
	return names
}

// ActiveSkin returns the name of the currently applied skin, or "" if none.
func ActiveSkin() string {
	return activeSkin
}

// ApplySkin activates the named skin by injecting its CSS into a single
// <style id="wprana-skin"> element in document.head. If a previous skin
// was active, its <style> element is replaced in place; the element is
// reused so the cascade is preserved without flicker.
//
// Returns false if the named skin is not registered (no change is made).
func ApplySkin(name string) bool {
	css, ok := skins[name]
	if !ok {
		G.Logf(1, "ApplySkin: skin %q not registered\n", name)
		return false
	}
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return false
	}
	style := doc.Call("getElementById", skinStyleID)
	if !style.Truthy() {
		style = doc.Call("createElement", "style")
		style.Set("id", skinStyleID)
		doc.Get("head").Call("appendChild", style)
	}
	style.Set("textContent", css)
	activeSkin = name
	G.Logf(3, "ApplySkin: %q applied (%d bytes)\n", name, len(css))
	return true
}

// ClearSkin removes the active skin's <style> element from document.head.
// Tokens fall back to widget-local defaults provided via var(name, fallback).
func ClearSkin() {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	style := doc.Call("getElementById", skinStyleID)
	if style.Truthy() {
		style.Call("remove")
	}
	activeSkin = ""
}
