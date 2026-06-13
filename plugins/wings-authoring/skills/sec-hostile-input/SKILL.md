---
name: sec-hostile-input
description: Input validation and parsing discipline for WINGS apps (djb "don't parse" + Margaret Hamilton's defensive design) — applied to YOUR boundaries — fetch/API responses, form fields, URL hash/query, postMessage, localStorage, and js.Values. Use when writing or changing any code in your app that reads data from outside the wasm module.
---

# Hostile input & parsing discipline, for your app

WINGS validates *its* boundaries for you — catalogs are checked and (optionally)
signature-verified at load, flex blocks and templates are parsed at build time,
the build lints your binding names. This skill is about the boundaries WINGS
can't see: the data your own app pulls in at runtime.

Two converging sources of the discipline:

- **djb:** parsing is where security bugs concentrate. Avoid parsing; when you
  must, parse *once*, at the boundary, into a typed Go value, and let the rest
  of the code consume only that typed value. Prefer formats too simple to
  misparse.
- **Margaret Hamilton (Apollo):** "the operator is trained and won't make
  mistakes" is not a defense — her dismissed prelaunch-bug report was later
  triggered for real on Apollo 8. Validate *everyone's* input: your end users,
  the URL, your own backend's responses.

## Rules

1. **Parse at the trust boundary, never again.** Each input crosses exactly one
   validator that yields a typed Go value; downstream functions take the typed
   value, not the raw string or `js.Value`. If a function deep in your call
   tree does `strings.Split` on something that arrived from a fetch, the
   boundary leaked.
2. **Reject, don't repair.** Malformed input → a typed `error`, and stop.
   Silently "fixing up" input creates two parsers — the real one and the one in
   the attacker's head. Your public helpers return `error`; the caller (you, at
   a higher level) decides whether that's fatal.
3. **Prefer dumb formats.** Decode a fetch body with `json.NewDecoder` into a
   *concrete struct* (call `DisallowUnknownFields()` when the schema is closed),
   not into `map[string]any` you then spelunk. If you design a wire format,
   make it a fixed shape, not a mini-language.
4. **Validate the *meaning*, not just the shape.** A string that parses as a
   URL still needs an allow-list before you `fetch` it or put it in a link. A
   numeric index from outside is bounds-checked before it indexes a slice. A
   language tag is matched against your known locales before you pass it to
   `wi18n.SetLang` — never interpolate a raw external string into a path or URL.
5. **User input is input — and so is the browser.** Form fields, `<input>`
   values, URL hash/query, `postMessage` payloads, `localStorage`,
   server responses: all untrusted. Check `js.Value.Type()` before reading.
   Failing silently is the worst outcome — surface the rejection.

## Your app's trust boundaries (audit points)

| Boundary | What you do |
|---|---|
| `fetch`/XHR response from your backend | check status; `json.Decoder` into a concrete struct; never trust the body's shape |
| Form / `<input>` / two-way-bound values | validate type & range before `obj.This.Set` or before sending upstream |
| URL hash (`{{#}}`) / query / `location` | allow-list values; never feed raw into `innerHTML` (see `sec-wasm-go`) or a fetch URL |
| `postMessage` / cross-window events | origin check first, then type-check the `js.Value` |
| `localStorage` / `sessionStorage` | user-editable — re-validate on read, don't trust your own last write |
| language/skin choices from the user | match against your registered locales/skins before applying |

What you do **not** need to re-validate: catalog JSON and `.sig`
(`wi18n` verifies, fail-closed once you set a public key), flex blocks and
templates (parsed and linted at build time). Don't add runtime "tolerance" on
top of those — extend the build-time check instead.

## Anti-patterns

- Regex "validation" that extracts a value but never *rejects* a bad one.
- Re-parsing the same string in two places with two slightly different grammars.
- `js.Value.String()` straight into `innerHTML` (see `sec-wasm-go`).
- A helper that takes `interface{}`/`js.Value` and "figures out" what it got —
  that's a parser in disguise; type it at the boundary (see
  `sec-minimal-trusted-code`).

Sibling skills: `sec-wasm-go` (the js.Value boundary), `sec-minimal-trusted-code`
(don't build runtime parsers), `sec-fuzzing` (if you do add a parser).
