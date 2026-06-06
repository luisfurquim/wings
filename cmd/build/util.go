package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// wingsRoot walks up from the current directory to the directory whose go.mod
// declares the wings module, and returns it. The build.sh wrappers invoke this
// tool with `go -C "$WPRANA"`, so the root is normally the working directory;
// the walk just makes a direct `go run ./cmd/build` from a subdir work too.
func wingsRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if isWingsModule(filepath.Join(dir, "go.mod")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not inside the github.com/luisfurquim/wings module")
		}
		dir = parent
	}
}

func isWingsModule(gomod string) bool {
	data, err := os.ReadFile(gomod)
	if err != nil {
		return false
	}
	return bytes.Contains(data, []byte("module github.com/luisfurquim/wings\n"))
}

// run executes name+args with the given working directory, streaming output to
// the parent. Extra environment entries (e.g. "GOOS=js") are appended to the
// inherited environment.
func run(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runOut runs name+args in dir and returns trimmed stdout.
func runOut(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(bytes.TrimSpace(out)), err
}

// copyFile copies src to dst, creating dst's parent directory as needed. The
// copied files (JS helpers, JSON catalogs) are small, so a read-then-write is
// fine.
func copyFile(dst, src string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// wasmExecPath locates wasm_exec.js inside the active GOROOT (Go 1.21+ keeps it
// under lib/wasm; older toolchains under misc/wasm).
func wasmExecPath() (string, error) {
	goroot, err := runOut("", "go", "env", "GOROOT")
	if err != nil {
		return "", err
	}
	for _, p := range []string{
		filepath.Join(goroot, "lib", "wasm", "wasm_exec.js"),
		filepath.Join(goroot, "misc", "wasm", "wasm_exec.js"),
	} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("wasm_exec.js not found in GOROOT=%s", goroot)
}

// buildGenI18n compiles cmd/gen_i18n from the wings tree into a temp binary so
// its transitive deps resolve against wings's go.sum (not a sub-module's), and
// returns the path plus a cleanup func.
func buildGenI18n(root string) (bin string, cleanup func(), err error) {
	tmp, err := os.MkdirTemp("", "gen_i18n")
	if err != nil {
		return "", func() {}, err
	}
	cleanup = func() { _ = os.RemoveAll(tmp) }
	bin = filepath.Join(tmp, "gen_i18n")
	if err := run(root, nil, "go", "build", "-buildvcs=false", "-o", bin, "./cmd/gen_i18n"); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return bin, cleanup, nil
}
