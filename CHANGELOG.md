# Changelog

All notable changes to WINGS are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims to
follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html) (pre-1.0: minor
bumps may carry breaking changes).

This is a curated history — release highlights, not every patch. For the full
per-commit record see the git log and tags.

## [0.17.0] — 2026-07-06

### Added
- **`w-text` rich-text editor widget** — a pluggable `contenteditable` editor
  in the `w-input` family (label/field/feedback anatomy, sizes, form
  participation). It ships a **`basic` profile**: a bold/italic/code toggle
  toolbar plus a block-style picker (`p`, `h1`–`h6`, `blockquote`, `pre`),
  rendered with WINGS's own `w-button`/`w-combobox` (so an app must also
  import those). Content is stored and submitted as EPUB-flavored HTML; bind
  `&value` to a `wings.FieldCodec` (e.g. `field.NewText()`) and it seeds the
  editor and reads back on blur. Native form lifecycle: `form.reset()` restores
  the mount-time content **and** clears the undo history, `<fieldset disabled>`
  makes the surface read-only. Toolbar labels are translatable
  (`<span slot="labels">` + built-in English fallback) and re-translate live
  on `SetLang`. Toggling a mark at a collapsed caret arms it as **pending**
  (Word behaviour): the next typing comes out marked — or escapes the mark
  when toggling off inside one — and moving the caret first disarms it.
- **`wtext` package — the editor engine and plugin API.** Split portable /
  js-wasm like the rest of WINGS: the plugin surface (`EditorCore`,
  `EditionPlugin`/`ClipboardPlugin`/`ToolbarPlugin`, the safe-by-construction
  `Fragment` builder, the sealed `Mark` constructors, exported toolbar helpers)
  compiles and unit-tests under the native toolchain, so a plugin author can
  test against a fake core without a browser; the js side is the mechanism
  (filtering walker, event spine, undo). Selections address the DOM through
  **opaque per-turn handles** (never a live node), so a plugin can neither hold
  nor leak a reference outside the editable root. Undo/redo is the core's own,
  DOM-level and bounded (100 steps / ~2 MB); `Txn` groups writes into one step.
  `RegisterProfile(name, Profile{…})` makes a profile selectable from a template
  via `profile="…"`.
- **`epubhtml` package — the portable content-security policy** (the "EPUB
  Content Document, no-script" profile). Pure Go (no `syscall/js`), so it is
  unit-tested and **fuzzed** natively: element/attribute allowlists,
  URL canonicalization (`http`/`https`/`mailto`/`#frag` only, userinfo
  rejected, IDN→punycode so homograph spoofs become visible, mailto rebuilt
  from validated parts to kill header injection, optional app `LinkPolicy`),
  invisible/bidi-control stripping (Trojan-Source aware; keeps ZWJ/ZWNJ), and a
  CSS declaration sanitizer for named classes. The editor sanitizes by
  **allowlist-copy through the browser's own `DOMParser`** — never a second Go
  HTML parse — so there is no parser differential for mutation-XSS to exploit,
  and pasted/loaded/dropped markup all take that one road. Fuzzing found three
  real IDN encode/decode asymmetries before ship.
- **`wings.OnDisconnect(tag, hook)`** — a per-instance teardown hook (mirroring
  `OnRetranslate`/`OnFormReset`), run when an instance leaves the DOM, for
  resources that reach outside the element's subtree and so escape the built-in
  `dom.AddEvent`/`dom.Observe` auto-release (a document-level listener, a
  hand-built `MutationObserver`). `w-text` uses it to detach its editor.

## [0.16.16] — 2026-07-04

### Added
- **`w-input` is a complete form citizen** — it now handles the form-associated
  lifecycle callbacks. `form.reset()` restores each field to its mount-time
  default value and clears validation; an ancestor `<fieldset disabled>`
  disables the field (the browser fires `formDisabledCallback` without touching
  the `disabled` attribute, so the widget reflects it to the inner input and a
  `:host([data-form-disabled])` hook).
- **`w-button type="reset"`** — a reset button inside a `<form>` now calls
  `form.reset()` (previously it only handled submit).
- **`wings.OnFormReset` / `wings.OnFormDisabled`** — per-tag hooks for the form
  lifecycle callbacks (mirroring `OnRetranslate`), wired through
  `prana_helper.js`. Any form-associated widget can opt in.

## [0.16.15] — 2026-07-03

### Added
- **`w-button` widget** — semantic variants (`secondary`/`primary`/`ghost`/
  `danger`/`success`), three sizes and shapes, loading spinner, prefix/suffix
  slots, full `::part()` surface, and host state attributes for pure-CSS
  theming.
- **`w-input` widget** — label/field/feedback anatomy with `::part()` on every
  sub-element, helper/error zones, clearable mode, character count, sizes, and
  host state hooks (`data-focused`, `data-invalid`, …) so a floating label or a
  danger border needs no Go changes.
- **Typed two-way binding** — `&value` now also works on custom elements, and
  a bound map entry implementing `wings.FieldCodec` (`fmt.Stringer` +
  `FromString`) keeps its Go type across the round-trip instead of decaying to
  a string. Add `wings.Validator` and every write-back validates, returning a
  message **id** that `w-input` resolves against translated
  `<span slot="errors" id="…">` nodes (or a document-level message table) and
  wires to the native constraint-validation API — `form.checkValidity()` and
  `:invalid` work for free, and messages re-resolve on `SetLang`.
- **`wings/field` package** — ready-made codecs/validators (`NewText`,
  `NewEmail`, `NewInt`, `NewPattern`); pure Go, unit-tested with the native
  toolchain. Empty values are valid by design — use `required` for that.
- **Material skins (`outlined`, `filled`, `underlined`)** — a new
  `CategoryMaterial` skin category governs the structural surface form of
  field widgets globally; `glasslighting` (Lighting) and the `glassmorphism`
  bundle (= `glass` + `glasslighting`) round out the set at 23 built-in skins.
- **`wings.OnRetranslate(tag, hook)`** — widgets can register a per-instance
  hook that runs after a `SetLang` re-render (used by `w-input` to refresh
  validation messages).
- **Native form participation (ElementInternals)** —
  `ComponentOpts.FormAssociated` defines a custom element as form-associated
  (`prana_helper.js` attaches internals, exposed as `_internals`). `w-input`
  mirrors the inner input's ValidityState into the internals, so inside a
  native `<form>` an invalid field (native constraints or a bound `Validator`)
  blocks `form.checkValidity()`/submission, and a `name` attribute puts the
  value in the submitted data. `w-button` gained the native-like `type`
  behavior: inside a form a click submits via `requestSubmit` (validation runs
  first); `type="button"` opts out.
- **`@input` event on `w-input`** — fires per keystroke; `@change` now follows
  native semantics and fires on blur, after the two-way write-back.
- **`dom.Observe` / `RmObserver` / `RmObserversUnder`** — registered
  MutationObservers, auto-released on component disconnect (mirrors the
  `dom.AddEvent` lifecycle). The built-in widgets migrated to it.
- **a11y in `w-input`** — `aria-invalid` on the inner input, `aria-describedby`
  wiring to helper/error, `role="alert"` on the error message, and `aria-label`
  reflected from the host to the real control.

### Changed
- **`label`, `helper`, `error` are translatable attributes by default** — the
  human-text attributes of `w-input` join the default `TranslatableAttrs` /
  `gen_i18n` set (alongside `title`/`placeholder`/`alt`/`aria-label`), so a
  `<w-input label="…" helper="…">` localizes without per-app wiring. Opt a
  value out with `translate="no"`, or edit the list via
  `AddTranslatableAttrs`/`RemoveTranslatableAttrs` and the `gen_i18n`
  `-attrs`/`-add-attrs`/`-no-attrs` flags.
- **`field.NewInt(min, max, notIntID, rangeID)`** — distinct message ids for
  "not a number" and "out of range".
- **wlate dogfoods the new widgets** — its raw `<button>`/`<input>` elements
  became `w-button`/`w-input` (tab chips, Approve/Save, copy button, inflection
  grid cells), styled via `::part()`.

### Fixed
- **`w-input` typing bug** — typing triggered a sync with the stale reactive
  value, overwriting the input mid-keystroke; the widget now writes the map
  value directly before any derived update.
- **MutationObserver leak** — observers built by hand in `w-input`/`w-button`/
  `w-tabs` pinned their Go callbacks forever (same class as the 0.16.14
  `dom.AddEvent` leak); all use `dom.Observe` now, freed on disconnect.
- **Validation id CSS-escaped** — a `Validator` id containing a quote/bracket
  made `querySelector` throw, panicking the whole WASM app; ids are now passed
  through `CSS.escape`.
- **Two-way binding hardened** — reads of a missing/non-string `value` property
  on a bound custom element no longer corrupt the map with placeholder strings;
  the change listener uses `addEventListener` (an author assigning `onchange`
  can't clobber it) and is detached before release, so a late event cannot hit
  a released callback.

### Removed
- **`prana2.js`** — the original JS prototype; the Go implementation in
  `refs.go` supersedes it entirely.

## [0.16.14] — 2026-06-15

### Added
- **App-flavored security skills in the `wings-authoring` plugin** —
  `sec-wasm-go`, `sec-hostile-input`, `sec-fail-operational`,
  `sec-supply-chain`, `sec-minimal-trusted-code`, and `sec-fuzzing`, re-aiming
  WINGS's own hardening practices at the apps you build with it. The
  `wings-component` skill also gained worked examples for state-driven views,
  reading native event values safely, loading data from a backend, and derived
  (filtered/sorted) lists.
- **Cross-tool wrappers to `AGENTS.md`** — thin Cursor (`.cursor/rules/`), Kiro
  (`.kiro/steering/`), and GitHub Copilot (`.github/copilot-instructions.md`)
  config files re-point to the canonical `AGENTS.md`, so non-Claude assistants
  pick up the guide through their native mechanism (no content is duplicated).

### Fixed
- **Native listeners are released on disconnect.** A listener registered with
  `dom.AddEvent` is now freed when its component leaves the DOM (new
  `dom.RmEventsUnder`, called from the disconnect path); previously it leaked
  across create/destroy cycles. `RmEvent` is idempotent, so calling it manually
  as well is still safe. Surfaced by validating the new `sec-wasm-go` skill
  against a generated component — the kind of bug the security work is meant to
  catch.

## [0.16.12] — 2026-06-13

### Added
- **Fuzz targets for the parsers** plus a `fuzz` build target.
  `go run ./cmd/build fuzz` runs every `Fuzz*` for a fixed budget each, covering
  the `expr` flex/reference readers, `codec`, the `gen_i18n` catalog aligner,
  catalog signature verification, the `<lang>.fmt.json` parser, and decimal
  formatting. Like `vulncheck`, it is not part of `all`.
- **On-demand watch mode in `dev`** (`WINGS_WATCH_MODE=on-demand`): source
  changes are only logged; the rebuild runs when you `touch REBUILD` at the app
  root — handy when an edit spans several files. Default `auto` keeps the
  rebuild-on-every-save behavior.
- **Typed `wi18n.CatalogSignatureError`** so `SetLang` error callbacks can
  branch on tampering (via `errors.As`) vs ordinary load failures. The
  live-demo `locale-switcher` now reverts its `<select>` on failure and fires
  an `@error` trigger; the demo shell opens a `w-dialog` with a
  signature-specific message.

### Security
- **Signature enforcement extended to secondary catalogs**: with a public key
  configured, `<lang>.inflections.json` and `<lang>.fmt.json` that load must
  also carry a valid `.sig` (absent optional files remain fine). `gen_i18n
  -sign-key` now signs them alongside the main catalogs.

### Changed
- **`verifyCatalog` and the fmt-config / decimal parsers dropped their
  `js && wasm` build tag** so they compile and fuzz natively; browser-only
  logic (the root runtime, `Intl` formatting) stays tagged.

### Fixed
- **`formatDecimal` mis-rendered the minimum `int64` amount** — negating it
  overflowed and leaked a stray double minus. Found by the new fuzzing.

## [0.16.11] — 2026-06-12

### Added
- **`vulncheck` build target.** `go run ./cmd/build vulncheck` runs govulncheck
  over every module in the repo — a native call-graph pass plus a `GOOS=js`
  package-level pass, since js-only packages are excluded from the native one
  by build constraints. Any finding fails the run. Part of the release
  checklist; not part of `all` (it needs network access for the vulnerability DB).

### Security
- **`golang.org/x/net` upgraded v0.53.0 → v0.55.0** (with sibling `x/*` bumps),
  clearing the five x/net vulnerabilities the first `vulncheck` run reported.
- **`toolchain go1.26.4` pinned in go.mod**, clearing the eight Go standard
  library vulnerabilities the same run reported (fixed in go1.26.2–1.26.4).
  Builds on older local toolchains transparently fetch 1.26.4.

### Changed
- **Six secure-coding skills versioned** (`.claude/skills/sec-*`): minimal
  trusted code, hostile input, fail-operational, supply chain, Go-on-WASM, and
  fuzzing — the practices the codebase follows, written down for AI-assisted
  sessions.
- **Local scratch dir renamed `work/` → `_work/`** so Go tooling (`./...`)
  skips it natively; it was failing `go test ./...` and showing up in scans.

## [0.16.10] — 2026-06-09

### Changed
- **README: the dev container gets its own section** plus a line under
  "Why WINGS?"; "What's New" pruned to the 0.16.x line. Docs-only release.

## [0.16.9] — 2026-06-09

> **First stable dev container.** The Docker rebuild-on-save path — experimental
> since `0.16.0-alpha` — is promoted to stable: serving a full app inside the
> container (build, `gen_i18n`, dictionary baking, and rebuild-on-save) is now
> validated end-to-end on a real app.

### Changed
- **The Docker dev container is no longer experimental.** The `⚠️ Experimental`
  notes are dropped and the version sheds its `-alpha` suffix. There is no code
  change from `0.16.8-alpha` — the watcher fixes from `0.16.6`–`0.16.8` ship here
  as the first stable cut.

## [0.16.8-alpha] — 2026-06-09

> **Pre-release.** Builds on `0.16.7-alpha`; the Docker dev path is still
> experimental (see the `0.16.0-alpha` note below).

### Fixed
- **A quick second edit is no longer dropped.** The watcher's loop guard ignored
  *all* events while a build ran and folded the post-build fingerprint over the
  whole tree, so a save made during the (seconds-long) wasm compile was both
  ignored and absorbed into the fingerprint — the edit silently never compiled.
  Echo detection was reworked: each watched file's content hash is recorded when
  its event arrives (before the rebuild it triggers) and only the build's actual
  outputs are recorded afterward, so a real edit — even one made mid-build, now
  re-examined when the build finishes — always rebuilds, while the build's own
  writes still hash to a known value and are ignored. The 5-second echo window is
  gone; the decision is purely by content.

### Fixed
- **The dev watcher's echo detection now covers all build outputs.** `0.16.6`
  fingerprinted only the gen_i18n catalogs, but a build also writes `*.i18n.html`
  template outputs next to their sources — those slipped through as "real edits"
  and kept the loop alive. The fingerprint now hashes the whole watched tree, so
  every file the build writes back is recognized as its own echo.

## [0.16.6-alpha] — 2026-06-09

> **Pre-release.** Builds on `0.16.5-alpha`; the Docker dev path is still
> experimental (see the `0.16.0-alpha` note below).

### Fixed
- **The dev watcher no longer rebuilds in an endless loop.** `gen_i18n` rewrites
  catalogs (and auto-flex writes `*.inflections.json`) into the watched i18n
  source tree, so the build's own output kept re-triggering it. The watcher now
  ignores events during a build and, for a few seconds after, tells the build's
  own writes apart from real edits by content hash: an event whose file still
  matches what the build wrote is the echo and is dropped, while a genuine edit
  (different content) rebuilds at once — even within that window.

## [0.16.5-alpha] — 2026-06-09

> **Pre-release.** Builds on `0.16.4-alpha`; the Docker dev path is still
> experimental (see the `0.16.0-alpha` note below).

### Fixed
- **`go.work` is no longer published in the module zip.** The repo's local-dev
  workspace file was tracked, so the Go proxy packed it into the wings module
  zip. A downstream `go get` then landed a `go.work` in the module cache whose
  `./example`, `./helpers/wlate`, `./live-demo` `use` entries point at
  sub-modules excluded from the zip — so compiling `gen_i18n` from the cached
  wings tree failed with "open example/go.mod: no such file or directory". The
  file is now `.gitignore`d: it stays on disk for local development, only out of
  what is published.
- **The dev loop builds `gen_i18n` with `GOWORK=off`.** Belt-and-suspenders for
  the above: it resolves `gen_i18n`'s deps against wings's own `go.sum` (the
  reason it builds from the module dir) and stays immune to any `go.work` in
  scope, including the ones already shipped in `0.16.3-alpha`/`0.16.4-alpha`.

### Retracted
- **`v0.16.3-alpha` and `v0.16.4-alpha`** — both shipped the stray `go.work` and
  break a from-cache `gen_i18n` build. `go` will skip them; pin `0.16.5-alpha`.

## [0.16.4-alpha] — 2026-06-08

> **Pre-release.** Builds on `0.16.3-alpha`; the Docker dev path is still
> experimental (see the `0.16.0-alpha` note below).

### Added
- **The dev container aligns the app's `go.mod` to `WINGS_VERSION`.** `gen_i18n`
  is compiled from the wings tree the app pins, so it must match the baked
  `wings-dev`/`dictbuild` binaries (installed from `WINGS_VERSION`). The loop now
  bumps the app's `go.mod` up to `WINGS_VERSION` when it is older (a `go get`,
  logged); a `go.mod` pinning a *newer* wings wins and only warns. Setting
  `WINGS_VERSION` is the webdev's opt-in — leave it at your `go.mod` version to
  change nothing.

### Fixed
- **Dev mode no longer fails on a fresh module cache.** A clean dev container
  (empty module-cache volume) left `go list -m -f {{.Dir}}` returning an empty
  wings dir, so the first build ran with no `go.mod` in scope ("go.mod file not
  found"). The loop now downloads the module before resolving its directory.

## [0.16.3-alpha] — 2026-06-07

> **Pre-release.** Builds on `0.16.2-alpha`; the Docker dev path is still
> experimental (see the `0.16.0-alpha` note below).

### Added
- **Region locales auto-fall-back to their base language for `-auto-flex`.** A
  catalog locale such as `en-US` or `es-AR` no longer needs its own dictionary:
  `dictbuild` bakes it from the base `en`/`es` source and writes the honest
  `en.db`/`es.db` (never a misleading `en-US.db`), and `gen_i18n` mirrors the
  same `en-US`→`en` fallback when it loads the dictionary. Portuguese keeps its
  localised `pt-BR`/`pt-PT` dictionaries, which are not interchangeable. The new
  `gen_i18n -dict-strict` flag (and `WINGS_DICT_STRICT` in the dev container)
  refuses the fallback, leaving a locale empty unless its exact `.db` exists.

### Fixed
- **`dictbuild -lang en-US` no longer aborts with "not in the auto-fetch
  table".** Region/script variants without a dedicated dictionary now resolve to
  their base language instead of failing, so baking the dictionaries for a
  region-tagged catalog (`pt-BR,en-US,es-AR`) works in one shot.

## [0.16.2-alpha] — 2026-06-07

> **Pre-release.** Builds on `0.16.1-alpha`; the Docker dev path is still
> experimental (see the `0.16.0-alpha` note below).

### Added
- **Dev mode serves i18n apps end-to-end, including modules in a sub-directory.**
  When `WINGS_DEFLANG` is set, the dev loop now publishes the freshly generated
  (and signed) catalogs into the webroot's `i18n/` on every rebuild, so a
  translated app works in the container with no extra steps. `WINGS_MAIN` now
  means the **module directory** (the one with `go.mod`); all `go` commands run
  there, so an app root that merely holds the module in a sub-dir and the webroot
  in a sibling — exactly the bundled live-demo (`live-demo/` + `docs/`) — works by
  copying folders. A new `WINGS_I18N_PATH` (relative to the module, default `.`)
  lets `gen_i18n`'s scan dir differ from the build target (the live-demo scans
  `mod` while building the module root). `dev/docker/` documents a copy-only
  recipe for running the live-demo in the container with only `.env` flags.

### Changed
- **Sub-modules use a `go.work` workspace instead of local `replace`s.** The
  `live-demo`, `example`, and `wlate` modules dropped their
  `replace …/wings => ../` directives (and `example` its local `goose` replace)
  in favour of a repo-root `go.work`. The "use local wings" override now lives in
  one file that does not travel when a module is copied out — so copying, say,
  `live-demo/` to a fresh folder resolves wings from the proxy (`require …/wings
  v0.15.9`) and builds with no `go.mod` edits, which is what the dev container
  needs. Lint in `dev` mode is now scoped to `WINGS_MAIN` rather than the whole
  app root.

### Fixed
- **`.env.example` no longer breaks on inline comments.** Comments are now on
  their own lines: docker compose reads an inline `# …` on an *empty* value as
  the value itself, so `WINGS_DICT_LANGS=  # comma list, e.g. pt-BR,en-US` was
  resolved to that comment string — triggering a (failing) dictionary build for
  an app that set nothing. Also clarified that dictionaries use language tags
  (`en`, `es`), not region tags (`en-US`, `es-AR`).

## [0.16.1-alpha] — 2026-06-07

> **Pre-release.** Fixes on top of `0.16.0-alpha`; the Docker dev path is still
> experimental (see the `0.16.0-alpha` note below).

### Fixed
- **Dev server no longer exposes the source tree.** The embedded server now
  disables directory listings: with `WINGS_WEBROOT` at the app root and no
  `index.html`, it previously served a browsable index of the source — `.env`
  files and signing keys included. Directory listings now return 404, dotfiles
  and `*.key`/`*.pem` are never served by direct path, and the server warns at
  startup when the webroot has no `index.html`.
- **Dev dictionary build uses an older GCC.** The throwaway dictionary stage now
  builds on Debian (GCC 10): recent GCC rejected unitex-core's `int x = NULL;`
  idiom, breaking the Unitex compile.
- **Clearer `dev` error when the wings module is unresolved.** Surfaces the
  underlying `go list` stderr instead of a bare "exit status 1".

## [0.16.0-alpha] — 2026-06-06

> **Pre-release.** The native `cmd/build dev` loop is tested and stable; the
> **Docker dev image is experimental and not yet validated end-to-end.** The
> image installs the dev tool via `go install …/cmd/build@<version>`, which
> requires this release to be published on the module proxy first — a
> chicken-and-egg that only clears once `v0.16.0-alpha` is tagged and pushed
> (then `WINGS_VERSION=v0.16.0-alpha` builds the image). The Docker path will be
> promoted to stable in `v0.16.0` after that round-trip is verified.

### Added
- **Dev container + `cmd/build dev` mode.** A zero-toolchain development loop for
  building your *own* wings app: edit source on the host and the system rebuilds
  `wings.wasm` and serves it on every save. The new `dev` mode in the build
  orchestrator (`go run ./cmd/build dev`, or the published binary) is generic —
  it resolves the wings module your app depends on via `go list -m`, lints your
  templates, optionally runs `gen_i18n` (when `WINGS_DEFLANG` is set), copies the
  JS helpers, compiles the wasm, serves the webroot, and watches the tree
  (debounced) for rebuilds. A failed build never stops the loop. Configuration is
  entirely through `WINGS_*` environment variables. `WINGS_HTTPD` lets you swap
  the built-in static server for your own backend command. File watching uses
  `github.com/fsnotify/fsnotify` (a new, pure-Go dependency).
- **i18n auto-flex and auto-translate in the dev loop.** When `WINGS_DEFLANG` is
  set, `WINGS_AUTO_FLEX` adds the dictionary inflection pass (`-auto-flex
  -dict-dir`) and `WINGS_AUTO_TRANSLATE` adds the machine/LLM pass. Translation
  backends are configured through `WINGS_TR_*` (`backend`, `url`, `model`, `key`,
  `timeout`), which synthesize a `gen_i18n.json` — but only when the app does not
  already ship one (a hand-authored config always wins).
- **Docker dev templates (`dev/docker/`).** Ready-to-copy `Dockerfile`,
  `docker-compose.yml`, and `.env.example` (plus a README): drop the three files
  into your app, `docker compose up`, and develop with your source bind-mounted
  and the module cache persisted across runs. No Go toolchain required on the
  host. The image is multi-stage: a throwaway builder bakes the Unitex
  dictionaries you list in `WINGS_DICT_LANGS` (compiling Unitex from source), so
  the lean final image carries only the resulting `<lang>.db` files.

## [0.15.9] — 2026-06-06

### Added
- **Go build orchestrator (`cmd/build`).** A single cross-platform tool replaces
  the four `build.sh` scripts: `go run ./cmd/build <lib|example|live-demo|wlate|all>`.
  It does in pure Go what the scripts shelled out for — SRI hashing
  (`crypto/sha512`), the wlate `.meta.json` placeholders (`encoding/json`), file
  copies, and the SRI rewrite — so `sed`, `openssl`, and `python3` are no longer
  needed and the build runs natively on Windows. The `build.sh` files are now
  thin wrappers around it (the old command still works).
- **Build-time lint for camelCase binding names.** Each target scans its
  templates and **fails the build** if a binding attribute name (`?cond`, `*arr`,
  `**arr`, `&attr`) contains an uppercase letter, reporting `file:line`. The
  browser lowercases attribute names, so such a binding silently never matches
  its model — previously an invisible render bug.

### Changed
- **Standardized the wasm binary name on `wings.wasm`.** Every demo and the docs
  now build and load `wings.wasm` — previously live-demo (`docs/`) and wlate
  (`dist/`) emitted `main.wasm`, while `example/` shipped a stale `main.wasm` the
  build no longer produced. The example is now self-contained like the others:
  the build copies `prana_helper.js` and `wasm_exec.js` into `example/`, so its
  `index.html` references them locally. Dropped the dead committed binaries
  `example/main.wasm` and `example/wprana.wasm` (the latter a pre-rename
  leftover).

### Fixed
- **Example app `?showExtra` binding.** `example/mywidget` used a camelCase
  conditional name that the browser lowercased, so the "Extra" block never
  rendered. Renamed to `?show_extra` (matching the model key and the README's
  Full Example). This is exactly the bug the new lint now blocks.
- **Live-demo build writes straight to the published `docs/`.** The durable
  follow-up to 0.15.5: `live-demo/build.sh` now emits directly into the
  repository-root `docs/` (the directory GitHub Pages serves) and the duplicate
  `live-demo/docs/` tree is gone. The two copies could no longer drift, so the
  live site can no longer go stale behind the build. `serve.go` serves the same
  unified `docs/` for local preview.

## [0.15.8] — 2026-06-06

### Added
- **wlate "Approve" button.** Marking a translation reviewed without editing it
  was only reachable through an unlabelled border strip (or `Alt+R`) — easy to
  miss, and Save does not change review state. A visible **✓ Approve** button now
  sits beside Save: it flips the current record's `revised` flag and advances to
  the next pending entry. Save stays persist-only.
- **wlate signs catalogs on save.** When the dev server is configured with a
  signing key (`sign_key` / `sign_key_password` in `server.conf`), each save
  re-signs the `<lang>.json` it just wrote, so a signed project's catalogs verify
  immediately — no separate build step to refresh the `.sig`. The signing and
  key-handling code moved out of `cmd/gen_i18n` into a new shared `wsign` package
  used by both the generator and the server. (Inflection catalogs stay unsigned,
  matching `gen_i18n`.)

### Changed
- **`gen_i18n` preserves translations across source edits.** A run now aligns
  the new source order against the committed deflang catalog: a string that only
  moved keeps its translation and `revised` flag (position-independent exact
  match), and an *edited* string is matched to its closest previous version
  (Levenshtein similarity, scoped between surrounding unchanged anchors) so its
  old translation is reused but flagged `revised=false` for re-review. Before,
  any edit — even a typo fix — reset the translation to empty. New strings start
  empty and removed ones are dropped, as before. Re-running on unchanged sources
  is a no-op (idempotent).

### Removed
- **Dead `i18n.db` artifact.** `gen_i18n` no longer writes the gob-encoded trie
  database (`loadDB`/`saveDB`): it was write-only — nothing ever read it back —
  and change detection now runs off the committed `i18n/*.json` catalogs. Any
  stale `i18n.db` on disk is harmless and ignored.

### Fixed
- **Live-demo localization.** The Tabs demo prose was hard-coded in English under
  a blanket `translate="no"`, so it never localized; it is now authored in the
  source language (pt-BR) with full en-US/es-AR catalogs, opting out only the
  inline code snippets. The i18n tab's "catalog by index" table had a stale
  hand-written source column (indices had drifted across versions); it is
  re-synced to the current catalog.

## [0.15.7] — 2026-06-04

### Documentation
- Cite in-web testing (`<w-test>` / `Testable()` / `<w-test-report>`) in the
  "Why WINGS?" table.

## [0.15.6] — 2026-06-04

### Added
- **In-web test harness (`<w-test>`).** A framework widget that wraps any prana
  widget and turns the live demo into a public, self-verifying test page. It
  spies on every event the wrapped subject fires (via the new `@all` channel),
  renders a live event log, and shows a ⏳/✅/❌ seal. With a `check="name"`
  attribute the seal is driven by a Go assertion registered via
  `wings.RegisterCheck` (`CheckFunc` over a `CheckCtx{Subject, Dom, Events}`),
  run on mount, after every captured event, and on a Re-run button; without
  `check=` the test is manual (human-toggled seal) for purely visual checks. The
  host carries `data-wtest-state` so a later headless runner can scrape results.
- **Catch-all event channels.** Two wildcard event bindings complement named
  `@handler` bindings: `@all` is an additive spy that fires on every event
  (including handled ones) and routes last, so assertions see the applied effect;
  `@else` fires only for events with no named handler. Their handler signature is
  `func(name string, params ...any)` — the event name is delivered typed as the
  first argument, since one handler serves many events (a plain `func(...any)` is
  still accepted, receiving the name prepended to the args). These make `<w-test>`
  able to observe any widget's full event stream.
- **Module self-tests + page report (`Testable()` + `<w-test-report>`).** A module
  can declare its own integration checks by implementing the optional `Testabler`
  interface (`Testable() map[string]wings.CheckFunc`); the runtime discovers them
  per live instance on mount. `<w-test-report>` collects the whole page's result
  via `wings.RunReport()` — every `<w-test>` card (including the human-judged
  visual ones, in whatever state they were left) plus every `Testable()` check —
  renders it as JSON (`{kind, label, state, detail}`), and fires a `report` event
  with that JSON. So a tester delivers one report of what passed and what failed
  with a single click. Transport/persistence is the app's job — WINGS only
  produces the report. Declare `Testable()` in a `wings_test`-tagged file so it
  compiles only into test builds, never production.
- **Test suites.** Unit tests plus a `js/wasm` harness (`run-wasm-tests.sh`) that
  runs the DOM-touching core under a minimal DOM shim, so the widget/runtime
  layer is testable without a browser.

### Fixed
- **`dictbuild` language matching.** BCP-47 case/format variants (e.g. `PT-BR`,
  `pt-br`) now resolve to the canonical key (`pt-BR`) after the raw-key match,
  while non-standard ISO 639-3 codes are preserved.

## [0.15.5] — 2026-06-02

### Fixed
- **Live-demo deployment.** The published site is served from the repository
  root `docs/`, but the build wrote to `live-demo/docs/`; the two had drifted, so
  the live site was stuck on a pre-i18n build. Synced the published `docs/` with
  the current build and dropped the stale `en-US.csv` catalog.

## [0.15.4] — 2026-06-02

### Added
- **Message reuse / composition (`=name` / `#name`).** Name a flex message with
  `=name` and render it elsewhere with `#name` — the last gap versus Fluent's
  message references. `#` is unified: `#42` is the literal rule index `gen_i18n`
  injects, `#name` is the index bound by a `=name`. A reuse site inherits the
  definition's control axes and overrides them per slot (`@gender`/`%count` win
  when present; declaring any `*engine` replaces the inherited engines), while the
  message content comes from the definition and resolves its variables in the
  context where it appears — strictly more than Fluent's static references. Names
  are global per catalog and resolved entirely at build time, so the runtime is
  unchanged and reuse works for both catalog and programmable messages. `==`
  escapes a literal `=`. The live demo's flex tab gains a reuse section.

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
