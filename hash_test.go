//go:build js && wasm

package wings

import (
	"syscall/js"
	"testing"
)

// setLocationHash drives the shim's location.hash directly so readHash/GoTo can
// be exercised. Restores "" on cleanup to avoid leaking into other tests.
func setLocationHash(t *testing.T, v string) {
	t.Helper()
	loc := js.Global().Get("location")
	loc.Set("hash", v)
	t.Cleanup(func() { loc.Set("hash", "") })
}

func TestReadHash(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"#section", "section"}, // leading '#' stripped
		{"bare", "bare"},        // no '#': returned verbatim
		{"", ""},                // empty
		{"#", ""},               // lone '#' → empty fragment
		{"#a#b", "a#b"},         // only the FIRST '#' is stripped
	}
	for _, c := range cases {
		setLocationHash(t, c.raw)
		if got := readHash(); got != c.want {
			t.Errorf("readHash() with location.hash=%q = %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestGoTo(t *testing.T) {
	setLocationHash(t, "")
	GoTo("target")
	// GoTo writes the fragment to location.hash; readHash reads it back.
	if got := js.Global().Get("location").Get("hash").String(); got != "target" {
		t.Errorf("after GoTo: location.hash = %q, want target", got)
	}
	if got := readHash(); got != "target" {
		t.Errorf("readHash after GoTo = %q, want target", got)
	}
}
