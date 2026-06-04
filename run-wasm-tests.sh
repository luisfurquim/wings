#!/usr/bin/env bash
# run-wasm-tests.sh — run the js/wasm test suites (the wings package and any
# other GOOS=js-only package) under Node.
#
# These packages cannot be tested by a plain `go test ./...` because they only
# build for GOOS=js GOARCH=wasm. This script sets the wasm target and routes
# each test binary through testsupport/wasm_test_exec.sh, which injects a
# minimal DOM shim so the browser-only package init() does not panic under Node.
#
# Requires Node on PATH (the stock $GOROOT/lib/wasm/go_js_wasm_exec runner).
#
# Usage:
#   ./run-wasm-tests.sh                 # test the root wings package
#   ./run-wasm-tests.sh -v .            # verbose
#   ./run-wasm-tests.sh -run TestSolve  # a single test
set -e
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
args=("$@")
# Default to the root package when no package/flags select one.
if [ ${#args[@]} -eq 0 ]; then
	args=(.)
fi
GOOS=js GOARCH=wasm exec go test -exec "${here}/testsupport/wasm_test_exec.sh" "${args[@]}"
