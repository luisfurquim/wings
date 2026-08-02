---
name: sec-wasm-go
description: Security peculiarities of Go compiled to WASM in the browser — syscall/js trust boundary, panic semantics, innerHTML/XSS invariant, secrets in browser memory, single-thread assumptions, GOOS=js tooling gaps. Use when writing or reviewing any GOOS=js code, DOM interop, or browser-delivered features.
---

# WASM + Go Security Peculiarities

Classic secure-coding advice assumes processes, a kernel, and an OS user
model. In the browser none of that exists — some djb tactics become
*impossible* and must be replaced, and Go-specific traps appear.

## Structural differences (no way around them)

1. **No process isolation → panic = total loss.** djb's "partition into
   mutually untrusting processes" cannot be done: one wasm module, one linear
   memory, one Go runtime. Any panic anywhere kills the whole app. This is
   why error-over-panic is *structural* here, not style: library code
   degrades and logs (see sec-fail-operational); panicking is the app dev's
   prerogative at top level only.
2. **The sandbox protects the USER from us, not us from inputs.** WASM's
   sandbox is not a security feature for the app. Inside the module there are
   no privilege levels — every byte of state is reachable from every bug.
   Compartmentalization must move to *build time* (gen_i18n does parsing,
   runtime consumes indices) and to *API choke points*.
3. **Secrets live in inspectable memory.** DevTools can read wasm memory.
   Never embed private keys, tokens, or credentials in the wasm or in
   shipped JS. Public keys are fine (live-demo `//go:embed *.pub` +
   `SetCatalogPublicKey` is the correct shape). Anything secret stays
   server-side.
4. **Constant-time code is not guaranteed.** JS/WASM engines JIT and may
   recompile mid-execution; timing properties of `crypto/subtle` are best
   effort. Don't hand-roll crypto in the app; verification-only primitives
   (ed25519.Verify on public data) are fine. Secret-key operations belong on
   the server or in Web Crypto.

## The syscall/js boundary is THE trust boundary

- Every `js.Value` arriving from the DOM, `fetch`, events, or postMessage is
  hostile input (sec-hostile-input applies): check `.Type()` before `.String()`
  / `.Int()`; a wrong-type access panics — and a panic is total (rule 1).
- **A call made with foreign data needs a guard: `internal/jsguard`.** Type
  checks cover a wrong-type access, not a method that THROWS — `matches()` on
  a selector a book wrote, `new Intl.NumberFormat` on a locale from a catalog,
  a method a browser version lacks. Use `jsguard.Do(op, fn)` or
  `jsguard.Value(op, fn)` instead of hand-rolling `defer/recover`: it was
  written by hand 13 times before the package existed, and 7 of those (the
  Intl cache) reported NOTHING, so a refused locale left numbers rendering
  wrong with no clue. It guards a BLOCK, which is the shape the boundary
  actually has. **Always log the returned `*PanicError`** — a guard that
  silently returns the zero value converts a browser refusal into a mystery.
- **innerHTML invariant** (established in the 239e73f hardening): untrusted
  strings never reach `innerHTML`/`insertAdjacentHTML`/`outerHTML`. Templates
  are build-time artifacts; runtime data goes in via `textContent`/attribute
  setters. Any new widget rendering path must preserve this — grep for
  `innerHTML` before closing work.
- `js.Value` references pin JS objects; long-lived registries of callbacks
  (`js.FuncOf`) leak and keep DOM alive — release them on component teardown.
- Origin checks on any postMessage/external-event listener.

## Go-on-js runtime traps

- **Single-threaded today, concurrent anyway:** goroutines interleave at
  scheduling points; races are logic bugs even without parallelism, and a
  blocking receive with no sender deadlocks the app (the `select{}` /
  `go SetLang(...)` lesson). Never block the event loop waiting on JS.
- **Tooling gaps:** golint/vet and Go Report Card skip `//go:build js` files;
  govulncheck's native pass misses js-only imports. Compensate: run the wasm
  test harness (`./run-wasm-tests.sh`, Node + DOM shim), run
  `GOOS=js GOARCH=wasm go vet ./...`, and keep parsers/pure logic in
  buildable-everywhere packages (like `expr/`) so native linters and fuzzers
  see them.
- **Delivery integrity:** the wasm binary and helper JS are served files —
  SRI on script tags, signed catalogs for fetched JSON, `Cache-Control:
  no-store` in dev servers so stale binaries don't masquerade as fixes.

## Checklist for GOOS=js code

- [ ] No panic on any external input path (js.Value type-checked first)
- [ ] No untrusted string into innerHTML-family sinks
- [ ] No secret material embedded or logged
- [ ] js.FuncOf callbacks released on teardown
- [ ] Logic that CAN live in a portable package does (fuzzable, lintable)
- [ ] Tested under the wasm harness, not only native
