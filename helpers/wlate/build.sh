#!/bin/bash
set -e

cd "$(dirname "$0")"

# Copy JS helpers
WPRANA=$(go list -m -f '{{.Dir}}' github.com/luisfurquim/wings)
cp "$WPRANA/prana_helper.js" dist/

GOROOT="$(go env GOROOT)"
if [ -f "$GOROOT/lib/wasm/wasm_exec.js" ]; then
   cp "$GOROOT/lib/wasm/wasm_exec.js" dist/
elif [ -f "$GOROOT/misc/wasm/wasm_exec.js" ]; then
   cp "$GOROOT/misc/wasm/wasm_exec.js" dist/
else
   echo "ERROR: wasm_exec.js not found in GOROOT=$GOROOT"
   exit 1
fi

# Regenerate wlate's own i18n template + catalogs.
# Deflang is pt-BR (wlate's source language); en-US and es-CO catalogs
# are remapped when source strings survive, seeded otherwise.
# gen_i18n is built from the wprana tree so its transitive deps (x/net/html,
# x/text/language) resolve against wprana's go.sum, not this sub-module's.
GEN_I18N_BIN="$(mktemp -d)/gen_i18n"
(cd "$WPRANA" && go build -buildvcs=false -o "$GEN_I18N_BIN" ./cmd/gen_i18n)
"$GEN_I18N_BIN" --path ./mod --deflang pt-BR

# Publish catalogs to dist/ under /wlate-i18n/ so wi18n can fetch them
# without colliding with the /i18n/ route used for the translated project.
mkdir -p dist/wlate-i18n
cp mod/i18n/*.json dist/wlate-i18n/

# Generate placeholder meta files for the demo project (dist/i18n/).
# These prevent 404 noise; context/ctxdetail are empty since the demo has
# no real HTML templates to extract positions from.
python3 - <<'PYEOF'
import json, os, glob
for data_file in glob.glob("dist/i18n/*.json"):
    if ".meta." in data_file:
        continue
    meta_file = data_file.replace(".json", ".meta.json")
    if os.path.exists(meta_file):
        continue
    with open(data_file, encoding="utf-8") as f:
        entries = json.load(f)
    n = len(entries) if isinstance(entries, list) else 0
    with open(meta_file, "w", encoding="utf-8") as f:
        json.dump([{"context": "", "ctxdetail": ""} for _ in range(n)], f, indent=2)
        f.write("\n")
PYEOF

# Build WASM binary (embeds mod/wlate/wlate.i18n.html)
CGO_ENABLED=0 GOOS=js GOARCH=wasm go build -buildvcs=false -o dist/main.wasm .

echo "Build complete. Run: go run serve.go"
