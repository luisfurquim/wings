package main

import "testing"

// reconstruct rebuilds a surface form from a lemma and the compact
// (DiffPos, Suffix) diff. DiffPos is counted in runes, and out-of-range
// positions are clamped to the lemma length.
func TestReconstruct(t *testing.T) {
	cases := []struct {
		name  string
		lemma string
		inf   Inflect
		want  string
	}{
		{"identity", "passar", Inflect{DiffPos: 6, Suffix: ""}, "passar"},
		{"suffix swap", "passar", Inflect{DiffPos: 4, Suffix: "ou"}, "passou"},
		{"full replace", "ir", Inflect{DiffPos: 0, Suffix: "vou"}, "vou"},
		{"accented runes", "café", Inflect{DiffPos: 3, Suffix: "ezinho"}, "cafezinho"},
		{"clamp past end", "ir", Inflect{DiffPos: 99, Suffix: "es"}, "ires"},
		{"empty lemma", "", Inflect{DiffPos: 0, Suffix: "x"}, "x"},
	}
	for _, c := range cases {
		if got := reconstruct(c.lemma, c.inf); got != c.want {
			t.Errorf("%s: reconstruct(%q, %+v) = %q, want %q",
				c.name, c.lemma, c.inf, got, c.want)
		}
	}
}

// DiffPos must index runes, not bytes: a 1-rune accented prefix is 2 bytes in
// UTF-8, so byte-slicing would corrupt the result.
func TestReconstructRuneAware(t *testing.T) {
	// "é" is 2 bytes; DiffPos=1 must keep exactly that one rune.
	got := reconstruct("école", Inflect{DiffPos: 1, Suffix: "X"})
	if got != "éX" {
		t.Errorf("rune-aware reconstruct = %q, want %q", got, "éX")
	}
}
