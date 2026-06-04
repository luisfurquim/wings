#!/usr/bin/env bash
# wasm_test_exec.sh — `go test -exec` wrapper for the js/wasm packages.
#
# `GOOS=js GOARCH=wasm go test` runs each test binary through an exec wrapper.
# The stock wrapper ($GOROOT/lib/wasm/go_js_wasm_exec) runs it under Node, but
# Node has no DOM, so the wings package's import-time init() panics. This
# wrapper injects testsupport/dom_shim.cjs via NODE_OPTIONS (which Node applies
# before running any script) and then delegates to the stock wrapper unchanged.
#
# Usage (see ./run-wasm-tests.sh for the full invocation):
#   GOOS=js GOARCH=wasm go test -exec "$PWD/testsupport/wasm_test_exec.sh" .
set -e
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export NODE_OPTIONS="--require=${here}/dom_shim.cjs ${NODE_OPTIONS:-}"
exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" "$@"
