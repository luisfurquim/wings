#!/bin/bash
set -e

cd "$(dirname "$0")"

# Output goes straight to the repo-root docs/ (one level up), which is what
# GitHub Pages serves. There is no live-demo/docs/ anymore — a second copy only
# ever drifted from this one.
OUT=../docs

WPRANA=$(go list -m -f '{{.Dir}}' github.com/luisfurquim/wings)
cp "$WPRANA/prana_helper.js" "$OUT/"

GOROOT="$(go env GOROOT)"
if [ -f "$GOROOT/lib/wasm/wasm_exec.js" ]; then
   cp "$GOROOT/lib/wasm/wasm_exec.js" "$OUT/"
elif [ -f "$GOROOT/misc/wasm/wasm_exec.js" ]; then
   cp "$GOROOT/misc/wasm/wasm_exec.js" "$OUT/"
else
   echo "ERROR: wasm_exec.js not found in GOROOT=$GOROOT"
   exit 1
fi

# Regenerate component i18n templates (mod/**/*.i18n.html) and catalogs
# (mod/i18n/*.json) from the .html sources. Deflang is pt-BR (the demo's source
# language); es-AR/en-US catalogs are remapped where source strings survive and
# seeded empty otherwise. gen_i18n is built from the wings tree so its
# transitive deps resolve against wings's go.sum, not this sub-module's.
GEN_I18N_BIN="$(mktemp -d)/gen_i18n"
(cd "$WPRANA" && go build -buildvcs=false -o "$GEN_I18N_BIN" ./cmd/gen_i18n)
# Sign the catalogs so the demo dogfoods signature verification. The keypair is
# committed on purpose (this is a public demo, not a secret-bearing app); the
# password below is intentionally not a secret. -sign-key writes a <lang>.json.sig
# sidecar next to every main catalog (inflections/fmt are not signed yet).
"$GEN_I18N_BIN" --path ./mod --deflang pt-BR \
   -sign-key ./gen_i18n.ed25519.key -sign-key-password 'wings-live-demo'

mkdir -p "$OUT/i18n"
# Ship only the browser-side catalogs (data + inflections) plus the .sig
# sidecars. The .meta.json companions carry server-only source positions
# (file:line:col) and must not leak to the published site. The .sig signs the
# mod/i18n bytes, which `cp` reproduces verbatim in $OUT/i18n.
cp mod/i18n/*.json "$OUT/i18n/"
cp mod/i18n/*.json.sig "$OUT/i18n/"
rm -f "$OUT"/i18n/*.meta.json

# -tags wings_test compiles modules' Testable() self-tests into the demo so the
# <w-test-report> card has something to run. A real app omits this tag for
# production builds (and uses it only for a throwaway test app in dev/CI).
GOOS=js GOARCH=wasm go build -buildvcs=false -tags wings_test -o "$OUT/main.wasm" .

# ── SRI hash injection ────────────────────────────────────────────────────────
# Compute SHA-384 integrity hashes for the JS helpers we just copied into $OUT
# and inject them into $OUT/index.html. This is the script that owns the
# published docs/, so it is the place that hashes the served files.
# Requires: openssl, sed (GNU or BSD both work).
sri_hash() {
    printf 'sha384-%s' "$(openssl dgst -sha384 -binary "$1" | openssl base64 -A)"
}

inject_sri() {
    local html="$1" file="$2" name hash
    name=$(basename "$file")
    hash=$(sri_hash "$file")
    # Idempotent: rewrites src="<name>" plus any existing integrity/crossorigin
    # so re-runs don't duplicate attributes.
    sed -i.bak \
        "s|src=\"${name}\"\\( integrity=\"[^\"]*\"\\)\\{0,1\\}\\( crossorigin=\"[^\"]*\"\\)\\{0,1\\}|src=\"${name}\" integrity=\"${hash}\" crossorigin=\"anonymous\"|g" \
        "$html"
    rm -f "${html}.bak"
}

inject_sri "$OUT/index.html" "$OUT/prana_helper.js"
inject_sri "$OUT/index.html" "$OUT/wasm_exec.js"

echo "Build complete. Run: go run serve.go"
