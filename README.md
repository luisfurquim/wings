# wprana

[![Go Reference](https://pkg.go.dev/badge/github.com/luisfurquim/wprana.svg)](https://pkg.go.dev/github.com/luisfurquim/wprana)
[![License: MPL 2.0](https://img.shields.io/badge/License-MPL%202.0-brightgreen.svg)](https://opensource.org/licenses/MPL-2.0)
[![Go Report Card](https://www.goreportcard.com/badge/github.com/luisfurquim/wprana?ts=1712345678)](https://www.goreportcard.com/report/github.com/luisfurquim/wprana?ts=1712345678)

> **[Live Demo](https://luisfurquim.github.io/wprana/)** — try it in your browser, no install needed.
> Or run locally: clone the repo, `cd live-demo && bash build.sh && go run serve.go`

**Build reactive Web Components in pure Go — no JavaScript framework required.**

wprana compiles to WebAssembly and gives you custom HTML elements with
automatic data binding, conditional rendering, array iteration, two-way
form binding, hash-based routing, and parent-child communication — all
authored in Go and running natively in the browser.

### Why WPrana?

| | |
|---|---|
| **Pure Go** | Write components, state, and logic entirely in Go. Templates stay in plain HTML. |
| **Reactive** | Change a value with `Set()` and the DOM updates automatically — no virtual DOM diffing overhead. |
| **Lightweight** | Direct DOM manipulation via targeted refs. No framework runtime to download beyond your WASM binary. |
| **Encapsulated** | Each component lives inside a Shadow DOM with scoped CSS — no style leaks, no naming collisions. |
| **Two-Way Binding** | The `&` prefix syncs `<input>`, `<select>`, and `<textarea>` with your Go data map in both directions. |
| **Hash Routing** | Built-in `{{#}}` binding and `wprana.GoTo()` for SPA navigation without a router library. |
| **Composable** | Nest components freely. Parent-to-child data flows via attributes; child-to-parent events flow via `@` triggers. |
| **Standard Web** | Uses native Custom Elements v1 and Shadow DOM — works alongside any existing page or framework. |

---

## Table of Contents

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
  - [wprana/dom — Events and Queries](#wpranadom--events-and-queries)
  - [wprana/timer — Timers](#wpranatimer--timers)
  - [wprana/location — Browser Location](#wpranalocation--browser-location)
  - [wprana.KeyStorage — Storage Interface](#wpranakeystorage--storage-interface)
  - [wprana/localstorage — LocalStorage](#wpranalocalstorage--localstorage)
  - [wprana/opfs — Origin Private File System](#wpranaopfs--origin-private-file-system)
  - [JavaScript Interop (core)](#javascript-interop-core)
- [Customizable Widgets](#customizable-widgets)
  - [Customizable Interface](#customizable-interface)
  - [wprana.Update — Dynamic CSS](#wpranaupdate--dynamic-css)
- [Built-in Widgets](#built-in-widgets)
  - [wprana/widget/combobox — Multi-select Combobox](#wpranawidgetcombobox--multi-select-combobox)
- [Internationalization (i18n)](#internationalization-i18n)
  - [Pipeline Overview](#pipeline-overview)
  - [wprana/wi18n — Runtime Lookup](#wpranawi18n--runtime-lookup)
  - [Runtime Locale Switching (SetLang)](#runtime-locale-switching-setlang)
  - [cmd/gen_i18n — Build-time Extractor](#cmdgen_i18n--build-time-extractor)
  - [Flexion — Plurals & Gender (SynPrinter)](#flexion--plurals--gender-synprinter)
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

## Quick Start

WASM binaries cannot be loaded from `file://` URLs. The snippet below
builds a hello-world component, copies the required runtime files, and
starts a tiny Go server so you can open the page in a browser.

```bash
# 1. Create the project
mkdir hello-wprana && cd hello-wprana
go mod init hello-wprana
go get github.com/luisfurquim/wprana

# 2. Copy the JS helpers from the wprana module
WPRANA=$(go list -m -f '{{.Dir}}' github.com/luisfurquim/wprana)
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
    "github.com/luisfurquim/wprana"
)

//go:embed hello.html
var htmlContent string

type Hello struct{}

func init() {
    wprana.Register("hello-world", htmlContent, "",
        func() wprana.PranaMod { return &Hello{} })
}

func (h *Hello) InitData() map[string]any {
    return map[string]any{"greeting": "Hello from Go + WASM!"}
}

func (h *Hello) Render(_ *wprana.PranaObj) {}
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
    "github.com/luisfurquim/wprana"
    _ "hello-wprana/mod/hello"
)

func main() { wprana.Main() }
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

A wprana project has the following structure:

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
    ├── prana_helper.js     # wprana JS bridge (from wprana package)
    └── wasm_exec.js        # Go WASM runtime
```

### HTML Page

The load order is critical. `prana_helper.js` **must** come before `wasm_exec.js`,
and both must come before the WASM binary:

```html
<!DOCTYPE html>
<html>
<head>
   <!-- 1. wprana JS bridge (defines window._pranaDef) -->
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
    "github.com/luisfurquim/wprana"

    // Side-effect imports: each init() registers a module via wprana.Register()
    _ "myapp/mod/mywidget"
)

func main() {
    wprana.Main() // Defines all custom elements and blocks forever
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
    "github.com/luisfurquim/wprana"
    "github.com/luisfurquim/wprana/timer"
)

//go:embed counter.html
var htmlContent string

//go:embed counter.css
var cssContent string

type Counter struct{}

func init() {
    wprana.Register(
        "my-counter",       // custom element tag name
        htmlContent,         // embedded HTML template
        cssContent,          // embedded CSS
        func() wprana.PranaMod { return &Counter{} },
        "title",             // observed attributes
    )
}

func (c *Counter) InitData() map[string]any {
    return map[string]any{
        "title": "Counter",
        "count": 0,
    }
}

func (c *Counter) Render(obj *wprana.PranaObj) {
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

wprana automatically monitors `window.location.hash` and triggers a sync on
**all** live component instances whenever the hash changes, so `{{#}}`
references are always up to date.

To change the hash programmatically from Go:

```go
wprana.GoTo("settings")   // sets window.location.hash = "settings"
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

The handler must be a `func(...any)` (or `wprana.TriggerHandler`) in the
parent's data map.

**Important:** In `InitData`, the `obj` parameter is not yet available —
it only becomes available in `Render`. If your handler needs `obj` (which
is almost always the case), use `wprana.TriggerHandler(nil)` as a
placeholder in `InitData`, then set the real handler in `Render`:

```go
func (app *App) InitData() map[string]any {
    return map[string]any{
        // Placeholder — obj is not available here
        "on_login":  wprana.TriggerHandler(nil),
        "on_logout": wprana.TriggerHandler(nil),
    }
}

func (app *App) Render(obj *wprana.PranaObj) {
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
func (c *MyComponent) Render(obj *wprana.PranaObj) {
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

wprana exposes a package-level function to change the URL hash fragment:

```go
// Navigate to a new view
wprana.GoTo("settings")  // -> window.location.hash = "#settings"

// Clear the hash
wprana.GoTo("")          // -> window.location.hash = ""
```

All `{{#}}` bindings and `?#="value"` conditionals update automatically.

## How DOM Updates Work

wprana does **not** use a Virtual DOM. Instead it relies on **direct,
targeted DOM manipulation** guided by a compile-time reference map.

### Reference Extraction

When a component is first connected, wprana walks the HTML template once
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
mutations. wprana skips the diff entirely: the reference map already knows
*which* DOM nodes depend on *which* data keys, so it can jump directly to
the affected nodes. This makes updates O(bindings) rather than O(tree
size), with no garbage from disposable tree snapshots — an important
property in a WASM environment where GC pauses are more noticeable.

## Helper Packages

Helper functions are organized into subpackages so applications only
import what they actually use, keeping the WASM binary lean.

### wprana/dom — Events and Queries

`import "github.com/luisfurquim/wprana/dom"`

Register DOM event listeners with automatic `preventDefault` and `stopPropagation`
support:

```go
func (c *MyComponent) Render(obj *wprana.PranaObj) {
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

### wprana/timer — Timers

`import "github.com/luisfurquim/wprana/timer"`

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

### wprana/location — Browser Location

`import "github.com/luisfurquim/wprana/location"`

```go
// Get window.location.href as *url.URL
loc, err := location.Get()

// Get top.location.href as *url.URL (useful inside iframes)
topLoc, err := location.GetTop()
```

### wprana.KeyStorage — Storage Interface

The `wprana.KeyStorage` interface defines a backend-agnostic key-value
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

Modules that need persistent storage should accept a `wprana.KeyStorage`
instead of a concrete type. This way the application's `main()` decides
which backend to use:

```go
// In a module package:
var Store wprana.KeyStorage

// In main():
import "github.com/luisfurquim/wprana/localstorage"
import "github.com/luisfurquim/wprana/opfs"

// Option A: localStorage backend
myModule.Store = localstorage.NewKV(nil, nil)

// Option B: OPFS backend (recommended for larger/sensitive data)
myModule.Store = opfs.New(nil, nil)
```

### wprana/localstorage — LocalStorage

`import "github.com/luisfurquim/wprana/localstorage"`

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

#### KV — Recommended API (implements wprana.KeyStorage)

`KV` is the recommended way to use localStorage. It implements
`wprana.KeyStorage`:

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
**not** implement `wprana.KeyStorage` (its `Set` and `Del` methods do not
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

### wprana/opfs — Origin Private File System

`import "github.com/luisfurquim/wprana/opfs"`

Access the browser's [Origin Private File System](https://developer.mozilla.org/en-US/docs/Web/API/File_System_API/Origin_private_file_system)
directly from Go WASM. Files are stored in a sandboxed, origin-scoped
filesystem that is invisible to the user and not subject to the same
storage limits as localStorage.

`opfs.Store` implements `wprana.KeyStorage` and uses the same
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

These functions remain in the core `wprana` package:

```go
// Access the global window object
window := wprana.JSGlobal()

// Create a persistent JS callback (must call Release() when done)
fn := wprana.JSFunc(func(this js.Value, args []js.Value) any {
    // handle callback
    return nil
})
defer fn.Release()

// Create a one-shot JS callback (auto-releases after first call)
fn := wprana.JSFuncOnce(func() {
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
  live instances immediately via `wprana.Update()`.

### wprana.Update — Dynamic CSS

```go
wprana.Update(tagName string, cssContent string)
```

Replaces the CSS of a registered custom element and updates the `<style>`
tag in the Shadow DOM of every live instance. Called automatically by
`ReplaceCSS`; can also be called directly for full CSS replacement.

## Built-in Widgets

### wprana/widget/combobox — Multi-select Combobox

`import _ "github.com/luisfurquim/wprana/widget/combobox"`

A multi-select combobox with type-ahead filtering, tag display, and
keyboard support.

```html
<wp-combobox
    options='["Alpha","Beta","Gamma"]'
    placeholder="Type to filter..."
    @notinlist="on_notinlist"
    @change="on_change">
</wp-combobox>
```

**Attributes:**

| Attribute | Description |
|-----------|-------------|
| `options` | JSON array of strings or `[{"label":"...","value":"..."},...]` objects |
| `placeholder` | Input placeholder text (default: "Type to filter...") |

**Events (via `@`):**

| Event | Args | Description |
|-------|------|-------------|
| `@notinlist` | typed string | Enter pressed with text not matching any option |
| `@change` | `[]any` of selected items | Selection changed (add or remove) |

**CSS Customization:**

The combobox CSS is split into two parts:

- **Vars** — CSS custom properties for colors, shadows, etc. Replace
  this to change the visual theme:

```go
cb := combobox.New()
cb.ReplaceCSS("Vars", `
:host {
    --cb-tag-bg: #1e293b;
    --cb-tag-color: #e2e8f0;
    --cb-tag-border: #475569;
    --cb-accent: #3b82f6;
    /* ... */
}
`)
```

- **Design** — Layout, spacing, transitions. Uses `var()` references for
  all colors, so changing Vars is enough for most themes.

Available CSS custom properties:

| Variable | Default | Used for |
|----------|---------|----------|
| `--cb-tag-bg` | `#ede9fe` | Selected tag background |
| `--cb-tag-color` | `#4c1d95` | Selected tag text |
| `--cb-tag-border` | `#c4b5fd` | Selected tag border |
| `--cb-rm-color` | `#7c3aed` | Remove button color |
| `--cb-rm-hover-bg` | `#ddd6fe` | Remove button hover background |
| `--cb-rm-hover-color` | `#dc2626` | Remove button hover color |
| `--cb-input-border` | `#d1d5db` | Input border |
| `--cb-input-focus-border` | `#7c3aed` | Input focus border |
| `--cb-input-focus-shadow` | `rgba(124,58,237,0.12)` | Input focus ring |
| `--cb-input-bg` | `#fff` | Input background |
| `--cb-drop-bg` | `#ffffff` | Dropdown background |
| `--cb-drop-border` | `#d1d5db` | Dropdown border |
| `--cb-drop-shadow` | (see vars.css) | Dropdown shadow |
| `--cb-scroll-thumb` | `#c4b5fd` | Scrollbar thumb |
| `--cb-opt-color` | `#1f2937` | Option text |
| `--cb-opt-hover-bg` | `#f5f3ff` | Option hover background |
| `--cb-opt-hover-color` | `#5b21b6` | Option hover text |
| `--cb-opt-active-bg` | `#ede9fe` | Option active background |
| `--cb-empty-color` | `#9ca3af` | "No results" text |

## Internationalization (i18n)

wprana ships with an end-to-end i18n pipeline: a build-time extractor that
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
            │ wprana.Printer   = lookup  │     installs Printer (text index →
            │ wprana.SynPrinter = flex   │     string) and SynPrinter (flex
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
attributes listed in `wprana.TranslatableAttrs` (default: `title`,
`placeholder`, `alt`, `aria-label`).

For plurals and gender agreement — cases where a single translation string
cannot reflect the target locale's grammar — wprana ships a parallel
pipeline keyed on inline **flex sigils** (`@var`/`%var`/`~word`/`#N`). See
[Flexion — Plurals & Gender (SynPrinter)](#flexion--plurals--gender-synprinter)
below for the full syntax and catalog format.

### wprana/wi18n — Runtime Lookup

`import _ "github.com/luisfurquim/wprana/wi18n"` — side-effect import only.

On `init()`, `wi18n`:

1. Detects the browser language from `navigator.languages[0]`, falling back
   to `navigator.language`, then `en-US`.
2. Sets `<html lang="…">` accordingly.
3. Registers itself on `wprana.InitWG` so `wprana.Main()` waits for the
   catalog to load before defining custom elements.
4. Fetches `<BasePath><lang>.json` (with fallback chain: full tag → base
   language → `en-US`) relative to the current page. `BasePath` defaults to
   `i18n/`; apps that ship their own catalog next to the project's catalog
   call `wi18n.SetBasePath("<dir>/")` from an `init()` that runs after
   `wi18n`'s (see the wlate self-i18n setup below for a concrete example).
5. Decodes the JSON as a `[]wi18n.Entry` array (see schema below); builds
   the lookup table from each entry's `content` field.
6. Replaces `wprana.Printer` with a function that parses the TextNode
   content (or attribute value, when the attribute is in
   `wprana.TranslatableAttrs`) as a decimal index and returns `table[idx]`.
   Entries whose `content` is empty fall back to rendering the raw index —
   a deliberate visual signal for missing translations.

If no catalog can be loaded, `wprana.Printer` stays as the default `ByPass`
and TextNodes render their raw numeric indices.

The bundle loaded at init is also stashed in an in-memory cache keyed by
the requested tag, so the first call to `wi18n.SetLang(<initial-lang>)`
later in the session is a cache hit (no re-fetch). See [Runtime Locale
Switching (SetLang)](#runtime-locale-switching-setlang) below.

```go
// Usage: import for side effects, that's it.
import (
    "github.com/luisfurquim/wprana"
    _ "github.com/luisfurquim/wprana/wi18n"
    _ "myapp/mod/mywidget"
)

func main() { wprana.Main() }

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
import "github.com/luisfurquim/wprana/wi18n"

// from a click handler, language picker, etc.
wi18n.SetLang("en-US", func(err error) {
    if err != nil {
        // No catalog available for "en-US" or any of its fallbacks.
        return
    }
    // Locale switch is complete; wprana.Locale is now "en-US"
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
  `wprana.Locale` and `<html lang="…">` are updated.
- Every live custom-element instance is then walked: each text node and
  attribute that the constructor stashed (via `_wi18nSrc` /
  `_wi18nAttr_*` JS expandos on the node) is re-translated, the new
  binding string is re-parsed, and the affected `PranaState` is
  re-synced. Existing `*items:i` clones, conditional state, two-way
  bindings and timer goroutines are preserved.

**Limitation.** The walker rewrites `TextSegs` / `Attrs.Segs` in place;
it does not currently create or destroy `DOMRefNode`s. If a translation
introduces or removes `{{...}}` placeholders that were absent from the
source-language version, those placeholders may render as raw text. Keep
the `{{...}}` shape consistent across locales for templates that depend
on it.

**Performance caches.** Two memoisation layers sit underneath the
runtime so the per-switch cost stays low even for large pages:

- *Intl instance cache* (`wprana/wi18n/intl_cache.go`).
  `Intl.NumberFormat` / `Intl.DateTimeFormat` instances are expensive to
  construct (each one parses CLDR locale data behind the syscall/js
  boundary) and essentially free to call. They are cached keyed by
  `(locale, options)` and reused across `SetLang` calls — so toggling
  back to a previous locale always hits warm formatters.
- *Parsed-template cache* (`wprana/parse_cache.go`). The re-bind walker
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
go run github.com/luisfurquim/wprana/cmd/gen_i18n \
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
- Writes `<file>.i18n.html` next to each source template. Embed these at
  compile time via `//go:embed` instead of the original HTML.
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
`alt`, and `aria-label` carry user-visible text just like text nodes do.
`gen_i18n` extracts the values of the following attributes by default:

```
title, placeholder, alt, aria-label
```

Three flags tune this set:

| Flag | Effect |
|---|---|
| `--attrs <list>` | Replaces the default list entirely. Comma-separated, case-insensitive. |
| `--add-attrs <list>` | Appends to the active list (default, or whatever `--attrs` produced). |
| `--no-attrs <list>` | Removes from the active list after additions, so it can also drop defaults. |

In `ctxdetail`, attribute occurrences are distinguished from text-node
occurrences by the tag format: `button[title]@path:line:col` vs.
`button@path:line:col`.

**Runtime mirror.** At runtime, `wprana.TranslatableAttrs` controls which
attributes the engine passes through `Printer`. Its default matches the
default `gen_i18n` set, and **must mirror** whatever flags produced the
catalog, otherwise attribute values render as raw decimal indices:

```go
// Option 1: assign a fresh slice (full override)
wprana.TranslatableAttrs = []string{"title", "placeholder"}

// Option 2: incremental — safe with other packages that may also tweak it
wprana.AddTranslatableAttrs("data-tip", "aria-placeholder")
wprana.RemoveTranslatableAttrs("title")
```

The helpers are case-insensitive, trim whitespace, and skip duplicates.
Both the assignment and the helper calls must run before `wprana.Main()`
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
| `--dict-dir <dir>` | Directory holding `<lang>.db` files. Default: `cmd/gen_i18n/dicts` under the wprana module. |
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
plural categories per gender. wprana's flex pipeline handles this by making
the grammar-shaping variables visible to the runtime via inline **sigils**.

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
(`@genero`, `%qt`) and routes path-bearing sigils through `wprana.Solve`
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
installs `wprana.SynPrinter` — a second printer hook invoked by the syncer
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

Without `wi18n` loaded, the default `wprana.NoFlexSynPrinter` renders the
rule index as `#N` — missing inflection support stays obvious on the page
instead of silently dropping content.

### Locale-Aware Formatting (FmtPrinter)

Text catalogs translate strings and flex blocks resolve plurals/gender. A
third axis — **values** — needs locale-aware treatment too: numbers use
different decimal and grouping separators across locales, currencies have
their own symbols and fractional digits, dates span a zoo of formats. The
`FmtPrinter` pipeline handles all of these through the same sigil already
used for the count axis of flex blocks: `%var`.

**Template syntax.** When a `{{...}}` binding contains exactly one `%var`
(optionally followed by a path tail), it is a **format block** — the
value is resolved from the data context and handed to `wprana.FmtPrinter`,
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

**Fallback behaviour.** Without `wi18n` imported, `wprana.FmtPrinter`
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
`wprana.json`) to set per-locale default units for any quantity:

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
| `wi18n/cooking` | `CookingVolume{Liters float64}` / `CookingWeight{Kilograms float64}` | L / kg | vol: `L` `mL` `cup` `tbsp` `tsp` `floz` · wt: `kg` `g` `lb` `oz` |

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
  lives under `-state-dir` (defaults to `$XDG_CACHE_HOME/wprana/dictbuild`).
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

`helpers/wlate/` is a wprana-built WASM app designed for translators to
review and edit catalogs side-by-side with a reference language.

**Features (implemented):**

- Two-panel layout: reference language (read-only) vs. target language
  (editable), with per-entry "revised" toggle (bordeaux left border = not
  yet reviewed; gray = reviewed).
- Two tabs: **Texto** (plain strings) and **Inflexões** (gender × CLDR
  plural category grid, using CSS Grid with `display: contents` on
  iteration wrappers).
- Keyboard shortcuts for navigation and toggling revised state (all
  rebindable via `wprana.json`).
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

**Self-i18n.** wlate itself is built as a translatable wprana app — the
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

1. **Registration** (`init()`): Module calls `wprana.Register()` to register the
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
        "on_login": wprana.TriggerHandler(nil),
    }
}

func (app *App) Render(obj *wprana.PranaObj) {
    // Real handler with obj in scope
    obj.This.Set("on_login", func(args ...any) {
        obj.This.Set("is_logged", true)
    })
}
```

**Child Render (login.go):**
```go
func (lgn *Login) Render(obj *wprana.PranaObj) {
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

If your template has multiple top-level elements, wprana automatically wraps them
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
    "github.com/luisfurquim/wprana"
    _ "myapp/mod/mywidget"
)

func main() {
    wprana.Main()
}
```

### mod/mywidget/mywidget.go
```go
//go:build js && wasm

package mywidget

import (
    _ "embed"
    "syscall/js"

    "github.com/luisfurquim/wprana"
    "github.com/luisfurquim/wprana/dom"
    "github.com/luisfurquim/wprana/timer"
)

//go:embed mywidget.html
var htmlContent string

//go:embed mywidget.css
var cssContent string

type MyWidget struct{}

func init() {
    wprana.Register(
        "my-widget",
        htmlContent,
        cssContent,
        func() wprana.PranaMod { return &MyWidget{} },
        "title",
    )
}

func (w *MyWidget) InitData() map[string]any {
    return map[string]any{
        "title":      "wprana live demo",
        "count":      0,
        "count2":     0,
        "items":      []any{},
        "show_extra": false,
        "extra":      "This is extra content toggled by a boolean conditional.",
        "input_val":  "",
        "mode":       "edit",
    }
}

func (w *MyWidget) Render(obj *wprana.PranaObj) {
    // Default to #list page if no hash fragment
    if js.Global().Get("location").Get("hash").String() == "" {
        wprana.GoTo("list")
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
                wprana.GoTo("list")
                return nil
            }, true, false)
    }
    navDash := dom.Query(obj.Dom, "#nav-dash")
    if len(navDash) > 0 {
        dom.AddEvent(navDash[0], "click",
            func(this js.Value, args []js.Value) any {
                wprana.GoTo("dashboard")
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

## Third-party data

`cmd/dictbuild` can download linguistic dictionaries from the
[UnitexGramLab/unitex-lingua](https://github.com/UnitexGramLab/unitex-lingua)
repository. These dictionaries are distributed under the
**[Lesser General Public License for Linguistic Resources (LGPLLR)](LICENSES/LGPLLR.txt)**.
They are **not stored in this repository** — they are fetched on demand
by `dictbuild -lang <tag>` and cached locally on the developer's machine.
WPrana's source code is not affected by the LGPLLR.

Copyright and per-language modification notices are in
[`cmd/gen_i18n/dicts/NOTICE.md`](cmd/gen_i18n/dicts/NOTICE.md).

**If you use `gen_i18n -auto-plurals`:** the generated
`*.inflections.json` files are derivative works of the LGPLLR-licensed
dictionaries. Before deploying them on your site, copy
[`NOTICE-TEMPLATE.md`](NOTICE-TEMPLATE.md) into your project's NOTICE
file and fill in the URL where your `i18n/` directory is accessible.
Users who fill plural forms by hand are not affected.

## License

This project is licensed under the [Mozilla Public License 2.0](LICENSE).
