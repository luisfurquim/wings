# Writing apps with WINGS — agent guide

WINGS builds browser UI as **Go compiled to WASM**: each component is a custom
element whose logic is Go, not JavaScript. This file is the always-on context;
deeper, area-specific guidance lives in the `wings-authoring` Claude plugin
(skills) and in `README.md` (section links below).

## Mental model

- A **module** is a custom element = three files with the same basename:
  `name.go` (logic), `name.html` (template), `name.css` (styles).
- `name.go` runs `wings.Register(tag, html, css, factory, ...observedAttrs)` in
  `init()` and implements **`PranaMod`**: `InitData() map[string]any` (initial
  state) and `Render(obj *wings.PranaObj)` (runs after connect, `obj` available).
- State lives in `obj.This` (a reactive store: `Set`/`Get`/`Append`/`DeleteAt`).
  The DOM updates automatically from data changes — **there is no virtual DOM**
  and you never touch innerHTML.
- Components are **shadow-DOM isolated**. i18n is resolved at **build time**
  (`gen_i18n` rewrites templates); you author plain text.
- All component files carry `//go:build js && wasm`. The app entry point is
  `func main() { wings.Main() }` with blank imports of each module.

## Non-negotiable gotchas (the #1 and #2 mistakes)

1. **Binding NAMES in attributes must be lowercase — use snake_case.** The
   browser lowercases attribute names, so a camelCase binding name silently
   becomes a no-op (wrong render, zero error). Applies to `?cond`, `*arr`,
   `**arr`, `&attr`. `{{textBinding}}` in text content is exempt but use
   snake_case everywhere for consistency.
   ```html
   <my-child ?is_logged show_extra="{{show_extra}}"></my-child>   <!-- CORRECT -->
   <my-child ?isLogged showExtra="{{showExtra}}"></my-child>      <!-- WRONG: no-op -->
   ```
   `go run ./cmd/build <target>` **fails the build** on any uppercase binding
   name, with `file:line`. Treat a lint failure here as "rename to snake_case".

2. **Event handlers that need `obj` can't be built in `InitData`** (`obj` isn't
   ready yet). Put `wings.TriggerHandler(nil)` as a placeholder in `InitData`,
   then set the real `func(args ...any){...}` in `Render` via `obj.This.Set`.

## Template syntax (quick reference)

| Syntax | Meaning |
|---|---|
| `{{x}}` / `{{a.b}}` | display a value (auto-updates) |
| `{{#}}` | current URL hash fragment |
| `?cond` `?!cond` `?x="v"` `?x!="v"` `?x^="v"` `?x$="v"` `?x*="v"` | conditional show/hide (truthy / falsy / eq / ne / prefix / suffix / contains) |
| `*arr:i` | repeat element per item (wrapped in `<span>`) |
| `**arr:i` | repeat first child per item (container kept) |
| `&value="{{v}}"` | two-way bind an `<input>`/`<select>`/`<textarea>` |
| `@event="handler"` | route a child event to a parent handler (child calls `obj.Trigger("event")`) |
| `{{@g %n ~word}}` `{{%price}}` | i18n flexion/format (see `wings-i18n`) |

`@event` is only for component events a child raises with `obj.Trigger`. Native
DOM events (a button **click**, etc.) are wired in Go inside `Render` — there is
no `@click`: give the element an `id`, then
`dom.AddEvent(dom.Query(obj.Dom, "#id")[0], "click", handler, false, false)`
(package `github.com/luisfurquim/wings/dom`).

If a template has multiple top-level elements WINGS wraps them in a `<span>`;
prefer a single root element for predictable styling.

## Build & run

- `go run ./cmd/build <lib|example|live-demo|wlate|all>` — builds + lints.
- `go run ./cmd/build dev` — dev server with rebuild-on-save (env `WINGS_*`).
- Copy the JS helpers (`prana_helper.js`, `wasm_exec.js`) next to your HTML.

## Where to go deeper

- Components, templates, events → README "Template Syntax", "Full Example";
  plugin skill `wings-component`.
- i18n (gen_i18n, flex/plurals/gender, formatting, SetLang) → README
  "Internationalization"; skill `wings-i18n`.
- Theming (`--wings-*` tokens, skins) → README "Skins"; skill `wings-skins`.
- Built-in widgets (button, input, rich-text editor, tabs, dialog, combobox,
  navbar, skin picker) → README "Built-in Widgets"; skill `wings-widgets` —
  prefer these over hand-rolling.
- Build/dev container → README "Quick Start", "Dev Container"; skill `wings-build`.
- Security when writing wings apps → app-flavored `sec-*` skills:
  `sec-wasm-go` (the js.Value boundary, panic = whole-app death, no secrets in
  wasm), `sec-hostile-input` (validate your fetch/form/URL input),
  `sec-fail-operational` (degrade, don't blank the page; bound every loop),
  `sec-supply-chain` (deps, SRI, catalog signing, `FROM wings`),
  `sec-minimal-trusted-code`, `sec-fuzzing`.
