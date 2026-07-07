package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// buildUnitexToolLogger compiles the UnitexToolLogger binary inside an already
// cloned unitex-core checkout. The Makefile target UNITEXTOOLLOGGERONLY=yes
// links only this single executable, which keeps the build to a fraction of a
// full Unitex compile (still a few minutes, but tractable). 64BITS=yes matches
// the platforms WINGS actually runs on; we don't support Windows here because
// the Makefile assumes MSVC/Dev-C++ on that target.
//
// The function returns the path to the compiled binary. If a binary already
// exists at the expected location it is reused — incremental rebuilds against
// a stale tree are out of scope; a force-rebuild can be obtained by removing
// tool/ entirely.
func buildUnitexToolLogger(coreRoot string) (string, error) {
	if runtime.GOOS == "windows" {
		return "", fmt.Errorf("automatic UnitexToolLogger build not supported on windows; pass -tool with a prebuilt binary")
	}
	binPath := filepath.Join(coreRoot, "bin", "UnitexToolLogger")
	if info, err := os.Stat(binPath); err == nil && !info.IsDir() {
		return binPath, nil
	}
	// The Makefile lives at <root>/build/Makefile in v3.3 and at
	// <root>/src/build/Makefile in master. Try both so a future bump of
	// unitexCoreTag past v3.3 doesn't break the build silently.
	var buildDir string
	for _, candidate := range []string{
		filepath.Join(coreRoot, "build"),
		filepath.Join(coreRoot, "src", "build"),
	} {
		if _, err := os.Stat(filepath.Join(candidate, "Makefile")); err == nil {
			buildDir = candidate
			break
		}
	}
	if buildDir == "" {
		return "", fmt.Errorf("no Makefile found under %s/{build,src/build}", coreRoot)
	}
	fmt.Fprintln(os.Stderr, "building UnitexToolLogger (this takes a few minutes the first time)…")
	cmd := exec.Command("make", "UNITEXTOOLLOGGERONLY=yes", "64BITS=yes")
	cmd.Dir = buildDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("make UnitexToolLogger: %w", err)
	}
	if _, err := os.Stat(binPath); err != nil {
		return "", fmt.Errorf("expected binary not produced at %s: %w", binPath, err)
	}
	return binPath, nil
}

// uncompressDela invokes UnitexToolLogger to expand a compiled DELAF (.bin
// plus its sibling .inf) back into the UTF-16 text dictionary that the rest of
// dictbuild knows how to parse. The .inf file must live next to the .bin with
// the same base name — UnitexToolLogger derives its path implicitly, so the
// caller is responsible for placing both files in the same directory before
// this is called.
//
// The output path is returned for convenience; it is always
// "<dir>/<base>.dic" where dir/base come from binPath.
func uncompressDela(toolPath, binPath string) (string, error) {
	dir := filepath.Dir(binPath)
	base := filepath.Base(binPath)
	stem := base[:len(base)-len(filepath.Ext(base))]
	dicPath := filepath.Join(dir, stem+".dic")
	if _, err := os.Stat(dicPath); err == nil {
		return dicPath, nil
	}
	cmd := exec.Command(toolPath, "Uncompress", "-o", dicPath, binPath)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("UnitexToolLogger Uncompress: %w", err)
	}
	if _, err := os.Stat(dicPath); err != nil {
		return "", fmt.Errorf("uncompress did not produce %s: %w", dicPath, err)
	}
	fmt.Fprintf(os.Stderr, "uncompressed %s\n", filepath.Base(dicPath))
	return dicPath, nil
}
