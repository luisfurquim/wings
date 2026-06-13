---
name: sec-supply-chain
description: Dependency and artifact integrity for YOUR WINGS app — adding/updating Go modules, pinning the wings version, govulncheck (native + GOOS=js passes), SRI on served scripts, signing the catalogs you ship, Docker FROM wings, never baking secrets into images. Use whenever your go.mod/go.sum changes, you prepare a release, or you serve/fetch an artifact.
---

# Supply chain & artifact integrity, for your app

A dependency is **trusted code you didn't write and won't review on every
update** — it runs with all the access your app has. The same philosophy that
keeps WINGS itself lean applies to the app you build on it.

## Dependencies

1. **Default answer to a new dep is no.** Every dependency is attack surface
   and audit burden. A candidate must beat "we write the 30 lines ourselves."
   Before adding one, run `go mod graph | grep <dep>` to see what it drags in,
   check its maintenance pulse, and ask whether you need 1% of its API.
2. **Transitive deps are real deps.** After any `go.mod` change run
   `go mod tidy && go mod graph` and skim what appeared; `go mod why -m <mod>`
   answers "who pulled this in." An indirect module you can't explain is a
   finding.
3. **`go.sum` is the integrity anchor — never bypass it.** No `GOFLAGS=-mod=mod`
   in CI, no `GONOSUMCHECK`/`GONOSUMDB`, no broad `GOPRIVATE` wildcards. Build
   with Go's default `-mod=readonly` so a build never silently rewrites deps.
4. **Scan for vulnerabilities — twice.** Run `govulncheck ./...` before each
   release, and **also** run it in `GOOS=js` mode. Native call-graph scanning
   excludes `//go:build js` packages by build constraint, so without the second
   pass your entire browser-side code is never scanned. Fix or explicitly
   triage any finding before tagging.
5. **Pin versions — including wings.** `require github.com/luisfurquim/wings
   vX.Y.Z` at a real tag, not a floating pseudo-version. `go install
   tool@vX.Y.Z`, never `@latest`, in anything reproducible. Toolchain and
   dependency bumps are deliberate, reviewed events, not background magic.

## Artifacts you ship or fetch

- **SRI on every `<script>` you serve.** Your wasm loader, `prana_helper.js`,
  and `wasm_exec.js` are static files an intermediary could tamper with — emit
  `integrity=` hashes and keep them current with the bytes you actually ship.
- **Sign the catalogs you fetch at runtime.** Catalog signing (ed25519) is
  opt-in by key and **fail-closed** once set: call `wi18n.SetCatalogPublicKey`
  and a missing or invalid `.sig` becomes a refusal, not a warning. Embed only
  the *public* key (`//go:embed *.pub`); the private key signs at build time and
  never ships. Any new runtime-fetched JSON should join this scheme from day one.
- **Reproducible builds are a verification tool.** A build that re-runs to
  byte-identical output lets you treat any unexpected diff as a tamper signal.
  Don't bake timestamps or other nondeterminism into your outputs.
- **Docker `FROM wings`.** Pin the base image (prefer a digest, or the
  `WINGS_VERSION` tag), keep build-time `ARG` separate from runtime `env`, and
  **never bake private keys, tokens, or passphrases into an image** — images are
  shared, layers are inspectable. Embed only public material.
- **Don't delete published tags.** If you ship a broken/insecure version, add a
  `retract` directive in `go.mod` so `go get` warns; deleting the tag breaks
  everyone who already pinned it.

## Release checklist for your app

- [ ] `go mod tidy` clean; no unexplained new indirect modules
- [ ] `govulncheck ./...` AND `GOOS=js GOARCH=wasm govulncheck ./...` clean/triaged
- [ ] `wings` pinned to a real tag; no `@latest` anywhere reproducible
- [ ] SRI present (and not duplicated) on every served `<script>`
- [ ] Every runtime-fetched artifact covered by signing or SRI
- [ ] No secret embedded in wasm, JS, or any Docker layer

Sibling skills: `sec-minimal-trusted-code` (the "fewest deps" principle),
`sec-wasm-go` (why the js pass matters), `wings-build` (build/dev/Docker setup).
