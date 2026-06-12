---
name: sec-hostile-input
description: Input validation and parsing discipline (djb "don't parse" + Margaret Hamilton's defensive design) for wings. Use when writing or changing any code that reads catalogs, templates, flex blocks, URLs, env vars, fetch responses, js.Values, or webdev-supplied attributes.
---

# Hostile Input & Parsing Discipline

Two converging sources:

- **djb:** parsing is where security bugs concentrate. Avoid parsing; when
  unavoidable, parse *once*, at the boundary, into a typed structure, and let
  interior code consume only the typed result. Prefer formats too simple to
  misparse.
- **Margaret Hamilton (Apollo):** "the operator is trained and won't make
  mistakes" is not a defense. Her prelaunch-program bug report was dismissed
  on exactly that argument — then Jim Lovell triggered it on Apollo 8 (P01
  wipe). Validate *everyone's* input: end users, webdevs, and our own build
  artifacts.

## Rules

1. **Parse at the trust boundary, never again.** Each input crosses exactly
   one validator that produces a typed Go value. Downstream functions take
   the typed value, not the raw string. If a function deep in the call tree
   does `strings.Split` on data that arrived from outside, the boundary
   leaked.
2. **Reject, don't repair.** Malformed input → typed `error` and stop
   (`feedback_errors_over_panic`: public API returns error; the app dev
   decides whether to panic). Fixing up input silently creates two parsers:
   the real one and the one in the attacker's head. Documented exception:
   `expr.ParseFlexBlock` leniency deliberately falls back to *literal
   passthrough* (render-as-text), which is a fail-closed default, not repair.
3. **Prefer dumb formats.** JSON via `encoding/json` into a concrete struct
   (`DisallowUnknownFields` when the schema is closed) beats any hand-rolled
   format. If you design a format, make it a fixed shape, not a language.
4. **Validate the *meaning*, not just the shape.** BCP-47 tags go through the
   guard (don't interpolate raw lang strings into URLs/paths); indices into
   catalogs are bounds-checked; file paths from config are cleaned and
   confined to the expected root (the dev server's dotfile/key blocking in
   `cmd/build` is the model).
5. **Webdev input is input.** Template attributes, binding names, wings.json,
   `WINGS_*` env vars. Silent failure is the worst outcome — the camelCase
   binding lint in `cmd/build/lint.go` exists because the HTML parser
   lowercases attribute names and the mismatch failed *silently*. New
   template-facing features must either work or fail the build with
   file:line; never render wrong quietly.

## wings trust boundaries (audit points)

| Boundary | Validator |
|---|---|
| Catalog JSON + `.sig` fetched at runtime | `wi18n/verify.go` (ed25519, fail-closed when key set) |
| Flex blocks in source HTML | `expr.ParseFlexBlock` / `TokenizeFlexContent` (build time) |
| DOM / js.Value from browser | type-check + `ValidateCell`-style checks before use |
| webdev templates | `cmd/build` lint (extend it; don't add runtime "tolerance") |
| env/config (`WINGS_*`, gen_i18n.json) | parse to struct at startup, fail fast |

## Anti-patterns

- Regex "validation" that extracts but doesn't reject.
- Re-parsing a string in two places with two grammars.
- `js.Value.String()` straight into `innerHTML` (see sec-wasm-go).
- Accepting both old and new syntax "for compatibility" without a lint that
  flags the old one.
