//go:build js && wasm

package wings

import (
	"strings"
	"testing"
)

func TestSkinNotRegisteredError(t *testing.T) {
	err := &SkinNotRegisteredError{Name: "ghost"}
	got := err.Error()
	if !strings.Contains(got, `"ghost"`) || !strings.Contains(got, "not registered") {
		t.Errorf("Error() = %q, want it to name the skin and say 'not registered'", got)
	}
}

func TestSkinConflictErrorSingular(t *testing.T) {
	err := &SkinConflictError{
		Name:                  "dark",
		Categories:            IdentitySkinCategories,
		Conflicts:             []string{"light"},
		ConflictingCategories: CategoryIdentity,
	}
	got := err.Error()
	// One conflict → singular "skin" plus the quoted name, no trailing "s".
	if !strings.Contains(got, `active skin "light"`) {
		t.Errorf("single conflict Error() = %q, want singular form naming \"light\"", got)
	}
	if strings.Contains(got, "active skins") {
		t.Errorf("single conflict should not use plural 'skins': %q", got)
	}
	if !strings.Contains(got, `"dark"`) {
		t.Errorf("Error() should name the failing skin: %q", got)
	}
}

func TestSkinConflictErrorPlural(t *testing.T) {
	err := &SkinConflictError{
		Name:                  "dark",
		Categories:            IdentitySkinCategories,
		Conflicts:             []string{"light", "autumn"},
		ConflictingCategories: CategoryIdentity,
	}
	got := err.Error()
	// Two conflicts → plural "skins" and both names, comma-joined and quoted.
	if !strings.Contains(got, "active skins") {
		t.Errorf("multi conflict should use plural 'skins': %q", got)
	}
	if !strings.Contains(got, `"light", "autumn"`) {
		t.Errorf("multi conflict should comma-join quoted names: %q", got)
	}
}

func TestQuoteAll(t *testing.T) {
	got := quoteAll([]string{"a", "b"})
	if len(got) != 2 || got[0] != `"a"` || got[1] != `"b"` {
		t.Errorf("quoteAll = %v, want [\"a\" \"b\"]", got)
	}
	if len(quoteAll(nil)) != 0 {
		t.Error("quoteAll(nil) should be empty")
	}
}
