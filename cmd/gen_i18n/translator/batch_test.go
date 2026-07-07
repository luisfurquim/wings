package translator

import (
	"fmt"
	"testing"
)

func makeEntry(label string, cells map[string]string) Entry {
	return Entry{Label: label, Cells: cells}
}

func TestBatch_Empty(t *testing.T) {
	if got := Batch(nil, 10, 1000); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
	if got := Batch([]Entry{}, 10, 1000); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestBatch_AllFitInOne(t *testing.T) {
	entries := []Entry{
		makeEntry("a", map[string]string{"": "hello"}),
		makeEntry("b", map[string]string{"": "world"}),
		makeEntry("c", map[string]string{"m.one": "foo"}),
	}
	batches := Batch(entries, 10, 1000)
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(batches))
	}
	if len(batches[0]) != 3 {
		t.Errorf("expected 3 entries in batch, got %d", len(batches[0]))
	}
}

func TestBatch_SplitByMaxEntries(t *testing.T) {
	entries := make([]Entry, 5)
	for i := range entries {
		entries[i] = makeEntry(fmt.Sprintf("e%d", i), map[string]string{"": "x"})
	}
	batches := Batch(entries, 2, 10000)
	// 5 entries with maxEntries=2 → batches of [2, 2, 1]
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches, got %d", len(batches))
	}
	if len(batches[0]) != 2 || len(batches[1]) != 2 || len(batches[2]) != 1 {
		t.Errorf("unexpected batch sizes: %d %d %d",
			len(batches[0]), len(batches[1]), len(batches[2]))
	}
}

func TestBatch_SplitByMaxChars(t *testing.T) {
	// Each entry has 10 bytes; limit is 25 → batches of [2, 2, 1].
	entries := make([]Entry, 5)
	for i := range entries {
		entries[i] = makeEntry(fmt.Sprintf("e%d", i), map[string]string{"": "0123456789"})
	}
	batches := Batch(entries, 1000, 25)
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches, got %d: %v", len(batches), batches)
	}
	if len(batches[0]) != 2 || len(batches[1]) != 2 || len(batches[2]) != 1 {
		t.Errorf("unexpected batch sizes: %d %d %d",
			len(batches[0]), len(batches[1]), len(batches[2]))
	}
}

func TestBatch_OversizedEntryAlone(t *testing.T) {
	// An entry larger than maxChars must still be placed in its own batch.
	small := makeEntry("small", map[string]string{"": "hi"})
	big := makeEntry("big", map[string]string{"": "this is way too long"})

	entries := []Entry{small, big, small}
	batches := Batch(entries, 1000, 10)
	// small(2) fits; big(20) > 10 → own batch; small(2) fits new batch.
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches, got %d", len(batches))
	}
	if batches[0][0].Label != "small" {
		t.Errorf("batch 0 should be 'small', got %q", batches[0][0].Label)
	}
	if batches[1][0].Label != "big" {
		t.Errorf("batch 1 should be 'big', got %q", batches[1][0].Label)
	}
	if batches[2][0].Label != "small" {
		t.Errorf("batch 2 should be 'small', got %q", batches[2][0].Label)
	}
}

func TestBatch_ExactBoundary(t *testing.T) {
	// Two entries of exactly 5 bytes each with maxChars=10 → fits in one batch.
	entries := []Entry{
		makeEntry("a", map[string]string{"": "hello"}),
		makeEntry("b", map[string]string{"": "world"}),
	}
	batches := Batch(entries, 1000, 10)
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch at exact boundary, got %d", len(batches))
	}
}

func TestBatch_MultipleCellsCharCount(t *testing.T) {
	// entryChars sums all cells: entry with 3 cells of 4 bytes each = 12 bytes.
	e := makeEntry("flex", map[string]string{
		"m.one":   "abcd",
		"m.other": "efgh",
		"f.one":   "ijkl",
	})
	if got := entryChars(e); got != 12 {
		t.Errorf("entryChars = %d, want 12", got)
	}

	batches := Batch([]Entry{e, e}, 1000, 15)
	// First entry: 12 bytes. Second: 12+12=24 > 15 → new batch.
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(batches))
	}
}
