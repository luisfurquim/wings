---
name: wings-component
description: Write a WINGS component (custom element in Go→WASM) — module file trio, wings.Register, the PranaMod lifecycle (InitData/Render), template binding syntax ({{expr}}, ?cond, *arr/**arr, &attr, @event), and parent-child events. Use when creating or editing a WINGS module, template, custom element, PranaMod, or any template binding.
---

# Writing a WINGS component

A component is a custom element whose logic is **Go compiled to WASM**. Read
`AGENTS.md` at the repo root first for the mental model and the two
non-negotiable gotchas; this skill is the working detail.

## The module is three files (same basename)

```
mymod/
  mymod.go    # init() registers + the PranaMod implementation
  mymod.html  # template with bindings
  mymod.css   # styles (shadow-DOM scoped)
```

Every component `.go` file starts with `//go:build js && wasm`.

## Minimal working component

`mymod.go`:
```go
//go:build js && wasm

package mymod

import (
	_ "embed"
	"github.com/luisfurquim/wings"
)

//go:embed mymod.html
var htmlContent string

//go:embed mymod.css
var cssContent string

type MyMod struct{}

func init() {
	wings.Register(
		"my-mod",      // custom element tag (must contain a hyphen)
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &MyMod{} },
		"title",       // ...observed attributes (re-render when they change)
	)
}

// InitData: initial state. obj is NOT available here.
func (m *MyMod) InitData() map[string]any {
	return map[string]any{
		"title":      "Hello",
		"count":      0,
		"items":      []any{},
		"show_extra": false,   // snake_case — see gotcha #1
	}
}

// Render: runs after connect; obj is available.
func (m *MyMod) Render(obj *wings.PranaObj) {
	obj.This.Set("items", []any{
		map[string]any{"label": "Alpha"},
		map[string]any{"label": "Beta"},
	})
}
```

`mymod.html`:
```html
<div class="widget">
  <h2>{{title}}</h2>
  <p>Count: <span>{{count}}</span></p>
  <ul>
    <li *items:i>{{items[i].label}}</li>
  </ul>
  <div ?show_extra>
    <p>Extra content</p>
  </div>
  <input &value="{{input_val}}" type="text" />
</div>
```

App entry point (`main.go`, native to the app, one per app):
```go
//go:build js && wasm
package main

import (
	"github.com/luisfurquim/wings"
	_ "myapp/mod/mymod" // blank-import every module to register it
)

func main() { wings.Main() }
```

## PranaMod lifecycle

- `InitData() map[string]any` — initial data map. `obj` is **not** ready.
- `Render(obj *wings.PranaObj)` — after the element connects. Here:
  - `obj.This` — reactive store: `Set(key, v)`, `Get(key) any`, `Append(key, v)`,
    `DeleteAt(key, i)`, `Delete(key)`. Changing data re-renders the DOM.
  - `obj.Element` / `obj.Dom` — the element / its shadow root (`js.Value`).
  - `obj.Trigger(name, args...)` — fire an event up to the parent.

## Template binding syntax

| Syntax | Meaning |
|---|---|
| `{{x}}`, `{{a.b}}`, `{{arr[i].f}}` | display a value; auto-updates |
| `{{#}}` | current URL hash fragment |
| `?cond` / `?!cond` | show if truthy / falsy |
| `?x="v"` `?x!="v"` `?x^="v"` `?x$="v"` `?x*="v"` | eq / ne / prefix / suffix / contains |
| `*arr:i` | repeat element per item (`i` is the loop index, used as `{{arr[i].field}}`; wrapped in `<span>`) |
| `**arr:i` | repeat first child per item (container kept) |
| `&value="{{v}}"` | two-way bind input/select/textarea |
| `@event="handler"` | route child event to a parent handler |

(i18n flexion/format sigils — `{{@g %n ~word}}`, `{{%price}}` — are covered by
the `wings-i18n` skill.)

### Switching views by state

`?key="value"` shows an element only when the reactive key **equals** that
string, so one status key can drive a set of mutually exclusive views — the
idiom for loading/ready/error and any small state machine. Set the key with
`obj.This.Set`; gate each view with its own equality test:

```go
func (m *MyMod) InitData() map[string]any {
	return map[string]any{"status": "loading"}
}
func (m *MyMod) Render(obj *wings.PranaObj) {
	go func() {
		if err := load(obj); err != nil {
			obj.This.Set("status", "error") // flips which view shows
			return
		}
		obj.This.Set("status", "ready")
	}()
}
```

```html
<p   ?status="loading">Loading…</p>
<p   ?status="error">Something went wrong.</p>
<ul  ?status="ready"><li *items:i>{{items[i].name}}</li></ul>
```

The compared value is a **data key**, not an attribute literal; changing it via
`obj.This.Set` re-renders and switches the visible view.

## Gotcha #1 — snake_case in binding NAMES (the most common mistake)

The browser lowercases attribute names. A binding **name** with an uppercase
letter (`?isLoading`, `*myItems`, `&myValue`, `showExtra="{{...}}"`) silently
becomes a **no-op**: wrong render, no error. Use snake_case.

- Affected: `?cond`, `*arr`, `**arr`, `&attr`, and any binding written as an
  attribute name (e.g. `show_extra="{{show_extra}}"`).
- Exempt: `{{textBinding}}` inside text content (parsed from text, not
  attributes) — but use snake_case there too for consistency.

`go run ./cmd/build <target>` scans templates and **fails the build** on any
uppercase binding name with `file:line`. If you hit that, rename to snake_case.

## Gotcha #2 — handlers that need `obj`

`obj` is unavailable in `InitData`. For event handlers (and anything needing
`obj`), put a placeholder in `InitData` and set the real handler in `Render`:

```go
func (a *App) InitData() map[string]any {
	return map[string]any{"on_save": wings.TriggerHandler(nil)}
}
func (a *App) Render(obj *wings.PranaObj) {
	obj.This.Set("on_save", func(args ...any) {
		obj.This.Set("saved", true)
	})
}
```

## Handling user interaction (native DOM events)

`@event` (below) is **only** for component events a child raises with
`obj.Trigger`. It does **not** bind a native DOM event such as a button click.
For those, attach a listener in `Render` with the `wings/dom` package: give the
element an `id` in the template, query it from the shadow root (`obj.Dom`), and
add the listener.

```go
import (
	"syscall/js"
	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
)

func (m *MyMod) Render(obj *wings.PranaObj) {
	if els := dom.Query(obj.Dom, "#clear-btn"); len(els) > 0 {
		dom.AddEvent(els[0], "click",
			func(this js.Value, args []js.Value) any {
				obj.This.Set("items", []any{})
				obj.Trigger("cleared") // optionally also notify the parent
				return nil
			}, false, false) // preventDefault, stopPropagation
	}
}
```

In the template the element only needs the id: `<button id="clear-btn">Clear</button>`.
There is no `@click` attribute — wiring native events is Go code in `Render`.

To **read a value** off the event target (an `<input>`'s text, a `<select>`'s
value), pull it from `this` inside the handler and **type-check before use**:
every `js.Value` from the DOM is untrusted, and a wrong-type read panics — which
in WASM kills the whole app. This is where the event wiring meets the
`sec-wasm-go` / `sec-hostile-input` rules:

```go
func (m *MyMod) Render(obj *wings.PranaObj) {
	if els := dom.Query(obj.Dom, "#search"); len(els) > 0 {
		dom.AddEvent(els[0], "input",
			func(this js.Value, args []js.Value) any {
				v := this.Get("value")
				if v.Type() != js.TypeString { // guard: wrong type would panic
					return nil
				}
				obj.This.Set("query", v.String()) // store it; recompute any derived list — see "Derived lists"
				return nil
			}, false, false)
	}
}
```

WINGS frees this listener for you when the component disconnects — no manual
cleanup needed.

## Parent → child data, child → parent events

`@event` routes **component** events (a child raised via `obj.Trigger`), not
native DOM events (see the section above for those).

- **Down:** the parent passes data via attributes:
  `<my-child label="{{title}}" ?is_open></my-child>` (child lists `label`,
  `is_open` in its observed attributes if they change at runtime).
- **Up:** parent template binds `@event="handler"`; child fires
  `obj.Trigger("event", args...)`. The handler is a `func(args ...any)` in the
  parent's data (see gotcha #2). `@event` attrs are read at trigger time and do
  **not** need to be observed.

## Async work, timers, and the teardown caveat

Do background or periodic work in `Render`: launch a goroutine and write results
through `obj.This.Set` (safe — WASM is single-threaded and the model re-renders).
Drive timing with `setTimeout` via the core helpers (or Go's `time`).

```go
func (m *MyMod) Render(obj *wings.PranaObj) {
	go func() {
		for obj.Element.Get("isConnected").Bool() { // stop when removed — see caveat
			done := make(chan struct{})
			wings.JSGlobal().Call("setTimeout", wings.JSFuncOnce(func() {
				n, _ := obj.This.Get("count").(int)
				obj.This.Set("count", n+1)
				close(done)
			}), 1000)
			<-done
		}
	}()
}
```

**Teardown caveat:** `PranaMod` has only `InitData` and `Render` — there is **no
disconnect/teardown hook**, and `Render` gets no cancellation context. On
disconnect WINGS frees what it can see (two-way bindings, the render goroutine,
and `dom.AddEvent` listeners under the element), but it **cannot interrupt a
goroutine you launched**: a loop that never exits keeps running after the
element leaves the DOM. For app-lifetime components that is harmless; for
components created and destroyed repeatedly, guard the loop with
`obj.Element.Get("isConnected").Bool()` and return when false. Never write a
`for {}` or `for range ticker.C` with no exit condition.

## Loading data from a backend (`fetch`)

You don't need a WINGS API for HTTP — Go's standard `net/http` **works as-is**
under `GOOS=js`: the runtime maps it onto the browser's Fetch API. The
non-obvious parts (this is the kind of thing that's easy to get subtly wrong):

- **Always from a goroutine** launched in `Render`, never inline — a blocking
  request on the event-loop path freezes the page (WASM is single-threaded).
- **Relative URLs are same-origin** (`/api/...` resolves against the page); a
  cross-origin call needs the server's CORS headers.
- **Cookies/credentials aren't sent cross-origin by default.** To include them,
  build the request and set a Fetch-option header —
  `req.Header.Set("js.fetch:credentials", "include")` — using
  `http.NewRequest` + `client.Do` (plain `http.Get` gives you no place to set
  it). Other `js.fetch:` headers (`mode`, `redirect`) tune the fetch the same way.
- **Raw sockets don't exist** in the browser — only `net/http` (fetch).
  `net.Dial`/TCP/UDP are unavailable; don't reach for them.
- **Validate what comes back** at this boundary: check the status, then decode
  into a *concrete struct* (not `map[string]any`) — see `sec-hostile-input`. The
  body is untrusted; never put it into `innerHTML` (`sec-wasm-go`). Drive the
  DOM through `obj.This`, as always.

```go
func load(obj *wings.PranaObj) error {
	cli := &http.Client{Timeout: 15 * time.Second} // Timeout maps to AbortController
	resp, err := cli.Get("/api/items")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("backend: %s", resp.Status)
	}
	var items []Item // concrete shape, parsed exactly once here
	dec := json.NewDecoder(resp.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&items); err != nil {
		return err
	}
	obj.This.Set("items", toAny(items)) // reactive store → DOM updates
	return nil
}
```

## Derived lists (search / filter / sort)

The reactive store holds **plain values** — `Set`/`Get`/`Append`/`DeleteAt`/
`Delete`. There are **no computed/derived values**: setting one key never
recomputes another. So to show a filtered or sorted view of a list, keep two
keys — the immutable source and the derived view — and recompute the derived
one yourself whenever an input changes, writing it back with `Set`:

```go
func (m *MyMod) InitData() map[string]any {
	return map[string]any{"all": []any{}, "visible": []any{}, "query": ""}
}

// recompute "visible" from the source "all" + current "query"
func applyFilter(obj *wings.PranaObj) {
	all, _ := obj.This.Get("all").([]any)
	q, _ := obj.This.Get("query").(string)
	q = strings.ToLower(strings.TrimSpace(q))
	out := make([]any, 0, len(all))
	for _, item := range all {
		row, _ := item.(map[string]any)
		name, _ := row["author"].(string)
		if q == "" || strings.Contains(strings.ToLower(name), q) {
			out = append(out, row)
		}
	}
	obj.This.Set("visible", out)
}
```

Call `applyFilter(obj)` from the search handler (right after
`Set("query", …)`) and after you populate `all`. The template repeats the
**derived** key — `<li *visible:i>{{visible[i].author}}</li>` — so the source
list stays intact while the view updates.

## Before you finish

- Run `go run ./cmd/build <target>` — it lints (catches gotcha #1) and builds.
- Single root element in the template avoids an auto `<span>` wrapper.
- Never set innerHTML or touch the DOM directly for data display — drive it
  through `obj.This`.

Deeper reference: README "Template Syntax", "Reactive Data API", "Parent-Child
Communication", "Full Example". Sibling skills: `wings-i18n`, `wings-skins`,
`wings-widgets` (tabs/dialog/combobox — don't hand-roll them), `wings-build`.
