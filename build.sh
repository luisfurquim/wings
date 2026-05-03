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
    local html="$1"
    local file="$2"
    local name
    name=$(basename "$file")
    local hash
    hash=$(sri_hash "$file")
    # Replace: src="<name>" or src="<name>" integrity="..." → add/update integrity=""
    sed -i.bak \
        "s|src=\"${name}\" integrity=\"[^\"]*\"|src=\"${name}\" integrity=\"${hash}\"|g;
         s|src=\"${name}\"\\([^/]\\)|src=\"${name}\" integrity=\"${hash}\" crossorigin=\"anonymous\"\\1|g" \
        "$html"
    rm -f "${html}.bak"
}

for html in docs/index.html live-demo/docs/index.html; do
    [ -f "$html" ] || continue
    inject_sri "$html" "prana_helper.js"
    inject_sri "$html" "wasm_exec.js"
done
