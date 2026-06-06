package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// build dispatches to the per-target builder. Every builder lints its templates
// first, then reproduces the work the corresponding build.sh used to do.
func build(root, target string) error {
	switch target {
	case "lib":
		return buildLib(root)
	case "example":
		return buildExample(root)
	case "live-demo":
		return buildLiveDemo(root)
	case "wlate":
		return buildWlate(root)
	case "all":
		for _, t := range []string{"lib", "example", "live-demo", "wlate"} {
			if err := build(root, t); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown target %q (want lib|example|live-demo|wlate|all)", target)
	}
}

// buildLib compile-checks the library for js/wasm and refreshes the SRI hashes
// in the published docs/index.html.
func buildLib(root string) error {
	if err := lintTemplates(filepath.Join(root, "widget")); err != nil {
		return err
	}
	if err := run(root, []string{"CGO_ENABLED=0", "GOOS=js", "GOARCH=wasm"}, "go", "build"); err != nil {
		return err
	}
	return injectDocsSRI(filepath.Join(root, "docs"))
}

// buildExample builds the example app's wasm binary.
func buildExample(root string) error {
	dir := filepath.Join(root, "example")
	if err := lintTemplates(dir); err != nil {
		return err
	}
	if err := copyHelpers(root, dir); err != nil {
		return err
	}
	return run(dir, []string{"GOOS=js", "GOARCH=wasm"}, "go", "build", "-o", "wings.wasm", ".")
}

// buildLiveDemo regenerates the live-demo straight into the published root
// docs/: JS helpers, signed i18n catalogs, the wasm binary, and SRI hashes.
func buildLiveDemo(root string) error {
	dir := filepath.Join(root, "live-demo")
	docs := filepath.Join(root, "docs")
	if err := lintTemplates(filepath.Join(dir, "mod")); err != nil {
		return err
	}
	if err := copyHelpers(root, docs); err != nil {
		return err
	}

	gen, cleanup, err := buildGenI18n(root)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := run(dir, nil, gen,
		"--path", "./mod", "--deflang", "pt-BR",
		"-sign-key", "./gen_i18n.ed25519.key",
		"-sign-key-password", "wings-live-demo"); err != nil {
		return err
	}
	if err := publishCatalogs(filepath.Join(dir, "mod", "i18n"), filepath.Join(docs, "i18n")); err != nil {
		return err
	}

	if err := run(dir, []string{"GOOS=js", "GOARCH=wasm"}, "go", "build",
		"-buildvcs=false", "-tags", "wings_test", "-o", filepath.Join(docs, "wings.wasm"), "."); err != nil {
		return err
	}
	return injectDocsSRI(docs)
}

// buildWlate regenerates the wlate translator app into helpers/wlate/dist.
func buildWlate(root string) error {
	dir := filepath.Join(root, "helpers", "wlate")
	dist := filepath.Join(dir, "dist")
	if err := lintTemplates(filepath.Join(dir, "mod")); err != nil {
		return err
	}
	if err := copyHelpers(root, dist); err != nil {
		return err
	}

	gen, cleanup, err := buildGenI18n(root)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := run(dir, nil, gen, "--path", "./mod", "--deflang", "pt-BR"); err != nil {
		return err
	}

	// Publish wlate's own UI catalogs under /wlate-i18n/ (distinct from the
	// /i18n/ route used for the project being translated).
	if err := copyGlob(filepath.Join(dir, "mod", "i18n", "*.json"), filepath.Join(dist, "wlate-i18n")); err != nil {
		return err
	}
	// Placeholder meta for the demo project catalogs already in dist/i18n.
	if err := generateMeta(filepath.Join(dist, "i18n")); err != nil {
		return err
	}

	if err := run(dir, []string{"CGO_ENABLED=0", "GOOS=js", "GOARCH=wasm"}, "go", "build",
		"-buildvcs=false", "-o", filepath.Join(dist, "wings.wasm"), "."); err != nil {
		return err
	}
	return injectDocsSRI(dist)
}

// copyHelpers copies prana_helper.js (from the wings root) and wasm_exec.js
// (from GOROOT) into the output directory.
func copyHelpers(root, out string) error {
	wasmExec, err := wasmExecPath()
	if err != nil {
		return err
	}
	if err := copyFile(filepath.Join(out, "prana_helper.js"), filepath.Join(root, "prana_helper.js")); err != nil {
		return err
	}
	return copyFile(filepath.Join(out, "wasm_exec.js"), wasmExec)
}

// injectDocsSRI refreshes the SRI hashes for both JS helpers in <out>/index.html.
func injectDocsSRI(out string) error {
	index := filepath.Join(out, "index.html")
	if err := injectSRI(index, filepath.Join(out, "prana_helper.js"), "prana_helper.js"); err != nil {
		return err
	}
	return injectSRI(index, filepath.Join(out, "wasm_exec.js"), "wasm_exec.js")
}

// publishCatalogs copies the browser-side catalogs (*.json incl. inflections)
// and their *.json.sig signatures into dst, then drops the server-only
// *.meta.json companions so source positions never reach the published site.
func publishCatalogs(src, dst string) error {
	if err := copyGlob(filepath.Join(src, "*.json"), dst); err != nil {
		return err
	}
	if err := copyGlob(filepath.Join(src, "*.json.sig"), dst); err != nil {
		return err
	}
	metas, err := filepath.Glob(filepath.Join(dst, "*.meta.json"))
	if err != nil {
		return err
	}
	for _, m := range metas {
		if err := os.Remove(m); err != nil {
			return err
		}
	}
	return nil
}

// copyGlob copies every file matching pattern into directory dst.
func copyGlob(pattern, dst string) error {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, m := range matches {
		if err := copyFile(filepath.Join(dst, filepath.Base(m)), m); err != nil {
			return err
		}
	}
	return nil
}
