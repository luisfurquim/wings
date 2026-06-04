//go:build js && wasm

package wings

import (
	"errors"
	"sort"
	"testing"
)

// resetSkinState clears the package-global skin registry between tests. Tests
// in this package run in the same binary and share these globals, so each test
// that mutates them starts from a clean slate.
//
// Note: ApplySkin's happy path cannot run here — injectSkinCSS short-circuits
// when document is unavailable (the unit-test path), so it never appends to
// activeSkins. Tests that need active skins seed activeSkins directly, which is
// legitimate because the test lives in package wings.
func resetSkinState() {
	skins = map[string]*skinEntry{}
	activeSkins = nil
	skinChangeHooks = nil
}

func TestRegisterSkinAndGetters(t *testing.T) {
	resetSkinState()
	RegisterSkin("theme-a", IdentitySkinCategories, "/*a*/")
	RegisterSkin("geo-b", GeometrySkinCategories, "/*b*/")

	names := ListSkins()
	sort.Strings(names)
	if len(names) != 2 || names[0] != "geo-b" || names[1] != "theme-a" {
		t.Errorf("ListSkins() = %v, want [geo-b theme-a]", names)
	}

	if cat, ok := SkinCategoriesOf("theme-a"); !ok || cat != IdentitySkinCategories {
		t.Errorf("SkinCategoriesOf(theme-a) = (%s,%v), want (%s,true)", cat, ok, IdentitySkinCategories)
	}
	if _, ok := SkinCategoriesOf("missing"); ok {
		t.Error("SkinCategoriesOf(missing) ok = true, want false")
	}

	infos := ListSkinInfos()
	if len(infos) != 2 {
		t.Fatalf("ListSkinInfos len = %d, want 2", len(infos))
	}

	// Re-registering overwrites rather than duplicating.
	RegisterSkin("theme-a", GeometrySkinCategories, "/*a2*/")
	if len(ListSkins()) != 2 {
		t.Errorf("re-register should overwrite, got %d skins", len(ListSkins()))
	}
	if cat, _ := SkinCategoriesOf("theme-a"); cat != GeometrySkinCategories {
		t.Errorf("re-register did not overwrite categories, got %s", cat)
	}
}

func TestApplySkinUnknown(t *testing.T) {
	resetSkinState()
	err := ApplySkin("nope")
	var nre *SkinNotRegisteredError
	if !errors.As(err, &nre) || nre.Name != "nope" {
		t.Errorf("ApplySkin(nope) err = %v, want *SkinNotRegisteredError{nope}", err)
	}
}

func TestApplySkinIdempotent(t *testing.T) {
	resetSkinState()
	RegisterSkin("theme-a", IdentitySkinCategories, "")
	activeSkins = []string{"theme-a"} // seed as already active
	if err := ApplySkin("theme-a"); err != nil {
		t.Errorf("re-applying an active skin should be a nil no-op, got %v", err)
	}
	if len(activeSkins) != 1 {
		t.Errorf("idempotent apply changed active set: %v", activeSkins)
	}
}

func TestApplySkinConflict(t *testing.T) {
	resetSkinState()
	RegisterSkin("light", IdentitySkinCategories, "")
	RegisterSkin("dark", IdentitySkinCategories, "") // shares CategoryIdentity
	activeSkins = []string{"light"}

	err := ApplySkin("dark")
	var ce *SkinConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("ApplySkin(dark) err = %v, want *SkinConflictError", err)
	}
	if ce.Name != "dark" {
		t.Errorf("conflict Name = %q, want dark", ce.Name)
	}
	if len(ce.Conflicts) != 1 || ce.Conflicts[0] != "light" {
		t.Errorf("conflict Conflicts = %v, want [light]", ce.Conflicts)
	}
	if !ce.ConflictingCategories.Has(CategoryIdentity) {
		t.Errorf("ConflictingCategories = %s, want to include Identity", ce.ConflictingCategories)
	}
	// A failed apply must not mutate the active set.
	if len(activeSkins) != 1 || activeSkins[0] != "light" {
		t.Errorf("failed apply mutated active set: %v", activeSkins)
	}
}

func TestActiveStateReads(t *testing.T) {
	resetSkinState()
	RegisterSkin("light", IdentitySkinCategories, "")
	RegisterSkin("sharp", GeometrySkinCategories, "")
	activeSkins = []string{"light", "sharp"}

	if got := ActiveSkin(); got != "sharp" {
		t.Errorf("ActiveSkin() = %q, want most-recent sharp", got)
	}
	if got := ActiveCategories(); got != IdentitySkinCategories|GeometrySkinCategories {
		t.Errorf("ActiveCategories() = %s, want Identity|Geometry composite", got)
	}

	// ActiveSkins returns a copy: mutating it must not affect internal state.
	snap := ActiveSkins()
	snap[0] = "tampered"
	if activeSkins[0] != "light" {
		t.Error("ActiveSkins() leaked the internal slice")
	}

	// Empty case.
	activeSkins = nil
	if ActiveSkin() != "" {
		t.Error("ActiveSkin() with none active should be empty")
	}
}

func TestConflictsWith(t *testing.T) {
	resetSkinState()
	RegisterSkin("light", IdentitySkinCategories, "")
	RegisterSkin("sharp", GeometrySkinCategories, "")
	activeSkins = []string{"light", "sharp"}

	// A new Identity skin conflicts only with "light".
	if got := ConflictsWith(CategoryIdentity); len(got) != 1 || got[0] != "light" {
		t.Errorf("ConflictsWith(Identity) = %v, want [light]", got)
	}
	// A Motion skin is orthogonal to both: no conflict.
	if got := ConflictsWith(MotionSkinCategories); len(got) != 0 {
		t.Errorf("ConflictsWith(Motion) = %v, want empty", got)
	}
}

func TestDeactivateSkin(t *testing.T) {
	resetSkinState()
	RegisterSkin("light", IdentitySkinCategories, "")
	RegisterSkin("sharp", GeometrySkinCategories, "")

	// Unknown skin → error.
	var nre *SkinNotRegisteredError
	if err := DeactivateSkin("ghost"); !errors.As(err, &nre) {
		t.Errorf("DeactivateSkin(ghost) err = %v, want *SkinNotRegisteredError", err)
	}

	// Registered but not active → nil no-op.
	if err := DeactivateSkin("light"); err != nil {
		t.Errorf("DeactivateSkin of inactive skin should be nil, got %v", err)
	}

	// Active → removed from the slice.
	activeSkins = []string{"light", "sharp"}
	if err := DeactivateSkin("light"); err != nil {
		t.Fatalf("DeactivateSkin(light): %v", err)
	}
	if len(activeSkins) != 1 || activeSkins[0] != "sharp" {
		t.Errorf("after deactivating light, active = %v, want [sharp]", activeSkins)
	}
}

func TestClearSkinsAndOnSkinChange(t *testing.T) {
	resetSkinState()
	RegisterSkin("light", IdentitySkinCategories, "")
	activeSkins = []string{"light"}

	calls := 0
	OnSkinChange(func() { calls++ })
	OnSkinChange(nil) // nil hooks are ignored, not stored

	ClearSkins()
	if len(activeSkins) != 0 {
		t.Errorf("ClearSkins left active = %v", activeSkins)
	}
	if calls != 1 {
		t.Errorf("OnSkinChange hook fired %d times, want 1", calls)
	}

	// ClearSkins on an empty set is a no-op and fires no hook.
	ClearSkins()
	if calls != 1 {
		t.Errorf("ClearSkins on empty set should not notify, calls = %d", calls)
	}
}
