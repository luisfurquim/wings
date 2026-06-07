package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// TestIsSensitivePath blocks dotfiles and private keys while allowing normal
// web assets, so the dev server never serves source secrets by direct path.
func TestIsSensitivePath(t *testing.T) {
	blocked := []string{"/.env", "/.git/config", "/sub/.env", "/gen_i18n.ed25519.key", "/certs/server.pem"}
	for _, p := range blocked {
		if !isSensitivePath(p) {
			t.Errorf("isSensitivePath(%q) = false, want true", p)
		}
	}
	allowed := []string{"/", "/index.html", "/wings.wasm", "/app.js", "/styles.css", "/i18n/pt-BR.json"}
	for _, p := range allowed {
		if isSensitivePath(p) {
			t.Errorf("isSensitivePath(%q) = true, want false", p)
		}
	}
}

// TestNoListFS_DisablesListing confirms a directory without index.html is a 404
// (no listing), while a regular file is served — so the dev server never dumps
// the source tree.
func TestNoListFS_DisablesListing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.FileServer(noListFS{http.Dir(dir)}))
	defer srv.Close()

	if code := getStatus(t, srv.URL+"/"); code != http.StatusNotFound {
		t.Errorf("GET / (no index.html) = %d, want 404 (listing must be disabled)", code)
	}
	if code := getStatus(t, srv.URL+"/app.js"); code != http.StatusOK {
		t.Errorf("GET /app.js = %d, want 200", code)
	}
}

func getStatus(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// TestI18nPathDefault: WINGS_I18N_PATH defaults to WINGS_MAIN, and overrides it
// when set (the live-demo needs gen to scan ./live-demo/mod while building
// ./live-demo).
func TestI18nPathDefault(t *testing.T) {
	t.Setenv("WINGS_MAIN", "./live-demo")
	t.Setenv("WINGS_I18N_PATH", "")
	cfg, err := loadDevConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.I18nPath != "./live-demo" {
		t.Errorf("I18nPath default = %q, want %q (= Main)", cfg.I18nPath, "./live-demo")
	}
	t.Setenv("WINGS_I18N_PATH", "./live-demo/mod")
	cfg, err = loadDevConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.I18nPath != "./live-demo/mod" {
		t.Errorf("I18nPath override = %q, want ./live-demo/mod", cfg.I18nPath)
	}
}

// TestPublishDevCatalogs copies *.json and *.json.sig into <WebRoot>/i18n while
// dropping the server-only *.meta.json.
func TestPublishDevCatalogs(t *testing.T) {
	app := t.TempDir()
	web := t.TempDir()
	src := filepath.Join(app, "mod", "i18n")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"pt-BR.json", "pt-BR.json.sig", "pt-BR.meta.json"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &devConfig{AppRoot: app, I18nPath: "mod", WebRoot: web}
	if err := publishDevCatalogs(cfg); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"pt-BR.json", "pt-BR.json.sig"} {
		if _, err := os.Stat(filepath.Join(web, "i18n", want)); err != nil {
			t.Errorf("expected %s published: %v", want, err)
		}
	}
	if _, err := os.Stat(filepath.Join(web, "i18n", "pt-BR.meta.json")); err == nil {
		t.Error("pt-BR.meta.json must not reach the webroot")
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
