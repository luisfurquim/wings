# Changelog

All notable changes to WINGS are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims to
follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html) (pre-1.0: minor
bumps may carry breaking changes).

This is a curated history — release highlights, not every patch. For the full
per-commit record see the git log and tags.

## [0.15.3] — 2026-06-01

### Fixed
- Literal punctuation in a programmable flex source block (e.g. a `:`) no longer
  aborts the block's build-time rewrite. `ParseFlexBlock` now treats any
  non-control token as literal content (matching `TokenizeFlexContent`), so a
  mistyped sigil degrades to visible text instead of silently leaving the block
  unprocessed.

## [0.15.2] — 2026-06-01

### Added
- **Arbitrary contextual selection.** A `$var` in a flex block is now passed to the
  CustomFlex engine as a `FlexSelector` (while still being emitted, like `%count`),
  so an engine can branch on any key — platform, formality, tenant — and inflect in
  the same block. This is WINGS's analog of Fluent's arbitrary selectors. The live
  demo adds a `PlatformHelper` engine that selects from an embedded JSON table.

## [0.15.1] — 2026-06-01

### Documentation
- Added a "built-in vs custom" worked example to the Programmable Flex section.

## [0.15.0] — 2026-06-01

### Added
- **Programmable flex (CustomFlex).** Inflection is no longer limited to the
  built-in gender/count selectors and the dictionary path: an app can now supply
  its own inflection engine. New core contract `CustomFlex` / `Prioritized` /
  `FlexSelector`; new flex sigils in the `{{ … }}` block grammar — `*var` (a
  CustomFlex engine, elected by `Priority()`), `$var` (dynamic value emitted
  verbatim), and `~$var` (a dynamic value to be inflected). The build-time
  pipeline splits each block into locale-invariant **control** metadata and a
  per-locale **content** template, so translators reorder the sentence per
  language while the engine wiring stays fixed.
- **Catalog signature verification.** Optional, opt-in ed25519 signing of i18n
  catalogs. `gen_i18n -genkey` creates a keypair (private key encrypted with
  Argon2id + AES-256-GCM); `gen_i18n -sign-key` writes a `<lang>.json.sig`
  sidecar per catalog; the app calls `wi18n.SetCatalogPublicKey()` to require a
  valid signature on every catalog it loads. With no key configured the behaviour
  is unchanged (no `.sig` is fetched). Covers the main `<lang>.json` catalog.
- The live demo now dogfoods both features: a `RemoteFlexer` engine that fetches
  inflected forms from static JSON, and signed catalogs verified at runtime.

### Fixed
- **Subresource Integrity (SRI) injection in the build scripts.** Each `build.sh`
  (repo root, `live-demo/`, `helpers/wlate/`) now computes the `integrity` hash
  from the files it actually serves, and the injection is idempotent (re-running
  a build no longer duplicates `integrity`/`crossorigin` attributes). `wlate`
  gained SRI it never had.

## [0.14.0] – [0.14.3] — 2026-05 — "WINGS"

### Changed
- **Renamed `wprana` → WINGS** across the module path, package, repository, cache
  directory, log prefixes, and documentation. See
  [Migrating from `wprana`](README.md#migrating-from-wprana). (`v0.14.0`–`v0.14.2`)

### Added
- **Internationalization (i18n) suite.** Build-time text extraction + runtime
  catalog lookup; plural/gender **flexion** via `{{ … }}` blocks; a 21-language
  dictionary-backed auto-flexion pass (`gen_i18n -auto-flex`); optional
  LLM/LibreTranslate auto-translation (`-auto-translate`); runtime locale switch
  via `wi18n.SetLang()`; CSS `content` i18n through `data-i18n` + `attr()`.
- **Locale-aware formatting & measures.** `FmtPrinter` with a `Numerical`
  interface and `Currency` type; named formats; nine measure packages with
  `New(value, unit)` constructors and per-locale overrides.
- **Skins — theming with `--wings-*` tokens.** `RegisterSkin` / `ApplySkin` and a
  global design-token system; 18 composable skins decomposed across five
  orthogonal categories (Identity, Geometry/Spacing, Depth, Motion, Atmosphere);
  `<skin-switcher>` widget. (`v0.14.3`) **User-defined skin categories**: the high
  16 category bits (48–63) are reserved for application use via `UserCategory(n)`
  / `RegisterCategoryName`.
- **Widgets.** `w-tabs` / `w-tabbutton` / `w-tab` (panel / menu / detached modes),
  `w-navbar`, `w-dialog`, and `w-combobox` (`mode="single|multi"`).
- **wlate** — a browser-based translation editor GUI for the catalogs.
- Catalog/console logging routed through the `goose` logger, configurable from
  `wings.json`.

## [0.13.0] – [0.13.12] — 2026-04

### Added
- String conditionals: equality/inequality and prefix / suffix / contains tests.
- The interactive [live demo](https://luisfurquim.github.io/wings/).

## [0.9.0] – [0.12.2] — 2026-03–04

### Added
- `combobox` widget and the `Customizable` module interface (`v0.9.0`).
- URL hash-fragment routing support (`{{#}}` binding, `wings.GoTo()`) (`v0.12.0`).
- Console `error` / `warn` / `info` logging support (`v0.11.0`).
- Falsy boolean conditionals.

### Changed
- All source comments and runtime messages translated to English.

## [0.1.0] – [0.8.4] — 2026-03

### Added
- Helper subpackages: `AddEvent` / `RmEvent` / `Query`, `GetLocation` /
  `GetTopLocation`, `localStorage`, `postMessage` wrapper, and OPFS access.
- The trigger mechanism for child-to-parent communication (replacing the earlier
  buggy `syncUp`).

### Fixed
- Numerous early reactive-core synchronization, circular-propagation, and
  reference bugs (notably around `**` double-star iteration).

## [0.0.1] — 2026-03-13

- Initial release: reactive Web Components in pure Go, compiled to WebAssembly.

[0.15.3]: https://github.com/luisfurquim/wings/compare/v0.15.2...v0.15.3
[0.15.2]: https://github.com/luisfurquim/wings/compare/v0.15.1...v0.15.2
[0.15.1]: https://github.com/luisfurquim/wings/compare/v0.15.0...v0.15.1
[0.15.0]: https://github.com/luisfurquim/wings/compare/v0.14.3...v0.15.0
