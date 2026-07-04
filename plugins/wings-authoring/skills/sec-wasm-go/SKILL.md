---
name: sec-wasm-go
description: Security peculiarities of Go-to-WASM in the browser, for apps you build with WINGS — the syscall/js trust boundary, panic = whole-app death, the innerHTML/XSS invariant, secrets in browser memory, single-thread concurrency, GOOS=js tooling gaps. Use when writing or reviewing any component (GOOS=js code), DOM interop via syscall/js, fetch, or browser-delivered features in your app.
---

# WASM + Go security, for your WINGS app

Your components are Go compiled to WASM running in the browser. Classic
secure-coding advice assumes processes, a kernel, and an OS user model — none
of that exists here. Some defenses become *impossible* and must be replaced by
others, and Go-specific traps appear. WINGS handles its own internals safely;
this is what *you* still own.

## Structural facts you can't design around

1. **No process isolation → a panic is total loss.** One wasm module, one
   linear memory, one Go runtime. A panic in *any* component blanks the whole
   page — every other component dies too. This is why returning errors instead
   of panicking is structural, not stylistic: your logic degrades and logs
   (stdlib `log`/`println` output lands in the browser console under wasm);
   panicking at top level is a deliberate "the app cannot continue" decision,
   not an error path.
2. **The WASM sandbox protects the USER from your code, not your code from
   input.** Inside the module there are no privilege levels — every byte of
   app state is reachable from every bug. There is no "secure enclave" in your
   wasm.
3. **Secrets live in inspectable memory.** DevTools can read wasm linear
   memory and any shipped JS. Never embed API keys, auth tokens, DB
   credentials, or private signing keys in the wasm or in served JS. *Public*
   keys are fine — embedding your catalog-signing public key with
   `//go:embed *.pub` + `wi18n.SetCatalogPublicKey` is the correct pattern.
   Anything secret stays on your server, reached over an authenticated fetch.
4. **Constant-time guarantees don't hold.** JS/WASM engines JIT and recompile;
   `crypto/subtle` timing is best-effort at most. Don't hand-roll crypto in the
   app. Verification of public data (`ed25519.Verify`) is fine; secret-key
   operations belong on the server or in Web Crypto.

## The syscall/js boundary is your trust boundary

- Every `js.Value` arriving from the DOM, `fetch`, events, `postMessage`, or
  `localStorage` is hostile input (see `sec-hostile-input`). Check `.Type()`
  before calling `.String()` / `.Int()` / `.Bool()` — a wrong-type access
  **panics**, and a panic is total (fact 1).
- **innerHTML invariant.** In your components you normally never touch the DOM
  directly — you drive it through `obj.This.Set` and let WINGS render. The
  danger is reaching *around* that: `someEl.Set("innerHTML", fetchedString)` or
  `insertAdjacentHTML` with any value that came from a user, a URL, or a
  backend is an XSS hole. Put untrusted runtime data in via `textContent` or an
  attribute setter, or (better) through `obj.This`. Grep your code for
  `innerHTML` / `outerHTML` / `insertAdjacentHTML` before shipping.
- `js.FuncOf` callbacks pin JS objects and keep the DOM alive. WINGS frees these
  for you on disconnect — its own (two-way `&attr` bindings, the render
  goroutine) *and* the listeners you register with `dom.AddEvent` and the
  MutationObservers you register with `dom.Observe` under the element. Never
  build a `MutationObserver` by hand (`js.Global().Get("MutationObserver")`) —
  its `js.FuncOf` pins the Go closure forever; use `dom.Observe`. What WINGS
  cannot reach into is a goroutine loop you launched — see the teardown caveat
  below.
- Add an **origin check** to any `postMessage`/cross-window listener you
  register before trusting `event.data`.

## Go-on-JS runtime traps

- **Single-threaded today, concurrent anyway.** Goroutines interleave at
  scheduling points, so data races are still logic bugs, and a blocking
  channel receive with no sender deadlocks the *entire* app (the classic
  `select{}` / "call `go SetLang(...)`, never block on it" lesson). Never block
  the event loop waiting on JS.
- **No teardown hook you can run code in.** `PranaMod` has only `InitData` and
  `Render`; `PranaObj` hands you no cancellation context. On disconnect WINGS
  cleans up the state it can see — cancels the render goroutine, releases
  two-way bindings, and frees the listeners you registered with `dom.AddEvent`
  under the element — but it can't interrupt a goroutine you launched. The
  practical consequences:
  - **One-shot async** (a single fetch in `Render`) needs no guard — it
    completes; a late `obj.This.Set` on a detached element is harmless.
  - **Repeating loops/timers** keep running after the element is gone, because
    WINGS can't reach into your goroutine. Guard them with
    `obj.Element.Get("isConnected").Bool()` and exit when false (see
    `wings-component`) — that poll is the only disconnect signal you get.
  - **Native `dom.AddEvent` listeners and `dom.Observe` observers** are freed
    for you on disconnect, so a component created and destroyed repeatedly
    won't leak them. You may still `dom.RmEvent` / `dom.RmObserver` early if
    you like — both are idempotent.
- **Tooling gaps.** `golint`/`go vet` and Go Report Card skip `//go:build js`
  files; govulncheck's native pass misses js-only imports. Compensate: run
  `GOOS=js GOARCH=wasm go vet ./...`, scan with the js imports mode too (see
  `sec-supply-chain`), and keep pure logic (parsing, validation, math) in
  packages that build everywhere so native linters and fuzzers can see them.
- **Delivery integrity.** Your wasm binary and the helper JS
  (`prana_helper.js`, `wasm_exec.js`) are served files: put SRI
  (`integrity=`) on your `<script>` tags, sign catalogs you fetch at runtime,
  and serve dev builds with `Cache-Control: no-store` so a stale binary never
  masquerades as a fix.

## Checklist for any component you write

- [ ] No panic on any external input path — every `js.Value` type-checked first
- [ ] No untrusted string into `innerHTML`/`outerHTML`/`insertAdjacentHTML`
- [ ] No secret material embedded in wasm/JS or logged to the console
- [ ] Repeating goroutine loops bounded by `isConnected` (bindings,
      `dom.AddEvent` listeners and `dom.Observe` observers are freed for you
      on disconnect)
- [ ] Pure logic lives in a portable package (lintable, fuzzable)
- [ ] `<script>` tags carry SRI; runtime-fetched JSON is signed

Sibling skills: `sec-hostile-input`, `sec-fail-operational`,
`sec-supply-chain`. Component lifecycle detail: `wings-component`.
