package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// devConfig holds the resolved settings for the `dev` mode, read from the
// WINGS_* environment variables (the docker-compose .env feeds these in). Every
// field has a sensible default so a bare `go run ./cmd/build dev` works from an
// app whose index.html, main package, and webroot all live in the same dir.
type devConfig struct {
	AppRoot     string          // working directory: the webdev's app source root
	Port        string          // WINGS_PORT — dev server port
	WebRoot     string          // WINGS_WEBROOT — abs dir holding index.html; wasm+helpers land here
	Main        string          // WINGS_MAIN — main package compiled to wasm (relative to AppRoot)
	I18nPath    string          // WINGS_I18N_PATH — dir gen_i18n traverses (default = Main); its /i18n is the catalog source
	Httpd       string          // WINGS_HTTPD — custom server command ("" = embedded server)
	DefLang     string          // WINGS_DEFLANG — if set, run gen_i18n with this default language
	GenI18nArgs []string        // WINGS_GENI18N_ARGS — extra gen_i18n flags
	BuildTags   string          // WINGS_BUILD_TAGS — extra -tags for go build
	WatchExt    map[string]bool // WINGS_WATCH_EXT — file extensions that trigger a rebuild
	Debounce    int             // WINGS_DEBOUNCE_MS — coalesce window for rapid saves

	// i18n inflection (dictionaries) and machine/LLM translation. These drive
	// the gen_i18n -auto-flex and -auto-translate passes; they have no effect
	// unless WINGS_DEFLANG is also set.
	AutoFlex      bool   // WINGS_AUTO_FLEX — pass -auto-flex (fill inflections from dicts)
	DictDir       string // WINGS_DICT_DIR — dir holding <lang>.db; passed as -dict-dir
	AutoTranslate bool   // WINGS_AUTO_TRANSLATE — pass -auto-translate
	TRBackend     string // WINGS_TR_BACKEND — "openai" | "libretranslate"
	TRURL         string // WINGS_TR_URL — backend endpoint
	TRModel       string // WINGS_TR_MODEL — model name (openai backend)
	TRKey         string // WINGS_TR_KEY — API key
	TRTimeout     string // WINGS_TR_TIMEOUT — e.g. "60s"
}

// loadDevConfig resolves the dev configuration from the environment, applying
// defaults for anything unset and turning relative paths into absolute ones
// rooted at the current working directory (the app source root).
func loadDevConfig() (*devConfig, error) {
	appRoot, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	cfg := &devConfig{
		AppRoot:     appRoot,
		Port:        envOr("WINGS_PORT", "8080"),
		Main:        envOr("WINGS_MAIN", "."),
		Httpd:       os.Getenv("WINGS_HTTPD"),
		DefLang:     os.Getenv("WINGS_DEFLANG"),
		GenI18nArgs: strings.Fields(os.Getenv("WINGS_GENI18N_ARGS")),
		BuildTags:   os.Getenv("WINGS_BUILD_TAGS"),
		WatchExt:    parseExt(envOr("WINGS_WATCH_EXT", "go,html,css,json")),

		AutoFlex:      envBool("WINGS_AUTO_FLEX"),
		DictDir:       os.Getenv("WINGS_DICT_DIR"),
		AutoTranslate: envBool("WINGS_AUTO_TRANSLATE"),
		TRBackend:     os.Getenv("WINGS_TR_BACKEND"),
		TRURL:         os.Getenv("WINGS_TR_URL"),
		TRModel:       os.Getenv("WINGS_TR_MODEL"),
		TRKey:         os.Getenv("WINGS_TR_KEY"),
		TRTimeout:     os.Getenv("WINGS_TR_TIMEOUT"),
	}

	webroot := envOr("WINGS_WEBROOT", ".")
	if !filepath.IsAbs(webroot) {
		webroot = filepath.Join(appRoot, webroot)
	}
	cfg.WebRoot = webroot

	// gen_i18n scans WINGS_I18N_PATH; defaults to the main package dir, which is
	// right for apps whose templates live under main. The live-demo overrides it
	// (gen scans ./live-demo/mod while the build target is ./live-demo).
	cfg.I18nPath = envOr("WINGS_I18N_PATH", cfg.Main)

	debounce := envOr("WINGS_DEBOUNCE_MS", "200")
	cfg.Debounce, err = strconv.Atoi(debounce)
	if err != nil || cfg.Debounce < 0 {
		return nil, fmt.Errorf("WINGS_DEBOUNCE_MS must be a non-negative integer, got %q", debounce)
	}
	return cfg, nil
}

// envOr returns the environment value for key, or def when it is unset or empty.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envBool reads a boolean toggle: "1", "true", "yes", "on" (case-insensitive)
// enable it; anything else (including unset) is false.
func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// parseExt turns a comma-separated extension list ("go,html, .css") into a set
// keyed by the lowercase extension with a leading dot (".go", ".html", ".css"),
// matching what filepath.Ext returns.
func parseExt(list string) map[string]bool {
	set := map[string]bool{}
	for _, e := range strings.Split(list, ",") {
		e = strings.TrimSpace(strings.ToLower(e))
		if e == "" {
			continue
		}
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		set[e] = true
	}
	return set
}
