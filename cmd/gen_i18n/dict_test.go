package main

import (
	"encoding/gob"
	"os"
	"path/filepath"
	"testing"
)

// writeTestDict gob-encodes a one-lemma Dict to <dir>/<name>.db so the loader
// resolution tests have a real file to find. The content is irrelevant here;
// only its presence and decodability matter.
func writeTestDict(t *testing.T, dir, name string) {
	t.Helper()
	d := Dict{
		Lemmas:    map[string]*Lemma{"gato": {Category: "N", Forms: map[string]Inflect{}}},
		FormIndex: map[string][]FormRef{},
	}
	f, err := os.Create(filepath.Join(dir, name+".db"))
	if err != nil {
		t.Fatalf("create %s.db: %v", name, err)
	}
	defer f.Close()
	if err := gob.NewEncoder(f).Encode(&d); err != nil {
		t.Fatalf("encode %s.db: %v", name, err)
	}
}

func TestLoadDictForLang(t *testing.T) {
	// Reset the strict global after the test (it is package-level state).
	defer func() { dictStrict = false }()

	t.Run("exact locale preferred over base", func(t *testing.T) {
		dir := t.TempDir()
		writeTestDict(t, dir, "en")
		writeTestDict(t, dir, "en-US")
		dictStrict = false
		d, path, err := loadDictForLang(dir, "en-US")
		if err != nil || d == nil {
			t.Fatalf("loadDictForLang = (%v, %q, %v); want a dict", d, path, err)
		}
		if filepath.Base(path) != "en-US.db" {
			t.Errorf("path = %q; want the exact en-US.db", path)
		}
	})

	t.Run("region falls back to base", func(t *testing.T) {
		dir := t.TempDir()
		writeTestDict(t, dir, "en") // only the base exists
		dictStrict = false
		d, path, err := loadDictForLang(dir, "en-US")
		if err != nil || d == nil {
			t.Fatalf("loadDictForLang = (%v, %q, %v); want the base dict", d, path, err)
		}
		if filepath.Base(path) != "en.db" {
			t.Errorf("path = %q; want the base en.db", path)
		}
	})

	t.Run("strict mode refuses the base fallback", func(t *testing.T) {
		dir := t.TempDir()
		writeTestDict(t, dir, "en") // base present, but exact locale absent
		dictStrict = true
		d, _, err := loadDictForLang(dir, "en-US")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if d != nil {
			t.Errorf("strict mode loaded a dict for en-US; want nil (no en-US.db)")
		}
	})

	t.Run("no candidate at all", func(t *testing.T) {
		dir := t.TempDir()
		dictStrict = false
		d, _, err := loadDictForLang(dir, "en-US")
		if err != nil || d != nil {
			t.Fatalf("loadDictForLang = (%v, %v); want (nil, nil)", d, err)
		}
	})

	t.Run("pt-BR does not fold to a non-existent pt base", func(t *testing.T) {
		dir := t.TempDir()
		writeTestDict(t, dir, "pt-BR")
		dictStrict = false
		d, path, err := loadDictForLang(dir, "pt-BR")
		if err != nil || d == nil {
			t.Fatalf("loadDictForLang = (%v, %q, %v); want the pt-BR dict", d, path, err)
		}
		if filepath.Base(path) != "pt-BR.db" {
			t.Errorf("path = %q; want pt-BR.db", path)
		}
	})
}
