package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestEnsureTranslatorConfig_WritesWhenAbsent verifies that the WINGS_TR_*
// settings synthesize a gen_i18n.json when none exists, with the expected shape.
func TestEnsureTranslatorConfig_WritesWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	cfg := &devConfig{
		AppRoot:       dir,
		AutoTranslate: true,
		TRBackend:     "libretranslate",
		TRURL:         "http://lt:5000",
		TRKey:         "secret",
		TRTimeout:     "30s",
	}
	if err := ensureTranslatorConfig(cfg); err != nil {
		t.Fatalf("ensureTranslatorConfig: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "gen_i18n.json"))
	if err != nil {
		t.Fatalf("expected gen_i18n.json to be written: %v", err)
	}
	var got genI18nFile
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Translator.Backend != "libretranslate" || got.Translator.URL != "http://lt:5000" {
		t.Errorf("unexpected translator config: %+v", got.Translator)
	}
}

// TestEnsureTranslatorConfig_RespectsExisting verifies that an app-authored
// gen_i18n.json is never overwritten by WINGS_TR_*.
func TestEnsureTranslatorConfig_RespectsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gen_i18n.json")
	original := []byte(`{"translator":{"backend":"openai","url":"http://mine"}}`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &devConfig{AppRoot: dir, AutoTranslate: true, TRBackend: "libretranslate", TRURL: "http://lt:5000"}
	if err := ensureTranslatorConfig(cfg); err != nil {
		t.Fatalf("ensureTranslatorConfig: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Errorf("existing gen_i18n.json was modified:\n got: %s\nwant: %s", data, original)
	}
}

// TestParseExt normalizes a comma list into a dotted, lowercased set.
func TestParseExt(t *testing.T) {
	got := parseExt("go, .HTML ,css,")
	for _, want := range []string{".go", ".html", ".css"} {
		if !got[want] {
			t.Errorf("parseExt missing %q in %v", want, got)
		}
	}
	if len(got) != 3 {
		t.Errorf("parseExt size = %d, want 3 (%v)", len(got), got)
	}
}

// TestEnvBool accepts the documented truthy tokens and rejects everything else.
func TestEnvBool(t *testing.T) {
	t.Setenv("WINGS_X", "yes")
	if !envBool("WINGS_X") {
		t.Error(`envBool("yes") = false, want true`)
	}
	t.Setenv("WINGS_X", "0")
	if envBool("WINGS_X") {
		t.Error(`envBool("0") = true, want false`)
	}
}
