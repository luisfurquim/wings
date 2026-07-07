package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// vulnScanner is installed at its latest release on purpose, unlike the pinned
// tools elsewhere: the scanner and its vulnerability DB evolve together, the
// point of the scan is detection freshness, and it never touches build outputs.
const vulnScanner = "golang.org/x/vuln/cmd/govulncheck@latest"

// vulncheck runs govulncheck over every module in the repo, twice each: a
// native source/call-graph pass, and a GOOS=js package-level pass — js-only
// packages are excluded by build constraints from the native pass, so without
// the second pass the wasm side of wings would never be scanned. The package
// scan is coarser (no call graph), which errs on the side of reporting.
//
// The scanner is installed once into a temp GOBIN and exec'd directly: `go run`
// can't be used for the js pass because GOOS=js in the environment would
// cross-compile the scanner itself to wasm.
func vulncheck(root string) error {
	tmp, err := os.MkdirTemp("", "govulncheck")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := run(root, []string{"GOBIN=" + tmp}, "go", "install", vulnScanner); err != nil {
		return err
	}
	bin := filepath.Join(tmp, "govulncheck")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	passes := []struct {
		label string
		env   []string
		args  []string
	}{
		{"native", nil, nil},
		{"js packages", []string{"GOOS=js", "GOARCH=wasm"}, []string{"-scan=package"}},
	}
	for _, mod := range []string{".", "example", "live-demo", filepath.Join("helpers", "wlate")} {
		dir := filepath.Join(root, mod)
		for _, pass := range passes {
			pkgs, err := listPackages(dir, pass.env)
			if err != nil {
				return fmt.Errorf("vulncheck %s (%s): %w", mod, pass.label, err)
			}
			if len(pkgs) == 0 {
				fmt.Printf("vulncheck %s (%s): no packages for this GOOS, skipped\n", mod, pass.label)
				continue
			}
			fmt.Printf("vulncheck %s (%s): %d packages\n", mod, pass.label, len(pkgs))
			if err := run(dir, pass.env, bin, append(pass.args, pkgs...)...); err != nil {
				return fmt.Errorf("vulncheck %s (%s): %w", mod, pass.label, err)
			}
		}
	}
	return nil
}

// listPackages enumerates the buildable import paths under dir for the given
// GOOS/GOARCH env, dropping broken packages — each pass must skip packages the
// other GOOS owns, and stray non-Go scratch files must not abort the scan.
func listPackages(dir string, env []string) ([]string, error) {
	cmd := exec.Command("go", "list", "-e", "-f", "{{if not .Error}}{{.ImportPath}}{{end}}", "./...")
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var pkgs []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			pkgs = append(pkgs, line)
		}
	}
	return pkgs, nil
}
