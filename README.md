<p align="center">
  <img src="https://raw.githubusercontent.com/luisfurquim/wings/main/assets/logo.png" alt="WINGS logo" width="200">
</p>

<h1 align="center">WINGS</h1>

<p align="center"><strong>Web IN Go Sphere</strong></p>

[![Go Reference](https://pkg.go.dev/badge/github.com/luisfurquim/wings.svg)](https://pkg.go.dev/github.com/luisfurquim/wings)
[![License: MPL 2.0](https://img.shields.io/badge/License-MPL%202.0-brightgreen.svg)](https://opensource.org/licenses/MPL-2.0)
[![Go Report Card](https://www.goreportcard.com/badge/github.com/luisfurquim/wings?ts=1712345678)](https://www.goreportcard.com/report/github.com/luisfurquim/wings?ts=1712345678)

> **[Live Demo](https://luisfurquim.github.io/wings/)** — try it in your browser, no install needed.
> Or run locally: clone the repo, `cd live-demo && bash build.sh && go run serve.go`

**Build reactive Web Components in pure Go — no JavaScript framework required.**

WINGS compiles to WebAssembly and gives you custom HTML elements with
automatic data binding, conditional rendering, array iteration, two-way
form binding, hash-based routing, and parent-child communication — all
authored in Go and running natively in the browser.

### Why WINGS?

| | |
|---|---|
| **Pure Go** | Write components, state, and logic entirely in Go. Templates stay in plain HTML. |
| **Reactive** | Change a value with `Set()` and the DOM updates automatically — no virtual DOM diffing overhead. |
| **Lightweight** | Direct DOM manipulation via targeted refs. No framework runtime to download beyond your WASM binary. |
| **Encapsulated** | Each component lives inside a Shadow DOM with scoped CSS — no style leaks, no naming collisions. |
| **Two-Way Binding** | The `&` prefix syncs `<input>`, `<select>`, and `<textarea>` with your Go data map in both directions. |
| **Hash Routing** | Built-in `{{#}}` binding and `wings.GoTo()` for SPA navigation without a router library. |
| **Composable** | Nest components freely. Parent-to-child data flows via attributes; child-to-parent events flow via `@` triggers. |
| **Standard Web** | Uses native Custom Elements v1 and Shadow DOM — works alongside any existing page or framework. |
| **Internationalized** | Build-time text extraction + runtime catalog lookup, with plural/gender flexion and locale-aware number/date/measure formatting. |
| **Themeable** | Global `--wings-*` design tokens; compose multiple runtime [skins](#skins--theming-with---wings--tokens) (colours, geometry, depth, motion, atmosphere). |

---

## What's New

Release highlights — full history in [CHANGELOG.md](CHANGELOG.md).

### v0.15.0

- **Programmable flex (CustomFlex)** — plug in your own inflection engine via the new `*var` / `$var` / `~$var` sigils.
- **Catalog signing** — opt-in ed25519 verification of i18n catalogs (`-genkey` / `-sign-key` + `SetCatalogPublicKey`).
- **Idempotent SRI** — `integrity` injection in the build scripts fixed and unified (`wlate` now included).

  → [Internationalization](#internationalization-i18n) · [Security](#security)

### v0.14.x

- **`wprana` → WINGS** — project rename ([migration guide](#migrating-from-wprana)).
- **i18n suite** — flexion, locale-aware formatting, physical measures, runtime `SetLang`.
- **Skins** — 18 composable skins across five families, plus user-defined categories (bits 48–63).
- **Widgets** — the `w-tabs` family and friends ([widgets](#built-in-widgets)), and the **wlate** translation editor.

---

## Table of Contents

- [What's New](#whats-new)
- [Migrating from `wprana`](#migrating-from-wprana)
- [Quick Start](#quick-start)
- [Project Setup](#project-setup)
- [Creating a Module](#creating-a-module)
- [Template Syntax](#template-syntax)
  - [Expression Binding](#expression-binding)
  - [Hash Fragment Binding](#hash-fragment-binding)
  - [Conditional Rendering](#conditional-rendering)
    - [Boolean (truthiness)](#boolean-truthiness)
    - [Negated Boolean](#negated-boolean-var)
    - [Equality](#equality-varvalue)
    - [Inequality](#inequality-varvalue)
    - [Prefix](#prefix-varvalue)
    - [Suffix](#suffix-varvalue)
    - [Contains](#contains-varvalue)
  - [Array Iteration](#array-iteration)
  - [Two-Way Binding](#two-way-binding)
  - [Events (Child to Parent)](#events-child-to-parent)
- [Reactive Data API](#reactive-data-api)
  - [Navigation — Hash Fragment](#navigation--hash-fragment)
- [How DOM Updates Work](#how-dom-updates-work)
- [Helper Packages](#helper-packages)
  - [wings/dom — Events and Queries](#wingsdom--events-and-queries)
  - [wings/timer — Timers](#wingstimer--timers)
  - [wings/location — Browser Location](#wingslocation--browser-location)
  - [wings.KeyStorage — Storage Interface](#wingskeystorage--storage-interface)
  - [wings/localstorage — LocalStorage](#wingslocalstorage--localstorage)
  - [wings/opfs — Origin Private File System](#wingsopfs--origin-private-file-system)
  - [JavaScript Interop (core)](#javascript-interop-core)
- [Customizable Widgets](#customizable-widgets)
  - [Customizable Interface](#customizable-interface)
  - [wings.Update — Dynamic CSS](#wingsupdate--dynamic-css)
- [Skins — Theming with --wings-* Tokens](#skins--theming-with---wings--tokens)
  - [Multi-skin composition](#multi-skin-composition)
  - [Built-in skins (18)](#built-in-skins-18)
  - [Registering your own skin](#registering-your-own-skin)
  - [Skin API](#skin-api)
- [Built-in Widgets](#built-in-widgets)
  - [wings/widget/combobox — Multi-select Combobox](#wingswidgetcombobox--multi-select-combobox)
  - [wings/widget/tabs — Tabbed Container](#wingswidgettabs--tabbed-container-w-tabs--w-tabbutton--w-tab)
  - [wings/widget/navbar — Record Navigation Toolbar](#wingswidgetnavbar--record-navigation-toolbar-w-navbar)
  - [wings/widget/skinswitcher — Skin Picker](#wingswidgetskinswitcher--skin-picker-skin-switcher)
- [Internationalization (i18n)](#internationalization-i18n)
  - [Pipeline Overview](#pipeline-overview)
  - [wings/wi18n — Runtime Lookup](#wingswi18n--runtime-lookup)
  - [Runtime Locale Switching (SetLang)](#runtime-locale-switching-setlang)
  - [cmd/gen_i18n — Build-time Extractor](#cmdgen_i18n--build-time-extractor)
    - [Opting out (translate=no)](#opting-out-translateno)
  - [Flexion — Plurals & Gender (SynPrinter)](#flexion--plurals--gender-synprinter)
  - [Programmable Flex — Custom Inflection Engines (CustomFlex)](#programmable-flex--custom-inflection-engines-customflex)
  - [Locale-Aware Formatting (FmtPrinter)](#locale-aware-formatting-fmtprinter)
  - [Physical Measure Packages](#physical-measure-packages)
  - [cmd/dictbuild & cmd/dictlookup — Flexion Dictionaries](#cmddictbuild--cmddictlookup--flexion-dictionaries)
  - [helpers/wlate — Translation Editor GUI](#helperswlate--translation-editor-gui)
    - [server.conf — mini-server configuration](#serverconf--mini-server-configuration)
- [Component Lifecycle](#component-lifecycle)
- [Parent-Child Communication](#parent-child-communication)
- [Syntax Quick-Reference](#syntax-quick-reference)
- [Important Notes](#important-notes)
- [Full Example](#full-example)
- [License](#license)

---

## Migrating from `wprana`

WINGS was previously published as **`wprana`**. The import path is now
`github.com/luisfurquim/wings` and the package identifier is `wings`.

**New code** imports it directly:

```go
import "github.com/luisfurquim/wings"

// wings.Register(...), wings.Main(), wings.GoTo(...), etc.
```

**Already have code that calls `wprana.Foo(...)`?** You don't have to touch every
call site. Point your module at the new path and alias the import back to
`wprana`:

```bash
go get github.com/luisfurquim/wings
```

```go
import wprana "github.com/luisfurquim/wings"

// every existing wprana.Foo(...) call keeps compiling unchanged
```

Only the `import` line changes — the rest of your code stays as-is. You can
migrate to the `wings.` name gradually, or not at all.

---

## Quick Start

WASM binaries cannot be loaded from `file://` URLs. The snippet below
builds a hello-world component, copies the required runtime files, and
starts a tiny Go server so you can open the page in a browser.

> **Windows users:** the snippets below and the project's `build.sh` scripts
> assume a POSIX shell. Run them under **WSL** (recommended — official
> Microsoft path, ships with Windows 10/11) or **Git Bash** (bundled with
> Git for Windows). Native `cmd.exe` / PowerShell will not execute them
> directly. For the helper scripts under `helpers/wlate/` and `live-demo/`,
> also ensure `python3` and `openssl` are on `PATH`.

```bash
# 1. Create the project
mkdir hello-wings && cd hello-wings
go mod init hello-wings
go get github.com/luisfurquim/wings

# 2. Copy the JS helpers from the wings module
WPRANA=$(go list -m -f '{{.Dir}}' github.com/luisfurquim/wings)
mkdir -p static
cp "$WPRANA/prana_helper.js" static/
# Go 1.24+: lib/wasm/   Go ≤1.23: misc/wasm/
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" static/ 2>/dev/null || \
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" static/
```

Create `static/index.html`:

```html
<!DOCTYPE html>
<html>
<head>
   <script src="prana_helper.js"></script>
   <script src="wasm_exec.js"></script>
   <script>
      const go = new Go();
      WebAssembly
         .instantiateStreaming(fetch("main.wasm"), go.importObject)
         .then(r => go.run(r.instance))
         .catch(console.error);
   </script>
</head>
<body>
   <hello-world></hello-world>
</body>
</html>
```

Create `mod/hello/hello.go`:

```go
//go:build js && wasm

package hello

import (
    _ "embed"
    "github.com/luisfurquim/wings"
)

//go:embed hello.html
var htmlContent string

type Hello struct{}

func init() {
    wings.Register("hello-world", htmlContent, "",
        func() wings.PranaMod { return &Hello{} })
}

func (h *Hello) InitData() map[string]any {
    return map[string]any{"greeting": "Hello from Go + WASM!"}
}

func (h *Hello) Render(_ *wings.PranaObj) {}
```

Create `mod/hello/hello.html`:

```html
<h1>{{greeting}}</h1>
```

Create `main.go`:

```go
//go:build js && wasm

package main

import (
    "github.com/luisfurquim/wings"
    _ "hello-wings/mod/hello"
)

func main() { wings.Main() }
```

Build and serve:

```bash
# 3. Build the WASM binary
GOOS=js GOARCH=wasm go build -o static/main.wasm .

# 4. Start a minimal dev server (paste into serve.go, then run it)
cat > serve.go.tmp <<'GOFILE'
//go:build ignore

package main

import (
    "fmt"
    "net/http"
)

func main() {
    fs := http.FileServer(http.Dir("static"))
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path[len(r.URL.Path)-5:] == ".wasm" {
            w.Header().Set("Content-Type", "application/wasm")
        }
        fs.ServeHTTP(w, r)
    })
    fmt.Println("Listening on http://localhost:8080")
    http.ListenAndServe(":8080", nil)
}
GOFILE
go run serve.go.tmp
```

Open **http://localhost:8080** and you should see "Hello from Go + WASM!".

## Project Setup

A WINGS project has the following structure:

```
myapp/
├── go.mod
├── main.go                 # WASM entry point
├── mod/
│   └── mywidget/
│       ├── mywidget.go     # Module logic (implements PranaMod)
│       ├── mywidget.html   # Template with binding syntax
│       └── mywidget.css    # Component styles
└── static/
    ├── index.html          # HTML page
    ├── prana_helper.js     # wings JS bridge (from wings package)
    └── wasm_exec.js        # Go WASM runtime
```

### HTML Page

The load order is critical. `prana_helper.js` **must** come before `wasm_exec.js`,
and both must come before the WASM binary:

```html
<!DOCTYPE html>
<html>
<head>
   <!-- 1. wings JS bridge (defines window._pranaDef) -->
   <script src="prana_helper.js"></script>

   <!-- 2. Go WASM runtime -->
   <script src="wasm_exec.js"></script>

   <!-- 3. Load and run the WASM binary -->
   <script>
      const go = new Go();
      WebAssembly
         .instantiateStreaming(fetch("main.wasm"), go.importObject)
         .then(result => go.run(result.instance))
         .catch(err => console.error(err));
   </script>
</head>
<body>
   <my-widget title="Demo"></my-widget>
</body>
</html>
```

### Entry Point (main.go)

```go
//go:build js && wasm

package main

import (
    "github.com/luisfurquim/wings"

    // Side-effect imports: each init() registers a module via wings.Register()
    _ "myapp/mod/mywidget"
)

func main() {
    wings.Main() // Defines all custom elements and blocks forever
}
```

## Creating a Module

Each module implements the `PranaMod` interface:

```go
type PranaMod interface {
    InitData() map[string]any    // Returns initial component state
    Render(obj *PranaObj)        // Called after connection to DOM
}
```

### Example Module

**mod/counter/counter.go**
```go
//go:build js && wasm

package counter

import (
    _ "embed"
    "github.com/luisfurquim/wings"
    "github.com/luisfurquim/wings/timer"
)

//go:embed counter.html
var htmlContent string

//go:embed counter.css
var cssContent string

type Counter struct{}

func init() {
    wings.Register(
        "my-counter",       // custom element tag name
        htmlContent,         // embedded HTML template
        cssContent,          // embedded CSS
        func() wings.PranaMod { return &Counter{} },
        "title",             // observed attributes
    )
}

func (c *Counter) InitData() map[string]any {
    return map[string]any{
        "title": "Counter",
        "count": 0,
    }
}

func (c *Counter) Render(obj *wings.PranaObj) {
    // Set up a ticker that increments count every second
    go func() {
        tk := timer.NewTicker(1000)
        defer tk.Stop()
        n := 0
        for range tk.Tick {
            n++
            obj.This.Set("count", n)
        }
    }()
}
```

**mod/counter/counter.html**
```html
<div class="counter">
   <h2>{{title}}</h2>
   <p>Count: <span>{{count}}</span></p>
</div>
```

## Template Syntax

### Expression Binding

Use `{{expression}}` to bind data values to the DOM. Expressions are automatically
updated when the data changes.

```html
<!-- Simple variable -->
<span>{{title}}</span>

<!-- Nested field access -->
<span>{{user.name}}</span>

<!-- Array access with literal index -->
<span>{{items[0]}}</span>

<!-- Array access with variable index -->
<span>{{items[i].label}}</span>

<!-- Attributes -->
<img src="{{avatar_url}}" alt="{{username}}" />
```

### Hash Fragment Binding

The special reference `{{#}}` resolves to the current URL hash fragment
(i.e. the portion of `window.location.hash` after the `#` sign).

```html
<!-- If URL is https://example.com/app#settings, displays "settings" -->
<span>Current view: {{#}}</span>

<!-- Conditional rendering based on hash -->
<div ?#="home">Home content here</div>
<div ?#="settings">Settings panel</div>
```

wings automatically monitors `window.location.hash` and triggers a sync on
**all** live component instances whenever the hash changes, so `{{#}}`
references are always up to date.

To change the hash programmatically from Go:

```go
wings.GoTo("settings")   // sets window.location.hash = "settings"
```

This fires the browser's `hashchange` event, which in turn updates every
`{{#}}` binding across all components.

### Conditional Rendering

Use the `?` prefix on an attribute to conditionally show or hide an element.
Seven forms are supported:

#### Boolean (truthiness)

```html
<!-- Shows only when show_details is truthy -->
<div ?show_details>
    <p>Details: {{details}}</p>
</div>

<!-- Shows only when items array has elements -->
<div ?items.length>
    <p>There are items!</p>
</div>
```

#### Negated Boolean (`?!var`)

Shows the element only when the variable is **falsy** (the logical negation of
the truthiness check):

```html
<!-- Shows only when is_loading is falsy -->
<div ?!is_loading>
    <p>Content has loaded.</p>
</div>

<!-- Shows only when items array is empty -->
<div ?!items.length>
    <p>No items found.</p>
</div>
```

#### Equality (`?var="value"`)

Shows the element only when the variable's string representation equals the
given value:

```html
<!-- Shows only when user_type is "A" (Author) -->
<div ?user_type="A">
    <p>Author-specific content</p>
</div>

<!-- Shows only when status is "active" -->
<span ?status="active" class="badge">Active</span>
```

#### Inequality (`?var!="value"`)

Shows the element only when the variable's string representation does **not**
equal the given value:

```html
<!-- Shows for any user_type except "R" (Reader) -->
<div ?user_type!="R">
    <p>Extra profile fields</p>
</div>

<!-- Hides when status is "deleted" -->
<div ?status!="deleted">
    <p>This record is visible</p>
</div>
```

#### Prefix (`?var^="value"`)

Shows the element only when the variable's string representation **starts with**
the given value:

```html
<!-- Shows only when url starts with "https" -->
<div ?url^="https">
    <p>Secure connection</p>
</div>
```

#### Suffix (`?var$="value"`)

Shows the element only when the variable's string representation **ends with**
the given value:

```html
<!-- Shows only when filename ends with ".pdf" -->
<div ?filename$=".pdf">
    <p>PDF document</p>
</div>
```

#### Contains (`?var*="value"`)

Shows the element only when the variable's string representation **contains**
the given value as a substring:

```html
<!-- Shows only when tags contain "urgent" -->
<div ?tags*="urgent">
    <p class="alert">Urgent item</p>
</div>
```

#### How operators map to HTML attributes

The browser parses these forms naturally — no special escaping is needed:

| Template syntax | HTML attr name | HTML attr value | Operator |
|---|---|---|---|
| `?cond` | `?cond` | *(empty)* | truthy |
| `?!cond` | `?!cond` | *(empty)* | negated truthy |
| `?cond="abc"` | `?cond` | `abc` | equality |
| `?cond!="abc"` | `?cond!` | `abc` | inequality |
| `?cond^="abc"` | `?cond^` | `abc` | starts with |
| `?cond$="abc"` | `?cond$` | `abc` | ends with |
| `?cond*="abc"` | `?cond*` | `abc` | contains |

> **Note:** Comparison operators `<` and `>` are not supported because they
> conflict with HTML tag syntax.

#### Behavior

When the condition is falsy (or the comparison fails), the element is replaced
by a comment node. When the condition becomes truthy (or the comparison
succeeds), the original element is restored.

**Truthiness rules** (for the boolean form, same as JavaScript):
- Falsy: `nil`, `false`, `0`, `""`, empty `[]any{}`
- Truthy: everything else

**Comparison rules** (for equality/inequality forms):
- The variable value is converted to its string representation via
  `fmt.Sprintf("%v", value)` before comparing with the attribute value.
- This means numeric values work as expected: `?count="0"` matches when
  count is `0` (int) or `"0"` (string).

### Array Iteration

Use the `*` prefix to repeat an element for each item in an array.

#### Single-element iteration (`*array:index`)

Creates a `<span>` wrapper around the repeated elements:

```html
<ul>
   <li *items:i>{{items[i].label}}</li>
</ul>
```

With data:
```go
"items": []any{
    map[string]any{"label": "Alpha"},
    map[string]any{"label": "Beta"},
    map[string]any{"label": "Gamma"},
}
```

Produces:
```html
<ul>
   <span>
      <li>Alpha</li>
      <li>Beta</li>
      <li>Gamma</li>
   </span>
</ul>
```

#### Container iteration (`**array:index`)

The parent element itself becomes the container (no extra wrapper):

```html
<!--
   Important! With **, the iterator attribute goes on the CONTAINER element
   (here <ul>), not on the repeated child. The first child element (<li>)
   becomes the template that is cloned for each array item.
-->
<ul **items:i>
   <li>{{items[i].label}}</li>
</ul>
```

Produces:
```html
<ul>
   <li>Alpha</li>
   <li>Beta</li>
   <li>Gamma</li>
</ul>
```

The index variable (`:i`) is available in the template. You can use `{{i}}` to
access the current iteration index.

### Two-Way Binding

Use the `&` prefix on an attribute to establish a two-way binding between a form
input and a data variable. Changes to the input update the data, and changes to
the data update the input.

```html
<input &value="{{username}}" type="text" placeholder="Username" />
<input &value="{{password}}" type="password" />
<select &value="{{selected}}">
    <option value="a">Option A</option>
    <option value="b">Option B</option>
</select>
<textarea &value="{{bio}}"></textarea>
```

Two-way binding only works with:
- `<input>`
- `<select>`
- `<textarea>`

And requires a pure reference (single `{{variable}}`), not mixed text like
`"prefix {{variable}}"`.

### Events (Child to Parent)

Use the `@` prefix in the **parent's template** to bind a child component event
to a handler function defined in the parent's data. The child fires the event
using `obj.Trigger("event_name")`.

```html
<!-- Parent template: @event_name="handler_name" -->
<my-login @login="on_login" @logout="on_logout"></my-login>
```

The naming works like this:
- `@login` is the **event name** — this is what the child passes to `Trigger`
- `"on_login"` is the **handler function name** — looked up in the parent's data map

The child fires the event using only the name **without** the `@` prefix:
```go
// In the CHILD's Render:
obj.Trigger("login")       // matches @login in parent's template
obj.Trigger("logout")      // matches @logout in parent's template
```

The handler must be a `func(...any)` (or `wings.TriggerHandler`) in the
parent's data map.

**Important:** In `InitData`, the `obj` parameter is not yet available —
it only becomes available in `Render`. If your handler needs `obj` (which
is almost always the case), use `wings.TriggerHandler(nil)` as a
placeholder in `InitData`, then set the real handler in `Render`:

```go
func (app *App) InitData() map[string]any {
    return map[string]any{
        // Placeholder — obj is not available here
        "on_login":  wings.TriggerHandler(nil),
        "on_logout": wings.TriggerHandler(nil),
    }
}

func (app *App) Render(obj *wings.PranaObj) {
    // Now obj is available — define the real handlers
    obj.This.Set("on_login", func(args ...any) {
        obj.This.Set("is_logged", true)
        obj.This.Set("is_anonymous", false)
    })
    obj.This.Set("on_logout", func(args ...any) {
        obj.This.Set("is_logged", false)
        obj.This.Set("is_anonymous", true)
    })
}
```

You can pass arguments from the child to the parent handler:
```go
obj.Trigger("login", username, token)
```

Note: `@` event attributes are read directly from the DOM at trigger time, so
they do **not** need to be listed in the child's observed attributes. Only
attributes whose values change at runtime (like `&` bindings) need to be observed.

See [Parent-Child Communication](#parent-child-communication) for a complete example.

## Reactive Data API

The `ReactiveData` type wraps a `map[string]any` and triggers automatic DOM
synchronization on every mutation.

```go
func (c *MyComponent) Render(obj *wings.PranaObj) {
    // Set a value (triggers DOM sync)
    obj.This.Set("title", "New Title")

    // Get a value
    title := obj.This.Get("title").(string)

    // Delete a key
    obj.This.Delete("old_field")

    // Append to an array
    obj.This.Append("items", map[string]any{"label": "New Item"})

    // Set array element at index
    obj.This.SetAt("items", 0, map[string]any{"label": "Updated"})

    // Delete array element at index
    obj.This.DeleteAt("items", 2)

    // Access the raw map directly (no automatic sync)
    obj.This.M["key"] = "value"
    // Then trigger sync manually:
    obj.This.Sync()
}
```

### Navigation — Hash Fragment

WINGS exposes a package-level function to change the URL hash fragment:

```go
// Navigate to a new view
wings.GoTo("settings")  // -> window.location.hash = "#settings"

// Clear the hash
wings.GoTo("")          // -> window.location.hash = ""
```

All `{{#}}` bindings and `?#="value"` conditionals update automatically.

## How DOM Updates Work

WINGS does **not** use a Virtual DOM. Instead it relies on **direct,
targeted DOM manipulation** guided by a compile-time reference map.

### Reference Extraction

When a component is first connected, WINGS walks the HTML template once
and builds a `DOMRefNode` tree — a lightweight map that records, for every
DOM node, which data keys appear in its text content and attributes. This
map is stored alongside the component's reactive state and never
rebuilt.

### Synchronization Cycle

Every mutation through the `ReactiveData` API (`Set`, `Delete`, `Append`,
`DeleteAt`, `SetAt`, or `Sync`) increments a global **epoch counter** and
kicks off a synchronization pass:

1. **Epoch guard** — each component tracks the last epoch it was synced at.
   If the current epoch matches, the sync is skipped, breaking circular
   propagation between parent and child components.

2. **Tree walk** — the engine walks the `DOMRefNode` tree in parallel with
   the live DOM tree. For each node it:
   - **Text nodes**: resolves all `{{expression}}` segments against the
     current data context and writes the result to `node.data` (or
     `element.value` for `<textarea>`).
   - **Attributes**: resolves each bound attribute's segments and calls
     `setAttribute` only when the new value differs from the current one.
   - **Conditionals** (`?`): evaluates the condition. If false, the element
     is replaced by a comment placeholder; if true and currently hidden,
     the original element is restored from the stored reference.
   - **Arrays** (`*` / `**`): compares the current array length to the
     number of child nodes. Adds clones of the template for new items,
     removes excess nodes for deleted items, and recursively syncs each
     child with its corresponding array element.
   - **Two-way bindings** (`&`): updates the input's `value` property
     and keeps the stored context pointer current so the `onchange`
     handler always writes back to the correct data key.

3. **Propagation** — after the local sync, observed attributes on child
   custom elements are updated via `setAttribute`, which triggers their
   own `attributeChangedCallback` and a downstream sync (subject to the
   same epoch guard).

### Why Not a Virtual DOM?

A virtual DOM diffs an entire tree snapshot to compute the minimum set of
mutations. WINGS skips the diff entirely: the reference map already knows
*which* DOM nodes depend on *which* data keys, so it can jump directly to
the affected nodes. This makes updates O(bindings) rather than O(tree
size), with no garbage from disposable tree snapshots — an important
property in a WASM environment where GC pauses are more noticeable.

## Helper Packages

Helper functions are organized into subpackages so applications only
import what they actually use, keeping the WASM binary lean.

### wings/dom — Events and Queries

`import "github.com/luisfurquim/wings/dom"`

Register DOM event listeners with automatic `preventDefault` and `stopPropagation`
support:

```go
func (c *MyComponent) Render(obj *wings.PranaObj) {
    forms := dom.Query(obj.Dom, "form")
    if len(forms) > 0 {
        // Register submit handler with preventDefault
        handlerID := dom.AddEvent(forms[0], "submit",
            func(this js.Value, args []js.Value) any {
                username := obj.This.Get("username").(string)
                password := obj.This.Get("password").(string)
                // ... handle login
                return nil
            },
            true,  // preventDefault
            false, // stopPropagation
        )

        // Later, to remove the handler:
        // dom.RmEvent(handlerID)
    }
}
```

**API:**

```go
func dom.AddEvent(el js.Value, eventName string,
    handler func(this js.Value, args []js.Value) any,
    preventDefault, stopPropagation bool) int64

func dom.RmEvent(id int64)

func dom.Query(el js.Value, selector string) []js.Value
```

### wings/timer — Timers

`import "github.com/luisfurquim/wings/timer"`

```go
// Sleep blocks the current goroutine for ms milliseconds,
// yielding control to the JS event loop.
timer.Sleep(2000)

// NewTicker sends on Tick channel every ms milliseconds.
// Call Stop() to release resources.
tk := timer.NewTicker(1000)
defer tk.Stop()
for range tk.Tick {
    // called every second
}

// SetTimeout schedules fn after delay ms.
// Returns a channel that closes on completion.
done := timer.SetTimeout(func() {
    fmt.Println("fired!")
}, 5000)
<-done // wait for it

// SetInterval schedules fn every interval ms.
// Returns a cancel function.
cancel := timer.SetInterval(func() {
    fmt.Println("tick")
}, 1000)
// later:
cancel()
```

### wings/location — Browser Location

`import "github.com/luisfurquim/wings/location"`

```go
// Get window.location.href as *url.URL
loc, err := location.Get()

// Get top.location.href as *url.URL (useful inside iframes)
topLoc, err := location.GetTop()
```

### wings.KeyStorage — Storage Interface

The `wings.KeyStorage` interface defines a backend-agnostic key-value
storage API. It accepts arbitrary Go values and relies on an
Encoder/Decoder pair for serialization. Any storage backend (localStorage,
OPFS, IndexedDB, etc.) can implement this interface:

```go
type KeyStorage interface {
    Set(key string, val any) error
    Get(key string, outval any) error
    Del(key string) error
    Exists(key string) (bool, int64, error)
}
```

Modules that need persistent storage should accept a `wings.KeyStorage`
instead of a concrete type. This way the application's `main()` decides
which backend to use:

```go
// In a module package:
var Store wings.KeyStorage

// In main():
import "github.com/luisfurquim/wings/localstorage"
import "github.com/luisfurquim/wings/opfs"

// Option A: localStorage backend
myModule.Store = localstorage.NewKV(nil, nil)

// Option B: OPFS backend (recommended for larger/sensitive data)
myModule.Store = opfs.New(nil, nil)
```

### wings/localstorage — LocalStorage

`import "github.com/luisfurquim/wings/localstorage"`

Access browser `localStorage` with pluggable serialization.

#### Encoder / Decoder

Implement these interfaces to choose your encoding strategy
(JSON, Gob+base64, etc.):

```go
type Encoder interface {
    Encode(inpval any) string
}

type Decoder interface {
    Decode(buf string, outval any) error
}
```

If you pass `nil` for either parameter, a built-in default codec is used.
It handles common Go types out of the box:

| Type | Encode | Decode |
|------|--------|--------|
| `string` | passthrough | passthrough |
| `[]byte` | `string(v)` | `[]byte(s)` |
| `bool` | `"true"` / `"false"` | `strconv.ParseBool` |
| `int`, `int8`--`int64` | `strconv.FormatInt` | `strconv.ParseInt` |
| `uint`, `uint8`--`uint64` | `strconv.FormatUint` | `strconv.ParseUint` |
| `float32`, `float64` | `strconv.FormatFloat` | `strconv.ParseFloat` |

#### KV — Recommended API (implements wings.KeyStorage)

`KV` is the recommended way to use localStorage. It implements
`wings.KeyStorage`:

```go
// Create with default codec (handles string, int, float, bool, etc.)
kv := localstorage.NewKV(nil, nil)

// Or with a custom encoder/decoder
kv := localstorage.NewKV(myEncoder, myDecoder)

// Store a value
err := kv.Set("username", "Ana")

// Retrieve a value (outval must be a pointer)
var name string
err := kv.Get("username", &name)
if errors.Is(err, localstorage.ErrKeyNotFound) {
    // key does not exist
}

// Check existence and get stored string length
exists, size, err := kv.Exists("username")

// Remove a key
err := kv.Del("username")
```

#### LS — Legacy API

`LS` provides the original API with pluggable Encoder/Decoder. It does
**not** implement `wings.KeyStorage` (its `Set` and `Del` methods do not
return errors). New code should use `KV` instead.

```go
ls := localstorage.New(myEncoder, myDecoder)

// Or with the default codec
ls := localstorage.New(nil, nil)

ls.Set("user", map[string]any{"name": "Ana", "age": 30})

var user map[string]any
err := ls.Get("user", &user)

ls.Del("user")

// Iteration helpers (not available on KV)
n := ls.Len()
name, ok := ls.Key(0)
ls.Clear()
```

### wings/opfs — Origin Private File System

`import "github.com/luisfurquim/wings/opfs"`

Access the browser's [Origin Private File System](https://developer.mozilla.org/en-US/docs/Web/API/File_System_API/Origin_private_file_system)
directly from Go WASM. Files are stored in a sandboxed, origin-scoped
filesystem that is invisible to the user and not subject to the same
storage limits as localStorage.

`opfs.Store` implements `wings.KeyStorage` and uses the same
Encoder/Decoder pattern as `localstorage.KV`. If `nil` is passed for
either parameter, the built-in default codec is used (same type table
as localstorage).

```go
// Create with default codec
store := opfs.New(nil, nil)

// Store a value
err := store.Set("my-key", "hello world")

// Retrieve a value (outval must be a pointer)
var val string
err := store.Get("my-key", &val)
if errors.Is(err, opfs.ErrNotFound) {
    // key does not exist
}

// Check existence and get stored size in bytes
exists, size, err := store.Exists("my-key")

// Remove a key (no error if it does not exist)
err := store.Del("my-key")
```

The store accesses OPFS via the asynchronous File System API
(`navigator.storage.getDirectory()`), called directly through
`syscall/js`. No Service Worker is required.

### JavaScript Interop (core)

These functions remain in the core `wings` package:

```go
// Access the global window object
window := wings.JSGlobal()

// Create a persistent JS callback (must call Release() when done)
fn := wings.JSFunc(func(this js.Value, args []js.Value) any {
    // handle callback
    return nil
})
defer fn.Release()

// Create a one-shot JS callback (auto-releases after first call)
fn := wings.JSFuncOnce(func() {
    // handle callback
})
```

## Customizable Widgets

Modules that implement only `PranaMod` have fixed CSS. Modules that also
implement `Customizable` allow consuming applications to replace parts of
their CSS at runtime — for example, changing the color scheme without
touching the layout rules.

### Customizable Interface

```go
// CSSPart is a named section of a component's CSS.
type CSSPart struct {
    Name    string
    Content string
}

// Customizable extends PranaMod with CSS customization.
type Customizable interface {
    PranaMod
    ListCSS() []CSSPart
    ReplaceCSS(key string, content string)
}
```

- **`ListCSS()`** returns the CSS parts in order. The order matters: for
  example, a "Vars" part defining CSS custom properties must come before
  a "Design" part that uses `var()` references.
- **`ReplaceCSS(key, content)`** replaces the named part and updates all
  live instances immediately via `wings.Update()`.

### wings.Update — Dynamic CSS

```go
wings.Update(tagName string, cssContent string)
```

Replaces the CSS of a registered custom element and updates the `<style>`
tag in the Shadow DOM of every live instance. Called automatically by
`ReplaceCSS`; can also be called directly for full CSS replacement.

## Skins — Theming with `--wings-*` Tokens

A **skin** is a CSS payload — a block of `--wings-*` custom-property definitions
at `:root` — paired with a `SkinCategory` bitmask declaring which design
dimensions it touches. Widgets read every token through a `var(--wings-X,
fallback)` reference, so they look correct with no skin active and restyle
globally the moment one is applied. There is no per-widget theming step.

```go
import (
    "github.com/luisfurquim/wings"
    _ "github.com/luisfurquim/wings/skins/light" // blank import → init() registers it
    _ "github.com/luisfurquim/wings/skins/glass"
)

func main() {
    if err := wings.ApplySkin("light"); err != nil { /* … */ }
    _ = wings.ApplySkin("glass") // composes on top of light
    wings.Main()
}
```

Each active skin owns a `<style id="wings-skin-NAME" data-wings-skin="NAME">`
in `document.head`; DOM order is activation order, so later skins cascade over
earlier ones.

### Multi-skin composition

Several skins can be active at once **provided their category bitmasks are
disjoint** (their AND is zero). This lets a *complete theme* (colours +
geometry + depth + …) compose with a *focused skin* that touches one orthogonal
dimension (e.g. `glass`, which is Atmosphere-only). Activating a skin whose
categories overlap an already-active skin returns a `*SkinConflictError`
instead of silently double-setting a token.

There are nine categories (`SkinCategory`, a `uint64` bitmask):

| Category | What it owns |
|---|---|
| `CategoryIdentity` | colours, surfaces, text, primary/secondary, borders, button colours, shadow *colour* |
| `CategoryGeometry` | corner-radius scale, border width/style |
| `CategoryDepth` | shadow *shapes* (offset/blur/spread); colour comes from Identity |
| `CategoryMotion` | transition durations/easing, hover-lift, active-scale |
| `CategoryInteraction` | focus-ring (chromatic feedback) |
| `CategoryTypography` | font family/size/weight *(reserved — no built-ins yet)* |
| `CategorySpacing` | padding / gap density |
| `CategoryLighting` | gradients, glows, gradient-shadow |
| `CategoryAtmosphere` | glass opacity, surface blur/saturate |

The split is principled: a token whose **value is a colour** belongs to a
chromatic category (Identity / Lighting / Interaction); a token whose value is a
**metric** (px, ms, ratio, transform) belongs to a structural/temporal one
(Geometry / Spacing / Depth / Motion). Shadows are split deliberately — the
*shape* is Depth, the *colour* rgba is Identity (`--wings-shadow-color-*`), and
Depth skins compose the final `--wings-shadow-*` that widgets read.

Convenience masks bundle the bits a typical skin of each kind declares:
`IdentitySkinCategories` (Identity|Lighting|Interaction), `GeometrySkinCategories`
(Geometry|Spacing), `DepthSkinCategories`, `MotionSkinCategories`.

### Built-in skins (18)

| Kind (mask) | Skins | Mutually exclusive? |
|---|---|---|
| Identity (theme) | `light` `dark` `autumn` `darkblueberry` `darkforest` `lightblueberry` `mushroom` `vividforest` | yes — pick one |
| Geometry | `sharp` `classic` `soft` | yes — pick one |
| Depth | `flat` `lifted` `floating` | yes — pick one |
| Motion | `gentle` `calm` `brisk` | yes — pick one |
| Atmosphere | `glass` | composes with anything |

A complete look is one from each column, e.g. the live-demo default is
`light + classic + lifted + calm` (optionally `+ glass`). Import each skin you
intend to use (blank import) so its `init()` registers it.

### Registering your own skin

```go
//go:embed skin.css
var css string

func init() {
    // Declare exactly the dimensions your CSS defines, so composition and
    // conflict detection work. A chromatic theme uses the convenience mask:
    wings.RegisterSkin("midnight", wings.IdentitySkinCategories, css)
    // A focused skin declares a single bit:
    // wings.RegisterSkin("rounded", wings.GeometrySkinCategories, css)
}
```

The token contract every skin fills lives in `skins/tokens.md`, organised one
section per category. Define the tokens your declared categories own; widgets
supply literal fallbacks for everything else.

### Private categories (your own design dimensions)

`SkinCategory` is a `uint64`, and its bit space is partitioned like IANA
address space. The low bits are framework-owned built-ins (9 today) and grow
upward; the **high 16 bits (48–63) are permanently reserved for you**. A
built-in will never be assigned a bit in that range, so a private category you
mint today keeps working across WINGS upgrades. Mint one with `UserCategory`,
give it a display name (optional, shown by `<skin-switcher>`), then use it like
any built-in category — it participates in the same disjoint-mask conflict
detection:

```go
// A private "Brand" dimension, orthogonal to every built-in category.
var CategoryBrand wings.SkinCategory

func init() {
    b, err := wings.UserCategory(0) // n in [0,16)
    if err != nil {
        log.Fatal(err) // your call — the library returns the error, never panics
    }
    CategoryBrand = b
    _ = wings.RegisterCategoryName(CategoryBrand, "Brand")
    wings.RegisterSkin("acme", CategoryBrand, acmeCSS)
}
```

An `acme` skin declared this way composes freely with `light + classic + …`
and only conflicts with another skin that also claims `CategoryBrand`.

### Skin API

| Function | Purpose |
|---|---|
| `RegisterSkin(name, categories, css)` | Register a skin (call from `init()`). |
| `ApplySkin(name) error` | Activate. Idempotent; returns `*SkinNotRegisteredError` or `*SkinConflictError`. |
| `DeactivateSkin(name) error` | Remove one active skin. |
| `ClearSkins()` | Deactivate all. |
| `ActiveSkins() []string` / `ActiveSkin() string` | All active (activation order) / most-recent. |
| `ActiveCategories() SkinCategory` | OR of every active skin's mask. |
| `ConflictsWith(categories) []string` | Active skins that would block `categories`. |
| `ListSkins() []string` / `ListSkinInfos() []SkinInfo` | Registered names / names+categories. |
| `SkinCategoriesOf(name) (SkinCategory, bool)` | A skin's declared mask. |
| `UserCategory(n) (SkinCategory, error)` | Mint the n-th private category bit (n in [0,16)); reserved from built-ins. |
| `RegisterCategoryName(bit, name) error` | Name a private category so it shows in `Names()`/`String()`/`<skin-switcher>`. |
| `CategoryUserMask` | Mask of all 16 private bits — `c & CategoryUserMask != 0` tests for any private category. |
| `OnSkinChange(fn func())` | Hook fired after every apply/deactivate/clear (used by `<skin-switcher>` to stay in sync). |

The `<skin-switcher>` built-in widget exposes all of this as UI — see

| Function | Purpose |
|---|---|
| `RegisterSkin(name, categories, css)` | Register a skin (call from `init()`). |
| `ApplySkin(name) error` | Activate. Idempotent; returns `*SkinNotRegisteredError` or `*SkinConflictError`. |
| `DeactivateSkin(name) error` | Remove one active skin. |
| `ClearSkins()` | Deactivate all. |
| `ActiveSkins() []string` / `ActiveSkin() string` | All active (activation order) / most-recent. |
| `ActiveCategories() SkinCategory` | OR of every active skin's mask. |
| `ConflictsWith(categories) []string` | Active skins that would block `categories`. |
| `ListSkins() []string` / `ListSkinInfos() []SkinInfo` | Registered names / names+categories. |
| `SkinCategoriesOf(name) (SkinCategory, bool)` | A skin's declared mask. |
| `OnSkinChange(fn func())` | Hook fired after every apply/deactivate/clear (used by `<skin-switcher>` to stay in sync). |

The `<skin-switcher>` built-in widget exposes all of this as UI — see
[Built-in Widgets](#built-in-widgets) below.

## Built-in Widgets

### wings/widget/combobox — Multi-select Combobox

`import _ "github.com/luisfurquim/wings/widget/combobox"`

A multi-select combobox with type-ahead filtering, tag display, and
keyboard support.

```html
<w-combobox
    options='["Alpha","Beta","Gamma"]'
    placeholder="Type to filter..."
    mode="multi"
    @notinlist="on_notinlist"
    @change="on_change">
</w-combobox>
```

**Attributes:**

| Attribute | Description |
|-----------|-------------|
| `options` | JSON array of strings or `[{"label":"...","value":"..."},...]` objects |
| `placeholder` | Input placeholder text (default: "Type to filter...") |
| `mode` | `"multi"` (default — tag-based multi-select) or `"single"` (replaces previous selection, hides tag display, shows label in the input) |
| `value` | Pre-selected value. In `single` mode this attribute is **authoritative**: if it disagrees with the current selection (e.g. the parent reverts it after the user cancels a confirmation dialog), the visible selection re-syncs to match. The re-sync is silent — `@change` does NOT fire — so a controlled parent can safely roll back without re-entering its own change handler. In `multi` mode `value` only seeds the initial selection. |

**Events (via `@`):**

| Event | Args | Description |
|-------|------|-------------|
| `@notinlist` | typed string | Enter pressed with text not matching any option |
| `@change` | `[]any` of selected items | Selection changed by user action. Programmatic re-sync via `value` does not fire this. |

**Theming.** The combobox reads its colours and metrics from the shared
[`--wings-*` skin tokens](#skins--theming-with---wings--tokens) — `--wings-input-*`
(field), `--wings-list-box-*` / `--wings-list-item-*` (dropdown), `--wings-tiny-element-*`
(tags), `--wings-remover-*` (tag remove button), plus `--wings-radius-*`,
`--wings-shadow-md` and `--wings-transition-fast`. Apply a skin and the combobox
follows; no per-widget theming needed. It also opts into the `glass` skin via
`--wings-surface-blur`/`--wings-surface-saturate` on the dropdown panel.

The "No results" label is rendered via CSS `content: attr(data-i18n)`, so it is
translatable through the normal i18n pipeline.

**Per-instance overrides.** The widget still implements `Customizable` with
`"Vars"` and `"Design"` parts, so a single instance can override CSS without a
skin via `cb.ReplaceCSS("Vars", …)` (see [Customizable Widgets](#customizable-widgets)).

### wings/widget/tabs — Tabbed Container (`w-tabs` / `w-tabbutton` / `w-tab`)

```go
import (
    _ "github.com/luisfurquim/wings/widget/tabs"
    _ "github.com/luisfurquim/wings/widget/tabbutton"
    _ "github.com/luisfurquim/wings/widget/tab"
)
```

`w-tabs` is a **controlled** container: the single source of truth for which
panel is visible is the host's `active` attribute (a `w-tab` `tid`, or a
positional index). Panels are `w-tab` children; the `w-tabbutton` buttons are
*optional sugar*.

**Shape 1 — with buttons (simple tabs).** Co-locate each button with its panel
as direct children (`button, panel, button, panel`). `w-tabs` routes the buttons
into a strip and a click sets `active` for you:

```html
<w-tabs mode="panel">
  <w-tabbutton active>Overview</w-tabbutton>
  <w-tab><h2>Overview</h2>…</w-tab>
  <w-tabbutton>Details</w-tabbutton>
  <w-tab>…</w-tab>
</w-tabs>
```

**Shape 2 — headless (controlled).** Provide *no* `w-tabbutton`. `w-tabs` adds no
chrome and renders its content transparently (passthrough), so your own layout
reaches the panels; drive selection by binding `active`:

```html
<w-tabs mode="detached" active="{{current}}">
  <header>…your own buttons set `current`…</header>
  <w-tab tid="a">…</w-tab>
  <w-tab tid="b">…</w-tab>
</w-tabs>
```

**Modes** (`mode` attribute on `w-tabs`):

| Mode | Layout |
|---|---|
| `panel` (default) | button strip on top (scrolls on overflow), panel below |
| `detached` | same shape, chip-like free-floating buttons, transparent panels |
| `menu` | button column on the left, panel on the right |
| `accordion` | each button is **moved** into its panel to become the native `<summary>`; a `w-tab` marked `active` starts open, then open/close is the platform's job |

**Attributes.** On `w-tabs`: `mode`, `active`. On `w-tabbutton` / `w-tab`:
`tid` (optional identifier for deep-link/programmatic selection — pairing is by
DOM adjacency, so it's not required), `active` (boolean; you may set it on the
initial markup to choose the starting tab), and `mode` (managed — stamped by
`w-tabs`; don't write it yourself).

**Event to parent.** `@change` fires after a user-driven (click/keyboard)
activation; `args[0]` is the selected `tid` (or positional index as a string).
It does **not** fire at init or for programmatic `active` changes.

> Panels keep their DOM across switches (hidden via CSS, not destroyed), so
> input values, scroll position and embedded widgets survive. `w-tabbutton` is a
> deliberately stateless, CSS-only leaf — that's what makes the accordion "move"
> safe. For one-button-to-one-content disclosure, native `<details>` is often
> simpler than `accordion`.

### wings/widget/navbar — Record Navigation Toolbar (`w-navbar`)

```go
import _ "github.com/luisfurquim/wings/widget/navbar"
```

A record-navigation toolbar — first / prev-many / prev / position input / next /
next-many / last — that forwards actions to the parent as triggers. It keeps no
internal state: current position and total are owned by the parent through bound
fields.

```html
<w-navbar
    nav_input="{{cur_record}}"
    total_count="{{record_count}}"
    @first="goFirst"     @prevmany="goPrevPage"
    @prev="goPrev"       @next="goNext"
    @nextmany="goNextPage" @last="goLast"
    @change="onSeek">
</w-navbar>
```

The position input is two-way bound, so typing updates `nav_input`; `@change`
then fires with the new value (`args[0]`), and Enter fires it immediately
without waiting for blur. `w-navbar` implements `Customizable` (`"Vars"` /
`"Design"`).

### wings/widget/skinswitcher — Skin Picker (`<skin-switcher>`)

```go
import _ "github.com/luisfurquim/wings/widget/skinswitcher"
```

A drop-in UI for the [skin system](#skins--theming-with---wings--tokens): one
checkbox per registered skin (so the user can stack a focused skin like `glass`
on top of a complete theme). It detects conflicts via the category bitmasks and,
on selecting a conflicting skin, auto-replaces the colliding active skin rather
than blocking. It registers an `OnSkinChange` hook so its UI stays in sync with
programmatic `ApplySkin` / `DeactivateSkin` calls. Just drop `<skin-switcher>`
into your markup.

## Internationalization (i18n)

WINGS ships with an end-to-end i18n pipeline: a build-time extractor that
rewrites HTML templates with stable numeric indices, a per-language catalog
loaded at runtime by a zero-config package, a GUI editor for translators,
and (optional) morphological dictionaries used to pre-fill plural/gender
flexions from a Unitex DELAF source.

### Pipeline Overview

```
            ┌────────────────────────────┐
            │ *.html (mod/**)            │  ── templates with natural text
            └────────────┬───────────────┘     (text nodes + translatable
                         │                      attributes like title, alt,
                         │   cmd/gen_i18n       placeholder, aria-label) +
                         ▼                      flex blocks {{@g %c ~w ...}}
            ┌────────────────────────────────────────────────┐
            │ *.i18n.html + i18n.db                          │
            │ i18n/<deflang>.json                            │  ── text catalog
            │ i18n/<deflang>.inflections.json  (if any flex) │  ── gender×CLDR
            └────────────┬───────────────────────────────────┘
                         │
              translator ▼ (helpers/wlate GUI — Texto + Inflexões tabs)
            ┌────────────────────────────────────────────────┐
            │ i18n/<lang>.json + <lang>.inflections.json     │
            └────────────┬───────────────────────────────────┘
                         │   runtime
                         ▼
            ┌────────────────────────────┐
            │ wi18n (WASM, side-effect)  │  ── fetches both JSONs in parallel,
            │ wings.Printer   = lookup  │     installs Printer (text index →
            │ wings.SynPrinter = flex   │     string) and SynPrinter (flex
            └────────────────────────────┘     block → locale-correct form)

   ┌──────────────────────────────────────────────────────────────┐
   │ Optional: cmd/dictbuild self-fetches the Unitex DELAF for a  │
   │ locale (gh:UnitexGramLab/unitex-lingua + auto-built          │
   │ UnitexToolLogger from gh:UnitexGramLab/unitex-core) and      │
   │ emits <lang>.db (gob) used by gen_i18n to pre-fill flexions. │
   │ cmd/dictlookup is a CLI inspector for that .db.              │
   └──────────────────────────────────────────────────────────────┘
```

The runtime side is intentionally tiny: a TextNode whose content is a
decimal number gets replaced by `table[n]` when the catalog is loaded, and
left untouched otherwise — so dynamic text produced via `{{expression}}`
passes through unchanged. The same lookup applies to values of the
attributes listed in `wings.TranslatableAttrs` (default: `title`,
`placeholder`, `alt`, `aria-label`, `data-i18n`).

For plurals and gender agreement — cases where a single translation string
cannot reflect the target locale's grammar — WINGS ships a parallel
pipeline keyed on inline **flex sigils** (`@var`/`%var`/`~word`/`#N`). See
[Flexion — Plurals & Gender (SynPrinter)](#flexion--plurals--gender-synprinter)
below for the full syntax and catalog format.

### wings/wi18n — Runtime Lookup

`import _ "github.com/luisfurquim/wings/wi18n"` — side-effect import only.

On `init()`, `wi18n`:

1. Detects the browser language from `navigator.languages[0]`, falling back
   to `navigator.language`, then `en-US`.
2. Sets `<html lang="…">` accordingly.
3. Registers itself on `wings.InitWG` so `wings.Main()` waits for the
   catalog to load before defining custom elements.
4. Fetches `<BasePath><lang>.json` (with fallback chain: full tag → base
   language → `en-US`) relative to the current page. `BasePath` defaults to
   `i18n/`; apps that ship their own catalog next to the project's catalog
   call `wi18n.SetBasePath("<dir>/")` from an `init()` that runs after
   `wi18n`'s (see the wlate self-i18n setup below for a concrete example).
5. Decodes the JSON as a `[]wi18n.Entry` array (see schema below); builds
   the lookup table from each entry's `content` field.
6. Replaces `wings.Printer` with a function that parses the TextNode
   content (or attribute value, when the attribute is in
   `wings.TranslatableAttrs`) as a decimal index and returns `table[idx]`.
   Entries whose `content` is empty fall back to rendering the raw index —
   a deliberate visual signal for missing translations.

If no catalog can be loaded, `wings.Printer` stays as the default `ByPass`
and TextNodes render their raw numeric indices.

The bundle loaded at init is also stashed in an in-memory cache keyed by
the requested tag, so the first call to `wi18n.SetLang(<initial-lang>)`
later in the session is a cache hit (no re-fetch). See [Runtime Locale
Switching (SetLang)](#runtime-locale-switching-setlang) below.

```go
// Usage: import for side effects, that's it.
import (
    "github.com/luisfurquim/wings"
    _ "github.com/luisfurquim/wings/wi18n"
    _ "myapp/mod/mywidget"
)

func main() { wings.Main() }

// Optional: read the selected language tag
lang := wi18n.Lang()
```

**Catalog schema.** The catalog is split across two parallel files: a
browser-side data file and a server-side metadata file.

`i18n/<lang>.json` — shipped to the browser:

```json
[
  { "content": "Dashboard", "revised": true },
  { "content": "", "revised": false, "source": "llm:gemma4" }
]
```

`i18n/<lang>.meta.json` — server-only, same length, parallel-indexed:

```json
[
  { "context": "mywidget/mywidget.html:7:42",
    "ctxdetail": "a@mywidget/mywidget.html:7:42<br/>h2@mywidget/mywidget.html:65:17" },
  { "context": "mywidget/mywidget.html:9:5",
    "ctxdetail": "p@mywidget/mywidget.html:9:5" }
]
```

- `content` — source string for the default language; translation for every
  other language. Empty means "not translated yet".
- `revised` — translator-maintained flag, flipped in the wlate GUI once a
  human has reviewed the entry. Preserved across `gen_i18n` runs when the
  underlying source string has not changed.
- `source` — optional provenance tag, set automatically when the entry was
  pre-filled (e.g. `"llm:gemma4"`, `"dict:unitex-lingua"`). Displayed as a
  badge in wlate so translators know which entries need human review.
- `context` (meta) — first occurrence as `<path>:<line>:<col>` (forward
  slashes on every OS).
- `ctxdetail` (meta) — every occurrence joined by `<br/>`, each formatted as
  `<tag>@<path>:<line>:<col>`. For attribute values the tag is written as
  `<element>[<attr>]` (e.g., `button[title]`).

### Runtime Locale Switching (SetLang)

Once the page has loaded, applications can switch the active locale on
the fly without reloading the page or rebuilding the DOM. Form input,
list contents, scroll position and component state survive the switch —
only the bindings driven by `Printer` / `SynPrinter` / `FmtPrinter` are
refreshed in place.

```go
import "github.com/luisfurquim/wings/wi18n"

// from a click handler, language picker, etc.
wi18n.SetLang("en-US", func(err error) {
    if err != nil {
        // No catalog available for "en-US" or any of its fallbacks.
        return
    }
    // Locale switch is complete; wings.Locale is now "en-US"
    // and every visible custom element has been re-translated.
})
```

`SetLang(tag, done)` always runs in a goroutine and returns immediately;
this is mandatory because the catalog fetch goes through the JS event
loop, and a synchronous wait inside an event handler would deadlock the
very `fetch().then` callback we are awaiting. The optional `done`
callback fires (also from the goroutine) once the switch completes or
fails. Pass `nil` if you do not need notification.

**How it works:**

- The first call to a given tag fetches `<BasePath><lang>.json` (and
  `<lang>.inflections.json`, if any), parses it, and caches the resulting
  bundle keyed by the requested tag. Subsequent calls to the same tag
  reuse the cache, so toggling between languages does not re-fetch.
- The active text and inflection tables are atomically swapped, then
  `wings.Locale` and `<html lang="…">` are updated.
- Every live custom-element instance is then walked: each text node and
  attribute that the constructor stashed (via `_wi18nSrc` /
  `_wi18nAttr_*` JS expandos on the node) is re-translated, the new
  binding string is re-parsed, and the affected `PranaState` is
  re-synced. Existing `*items:i` clones, conditional state, two-way
  bindings and timer goroutines are preserved.

**Limitation — placeholder *set*, not order.** Translations may freely
**reorder** `{{...}}` placeholders to match the target language's word order.
A mixed text node (words *and* placeholders) is indexed **whole, with its
`{{...}}` inline**, so the catalog string for

```text
pt-BR:  O usuário {{nome}} comprou {{produto}}
```

is translated as a single unit and the placeholders can move anywhere:

```text
ja:     {{produto}} を {{nome}} が購入しました
```

At construction and on every `SetLang`, the runtime sets the node to
`Printer(index)` (the translated string) **and then re-parses it for
bindings**, so any permutation of the *same* placeholders binds correctly —
word-order differences (SVO → SOV, etc.) are fully supported.

What the `SetLang` walker does **not** do is create or destroy `DOMRefNode`s
on the fly: if a translation *adds* a `{{...}}` placeholder that was **absent**
from the source-language string, or *removes* one, that delta may render as
raw text. In short: keep the *set* of placeholders identical across locales;
their *order* is free.

**Performance caches.** Two memoisation layers sit underneath the
runtime so the per-switch cost stays low even for large pages:

- *Intl instance cache* (`wings/wi18n/intl_cache.go`).
  `Intl.NumberFormat` / `Intl.DateTimeFormat` instances are expensive to
  construct (each one parses CLDR locale data behind the syscall/js
  boundary) and essentially free to call. They are cached keyed by
  `(locale, options)` and reused across `SetLang` calls — so toggling
  back to a previous locale always hits warm formatters.
- *Parsed-template cache* (`wings/parse_cache.go`). The re-bind walker
  memoises `expr.ParseText` by its input string. Many instances of the
  same custom element produce identical translated strings, so the parse
  cost is paid once per unique `(locale, source string)` pair instead of
  once per node.

Neither cache is invalidated on `SetLang`: both are keyed on values
that stay stable for the lifetime of the catalog.

**Worked example.** The `live-demo/` directory at the repository root
ships a single tabbed application that exercises every feature
described in this README — including a locale switcher in the header
that demonstrates `SetLang` simultaneously refreshing the basics, flex,
fmt and i18n tabs across pt-BR / en-US / es-AR.

### cmd/gen_i18n — Build-time Extractor

```
go run github.com/luisfurquim/wings/cmd/gen_i18n \
    --path ./mod \
    --deflang pt-BR
```

What it does:

- Walks the directory tree, processes every `*.html` (skips files already
  ending in `.i18n.html`).
- For each HTML file, parses the DOM, extracts the natural text from every
  `TextNode` and the value of each element attribute whose name is in the
  translatable attribute set (see flags below), inserts the string into a shared
  trie keyed by an octal hash of the text, and replaces the node content
  (or attribute value) with the trie's decimal index.
- **Leaves runtime placeholders verbatim.** A text node (or attribute value)
  that is *only* `{{…}}` bindings plus whitespace — e.g. `{{count}}`,
  `{{%price}}`, `{{%dist:km}}` — carries no words to translate, so it is left
  untouched and never assigned a catalog index. This keeps the catalog clean
  and prevents non-deflang locales from rendering a bare index where the
  binding should be. (Flex blocks such as `{{@g %c …}}` are the deliberate
  exception: they *are* extracted — see Flex-block extraction below.)
- Skips any element (and its subtree) marked `translate="no"` — see
  [Opting out](#opting-out-translateno) below.
- Writes `<file>.i18n.html` next to each source template. Embed these at
  compile time via `//go:embed` instead of the original HTML. (Note: the
  parser emits a wrapping `<html><head></head><body>…</body></html>` — this is
  harmless; the framework sets the template via `<template>.innerHTML`, which
  dissolves those wrappers and keeps the fragment.)
- Persists the trie to `i18n.db` (gob + 64-bit epoch version header) at
  the root of `--path`, so the next run reuses indices for unchanged
  strings.
- Writes two parallel files per language: `i18n/<lang>.json` (browser
  bundle — `content`, `revised`, optional `source`) and
  `i18n/<lang>.meta.json` (server-only — `context`, `ctxdetail`). The
  deflang `.json` has `content` set to the source string for every entry;
  other `<lang>.json` files are remapped in place across runs: surviving
  translations and `revised` flags are carried over; disappeared strings
  are reset to empty.
- Validates `--deflang` as a BCP 47 tag via `golang.org/x/text/language`
  (falls back to `en-US` on invalid input).
- If legacy `<lang>.csv` files exist without a corresponding `<lang>.json`,
  they are converted once (the legacy `!` marker, if present, is stripped).
  This is a one-shot migration; CSV is no longer the on-disk format.

**Translatable-attribute flags.** Attributes like `title`, `placeholder`,
`alt`, `aria-label`, and `data-i18n` carry user-visible text just like text
nodes do. `gen_i18n` extracts the values of the following attributes by
default:

```
title, placeholder, alt, aria-label, data-i18n
```

**CSS `content:` via `data-i18n`.** CSS pseudo-elements (`::before` /
`::after`) can render translatable text without any CSS-level i18n
machinery. Declare the CSS once:

```css
.breadcrumb::before { content: attr(data-i18n); }
```

Then put the source text on the element as `data-i18n`:

```html
<span class="breadcrumb" data-i18n="Início"></span>
```

`gen_i18n` assigns a catalog index to the attribute value exactly as it
does for text nodes; the runtime `Printer` translates it at construction
time and again on every `SetLang()` call; the browser picks it up through
the standard CSS `attr()` function. No CSS parsing, no custom properties,
no extra runtime code.

Three flags tune this set:

| Flag | Effect |
|---|---|
| `--attrs <list>` | Replaces the default list entirely. Comma-separated, case-insensitive. |
| `--add-attrs <list>` | Appends to the active list (default, or whatever `--attrs` produced). |
| `--no-attrs <list>` | Removes from the active list after additions, so it can also drop defaults. |

In `ctxdetail`, attribute occurrences are distinguished from text-node
occurrences by the tag format: `button[title]@path:line:col` vs.
`button@path:line:col`.

#### Opting out (`translate="no"`)

`gen_i18n` honors the standard HTML
[`translate`](https://developer.mozilla.org/docs/Web/HTML/Global_attributes/translate)
attribute. An element marked `translate="no"` — and everything inside it — is
excluded from extraction (both text nodes and translatable attributes); a
nested `translate="yes"` re-enables it. The attribute is **build-time only**:
the wi18n runtime ignores it and still substitutes any numeric index it finds,
so a node can hold a *literal* index under `translate="no"` and still render
live.

Use it for content that must stay verbatim regardless of locale:

```html
<!-- Language autonyms never translate (each shows in its own language) -->
<select translate="no">
  <option value="pt-BR">Português</option>
  <option value="en-US">English</option>
  <option value="es-AR">Español</option>
</select>

<!-- Verbatim prose (e.g. an English API demo) kept out of the catalog -->
<div class="api-notes" translate="no"> … </div>
```

The combination — build-time skip + runtime substitution — also enables a
self-referential demo: a table whose cells contain literal catalog indices
under `translate="no"` won't be re-indexed by `gen_i18n`, yet the runtime
renders each index live (the live-demo's "Tradução" tab does exactly this).

**Runtime mirror.** At runtime, `wings.TranslatableAttrs` controls which
attributes the engine passes through `Printer`. Its default matches the
default `gen_i18n` set, and **must mirror** whatever flags produced the
catalog, otherwise attribute values render as raw decimal indices:

```go
// Option 1: assign a fresh slice (full override)
wings.TranslatableAttrs = []string{"title", "placeholder"}

// Option 2: incremental — safe with other packages that may also tweak it
wings.AddTranslatableAttrs("data-tip", "aria-placeholder")
wings.RemoveTranslatableAttrs("title")
```

The helpers are case-insensitive, trim whitespace, and skip duplicates.
Both the assignment and the helper calls must run before `wings.Main()`
finishes initialization (an `init()` in `package main` is the canonical
spot).

**Flex-block extraction.** In the same pass, `gen_i18n` scans every `{{...}}`
binding for flex sigils (`@`/`%`/`~`/`#N` — see next section). Each distinct
block is assigned a numeric rule index, rewritten to its canonical `{{@g %c
~word #N}}` form in the `.i18n.html`, and emitted as a row in
`i18n/<deflang>.inflections.json` (with a parallel `.inflections.meta.json`
holding `context`/`ctxdetail`). Translator-maintained `<lang>.inflections.json`
files are remapped in place across runs, same as the text catalog.

**Auto-fill flags.**

| Flag | Effect |
|---|---|
| `--auto-flex` | Consult per-language dictionaries (`.db` files built by `cmd/dictbuild`) to auto-fill empty inflection cells. Output is tagged `source: "dict:unitex-lingua"` and flagged for human review. |
| `--dict-dir <dir>` | Directory holding `<lang>.db` files. Default: `cmd/gen_i18n/dicts` under the WINGS module. |
| `--auto-translate` | Use the LLM/MT backend configured in `gen_i18n.json` to pre-fill text and flex entries that the dictionary pass could not fill. All LLM output is tagged with the model name and flagged `revised: false` for human review. |

**Degenerate-deflang lint.** When the deflang has no gender axis (e.g. `en-US`)
but at least one target locale does (e.g. `pt-BR`), any flex block missing
`@var` would collapse every row into a single gender column — leaving the
gendered locale's translator with no way to supply masculine/feminine forms.
`gen_i18n` emits a `lint:` warning on stderr pointing at the first occurrence
of each such block, so the webdev can add the `@<var>` sigil before shipping.

### Flexion — Plurals & Gender (SynPrinter)

Text catalogs work when one source string maps to exactly one translated
string. They break down for grammars that inflect: English "1 student / 2
students" has two forms, Portuguese "1 aluno aprovado / 2 alunos aprovados
/ 1 aluna aprovada / 2 alunas aprovadas" has four, and Arabic has six CLDR
plural categories per gender. WINGS's flex pipeline handles this by making
the grammar-shaping variables visible to the runtime via inline **sigils**.

**The sigils *are* selectors, not just morphology.** `@var` selects a row by
gender — exactly what Fluent expresses as
`{ $gender -> [male] … [female] … *[other] … }` — and `%var` selects the
CLDR plural category for the count. The per-cell table (`m.one`, `f.other`,
…) is the variant set those selectors choose from, so contextual selection
by gender and number is a first-class feature, not a side effect. What WINGS
adds on top is optional **morphological auto-fill**: `gen_i18n --auto-flex`
pre-populates those cells from Unitex DELAF dictionaries (see
[cmd/dictbuild](#cmddictbuild--cmddictlookup--flexion-dictionaries)), so a
translator reviews suggestions instead of hand-writing every gender×number
form.

**Template syntax.** Inside a `{{...}}` binding, four sigils signal a flex
block (as opposed to a plain reference):

| Sigil | Role | Example |
|---|---|---|
| `@var` | Gender axis — value is the row key (e.g. `"m"`, `"f"`). Not emitted into the rendered string. | `{{@genero ...}}` |
| `%var` | Count axis — value is the integer count. Emitted at its position in the rule (`{n}` placeholder inside cells). | `{{%qt ...}}` |
| `~word` | Flex marker — a lemma that will be inflected by the translator. Consumed at build time only (gen_i18n uses it to suggest default forms from `<lang>.db`). | `{{... ~aluno ~aprovado}}` |
| `#N` | Rule index — injected by `gen_i18n` during rewriting; the webdev does **not** write this by hand. | `{{@genero %qt ~aluno #42}}` |

**Path-based variables.** Both `@var` and `%var` accept full reference
paths — useful when the axis lives inside a struct or array element:

```html
<!-- Gender from a user record, count from the current cart line -->
<p>{{ @user.gender %cart[idx].qty ~aluno aprovado }}</p>
```

The resolver falls back to the cheap single-level lookup for bare names
(`@genero`, `%qt`) and routes path-bearing sigils through `wings.Solve`
against the live data context.

**Order matters inside the block.** `%var` emits the count value where it
appears in the rule, so placement controls the output. For "os 10 alunos"
write `~o %qt ~aluno` — placing `%qt` between the article and the noun. A
verb that agrees with number must carry its own `~` (e.g. `~ganhou`);
omitting it leaves a singular verb glued to a plural subject.

**Catalog schema.** Like the text catalog, the inflections catalog is split
across two parallel files.

`i18n/<lang>.inflections.json` — shipped to the browser:

```json
[
  {
    "label":   "o {n} aluno aprovado ganhou uma bolsa",
    "revised": false,
    "cells": {
      "m.one":   "o {n} aluno aprovado ganhou uma bolsa",
      "m.other": "os {n} alunos aprovados ganharam uma bolsa",
      "f.one":   "a {n} aluna aprovada ganhou uma bolsa",
      "f.other": "as {n} alunas aprovadas ganharam uma bolsa"
    },
    "sources": {
      "m.one": "dict:unitex-lingua",
      "f.one": "llm:gemma4"
    }
  }
]
```

`i18n/<lang>.inflections.meta.json` — server-only, parallel-indexed:

```json
[
  {
    "context":   "pages/result.html:12:8",
    "ctxdetail": "caption@pages/result.html:12:8"
  }
]
```

- `label` — translator-facing stem (the non-sigil tokens from the original
  block), used as a visible placeholder when no matching cell is found.
- `cells` — map keyed by `<gender>.<cldr-category>`. CLDR categories vary
  per locale (`zero`, `one`, `two`, `few`, `many`, `other`). Locales without
  a gender axis use a single empty-string gender key (`.one`, `.other`).
  `{n}` inside a cell is replaced by the numeric count at render time.
- `revised` — same semantics as the text catalog flag.
- `sources` — optional per-cell provenance map (same tag format as `source`
  in the text catalog). Omitted when empty.
- `context` / `ctxdetail` (meta) — same format as the text `.meta.json`.

**Runtime behavior.** When `wi18n` is imported, it fetches
`<lang>.inflections.json` in parallel with the main text catalog and
installs `wings.SynPrinter` — a second printer hook invoked by the syncer
on every flex block. `SynPrinter` resolves the gender and count variables
from the live data context, computes the CLDR plural category for the
current locale, and looks up `cells["<gender>.<cat>"]`.

The fallback chain handles sparse catalogs:

1. Explicit `<gender>.zero` wins when count is exactly 0 (useful in pt-BR
   where CLDR folds 0 into `one`).
2. Empty `zero` cell → try `<gender>.one`.
3. Any other empty cell → try `<gender>.other`.
4. Still empty → render the rule `Label` (the translator-facing stem) as a
   visible placeholder, rather than blank.

Without `wi18n` loaded, the default `wings.NoFlexSynPrinter` renders the
rule index as `#N` — missing inflection support stays obvious on the page
instead of silently dropping content.

### Programmable Flex — Custom Inflection Engines (CustomFlex)

The built-in `@`/`%` selectors plus dictionary auto-fill cover gender and number
out of the box, zero-config. When an app needs inflection the catalog can't
express — a remote morphology service, a domain-specific declension, a language
the dictionaries don't cover — it can supply its **own** engine. The principle:
WINGS shapes flexion at build time and integrates it at runtime; *implementing*
the inflection becomes the app's responsibility. With no engine available the raw
lemma is emitted (visible, not a silent drop) — by design.

**Additional sigils.** Three sigils extend the block grammar for custom engines:

| Sigil | Role |
|---|---|
| `*var` | A **CustomFlex engine** — a candidate to inflect the block's lemmas, elected by `Priority()`. May also act as a selector. |
| `$var` | A **dynamic value emitted verbatim** (not inflected) — e.g. a proper name interleaved in the sentence. Also exposed to the engine as a selector. |
| `~$var` | A **dynamic value to be inflected** — a `~word` whose text arrives at runtime. |

`~` means "inflect this", `$` means "dynamic value", `*` means "engine" — they're
orthogonal (`~$var` = `~` + `$var`). The engine is always `*` (a `$` value is pure
data and can't be a motor). Blocks must not be nested.

**The contract** (core `wings` package):

```go
type CustomFlex interface {
    // Flex returns the inflected form of word given the context. Selector
    // order is not guaranteed — identify by Name/Sigil, never by position.
    Flex(word string, selectors ...FlexSelector) (string, error)
    String() string // text this token emits at its position ("" = selector only)
}
type Prioritized interface { Priority() uint } // optional; absent ⇒ 0
type FlexSelector struct { Sigil byte; Name string; Value any }
```

**Engine election (one per block).** Candidates are the `*var` / `~$var` values
that implement `CustomFlex`; the highest `Priority()` wins and the others become
selectors passed to it. `@gender` / `%count` are selectors, never candidates. The
built-in catalog is the implicit engine, used only when there is no custom
candidate — so the zero-config path is unchanged. A tie between user candidates is
an error: it logs and falls back to emitting the raw lemmas.

**Build-time split.** `gen_i18n` keys the catalog format on the sigils it sees. A
block with only `@`/`%`/`~word` keeps the per-cell phrase catalog above. A block
carrying `*`/`~$` is split into locale-invariant **control** metadata (the
`@gender` / `%count` bindings, the `*engine`, the `#N` index) and a per-locale
**content** template (literal text + `$var`/`~$var`/`~word` + the count position)
that translators reorder freely — whitespace is preserved token-by-token, so a
space-free script like Japanese composes correctly.

**Async engines (the critical pattern).** `Flex` is synchronous and runs inside
the render epoch — it must not block on `fetch` (the same deadlock class as
`SetLang`). For a remote source: return the **cached** form; on a miss return a
**placeholder** (the raw value) and start the request in a goroutine; when it
resolves, populate the cache and re-trigger the render (`obj.This.Set`). The live
demo's `RemoteFlexer` demonstrates this end to end.

**Failure modes** — all visible, never silent: a tie between user engines, a
`~$var` left to the catalog-default engine (which has no runtime form for it), or
a `Flex` that returns an error each emit the raw value and log. `gen_i18n` emits a
soft `lint:` note when it sees `~$` without a `*` in a block.

#### By example — built-in vs custom

The live demo runs both kinds of block side by side. Comparing them shows where
each one keeps its inflection knowledge.

**Built-in (catalog-backed).** Gender and number, looked up from a table:

```html
<p>{{ @gender %qt #0 }}</p>
```

```go
// The component supplies only the *values* the selectors read:
func (w *Demo) InitData() map[string]any {
    return map[string]any{"gender": "f", "qt": 2}
}
```

There is **no inflection code**. A translator fills the per-cell table
(`m.one`, `m.other`, `f.one`, `f.other`, …) in `<lang>.inflections.json` —
optionally pre-filled by `gen_i18n --auto-flex` from dictionaries — and at runtime
WINGS picks the cell by gender + CLDR plural category. The forms exist **at build
time**; runtime is an indexed lookup.

**Custom (engine-backed).** A lemma chosen at runtime, inflected by your code:

```html
<p>{{ %qt *flexer ~$produto }}</p>
```

```go
// A minimal synchronous engine: pluralise Portuguese by appending "s".
type PtPluralizer struct{}

func (PtPluralizer) String() string { return "" } // contributes no text of its own

func (PtPluralizer) Flex(word string, sel ...wings.FlexSelector) (string, error) {
    n := 1
    for _, s := range sel {
        if s.Sigil == '%' { // the count axis, arriving from %qt
            if v, ok := s.Value.(int); ok {
                n = v
            }
        }
    }
    if n == 1 {
        return word, nil
    }
    return word + "s", nil // toy rule — a real engine would handle irregulars
}

// Optionally implement Priority() uint to outrank other engines / the catalog.

func (w *Demo) InitData() map[string]any {
    return map[string]any{
        "qt":      1,
        "produto": "maçã",         // dynamic: e.g. bound to a <select>
        "flexer":  PtPluralizer{},  // the *flexer engine
    }
}
```

Here `~$produto` is a lemma whose **text is dynamic** (the user picks it from a
dropdown), so it cannot live in a build-time catalog; `*flexer` inflects it at
runtime, and `%qt` is handed to the engine as a selector.

| | Built-in (`@`/`%` + `~word`) | Custom (`*engine` + `~$var`) |
|---|---|---|
| Who provides the forms | translator (catalog cells) + optional dict auto-fill | app developer (engine code) |
| When | build time → indexed lookup at runtime | runtime |
| Best for | fixed phrases, known lemmas, gender/number | dynamic lemmas, remote services, rules a table can't hold |
| When absent | renders the `Label` stem | renders the raw value |

The engine above is synchronous. For a backend-driven engine — where `Flex`
cannot block on the network — see the demo's
[`RemoteFlexer`](live-demo/mod/tabs/flex/remoteflexer.go): it returns the raw word
as a placeholder on a cache miss, fetches in a goroutine, and re-runs the render
(`obj.This.Set`) once the form arrives.

### Locale-Aware Formatting (FmtPrinter)

Text catalogs translate strings and flex blocks resolve plurals/gender. A
third axis — **values** — needs locale-aware treatment too: numbers use
different decimal and grouping separators across locales, currencies have
their own symbols and fractional digits, dates span a zoo of formats. The
`FmtPrinter` pipeline handles all of these through the same sigil already
used for the count axis of flex blocks: `%var`.

**Template syntax.** When a `{{...}}` binding contains exactly one `%var`
(optionally followed by a path tail), it is a **format block** — the
value is resolved from the data context and handed to `wings.FmtPrinter`,
which picks the rendering based on the Go type of the value:

```html
<!-- Plain numeric value (locale separators) -->
<p>Total: {{%count}} unidades</p>

<!-- Nested path -->
<p>Saldo: {{%user.balance}}</p>

<!-- Array element — identical semantics to a plain reference path -->
<td>{{%invoices[idx].total}}</td>
```

A `%var` that shares a `{{...}}` with any of `@var`, `~word`, `#N`, or
other literal tokens is interpreted as a flex block count axis instead
(see the previous section) — the lone-`%var` rule is the only ambiguity
cleared at parse time, and it falls on the common case.

**Type-directed rendering.** `wi18n`'s `FmtPrinter` dispatches on the value's
Go type in this order:

| Type | Output |
|---|---|
| `nil` | empty string |
| `int`, `int64`, `uint`, `uint64`, etc. | `Intl.NumberFormat` integer (grouping per locale) |
| `float32`, `float64` | `Intl.NumberFormat` default precision |
| `time.Time` | `Intl.DateTimeFormat` (epoch ms bridge) |
| `js.Value` holding a JS `Date` | `Intl.DateTimeFormat` direct |
| anything implementing [`wi18n.Numerical`](#locale-aware-formatting-fmtprinter) | `v.Format(locale, formatName)` → `(string, error)`; error stops rendering |
| anything else | `fmt.Sprint` fallback |

`wi18n` uses the browser's `Intl` API rather than bundling locale tables
(ICU/CLDR) inside the WASM binary — keeps the artifact small and honors
the browser's tz database for dates.

**Numerical interface — customization without registries.** Any Go type
that implements the following interface is treated as a first-class
formattable value:

```go
type Numerical interface {
    Format(locale, formatName string) (string, error)
}
```

There is no registration step; satisfying the interface is the
registration. `formatName` comes from the `%var:formatName` template
syntax (empty string when the template uses bare `%var`).

Returning a non-nil error stops rendering for that binding: `FmtPrinter`
discards the returned string, logs the error with locale/formatName
context, and emits an empty string. Returning `("", nil)` produces an
empty string without triggering the error path. The implementation may
log domain-level detail before returning the error; `FmtPrinter` adds
the locale/formatName context on top.

**wi18n.Currency — built-in example of `Numerical`.**

```go
type Currency struct {
    Amount int64  // smallest unit (centavos, cents, yen-units)
    Code   string // ISO 4217 (BRL, USD, JPY, BHD)
}
```

`Currency` implements `Numerical`. The amount is stored as a signed `int64`
in the currency's minor unit — no float rounding surprises on financial
data — and the ISO code travels with the value so multi-currency templates
work by iterating a `[]Currency` naturally:

```go
// mod/invoice/invoice.go
data := map[string]any{
    "lines": []wi18n.Currency{
        {Amount: 123450, Code: "BRL"}, // R$ 1.234,50 in pt-BR
        {Amount: 9999,   Code: "USD"}, // $99.99 in en-US
    },
}
```

```html
<!-- mod/invoice/invoice.html -->
<tr *lines:i><td>{{%lines[i]}}</td></tr>
```

Applications with a single fixed currency typically wrap `Currency` in a
helper:

```go
func BRL(n int64) wi18n.Currency { return wi18n.Currency{Amount: n, Code: "BRL"} }
```

Or define their own domain type implementing `Numerical` and delegating to
`Currency` internally — that is exactly the pattern the built-in physical
measure packages follow (see [Physical Measure Packages](#physical-measure-packages)).

An ISO 4217 table (embedded in `wi18n/currency_iso4217.go`) decides the
number of fractional digits — most currencies use 2, with documented
exceptions for zero-decimal (JPY, KRW, VND, …), three-decimal (BHD, KWD,
…), and four-decimal (CLF, UYW) cases.

**Fallback behaviour.** Without `wi18n` imported, `wings.FmtPrinter`
stays as the default `NoFmtFmtPrinter`, which renders values via
`fmt.Sprint` — locale-incorrect but never blank. When `wi18n` is loaded
but the browser's `Intl` rejects a locale/currency combination (or the
environment has no `Intl` at all), each formatter falls back to a
locale-agnostic rendering (`strconv`, plain decimal point, RFC 3339 for
dates) so the page stays readable.

**Named formats (`%var:formatName`).** The `:formatName` suffix is
interpreted differently depending on the value type:

- **`Numerical` types** (measure packages, `Currency`, custom types):
  `formatName` is passed as the second argument to `Format`; the
  implementation decides what to do with it — measure packages treat it as
  a unit override (`{{%dist:km}}`), `Currency` ignores it.
- **Native scalars** (`int`, `float64`, etc.) and **`time.Time` / JS
  `Date`**: `fmtPrinter` looks up `formatName` in the current locale's
  `<lang>.fmt.json` `"named"` section and, if found, renders through the
  corresponding `Intl.NumberFormat` or `Intl.DateTimeFormat`
  (`{{%count:compact}}`). If the name is not configured, rendering falls
  back to the default locale-aware behavior and `formatName` is silently
  ignored.

The suffix is optional: bare `{{%dist}}` passes an empty string.

**Per-locale `<lang>.fmt.json`.** A file placed next to the text catalog
(e.g. `i18n/en-US.fmt.json`) carries two optional entry shapes loaded
automatically by `wi18n` when the locale is switched:

```json
{
  "km":      { "decimals": 2 },
  "mi":      { "decimals": 3 },
  "compact": { "type": "number", "notation": "compact" },
  "short":   { "type": "date",   "dateStyle": "short"  }
}
```

Entries **with** a `"type"` field are **named scalar formats**: every
key except `"type"` is forwarded verbatim as an Intl option object.
`"type": "number"` builds an `Intl.NumberFormat`; `"type": "date"` builds
an `Intl.DateTimeFormat`. The resulting formatter is cached per
(locale, name) and reused across renders.

Entries **without** `"type"` are **unit-precision overrides** (`"decimals"`
key): `wi18n.UnitDecimals(locale, unit)` returns the value, and measure
packages consult it before their built-in default. `wi18n.NamedFmt(locale,
name)` exposes named scalar entries to callers that need the raw Intl
options.

**`wi18n.SetConfig` — app-level unit overrides.** Call
`wi18n.SetConfig(jsonBytes)` at startup (e.g. from an embedded
`wings.json`) to set per-locale default units for any quantity:

```json
{ "measures": { "pt-BR": { "length": "km" }, "en-US": { "length": "mi" } } }
```

`wi18n.MeasureDefault(quantity, locale)` returns the configured unit, or
`("", false)` when none is set. Measure packages consult this before
falling back to their built-in locale heuristics.

### Physical Measure Packages

Eight packages under `wi18n/` model common physical quantities. Each
stores the value in a canonical SI unit and implements `wi18n.Numerical`
(the `//go:build js && wasm` `Format` method is in a separate file so the
pure-Go math is testable on the host):

| Package | Type | Canonical | Unit names |
|---|---|---|---|
| `wi18n/length` | `Length{Meters float64}` | m | `m` `km` `cm` `mm` `mi` `ft` `yd` `in` `nmi` `league` |
| `wi18n/temperature` | `Temperature{Kelvin float64}` | K | `k`/`kelvin` `c`/`celsius` `f`/`fahrenheit` `r`/`rankine` |
| `wi18n/speed` | `Speed{MetersPerSecond float64}` | m/s | `ms` `kmh` `mph` `kn` `fps` |
| `wi18n/volume` | `Volume{Liters float64}` | L | `L` `mL` `dL` `m3` `floz` `pt` `qt` `gal` `galimp` |
| `wi18n/weight` | `Weight{Kilograms float64}` | kg | `kg` `g` `mg` `t` `lb` `oz` `st` |
| `wi18n/area` | `Area{SquareMeters float64}` | m² | `m2` `km2` `cm2` `mm2` `ha` `mi2` `ft2` `yd2` `in2` `ac` |
| `wi18n/fueleconomy` | `FuelEconomy{LitersPer100km float64}` | L/100 km | `l100km` `mpg` `mpgimp` `kml` |
| `wi18n/cooking` | `Volume{Liters float64}` / `Weight{Kilograms float64}` | L / kg | vol: `L` `mL` `cup` `tbsp` `tsp` `floz` · wt: `kg` `g` `lb` `oz` |

**Template usage.** The data key holds a value of the measure type; the
template uses `%var` (locale default) or `%var:unit` (explicit unit):

```go
// mod/trip/trip.go
func (t *Trip) InitData() map[string]any {
    return map[string]any{
        "dist": length.Length{Meters: 42195},
        "temp": temperature.Temperature{Kelvin: 310.15},
    }
}
```

```html
<!-- locale default -->
<td>{{%dist}}</td>

<!-- explicit unit override -->
<td>{{%dist:km}}</td>
<td>{{%dist:mi}}</td>
<td>{{%temp:f}}</td>
```

**Locale defaults.** Each package's `DefaultUnit(locale)` applies common
regional conventions: `length` → `mi` for `en-US`/`en-GB`/`en-LR`/`my`,
`temperature` → `f` for `en-US`/`en-BS`/`en-BZ`/`en-KY`,
`speed` → `mph` for the same imperial group, `volume` → `gal` for `en-US`,
`weight` → `lb` for `en-US`, `fueleconomy` → `mpg` for `en-US` and
`mpgimp` for `en-GB`. All other locales get SI/metric defaults.

**Unit name constraints.** Unit identifiers contain only ASCII letters and
digits — no `/` or `-` — so they are safe as `%var:unit` tokens in
templates (`kmh` not `km/h`, `floz` not `fl-oz`, `ms` not `m/s`).

**`fueleconomy` inverse units.** `mpg`, `mpgimp`, and `kml` are inversely
proportional to `LitersPer100km`. `Format` and `Convert` return a non-nil
error when `LitersPer100km ≤ 0` for these units (avoids +Inf).

### cmd/dictbuild & cmd/dictlookup — Flexion Dictionaries

These two tools convert a Unitex/GramLab DELAF `.dic` (UTF-16 text) into a
compact Go-native lookup structure used to pre-fill plural/gender flexions
during the build.

`dictbuild` has two invocation modes:

- **`dictbuild -lang <tag>`** — fetch mode. Downloads the compiled
  `.bin`/`.inf` for the requested locale from
  [`UnitexGramLab/unitex-lingua`](https://github.com/UnitexGramLab/unitex-lingua),
  shallow-clones [`UnitexGramLab/unitex-core`](https://github.com/UnitexGramLab/unitex-core)
  at a pinned tag (currently `v3.3`), builds `UnitexToolLogger` from source
  with `make UNITEXTOOLLOGGERONLY=yes 64BITS=yes`, expands the compiled
  dictionary back to UTF-16 text via `Uncompress`, and parses the result.
  Persistent state (cloned tools, compiled binary, cached `.bin`/`.inf`)
  lives under `-state-dir` (defaults to `$XDG_CACHE_HOME/wings/dictbuild`).
  Pass `-tool /path/to/UnitexToolLogger` to skip the auto-build if you
  already have the binary. Output: `<tag>.db` in `-out` (defaults to `.`).
  Linux/macOS only — Windows requires a manual MSVC build.
- **`dictbuild [-out <dir>] <input.dic> <tag>`** — legacy mode for users
  who already have a UTF-16 DELAF on hand (e.g. produced by Unitex/GramLab
  desktop or pulled from an internal mirror).

The auto-fetch table covers every language directory in unitex-lingua
that ships at least one usable DELAF `.bin/.inf` pair: `de`, `el`, `en`,
`es`, `fi`, `fr`, `grc`, `it`, `la`, `mg`, `nn`, `no`, `oge`, `pl`,
`pt-BR`, `pt-PT`, `ru`, `sr-Cyrl`, `sr-Latn`, `th`, `zh`. For each
locale the most general/inflected dictionary upstream provides was
chosen; some entries are samples or partial dictionaries (`el`'s
"30percent", `nn`'s "Dela-sample", `grc`'s demo) because that is all
upstream ships. `ar` (separate noun and verb DELAFs) and `ko` (empty
upstream `Dela/`) are intentionally not listed. The dictbuild parser is
PT-centric in its verb-class aliasing (see `aliasHomograph`), so
`<lang>.db` for non-Portuguese locales is structurally correct but may
need language-specific tweaks on the gen_i18n side to interpret verbal
classes.

In both modes the tool applies DELAF-specific filters:

- `+Pr` (proper names) → dropped
- `+PRO` (enclitic pronoun) → dropped
- Imperative forms (code `Y*`) → dropped
- Finite 1st/2nd person verbal forms → dropped (only 3rd person, infinitive,
  gerund, and participle are needed for count/gender agreement in
  templates)

Each entry is decomposed into three axes — `Class` (tense/mood stem),
`Genre` (`m`/`f`/`n`/empty), `Count` (`s`/`p`/empty). The resulting shape:

```go
type Dict struct {
    Lemmas    map[string]*Lemma   // canonical form → grouped inflections
    FormIndex map[string][]FormRef // surface form → refs back into Lemmas
}
type FormRef struct { Lemma, Class, Genre, Count string }
type Lemma   struct {
    Category string                // "N", "V", "A", "ADV", ...
    Forms    map[string]Inflect    // key = Class+Genre+Count, e.g. "ms", "fp"
}
type Inflect struct { DiffPos int; Suffix string } // DiffPos in runes
```

**`dictlookup <file.db> <word>`** is a human-readable inspector. It prints
every `FormIndex` hit for the queried word, resolves each hit to its parent
`Lemma`, and reconstructs the surface form of every kept inflection so the
output needs no post-processing.

```bash
$ dictlookup pt-BR.db passou
loaded pt-BR.db: 128430 lemmas, 984712 form entries

FormIndex["passou"] → 1 reference(s):
  [0] Lemma="passar" Class="J" Genre="" Count="s"
      passar (V)
        W:    passar       (infinitive)
        G:    passando     (gerund)
        K:ms  passado      ...
        ...
```

### helpers/wlate — Translation Editor GUI

`helpers/wlate/` is a wings-built WASM app designed for translators to
review and edit catalogs side-by-side with a reference language.

**Features (implemented):**

- Two-panel layout: reference language (read-only) vs. target language
  (editable), with per-entry "revised" toggle (bordeaux left border = not
  yet reviewed; gray = reviewed).
- Two tabs: **Texto** (plain strings) and **Inflexões** (gender × CLDR
  plural category grid, using CSS Grid with `display: contents` on
  iteration wrappers).
- Keyboard shortcuts for navigation and toggling revised state (all
  rebindable via `wings.json`).
- Filter toggle: show only unrevised entries.
- Unsaved-changes guard (in-app dialog + `beforeunload`).
- On-save file creation: if the target catalog doesn't exist, wlate
  creates it mirroring the reference structure with empty content and
  `revised: false`.
- JSON schemas:

  ```json
  // i18n/<lang>.json — browser bundle (content + provenance)
  [{"content": "...", "revised": false, "source": "llm:gemma4"}]

  // i18n/<lang>.meta.json — server-only, parallel-indexed
  [{"context": "pages/result.html:12:8",
    "ctxdetail": "th@pages/result.html:12:8<br/>button[title]@pages/result.html:40:17"}]

  // i18n/<lang>.inflections.json
  // Sigil order matters: %qt emits the number where it appears, so place
  // it AFTER ~o and BEFORE ~aluno for "os 10 alunos" (not "10 os alunos").
  // Verbs that conjugate with number (~ganhou/ganharam) need the ~ too —
  // missing a ~ on a number-agreeing verb causes "10 alunos ganhou" bugs.
  [{"label": "o {n} aluno aprovado ganhou uma bolsa", "revised": false,
    "cells": {"m.one": "...", "f.other": "..."},
    "sources": {"m.one": "dict:unitex-lingua"}}]

  // i18n/<lang>.inflections.meta.json — server-only, parallel-indexed
  [{"context": "...", "ctxdetail": "caption"}]
  ```

**Collaboration & review come from your VCS.** Translation and inflection
catalogs are plain files in your repo tree, so branch / PR / merge / blame /
review apply to them exactly as to code — no separate TMS to provision or
sync. A translator works on a branch, opens a pull request, and reviewers diff
the JSON like any other change. wlate adds the editing ergonomics on top:
per-entry approval (`revised`), per-cell provenance (translator email when
OAuth2 is configured), and integrated MT/LLM pre-fill (`--auto-translate`). The
design premise is deliberate — a team that versions its code already has the
workflow translations need, and a team that doesn't version its code won't
want a heavier localization pipeline either.

**Self-i18n.** wlate itself is built as a translatable WINGS app — the
editor eats its own dog food. Its templates live under
`helpers/wlate/mod/wlate/`, `build.sh` runs `gen_i18n` against that tree
before compiling the WASM, and the resulting catalogs are published to
`dist/wlate-i18n/` so they don't collide with the `/i18n/*` route used
for the project being translated. On startup, `main.go` calls
`wi18n.SetBasePath("wlate-i18n/")` to point the runtime fetch at that
directory. Source language is pt-BR; en-US ships fully translated, es-CO
ships as a pt-BR copy with `revised=false` for human review.

Build and run:

```bash
cd helpers/wlate
bash build.sh                 # runs gen_i18n, copies catalogs, builds WASM
go run serve.go <project-dir> # starts the mini-server (see below)
```

#### server.conf — mini-server configuration

`serve.go` is both a dev server and a production-capable mini-server. If a
file named `server.conf` sits next to the executable, it is parsed as
`key=value` lines (comments starting with `#` and blank lines allowed;
values may be single- or double-quoted). Without `server.conf`, the
server keeps its original defaults (listens on `:8080`, plain HTTP, no
auth).

**Basic keys:**

| Key | Effect |
|---|---|
| `cert=<name>` | Enables TLS. Loads `<name>.crt` (or `<name>.pem`) from the executable's directory; if the file doesn't contain the private key, also loads `<name>.key`. Fatal on any load error. |
| `listen=<address>` | Address passed verbatim to `net.Listen("tcp", …)`. Fatal on bind error. |
| `root=<path>` | Overrides the positional `<project-dir>` argument. |

**OAuth2 / OIDC keys (all required together when any is set):**

| Key | Effect |
|---|---|
| `oauth2_issuer=<url>` | OIDC issuer URL. Discovery via `/.well-known/openid-configuration`; the provider must expose `userinfo_endpoint`. |
| `oauth2_client_id=<id>` | Client ID registered with the provider. |
| `oauth2_client_secret=<s>` | Client secret. |
| `oauth2_redirect_url=<url>` | Absolute callback URL; typically `https://<host>/oauth2/callback`. |
| `oauth2_allowed=<path>` | Optional. Path to a text file listing one allowed e-mail per line (blank lines and `#` comments ignored). If omitted, any authenticated user is allowed. |

When OAuth2 is configured, every non-`/oauth2/*` request is gated: GET
requests without a valid session cookie redirect to the provider's
authorization endpoint; other methods return `401`. The callback exchanges
the code for an access token, fetches the e-mail from `userinfo`, checks
the allowlist, and issues a `wlate_session` cookie (HttpOnly, SameSite=Lax,
`Secure` when served over TLS, 12 h TTL). Sessions are stored in-memory.
`GET /oauth2/logout` invalidates the session cookie.

**Example `server.conf`:**

```ini
# Bind and TLS
listen=:8443
cert=wlate

# Project root (overrides CLI arg)
root=/var/lib/wlate/mytranslations

# Gated access via Google OIDC
oauth2_issuer=https://accounts.google.com
oauth2_client_id=1234567890-abc.apps.googleusercontent.com
oauth2_client_secret=GOCSPX-…
oauth2_redirect_url=https://wlate.example.com/oauth2/callback
oauth2_allowed=/etc/wlate/allowed.txt
```

```text
# /etc/wlate/allowed.txt — one e-mail per line
translator1@example.com
translator2@example.com
```

OAuth2 support uses only the Go standard library (no `go-oidc` / `x/oauth2`
dependencies).

## Component Lifecycle

1. **Registration** (`init()`): Module calls `wings.Register()` to register the
   custom element tag, template, CSS, factory function, and observed attributes.

2. **Construction** (automatic): When the browser encounters the custom element tag,
   the constructor creates a shadow root, injects CSS, parses the template, calls
   `InitData()` for initial state, and sets up data bindings.

3. **Connection** (automatic): When the element is inserted into the DOM,
   `connectedCallback` fires and sets the ready flag.

4. **Render** (automatic): Once connected, `Render(obj)` is called with the
   `PranaObj` containing:
   - `obj.This` — `*ReactiveData` for reading/writing state
   - `obj.Dom` — `js.Value` of the container SPAN in the shadow root
   - `obj.Element` — `js.Value` of the custom element itself
   - `obj.Trigger` — function to dispatch events to the parent component

5. **Attribute Changes** (automatic): When an observed attribute changes,
   the new value is copied into the data map and a sync is triggered.

6. **Disconnection** (automatic): When the element is removed from the DOM,
   event handlers and bindings are cleaned up.

## Parent-Child Communication

### Passing Data Down (Attributes)

The parent passes data to children via attributes with `{{expression}}` bindings:

```html
<!-- Parent template -->
<my-child
    title="{{page_title}}"
    is_logged="{{is_logged}}"
    is_anonymous="{{is_anonymous}}"
></my-child>
```

Data flows one-way: parent to child. When the parent's data changes, the
child's attributes are updated automatically. To communicate from child
back to parent, use [Triggers](#dispatching-events-up-trigger).

### Dispatching Events Up (Trigger)

Children fire named events that invoke handler functions in the parent.
The `@` attribute in the parent's template maps event names to handler names:

**Parent template (app.html):**
```html
<!-- @login maps event "login" to handler function "on_login" -->
<my-login @login="on_login"></my-login>
```

**Parent code (app.go):**
```go
func (app *App) InitData() map[string]any {
    return map[string]any{
        // Placeholder — obj not available yet
        "on_login": wings.TriggerHandler(nil),
    }
}

func (app *App) Render(obj *wings.PranaObj) {
    // Real handler with obj in scope
    obj.This.Set("on_login", func(args ...any) {
        obj.This.Set("is_logged", true)
    })
}
```

**Child Render (login.go):**
```go
func (lgn *Login) Render(obj *wings.PranaObj) {
    // Trigger uses the event name (without @), not the handler name
    obj.Trigger("login", username)  // matches @login in parent template
}
```

The flow is: `obj.Trigger("login")` -> looks up `@login` attribute on the
child element -> finds `"on_login"` -> resolves `on_login` in the parent's
data map -> calls the function.

## Syntax Quick-Reference

| Prefix | Name | Example | Description |
|--------|------|---------|-------------|
| `{{ }}` | Binding | `{{user.name}}` | Display a data value. Updates automatically on change. |
| `{{#}}` | Hash | `{{#}}` | Current URL hash fragment. Updates on `hashchange`. |
| `?` | Conditional | `?is_admin` | Show/hide element based on truthiness. |
| `?!` | Negated | `?!is_loading` | Show element only when value is **falsy**. |
| `?="val"` | Equality | `?status="active"` | Show element only when value equals `"val"`. |
| `?!="val"` | Inequality | `?status!="deleted"` | Show element only when value does **not** equal `"val"`. |
| `?^="val"` | Prefix | `?url^="https"` | Show element only when value **starts with** `"val"`. |
| `?$="val"` | Suffix | `?name$=".pdf"` | Show element only when value **ends with** `"val"`. |
| `?*="val"` | Contains | `?tags*="urgent"` | Show element only when value **contains** `"val"`. |
| `*` | Iteration | `*items:i` | Repeat element for each array item (wrapped in `<span>`). |
| `**` | Iteration (no wrap) | `**items:i` | Repeat first child for each item (container stays). |
| `&` | Two-way | `&value="{{val}}"` | Sync `<input>` / `<select>` / `<textarea>` with data. |
| `@` | Event | `@click="on_save"` | Dispatch child event to parent handler function. |
| `@var` | Flex gender (i18n) | `{{@genero ~o %qt ~aluno}}` | Gender axis in a flexion block. Value keys the `<gender>.<category>` row. |
| `%var` | Flex count (i18n) | `{{%qt ~aluno}}` | Count axis in a flexion block. Emitted at its position; drives CLDR plural category. |
| `%var` (lone) | Format (i18n) | `{{%preco}}` | Locale-aware formatting. Type-directed: ints/floats, `time.Time`, `wi18n.Currency`, or any `Numerical`. |
| `~word` | Flex stem (i18n) | `{{~aluno ~aprovado}}` | Build-time marker for a word the translator will inflect. Consumed by `gen_i18n`. |
| `#N` | Flex rule index (i18n) | `{{@genero %qt #42}}` | Auto-assigned by `gen_i18n` when rewriting `.i18n.html`. Webdev never writes this. |

## Important Notes

### Attribute Names Are Lowercased

HTML attribute names are always converted to lowercase by the browser. This means
template variables used in attributes (`?condition`, `&attr`, `@event`) must use
**lowercase names only**. Use snake_case for multi-word identifiers.
The `!` suffix used for inequality (`?var!="val"`) is preserved by the browser
since only letters are lowercased:

```go
// CORRECT
"is_logged":    false,
"is_anonymous": true,

// WRONG - will not match because browser lowercases attributes
"isLogged":    false,
"isAnonymous": true,
```

```html
<!-- CORRECT -->
<my-child ?is_logged is_anonymous="{{is_anonymous}}"></my-child>

<!-- WRONG -->
<my-child ?isLogged isAnonymous="{{isAnonymous}}"></my-child>
```

Note: variables used only in text content (`{{camelCase}}`) are not affected by
this restriction, since they are parsed from text nodes, not attributes. However,
for consistency, snake_case is recommended everywhere.

### Template Root Element

If your template has multiple top-level elements, WINGS automatically wraps them
in a `<span>`. For predictable styling, consider using a single root element:

```html
<!-- Multiple roots (auto-wrapped in span) -->
<header>...</header>
<main>...</main>

<!-- Single root (no wrapper needed) -->
<div>
    <header>...</header>
    <main>...</main>
</div>
```

## Full Example

### main.go
```go
//go:build js && wasm

package main

import (
    "github.com/luisfurquim/wings"
    _ "myapp/mod/mywidget"
)

func main() {
    wings.Main()
}
```

### mod/mywidget/mywidget.go
```go
//go:build js && wasm

package mywidget

import (
    _ "embed"
    "syscall/js"

    "github.com/luisfurquim/wings"
    "github.com/luisfurquim/wings/dom"
    "github.com/luisfurquim/wings/timer"
)

//go:embed mywidget.html
var htmlContent string

//go:embed mywidget.css
var cssContent string

type MyWidget struct{}

func init() {
    wings.Register(
        "my-widget",
        htmlContent,
        cssContent,
        func() wings.PranaMod { return &MyWidget{} },
        "title",
    )
}

func (w *MyWidget) InitData() map[string]any {
    return map[string]any{
        "title":      "wings live demo",
        "count":      0,
        "count2":     0,
        "items":      []any{},
        "show_extra": false,
        "extra":      "This is extra content toggled by a boolean conditional.",
        "input_val":  "",
        "mode":       "edit",
    }
}

func (w *MyWidget) Render(obj *wings.PranaObj) {
    // Default to #list page if no hash fragment
    if js.Global().Get("location").Get("hash").String() == "" {
        wings.GoTo("list")
    }

    // Populate items
    obj.This.Set("items", []any{
        map[string]any{"label": "Alpha"},
        map[string]any{"label": "Beta"},
        map[string]any{"label": "Gamma"},
    })

    // Keep input_val in sync on every keystroke
    inputs := dom.Query(obj.Dom, "input[type=\"text\"]")
    if len(inputs) > 0 {
        dom.AddEvent(inputs[0], "input",
            func(this js.Value, args []js.Value) any {
                obj.This.Set("input_val", inputs[0].Get("value").String())
                return nil
            }, false, false)
    }

    // Form submit: add item
    forms := dom.Query(obj.Dom, "form")
    if len(forms) > 0 {
        dom.AddEvent(forms[0], "submit",
            func(this js.Value, args []js.Value) any {
                val := obj.This.Get("input_val").(string)
                if val != "" {
                    obj.This.Append("items", map[string]any{"label": val})
                    obj.This.Set("input_val", "")
                }
                return nil
            }, true, false)
    }

    // Toggle mode button
    toggleBtns := dom.Query(obj.Dom, "#btn-toggle-mode")
    if len(toggleBtns) > 0 {
        dom.AddEvent(toggleBtns[0], "click",
            func(this js.Value, args []js.Value) any {
                mode := obj.This.Get("mode").(string)
                if mode == "edit" {
                    obj.This.Set("mode", "readonly")
                } else {
                    obj.This.Set("mode", "edit")
                }
                return nil
            }, false, false)
    }

    // Toggle extra button
    extraBtns := dom.Query(obj.Dom, "#btn-toggle-extra")
    if len(extraBtns) > 0 {
        dom.AddEvent(extraBtns[0], "click",
            func(this js.Value, args []js.Value) any {
                show := obj.This.Get("show_extra").(bool)
                obj.This.Set("show_extra", !show)
                return nil
            }, false, false)
    }

    // Navigation links
    navList := dom.Query(obj.Dom, "#nav-list")
    if len(navList) > 0 {
        dom.AddEvent(navList[0], "click",
            func(this js.Value, args []js.Value) any {
                wings.GoTo("list")
                return nil
            }, true, false)
    }
    navDash := dom.Query(obj.Dom, "#nav-dash")
    if len(navDash) > 0 {
        dom.AddEvent(navDash[0], "click",
            func(this js.Value, args []js.Value) any {
                wings.GoTo("dashboard")
                return nil
            }, true, false)
    }

    // Page 1 counter: every 2 seconds
    go func() {
        tk := timer.NewTicker(2000)
        defer tk.Stop()
        n := 0
        for range tk.Tick {
            n++
            obj.This.Set("count", n)
        }
    }()

    // Page 2 counter: every 5 seconds
    go func() {
        tk := timer.NewTicker(5000)
        defer tk.Stop()
        n := 0
        for range tk.Tick {
            n++
            obj.This.Set("count2", n)
        }
    }()
}
```

### mod/mywidget/mywidget.html
```html
<div class="widget">
   <h1>{{title}}</h1>

   <!-- Navigation -->
   <nav>
      <a id="nav-list" href="#list">List</a>
      <a id="nav-dash" href="#dashboard">Dashboard</a>
   </nav>

   <!-- Toolbar -->
   <div class="toolbar">
      <button id="btn-toggle-mode">Toggle Mode</button>
      <button id="btn-toggle-extra">Toggle Extra</button>
      <span class="mode-badge">Mode: <strong>{{mode}}</strong></span>
   </div>

   <!-- Page 1: List -->
   <div ?#="list" class="page page-list">
      <h2>Item List</h2>
      <p>Auto-counter (every 2s): <span class="counter">{{count}}</span></p>

      <!-- Boolean conditional -->
      <div ?show_extra class="extra-box">
         <p>{{extra}}</p>
      </div>

      <!-- Negated boolean conditional -->
      <div ?!show_extra class="extra-hint">
         <p>Click "Toggle Extra" to reveal extra content.</p>
      </div>

      <!-- Equality conditional: only in edit mode -->
      <div ?mode="edit">
         <form>
            <input &value="{{input_val}}" type="text" placeholder="Add item..." />
            <input type="submit" value="Add" />
         </form>
      </div>

      <!-- Inequality conditional: hidden when readonly -->
      <div ?mode!="readonly" class="edit-hint">
         <p>Form is visible because mode != "readonly".</p>
      </div>

      <!-- Prefix conditional -->
      <div ?mode^="ed" class="cond-demo">
         <p>Prefix match: mode starts with "ed"</p>
      </div>

      <!-- Suffix conditional -->
      <div ?mode$="it" class="cond-demo">
         <p>Suffix match: mode ends with "it"</p>
      </div>

      <!-- Contains conditional -->
      <div ?mode*="d" class="cond-demo">
         <p>Contains match: mode contains "d"</p>
      </div>

      <ul>
         <li *items:i>{{items[i].label}}</li>
      </ul>
   </div>

   <!-- Page 2: Dashboard -->
   <div ?#="dashboard" class="page page-dash">
      <h2>Dashboard</h2>
      <p>Slow counter (every 5s): <span class="counter counter-lg">{{count2}}</span></p>

      <div class="dash-grid">
         <div class="dash-card">
            <h3>Items</h3>
            <ul>
               <li *items:i>{{items[i].label}}</li>
            </ul>
         </div>
         <div class="dash-card">
            <h3>Status</h3>
            <p>Mode: <strong>{{mode}}</strong></p>
            <p>Extra visible: <strong>{{show_extra}}</strong></p>
            <p>Fast counter: <strong>{{count}}</strong></p>
         </div>
      </div>

      <!-- Boolean conditional on dashboard too -->
      <div ?show_extra class="extra-box">
         <p>{{extra}}</p>
      </div>
   </div>
</div>
```

### mod/mywidget/mywidget.css
```css
.widget {
   max-width: 600px;
   margin: 24px auto;
   padding: 24px;
   font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
   color: #1a1a2e;
   background: #fff;
   border-radius: 12px;
   box-shadow: 0 2px 12px rgba(0,0,0,0.08);
}
h1 { margin: 0 0 12px; font-size: 1.4em; color: #16213e; }
nav { display: flex; gap: 8px; margin-bottom: 16px; border-bottom: 2px solid #e8e8e8; padding-bottom: 12px; }
nav a { padding: 6px 16px; border-radius: 6px; text-decoration: none; color: #0f3460; font-weight: 600; background: #e8f0fe; cursor: pointer; }
nav a:hover { background: #c5d9f7; }
.toolbar { display: flex; align-items: center; gap: 10px; margin-bottom: 16px; flex-wrap: wrap; }
.toolbar button { padding: 6px 14px; border: 1px solid #ccc; border-radius: 6px; background: #f4f4f9; cursor: pointer; }
.toolbar button:hover { background: #dde; }
.mode-badge { font-size: 0.85em; color: #555; }
h2 { margin: 0 0 12px; font-size: 1.15em; color: #0f3460; }
.counter { display: inline-block; background: #0f3460; color: #fff; padding: 2px 10px; border-radius: 4px; font-weight: 700; }
.counter-lg { font-size: 2em; padding: 8px 20px; border-radius: 8px; }
.extra-box { background: #e8f5e9; border-left: 4px solid #4caf50; padding: 8px 12px; border-radius: 4px; margin: 10px 0; }
.extra-hint { background: #fff3e0; border-left: 4px solid #ff9800; padding: 8px 12px; border-radius: 4px; margin: 10px 0; font-style: italic; }
.cond-demo { background: #ede7f6; border-left: 4px solid #7e57c2; padding: 6px 12px; border-radius: 4px; margin: 6px 0; font-size: 0.9em; }
form { display: flex; gap: 8px; margin: 12px 0; }
input[type="text"] { flex: 1; padding: 6px 10px; border: 1px solid #ccc; border-radius: 6px; }
input[type="submit"] { padding: 6px 16px; background: #0f3460; color: #fff; border: none; border-radius: 6px; cursor: pointer; font-weight: 600; }
ul { list-style: none; padding: 0; }
li { padding: 6px 10px; background: #f9f9fb; border-radius: 4px; margin-bottom: 4px; border: 1px solid #eee; }
.dash-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin: 16px 0; }
.dash-card { background: #f4f4f9; border-radius: 8px; padding: 12px; border: 1px solid #e0e0e0; }
.dash-card h3 { margin: 0 0 8px; font-size: 1em; color: #16213e; }
```

## Security

### Shadow DOM isolation

Every custom element registered with `Register` uses an **open** shadow root
(`mode: "open"`) by default.  This means any same-page script can access the
component's internal DOM via `element.shadowRoot`.

**Relevant advisories:** CVE-2019-11730 (Firefox same-origin bypass via shadow
DOM adoption), GHSA-wh77-3x4m-4q9g (Lit SSR shadow DOM template injection).

If you need stronger DOM isolation, use `RegisterWithOpts` and set
`ComponentOpts.Closed = true`:

```go
wings.RegisterWithOpts("my-widget", html, css,
    wings.ComponentOpts{Closed: true},
    func() wings.PranaMod { return &MyWidget{} },
)
```

With `Closed: true` the shadow root's `mode` is `"closed"` — external scripts
get `null` from `element.shadowRoot`.  The WINGS runtime itself accesses the
shadow root via the reference stored at construction time, so all internal
features (CSS injection, `Update()`, data binding) continue to work normally.

**Trade-offs of closed mode:**
- Testing tools that query the shadow root via `element.shadowRoot` will break
  and must be refactored to use events, attributes, or test IDs.
- DevTools in some browsers show closed shadow roots with limited visibility.
- Closed mode is not an absolute security barrier — slotted content, CSS custom
  properties, and custom events still cross shadow boundaries.

### innerHTML invariant

`domCreateTemplate(html)` sets `template.innerHTML` internally.  The `html`
argument **must** be a compile-time constant (typically a `//go:embed` string).
Never pass runtime, user-supplied, or server-fetched content to this function —
`innerHTML` is a DOM XSS sink.

### WASM memory isolation

`wasm_exec.js` by default stores the Go runtime object as `window.go`, which
exposes the WASM linear memory buffer to any same-origin script.  The
`index.html` templates in this repository wrap the instantiation in an
async IIFE so `go` never reaches `window`:

```js
(async () => {
    const go = new Go();
    const r = await WebAssembly.instantiateStreaming(fetch("main.wasm"), go.importObject);
    go.run(r.instance);
})().catch(console.error);
```

Copy this pattern in your own `index.html`.

### Script integrity (SRI)

`build.sh` injects `integrity="sha384-..."` attributes on the `<script>` tags
for `prana_helper.js` and `wasm_exec.js` after every build.  This prevents a
compromised CDN or build pipeline from silently substituting modified helpers.

### Catalog signing (ed25519)

`gen_i18n` can sign every `.json` catalog with an ed25519 key.  The WASM app
embeds the matching public key and the `wi18n` loader verifies each catalog
before using it.

**Generate a keypair (run once per project):**

```
gen_i18n -genkey -sign-key-password <strong-password>
```

This writes `gen_i18n.ed25519.key` (encrypted private key) and
`gen_i18n.ed25519.pub` (public key for embedding).

**Sign catalogs on every run:**

```
gen_i18n --path . --deflang en-US \
         -sign-key gen_i18n.ed25519.key \
         -sign-key-password <password>
```

A `.json.sig` sidecar is written next to each `.json` file.

**Verify in your WASM app:**

```go
//go:embed gen_i18n.ed25519.pub
var catalogPubKey []byte

func main() {
    if err := wi18n.SetCatalogPublicKey(catalogPubKey); err != nil {
        log.Fatal(err)
    }
    wings.Main()
}
```

Enforcement is opt-in and, once on, strict. With **no** public key configured,
verification is skipped and no `.sig` sidecar is even fetched (backward
compatible). Once a key **is** configured, every catalog the app loads MUST carry
a valid `.sig`: a missing sidecar is treated as tampering and the catalog is
rejected — the loader never falls through to an unsigned, possibly forged
catalog. This currently covers the main `<lang>.json`; `inflections.json` /
`fmt.json` enforcement is planned.

## Third-party data

`cmd/dictbuild` can download linguistic dictionaries from the
[UnitexGramLab/unitex-lingua](https://github.com/UnitexGramLab/unitex-lingua)
repository. These dictionaries are distributed under the
**[Lesser General Public License for Linguistic Resources (LGPLLR)](LICENSES/LGPLLR.txt)**.
They are **not stored in this repository** — they are fetched on demand
by `dictbuild -lang <tag>` and cached locally on the developer's machine.
WINGS's source code is not affected by the LGPLLR.

Copyright and per-language modification notices are in
[`cmd/gen_i18n/dicts/NOTICE.md`](cmd/gen_i18n/dicts/NOTICE.md).

**If you use `gen_i18n -auto-plurals`:** the generated
`*.inflections.json` files are derivative works of the LGPLLR-licensed
dictionaries. Before deploying them on your site, copy
[`NOTICE-TEMPLATE.md`](NOTICE-TEMPLATE.md) into your project's NOTICE
file and fill in the URL where your `i18n/` directory is accessible.
Users who fill plural forms by hand are not affected.

## Logo & credits

The WINGS logo (`assets/logo.png`) is a winged adaptation of the **Go gopher**.
The Go gopher was designed by **Renée French** and is licensed under the
[Creative Commons Attribution 3.0 license](https://creativecommons.org/licenses/by/3.0/).
This derivative keeps that attribution.

The illustration was produced with AI assistance and is provided **for use as
the WINGS project mascot only**. It is a separate asset from the source code:
it is *not* covered by the project's MPL 2.0 license, and (as an
AI-generated work) no independent copyright is claimed over it.

"Go" and the Go gopher are associated with the Go project / Google; WINGS is an
independent project and is **not affiliated with or endorsed by** them.

## License

This project is licensed under the [Mozilla Public License 2.0](LICENSE).
