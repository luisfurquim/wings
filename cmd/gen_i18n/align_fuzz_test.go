package main

import (
	"reflect"
	"strings"
	"testing"
)

// dedupLines splits a fuzzed blob into lines and removes duplicates, honoring
// alignSources' documented precondition that both catalogs hold unique strings
// (gen_i18n dedups by content via resolveHash before calling it).
func dedupLines(s string) []string {
	if s == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if seen[line] {
			continue
		}
		seen[line] = true
		out = append(out, line)
	}
	return out
}

// FuzzAlignSources hammers the catalog aligner with arbitrary old/new source
// sets. Properties: never panics; the returned index for every new source is a
// valid old position or -1 (so no out-of-range read happens downstream); and
// the result is deterministic (running twice equals running once — the
// restart-protection property made testable).
func FuzzAlignSources(f *testing.F) {
	f.Add("a\nb\nc", "a\nb\nc")
	f.Add("hello\nworld", "hello\nworld!")
	f.Add("one\ntwo\nthree", "three\ntwo\none")
	f.Add("", "new")
	f.Add("old", "")
	f.Fuzz(func(t *testing.T, oldBlob, newBlob string) {
		oldSrc := dedupLines(oldBlob)
		newSrc := dedupLines(newBlob)

		oldIdx, kind := alignSources(oldSrc, newSrc)

		if len(oldIdx) != len(newSrc) || len(kind) != len(newSrc) {
			t.Fatalf("length mismatch: oldIdx=%d kind=%d newSrc=%d",
				len(oldIdx), len(kind), len(newSrc))
		}
		for i, idx := range oldIdx {
			if idx < -1 || idx >= len(oldSrc) {
				t.Fatalf("oldIdx[%d]=%d out of range [-1,%d)", i, idx, len(oldSrc))
			}
		}

		oldIdx2, kind2 := alignSources(oldSrc, newSrc)
		if !reflect.DeepEqual(oldIdx, oldIdx2) || !reflect.DeepEqual(kind, kind2) {
			t.Fatalf("not deterministic: %v/%v vs %v/%v", oldIdx, kind, oldIdx2, kind2)
		}
	})
}
