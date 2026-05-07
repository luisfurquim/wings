//go:build js && wasm

// Package skinswitcher provides a skin-switcher custom element that
// lets the user pick among the registered wprana skins at runtime.
//
// The widget enumerates skins via wprana.ListSkins(), pre-selects the
// one returned by wprana.ActiveSkin(), and calls wprana.ApplySkin on
// every <select> change. The skin swap is reflected immediately in
// every wprana custom element on the page through the cascade of
// --wings-* tokens injected at :root.
//
// # Usage in parent template
//
//	<skin-switcher></skin-switcher>
//
// No attributes or triggers; it is fully self-contained. Sort order of
// the dropdown is alphabetical.
//
// # CSS Customization
//
// Implements wprana.Customizable. CSS is split into "Vars" and "Design"
// parts; defaults reference the global --wings-* tokens with fallbacks,
// so the switcher always matches the active skin.
package skinswitcher

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wprana"
)

const elementTag = "skin-switcher"

// G is the logger for this module.
var G goose.Alert

//go:embed skinswitcher.html
var htmlContent string

//go:embed vars.css
var varsCSS string

//go:embed design.css
var designCSS string

var cssParts = []wprana.CSSPart{
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

// New creates a new SkinSwitcher instance.
func New() *SkinSwitcher {
	return &SkinSwitcher{}
}

func init() {
	G.Set(3)
	cssParts[0].Content = varsCSS
	cssParts[1].Content = designCSS
	wprana.Register(
		elementTag,
		htmlContent,
		buildCSS(),
		func() wprana.PranaMod { return &SkinSwitcher{} },
	)
	G.Logf(3, "skin-switcher: module registered\n")
}

// SkinSwitcher implements wprana.PranaMod and wprana.Customizable.
type SkinSwitcher struct {
	obj *wprana.PranaObj
}

var _ wprana.Customizable = (*SkinSwitcher)(nil)

func (s *SkinSwitcher) ListCSS() []wprana.CSSPart {
	result := make([]wprana.CSSPart, len(cssParts))
	copy(result, cssParts)
	return result
}

func (s *SkinSwitcher) ReplaceCSS(key, content string) {
	for i := range cssParts {
		if cssParts[i].Name == key {
			cssParts[i].Content = content
			wprana.Update(elementTag, buildCSS())
			return
		}
	}
	G.Logf(1, "ReplaceCSS: key %q not found\n", key)
}

func (s *SkinSwitcher) InitData() map[string]any {
	return map[string]any{
		"skin_options": "[]",
		"active_skin":  "",
	}
}

func (s *SkinSwitcher) Render(obj *wprana.PranaObj) {
	s.obj = obj

	names := wprana.ListSkins()
	sort.Strings(names)

	opts := make([]map[string]string, len(names))
	for i, name := range names {
		opts[i] = map[string]string{"label": name, "value": name}
	}
	optsJSON, _ := json.Marshal(opts)
	obj.This.Set("skin_options", string(optsJSON))

	if active := wprana.ActiveSkin(); active != "" {
		obj.This.Set("active_skin", active)
	}

	obj.This.M["on_change"] = wprana.TriggerHandler(func(args ...any) {
		if len(args) == 0 {
			return
		}
		selected, ok := args[0].([]any)
		if !ok || len(selected) == 0 {
			return
		}
		m, ok := selected[0].(map[string]any)
		if !ok {
			return
		}
		next, ok := m["value"].(string)
		if !ok {
			return
		}
		if !wprana.ApplySkin(next) {
			G.Logf(1, "skin-switcher: ApplySkin(%q) failed\n", next)
		}
	})
}
