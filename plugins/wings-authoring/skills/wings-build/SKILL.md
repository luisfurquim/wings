---
name: wings-build
description: Set up, build, and serve a WINGS app — Go module setup, the WASM build (GOOS=js), copying the JS runtime helpers, a static dev server, and the cross-platform cmd/build orchestrator (targets + the camelCase binding lint + rebuild-on-save dev mode). Use when scaffolding a new WINGS project, configuring its build, wiring main.go, or setting up a dev server / dev loop.
---

# Building and serving a WINGS app

WINGS components compile to a single WebAssembly binary loaded by a tiny JS
shim. A WASM page **cannot be opened from `file://`** — it must be served over
HTTP with the correct MIME type.

## Project layout

```
myapp/
  go.mod                 # module myapp
  main.go                # package main, wings.Main(), blank-imports modules
  mod/<name>/<name>.{go,html,css}   # your components (see wings-component)
  static/                # web root: index.html + the 3 runtime files
    index.html
    prana_helper.js
    wasm_exec.js
    wings.wasm           # build output
```

Set it up:
```bash
go mod init myapp
go get github.com/luisfurquim/wings
```

`main.go` — one per app; every component is registered by **blank-importing**
its package:
```go
//go:build js && wasm
package main

import (
	"github.com/luisfurquim/wings"
	_ "myapp/mod/hello"            // each module
	_ "github.com/luisfurquim/wings/wi18n" // optional: runtime i18n (see wings-i18n)
)

func main() { wings.Main() }
```

## The two runtime helper files

Copy both into your web root next to the wasm:
```bash
W=$(go list -m -f '{{.Dir}}' github.com/luisfurquim/wings)
cp "$W/prana_helper.js" static/
# Go 1.24+: lib/wasm/ ; older: misc/wasm/
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" static/ 2>/dev/null \
  || cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" static/
```

`static/index.html` loads them and streams the wasm:
```html
<!DOCTYPE html>
<html>
<head>
  <script src="prana_helper.js"></script>
  <script src="wasm_exec.js"></script>
  <script>
    const go = new Go();
    WebAssembly.instantiateStreaming(fetch("wings.wasm"), go.importObject)
      .then(r => go.run(r.instance)).catch(console.error);
  </script>
</head>
<body>
  <hello-world></hello-world>   <!-- your custom element tag -->
</body>
</html>
```

## Build and serve

```bash
GOOS=js GOARCH=wasm go build -o static/wings.wasm .
```

Then serve `static/` over HTTP with `.wasm` as `application/wasm` (any static
server that sets that MIME; a tiny Go server works — see the README Quick Start).
Opening `index.html` from disk will not work.

## Dev loop and the binding lint (recommended)

The wings module ships a cross-platform orchestrator that adds rebuild-on-save,
template linting, and i18n generation — no `sed`/`make`/`python` needed:

```bash
go run github.com/luisfurquim/wings/cmd/build@latest dev
```

`dev` watches your sources, rebuilds the wasm on save, serves the web root, and
**lints every template**, failing on a binding NAME with an uppercase letter
(`?cond`, `*arr`, `**arr`, `&attr`) — the silent-no-op trap from `wings-component`
gotcha #1. Configure it with `WINGS_*` env vars (`WINGS_PORT`, `WINGS_WEBROOT`,
`WINGS_MAIN`, `WINGS_WATCH_MODE`, i18n options, …); see `dev/docker/README.md`
in the wings repo for the full reference and a Docker dev-container setup.

If you build manually instead, you don't get the lint — so keep binding names
snake_case yourself.

## Notes

- Everything component-side is `//go:build js && wasm`; the build sets
  `GOOS=js GOARCH=wasm`.
- `go run ./cmd/build <lib|example|live-demo|wlate|all|vulncheck|fuzz>` are the
  wings **repo's own** targets (for building wings itself), not your app — your
  app uses `dev` or the manual build above.
- Deeper: README "Quick Start", "Dev Container", "Project Setup".
