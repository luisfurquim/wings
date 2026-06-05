package main

import "testing"

// check verifies the per-index oldIdx/kind against expectations.
func check(t *testing.T, name string, oldSrc, newSrc []string, wantIdx []int, wantKind []matchKind) {
	t.Helper()
	gotIdx, gotKind := alignSources(oldSrc, newSrc)
	if len(gotIdx) != len(newSrc) {
		t.Fatalf("%s: len(oldIdx)=%d want %d", name, len(gotIdx), len(newSrc))
	}
	for i := range newSrc {
		if gotIdx[i] != wantIdx[i] || gotKind[i] != wantKind[i] {
			t.Errorf("%s: [%d]=%q -> (idx=%d,kind=%d), want (idx=%d,kind=%d)",
				name, i, newSrc[i], gotIdx[i], gotKind[i], wantIdx[i], wantKind[i])
		}
	}
}

func TestAlignExactInOrder(t *testing.T) {
	old := []string{"A", "B", "C"}
	nw := []string{"A", "B", "C"}
	check(t, "in-order", old, nw,
		[]int{0, 1, 2},
		[]matchKind{matchExact, matchExact, matchExact})
}

func TestAlignMoved(t *testing.T) {
	// Reordered but identical: every string keeps its old index regardless of
	// position (global content match), so nothing is lost on a move.
	old := []string{"A", "B", "C"}
	nw := []string{"C", "A", "B"}
	check(t, "moved", old, nw,
		[]int{2, 0, 1},
		[]matchKind{matchExact, matchExact, matchExact})
}

func TestAlignInsertMiddle(t *testing.T) {
	// Insertion shifts later indices; exact matches still line up, the inserted
	// string is brand-new.
	old := []string{"A", "B", "C"}
	nw := []string{"A", "k", "B", "C"}
	check(t, "insert", old, nw,
		[]int{0, -1, 1, 2},
		[]matchKind{matchExact, matchNone, matchExact, matchExact})
}

func TestAlignDelete(t *testing.T) {
	old := []string{"A", "B", "C"}
	nw := []string{"A", "C"}
	check(t, "delete", old, nw,
		[]int{0, 2},
		[]matchKind{matchExact, matchExact})
}

func TestAlignEditFuzzy(t *testing.T) {
	// A small edit becomes a fuzzy match onto the old index, so the translation
	// is reused (caller forces revised=false).
	old := []string{"Save"}
	nw := []string{"Save!"}
	check(t, "edit", old, nw,
		[]int{0},
		[]matchKind{matchFuzzy})
}

func TestAlignEditDisambiguatedByGap(t *testing.T) {
	// Two simultaneous edits between the same anchors: order within the gap
	// keeps each edit paired to its own ancestor even though both candidates
	// are similar.
	old := []string{"X", "Hello world", "Goodbye world", "Y"}
	nw := []string{"X", "Hello, world!", "Goodbye, world!", "Y"}
	check(t, "gap-disambig", old, nw,
		[]int{0, 1, 2, 3},
		[]matchKind{matchExact, matchFuzzy, matchFuzzy, matchExact})
}

func TestAlignUnrelatedIsNew(t *testing.T) {
	// A genuinely different new string falls below the threshold: brand-new.
	old := []string{"A"}
	nw := []string{"A", "completely unrelated sentence"}
	check(t, "unrelated", old, nw,
		[]int{0, -1},
		[]matchKind{matchExact, matchNone})
}

func TestAlignEmptyOld(t *testing.T) {
	// First run: no previous catalog, everything is new.
	check(t, "empty-old", nil, []string{"A", "B"},
		[]int{-1, -1},
		[]matchKind{matchNone, matchNone})
}

func TestAlignEditAndMovedAcrossAnchorNotMatched(t *testing.T) {
	// Edited AND moved across an anchor: documented worst case — treated as
	// new + deleted (no reuse), same as the pre-fuzzy behavior.
	old := []string{"alpha text", "B", "C"}
	nw := []string{"B", "C", "alpha text edited"}
	gotIdx, gotKind := alignSources(old, nw)
	if gotKind[2] != matchNone || gotIdx[2] != -1 {
		t.Errorf("moved+edited: got (idx=%d,kind=%d), want new (idx=-1,kind=0)", gotIdx[2], gotKind[2])
	}
}

func TestLevRatio(t *testing.T) {
	if r := levRatio("abc", "abc"); r != 1 {
		t.Errorf("identical ratio=%v want 1", r)
	}
	if r := levRatio("", ""); r != 1 {
		t.Errorf("empty ratio=%v want 1", r)
	}
	if r := levRatio("Save", "Save!"); r < 0.7 {
		t.Errorf("Save/Save! ratio=%v want >=0.7", r)
	}
	if r := levRatio("café", "cafe"); r < 0.7 {
		t.Errorf("accented ratio=%v want >=0.7", r)
	}
	if r := levRatio("hello", "xyzzy"); r > 0.3 {
		t.Errorf("dissimilar ratio=%v want low", r)
	}
}

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "abc", 3},
		{"kitten", "sitting", 3},
		{"café", "cafe", 1},
	}
	for _, c := range cases {
		if got := levenshtein([]rune(c.a), []rune(c.b)); got != c.want {
			t.Errorf("levenshtein(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}
