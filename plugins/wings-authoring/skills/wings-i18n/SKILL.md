---
name: wings-i18n
description: Internationalize a WINGS app — author plain text that the gen_i18n build step extracts into per-language catalogs, runtime lookup via wi18n, translatable attributes, opting out, plurals/gender flexion ({{@g %n ~word}}), locale-aware formatting ({{%price}}), and runtime locale switching (SetLang). Use when adding translatable text, plurals/gender, number/currency/date formatting, or a language switcher.
---

# Internationalization

The model: **author natural text** in templates. A build step (`gen_i18n`)
rewrites templates, replacing each translatable string with a stable numeric
index and emitting a per-language JSON catalog. At runtime the `wi18n` package
loads the catalog and swaps each index back to the translated string. You never
write index numbers by hand.

## Enable it

- Blank-import the runtime: `_ "github.com/luisfurquim/wings/wi18n"` in `main.go`.
  It auto-detects the browser language, loads `i18n/<lang>.json`, and makes
  `wings.Main()` wait for the catalog before rendering.
- Generate catalogs at build time. The simplest path is the build orchestrator
  (`go run github.com/luisfurquim/wings/cmd/build@latest dev` with i18n options,
  or the repo's `gen_i18n`); it produces `*.i18n.html` + `i18n/<deflang>.json`.
- Ship `i18n/<lang>.json` (one per language) in your web root.

## What gets translated

- **Text nodes**: `<h2>Dashboard</h2>` → extracted.
- **Translatable attributes** only: `title`, `placeholder`, `alt`, `aria-label`,
  `data-i18n` (`wings.TranslatableAttrs`). Other attribute text is left alone.
- **NOT** dynamic text: `{{expression}}` output passes through untranslated — so
  translate the template literals, not the data. If you need a data value
  localized, format it (see formatting below) or translate it in your data.

Opt a subtree out with the standard HTML `translate="no"` (inherited by
children) — useful for code samples, proper nouns, or literal index demos.

For CSS-generated text use `data-i18n="..."` + `content: attr(data-i18n)` so it
flows through the same pipeline (don't hardcode user-visible text in CSS).

## Plurals & gender (flexion)

A single translated string can't express a target language's plural/gender
agreement. Use inline **flex sigils** inside a `{{...}}` block (see the syntax
table in `wings-component`):

- `@var` — gender axis (the value selects a gender row).
- `%var` — count axis (emitted at its position; drives the CLDR plural category).
- `~word` — a stem the translator will inflect.
- `#N` — a rule index **auto-assigned by gen_i18n**; never write it yourself.

```html
<p>{{@gender %qt ~aluno ~aprovado}}</p>
```
gen_i18n rewrites this to a `#N` rule and emits an `i18n/<lang>.inflections.json`
(gender × CLDR-category cells) for translators. For programmable inflection
engines see README "Programmable Flex (CustomFlex)".

## Locale-aware formatting

A **lone** `{{%var}}` (no `~`/`@`) formats a value for the locale — type-directed:
ints/floats, `time.Time`, `wi18n.Currency`, or any type implementing
`wi18n.Numerical`. Named formats: `{{%var:formatName}}` keyed by `<lang>.fmt.json`.

```html
<span>{{%price}}</span>   <!-- formatted "R$ 1.234,50" / "$1,234.50" by locale -->
```
`wi18n.Currency` is money as an **integer in the currency's minor unit** (avoids
float rounding), not a float:
```go
type Currency struct {
	Amount int64  // minor units: centavos/cents (123450 == 1234.50)
	Code   string // ISO 4217, e.g. "BRL", "USD", "JPY"
}
// e.g. "price": wi18n.Currency{Amount: 123450, Code: "BRL"}
```
Physical units (length, temperature, speed, …) have their own measure packages;
see README "Physical Measure Packages".

## Runtime language switching

Switch language live with `wi18n.SetLang`:
```go
wi18n.SetLang("es-AR", func(err error) {
	if err != nil { /* handle: revert UI, show dialog */ }
})
```
- It re-fetches the catalog (cached after first load) and re-binds the DOM.
- **Run it off the render goroutine** (e.g. `go wi18n.SetLang(...)`) — calling it
  synchronously from an event handler that the runtime is also driving can
  deadlock.
- The error callback is where you branch on failure. With catalog signing on
  (`wi18n.SetCatalogPublicKey`, ed25519), a tampered/unsigned catalog yields a
  typed `*wi18n.CatalogSignatureError` (`errors.As`) — the bundle is refused
  (fail-closed). A language-switcher component should branch on it: revert the
  `<select>` to the current locale and surface the error (e.g. open a `w-dialog`).

## Notes

- `wi18n.Lang()` returns the active tag.
- Empty `content` in a catalog renders the raw index — a deliberate
  missing-translation signal.
- Deeper: README "Internationalization", "Flexion (SynPrinter)", "FmtPrinter",
  "Runtime Locale Switching". Translators use the `helpers/wlate` GUI.
