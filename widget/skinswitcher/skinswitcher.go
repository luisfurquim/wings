//go:build js && wasm

// Package skinswitcher provides the <skin-switcher> custom element.
//
// V2 differences from v1:
//
//   - Multi-skin: skins compose along disjoint categories. The widget
//     shows checkboxes (one per registered skin) instead of a single
//     combobox so the user can stack a focused skin (glass) on top of a
//     complete theme (mushroom).
//   - Conflict detection: when a candidate skin shares a category with
//     an active skin, its checkbox is disabled and an inline note shows
//     which skin it conflicts with.
//   - External-change sync: the widget registers a callback with
//     wprana.OnSkinChange so programmatic Apply/Deactivate calls keep the
//     UI in sync without manual refresh.
//
// # Usage in parent template
//
//	<skin-switcher></skin-switcher>
//
// No attributes or trigger handlers required. Skins are listed in
// alphabetical order; a small badge displays the count of active skins.
package skinswitcher

import (
	_ "embed"
	"sort"
	"strings"
	"syscall/js"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wprana"
	"github.com/luisfurquim/wprana/dom"
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
		"skin_list":         []any{},
		"active_count":      0,
		"active_cats_label": "",
	}
}

// rebuildList computes the per-skin item view-model and pushes it into
// the reactive data. Called on Render and after every change.
//
// Skins that would conflict with active ones are NOT disabled — clicking
// them auto-deactivates the conflicting actives (handled in onChange).
// The view-model carries a "replaces X" hint so the user knows what is
// about to happen before clicking.
func (s *SkinSwitcher) rebuildList() {
	infos := wprana.ListSkinInfos()
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })

	active := wprana.ActiveSkins()
	activeSet := make(map[string]bool, len(active))
	for _, n := range active {
		activeSet[n] = true
	}

	items := make([]any, 0, len(infos))
	for _, info := range infos {
		conflicts := []string{}
		if !activeSet[info.Name] {
			conflicts = wprana.ConflictsWith(info.Categories)
		}
		hint := ""
		if len(conflicts) > 0 {
			hint = "replaces " + strings.Join(conflicts, ", ")
		}
		items = append(items, map[string]any{
			"name":     info.Name,
			"cats":     strings.Join(info.Categories.Names(), " · "),
			"conflict": hint, // template field name kept for backwards compat
			"_active":  activeSet[info.Name],
		})
	}

	s.obj.This.Set("skin_list", items)
	s.obj.This.Set("active_count", len(active))

	cats := wprana.ActiveCategories()
	if cats == wprana.CategoryNone {
		s.obj.This.Set("active_cats_label", "")
	} else {
		s.obj.This.Set("active_cats_label", "Active categories: "+cats.String())
	}
}

// applyCheckboxStates walks the rendered list and sets the .checked
// DOM property. The template engine does not bind boolean attributes
// reactively, so we apply them imperatively after each rebuild.
// Conflicting skins are NOT disabled — clicking one auto-replaces.
func (s *SkinSwitcher) applyCheckboxStates() {
	items, _ := s.obj.This.Get("skin_list").([]any)
	cbs := dom.Query(s.obj.Dom, ".ssw-cb")
	if len(cbs) != len(items) {
		// Sync may race with the template render; re-schedule on next tick.
		js.Global().Call("requestAnimationFrame", js.FuncOf(func(_ js.Value, _ []js.Value) any {
			s.applyCheckboxStates()
			return nil
		}))
		return
	}
	for i, raw := range items {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		active, _ := m["_active"].(bool)
		cbs[i].Set("checked", active)
		cbs[i].Set("disabled", false)
	}
}

// onChange handles a checkbox change event. Identifies which skin via
// data-skin and toggles its activation.
//
// On *check*, if the new skin shares a category with any currently
// active skin, the conflicting actives are deactivated first so the
// new one can apply. This is what the user expects from a "switcher":
// clicking a different theme replaces the previous one rather than
// refusing the action.
//
// On *uncheck*, simply deactivate.
//
// Errors leave the model untouched and the UI is re-synced to truth.
func (s *SkinSwitcher) onChange(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return nil
	}
	target := args[0].Get("target")
	if !target.Truthy() {
		return nil
	}
	tagName := target.Get("tagName")
	if tagName.IsUndefined() || !strings.EqualFold(tagName.String(), "INPUT") {
		return nil
	}
	name := target.Call("getAttribute", "data-skin")
	if name.IsNull() || name.IsUndefined() || name.String() == "" {
		return nil
	}
	skin := name.String()
	checked := target.Get("checked").Bool()

	var err error
	if checked {
		// Auto-replace: deactivate any active skin that shares a category
		// with the candidate before applying. SkinCategoriesOf returns the
		// declared bitmask; ConflictsWith narrows to the actives that
		// overlap. Deactivation errors are logged but do not abort the
		// flow — the subsequent Apply will surface a real problem.
		if cats, ok := wprana.SkinCategoriesOf(skin); ok {
			for _, conflicting := range wprana.ConflictsWith(cats) {
				if derr := wprana.DeactivateSkin(conflicting); derr != nil {
					G.Logf(1, "skin-switcher: deactivate %q before %q: %s\n",
						conflicting, skin, derr.Error())
				}
			}
		}
		err = wprana.ApplySkin(skin)
	} else {
		err = wprana.DeactivateSkin(skin)
	}
	if err != nil {
		G.Logf(1, "skin-switcher: %s\n", err.Error())
		// On error, OnSkinChange will not fire. Rebuild explicitly so
		// the optimistic checkbox flip done by the browser is reverted
		// to match real model state.
		s.rebuildList()
		s.applyCheckboxStates()
	}
	return nil
}

func (s *SkinSwitcher) Render(obj *wprana.PranaObj) {
	s.obj = obj
	s.rebuildList()
	s.applyCheckboxStates()

	// Click delegation on the list — catches any checkbox change.
	if list := dom.Query(obj.Dom, ".ssw-list"); len(list) > 0 {
		dom.AddEvent(list[0], "change", s.onChange, false, false)
	}

	// React to programmatic Apply/Deactivate from anywhere.
	wprana.OnSkinChange(func() {
		s.rebuildList()
		s.applyCheckboxStates()
	})
}
