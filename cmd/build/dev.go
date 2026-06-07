package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// dev runs the generic development loop for a webdev's own wings app: it builds
// the app's wings.wasm once, serves the webroot (embedded server or a custom
// WINGS_HTTPD), then watches the source tree and rebuilds on every save. Unlike
// the repo targets it never assumes it runs inside the wings module — it
// resolves the wings module the app depends on via `go list -m`, so the same
// binary works from any app directory (and inside the dev container).
func dev() error {
	cfg, err := loadDevConfig()
	if err != nil {
		return err
	}
	wingsDir, err := runOut(cfg.ModuleDir, "go", "list", "-m", "-f", "{{.Dir}}", "github.com/luisfurquim/wings")
	if err != nil {
		return fmt.Errorf("resolving the github.com/luisfurquim/wings module from %s: %w%s\n"+
			"WINGS_MAIN must point at the module dir (the one with go.mod), and that go.mod must "+
			"require github.com/luisfurquim/wings (local replace targets must be reachable too)", cfg.ModuleDir, err, cmdStderr(err))
	}

	devLogf("module:     %s", cfg.ModuleDir)
	devLogf("wings:      %s", wingsDir)
	devLogf("webroot:    %s", cfg.WebRoot)

	// Initial build. A failure here is logged but not fatal: the server still
	// comes up so the webdev can fix the error and save again.
	if err := buildOnce(cfg, wingsDir); err != nil {
		devLogf("initial build failed: %v", err)
	}
	if err := startServer(cfg); err != nil {
		return err
	}
	return watch(cfg, wingsDir)
}

// buildOnce runs the full app pipeline a single time: lint → optional gen_i18n
// → copy JS helpers → compile wasm. Each step's helpers are shared with the repo
// targets (lint.go, util.go, targets.go); only the module resolution differs
// (the app's resolved wings dir instead of the repo root).
func buildOnce(cfg *devConfig, wingsDir string) error {
	// Lint the module's templates, not the whole app root: the app root may be a
	// parent dir (e.g. holding live-demo/ + docs/) and sweep unrelated trees.
	if err := lintTemplates(cfg.ModuleDir); err != nil {
		return err
	}
	if cfg.DefLang != "" {
		if err := runGenI18n(cfg, wingsDir); err != nil {
			return err
		}
	}
	if err := copyHelpers(wingsDir, cfg.WebRoot); err != nil {
		return err
	}
	// Build runs from the module dir, so the target is the module root ".".
	args := []string{"build", "-buildvcs=false"}
	if cfg.BuildTags != "" {
		args = append(args, "-tags", cfg.BuildTags)
	}
	args = append(args, "-o", filepath.Join(cfg.WebRoot, "wings.wasm"), ".")
	if err := run(cfg.ModuleDir, []string{"GOOS=js", "GOARCH=wasm"}, "go", args...); err != nil {
		return err
	}
	devLogf("build ok → %s", filepath.Join(cfg.WebRoot, "wings.wasm"))
	return nil
}

// runGenI18n compiles gen_i18n from the resolved wings tree and runs it against
// the app, so the catalogs stay current on every rebuild. It reuses
// buildGenI18n (util.go) pointed at the app's wings module rather than the repo.
// The -auto-flex (dictionary) and -auto-translate (LLM/MT) passes are enabled
// from the WINGS_* config; the latter is wired through a synthesized
// gen_i18n.json (see ensureTranslatorConfig).
func runGenI18n(cfg *devConfig, wingsDir string) error {
	if cfg.AutoTranslate {
		if err := ensureTranslatorConfig(cfg); err != nil {
			return err
		}
	}
	gen, cleanup, err := buildGenI18n(wingsDir)
	if err != nil {
		return err
	}
	defer cleanup()
	args := []string{"--path", cfg.I18nPath, "--deflang", cfg.DefLang}
	if cfg.AutoFlex {
		args = append(args, "-auto-flex")
		if cfg.DictDir != "" {
			args = append(args, "-dict-dir", cfg.DictDir)
		}
		if cfg.DictStrict {
			args = append(args, "-dict-strict")
		}
	}
	if cfg.AutoTranslate {
		args = append(args, "-auto-translate")
	}
	args = append(args, cfg.GenI18nArgs...)
	if err := run(cfg.ModuleDir, nil, gen, args...); err != nil {
		return err
	}
	return publishDevCatalogs(cfg)
}

// publishDevCatalogs copies the freshly generated catalogs (+ .json.sig
// signatures) from <I18nPath>/i18n into <WebRoot>/i18n, so the browser fetches
// the current i18n on each rebuild — mirroring buildLiveDemo's publishCatalogs
// (which also drops the server-only *.meta.json). Apps that embed their catalogs
// have no source i18n dir; that case is skipped silently.
func publishDevCatalogs(cfg *devConfig) error {
	src := filepath.Join(cfg.ModuleDir, cfg.I18nPath, "i18n")
	if _, err := os.Stat(src); err != nil {
		return nil //nolint:nilerr // no catalog dir → nothing to publish
	}
	dst := filepath.Join(cfg.WebRoot, "i18n")
	if err := publishCatalogs(src, dst); err != nil {
		return err
	}
	devLogf("published catalogs → %s", dst)
	return nil
}

// cmdStderr extracts the captured stderr from a failed exec (runOut uses
// cmd.Output(), which stashes the child's stderr in ExitError.Stderr). It is
// returned as a "\n  <stderr>" suffix so the underlying `go` diagnostic reaches
// the user instead of a bare "exit status 1".
func cmdStderr(err error) string {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if msg := strings.TrimSpace(string(ee.Stderr)); msg != "" {
			return "\n  " + strings.ReplaceAll(msg, "\n", "\n  ")
		}
	}
	return ""
}

// devLogf writes a timestamp-free, prefixed status line to stderr. The dev loop
// is chatty by design (it is an interactive tool), so keep it on stderr to leave
// stdout free for any spawned WINGS_HTTPD.
func devLogf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "wings dev: "+format+"\n", args...)
}
