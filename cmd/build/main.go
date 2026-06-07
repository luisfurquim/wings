// Command build is the WINGS build orchestrator. It replaces the per-directory
// build.sh scripts with a single cross-platform Go tool — no sed, openssl, or
// python3 required. Run it from the wings module root (the build.sh wrappers do
// this for you via `go -C`):
//
//	go run ./cmd/build <target>
//
// Targets: lib, example, live-demo, wlate, all. Every target first lints its
// HTML templates for camelCase binding names (see lint.go) and fails the build
// if any are found.
//
// The dev target is different: it builds and serves an arbitrary wings app with
// rebuild-on-save, for a webdev developing their own app (see dev.go). It does
// not assume it runs inside the wings module, so it is dispatched before the
// repo-root lookup.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		usage()
		os.Exit(2)
	}
	if os.Args[1] == "dev" {
		if err := dev(); err != nil {
			fatal(err)
		}
		return
	}
	root, err := wingsRoot()
	if err != nil {
		fatal(err)
	}
	if err := build(root, os.Args[1]); err != nil {
		fatal(err)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: go run ./cmd/build <lib|example|live-demo|wlate|all|dev>")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "build:", err)
	os.Exit(1)
}
