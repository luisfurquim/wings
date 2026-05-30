#!/bin/bash
set -e

cd "$(dirname "$0")"

WPRANA=$(go list -m -f '{{.Dir}}' github.com/luisfurquim/wings)
cp "$WPRANA/prana_helper.js" docs/

GOROOT="$(go env GOROOT)"
if [ -f "$GOROOT/lib/wasm/wasm_exec.js" ]; then
   cp "$GOROOT/lib/wasm/wasm_exec.js" docs/
elif [ -f "$GOROOT/misc/wasm/wasm_exec.js" ]; then
   cp "$GOROOT/misc/wasm/wasm_exec.js" docs/
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
"$GEN_I18N_BIN" --path ./mod --deflang pt-BR

mkdir -p docs/i18n
# Ship only the browser-side catalogs (data + inflections). The .meta.json
# companions carry server-only source positions (file:line:col) and must not
# leak to the published site.
cp mod/i18n/*.json docs/i18n/
rm -f docs/i18n/*.meta.json

GOOS=js GOARCH=wasm go build -buildvcs=false -o docs/main.wasm .

echo "Build complete. Run: go run serve.go"
