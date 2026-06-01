#!/bin/bash
set -e

CGO_ENABLED=0 GOOS=js GOARCH=wasm go build

# ── SRI hash injection ────────────────────────────────────────────────────────
# Compute SHA-384 integrity hashes for the JS helper files and inject them into
# every index.html that references those files.  Re-run after any change to
# prana_helper.js or wasm_exec.js.
#
# Requires: openssl, sed (GNU or BSD both work)

sri_hash() {
    local file="$1"
    printf 'sha384-%s' "$(openssl dgst -sha384 -binary "$file" | openssl base64 -A)"
}

inject_sri() {
    local html="$1" file="$2" name hash
    name=$(basename "$file")
    hash=$(sri_hash "$file")
    # Idempotent: matches src="<name>" plus any integrity/crossorigin already
    # present and rewrites the trio fresh (re-runs don't duplicate attributes).
    sed -i.bak \
        "s|src=\"${name}\"\\( integrity=\"[^\"]*\"\\)\\{0,1\\}\\( crossorigin=\"[^\"]*\"\\)\\{0,1\\}|src=\"${name}\" integrity=\"${hash}\" crossorigin=\"anonymous\"|g" \
        "$html"
    rm -f "${html}.bak"
}

# Hash the files actually served from docs/ (the repo root has no wasm_exec.js).
# live-demo/ owns its own SRI injection in its own build.sh.
inject_sri docs/index.html docs/prana_helper.js
inject_sri docs/index.html docs/wasm_exec.js
