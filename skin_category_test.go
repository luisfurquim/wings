//go:build js && wasm

package wings

import (
	"errors"
	"testing"
)

func TestCategoryHas(t *testing.T) {
	combo := CategoryIdentity | CategoryLighting | CategoryInteraction
	if !combo.Has(CategoryIdentity) {
		t.Error("combo should Has(Identity)")
	}
	if !combo.Has(CategoryIdentity | CategoryLighting) {
		t.Error("Has must test every bit in the argument")
	}
	if combo.Has(CategoryGeometry) {
		t.Error("combo should not Has(Geometry)")
	}
	if combo.Has(CategoryIdentity | CategoryGeometry) {
		t.Error("Has must be false when any requested bit is absent")
	}
	if !CategoryAll.Has(CategoryNone) {
		t.Error("any mask Has(None) — empty set is always contained")
	}
}

func TestCategoryConflicts(t *testing.T) {
	// Identity-theme vs geometry-skin: disjoint, must NOT conflict (the whole
	// point of the bitmask design — they compose).
	if IdentitySkinCategories.Conflicts(GeometrySkinCategories) {
		t.Error("Identity and Geometry skins should be composable (no conflict)")
	}
	// Two identity themes share CategoryIdentity → conflict.
	if !IdentitySkinCategories.Conflicts(CategoryIdentity) {
		t.Error("two Identity-touching skins must conflict")
	}
	// Depth and Motion are single distinct bits → no conflict.
	if DepthSkinCategories.Conflicts(MotionSkinCategories) {
		t.Error("Depth and Motion are disjoint, must not conflict")
	}
	if CategoryNone.Conflicts(CategoryAll) {
		t.Error("None conflicts with nothing")
	}
}

func TestCategoryString(t *testing.T) {
	if got := CategoryNone.String(); got != "None" {
		t.Errorf("CategoryNone.String() = %q, want %q", got, "None")
	}
	if got := CategoryIdentity.String(); got != "Identity" {
		t.Errorf("single category String() = %q, want %q", got, "Identity")
	}
	// Declaration order is Identity < Geometry < ... regardless of OR order.
	got := (CategoryGeometry | CategoryIdentity).String()
	if got != "Identity|Geometry" {
		t.Errorf("String() = %q, want declaration order %q", got, "Identity|Geometry")
	}
}

func TestCategoryNames(t *testing.T) {
	names := (CategoryIdentity | CategoryAtmosphere).Names()
	if len(names) != 2 || names[0] != "Identity" || names[1] != "Atmosphere" {
		t.Errorf("Names() = %v, want [Identity Atmosphere]", names)
	}
	if n := CategoryNone.Names(); len(n) != 0 {
		t.Errorf("CategoryNone.Names() = %v, want empty", n)
	}
	// CategoryAll names every built-in: there are 9 declared categories.
	if n := CategoryAll.Names(); len(n) != 9 {
		t.Errorf("CategoryAll.Names() has %d entries, want 9", len(n))
	}
}

func TestUserCategory(t *testing.T) {
	for n := uint(0); n < userCategoryCount; n++ {
		bit, err := UserCategory(n)
		if err != nil {
			t.Fatalf("UserCategory(%d): unexpected error %v", n, err)
		}
		// Must land in the reserved high range and never collide with built-ins.
		if bit&CategoryUserMask == 0 {
			t.Errorf("UserCategory(%d)=%#x outside user mask", n, uint64(bit))
		}
		if bit&CategoryAll != 0 {
			t.Errorf("UserCategory(%d)=%#x collides with a built-in category", n, uint64(bit))
		}
	}
	// Out-of-range index returns the sentinel.
	if _, err := UserCategory(userCategoryCount); !errors.Is(err, ErrUserCategoryRange) {
		t.Errorf("UserCategory(%d) err = %v, want ErrUserCategoryRange", userCategoryCount, err)
	}
}

func TestRegisterCategoryName(t *testing.T) {
	brand, err := UserCategory(0)
	if err != nil {
		t.Fatal(err)
	}
	if err := RegisterCategoryName(brand, "Brand"); err != nil {
		t.Fatalf("RegisterCategoryName: %v", err)
	}
	// Named user bit surfaces in Names().
	names := brand.Names()
	if len(names) != 1 || names[0] != "Brand" {
		t.Errorf("named user category Names() = %v, want [Brand]", names)
	}
	// An un-named user bit falls back to "User<n>".
	other, _ := UserCategory(5)
	if names := other.Names(); len(names) != 1 || names[0] != "User5" {
		t.Errorf("un-named user category Names() = %v, want [User5]", names)
	}

	// Rejections: zero, multi-bit, and built-in bits are not user categories.
	for _, bad := range []SkinCategory{
		CategoryNone,     // zero
		brand | other,    // two bits
		CategoryIdentity, // built-in, not in user range
	} {
		if err := RegisterCategoryName(bad, "x"); !errors.Is(err, ErrNotUserCategory) {
			t.Errorf("RegisterCategoryName(%#x) err = %v, want ErrNotUserCategory", uint64(bad), err)
		}
	}
}
