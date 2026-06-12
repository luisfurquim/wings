---
name: sec-supply-chain
description: Dependency and artifact integrity for wings — adding/updating Go modules, auditing transitive deps, govulncheck, reproducible builds, SRI, catalog signing, Docker base images. Use whenever go.mod/go.sum changes, a release is prepared, or fetched/served artifacts are touched.
---

# Supply Chain & Artifact Integrity

Neither djb nor Apollo had `go.mod` — but the philosophy transfers directly:
a dependency is **trusted code you didn't write and don't review on update**.
This skill covers the gap.

## Dependencies

1. **Default answer to a new dep is no.** wings has 6 direct requires; that
   smallness is a security feature. A candidate dep must beat "we write the
   50 lines ourselves" (djb: eliminate trusted code) and be inspected:
   `go mod graph | grep <dep>` for what it drags in, maintenance pulse,
   whether we need 1% of its API.
2. **Transitive deps are real deps.** After any go.mod change run
   `go mod tidy && go mod graph` and skim what appeared. `go mod why -m X`
   answers "who brought this". Indirect modules you can't explain are a
   finding.
3. **go.sum is the integrity anchor.** Never bypass it (`GOFLAGS=-mod=mod` in
   CI, `GONOSUMCHECK`, `GONOSUMDB`, `GOPRIVATE` wildcards). Builds should run
   `-mod=readonly` (Go's default) so a build never silently rewrites deps.
4. **Vulnerability scanning:** run `go run ./cmd/build vulncheck` before each
   release. It runs govulncheck over every module in the repo, twice each: a
   native call-graph pass (low noise) and a `GOOS=js` imports pass — js-only
   packages are excluded from the native pass by build constraints, so without
   the second pass the wasm side would never be scanned. Any finding fails the
   run; fix or explicitly triage before tagging. Implementation:
   `cmd/build/vulncheck.go`.
5. **Pin tools by version.** `go install pkg@vX.Y.Z` (the Dockerfile already
   pins via `WINGS_VERSION`) — never `@latest` in anything reproducible.
   Toolchain updates are deliberate, reviewed events (see
   project_crypto_update_decision: no auto-upgrade magic in builds).

## Artifacts we ship / fetch

- **Reproducible builds are a verification tool.** `cmd/build` parity
  (re-running leaves `git status docs/ dist/` empty, byte-identical wasm) is
  effectively a reproducibility check — treat a parity break as a security
  signal, not an annoyance. Don't add timestamps/nondeterminism to outputs.
- **SRI** on every `<script>` we emit (`prana_helper.js`, `wasm_exec.js`):
  maintained by the build, idempotent. New scripts in served HTML must get
  `integrity=` the same way.
- **Catalog signing (ed25519)** is opt-in by key, fail-closed once a key is
  set: with `SetCatalogPublicKey`, a missing/invalid `.sig` is a refusal.
  Backlog: extend enforcement to `inflections.json`/`fmt.json` (rule: an
  optional file that *exists* must verify; its absence is fine). Any NEW
  runtime-fetched artifact must join this scheme from day one.
- **Releases:** if a published version is broken/insecure, use `retract` in
  go.mod (done for v0.16.3/0.16.4-alpha) — don't delete tags.
- **Docker:** base images pinned (prefer digest pins for the final stage);
  build-time ARG vs runtime env kept separate (`WINGS_DICT_LANGS` is a build
  ARG by design); never bake private keys into images — the live-demo embeds
  only the *public* key, the private key + passphrase are demo-only dogfood.

## Release checklist additions

- [ ] `go mod tidy` clean; no unexplained new indirect modules
- [ ] `govulncheck ./...` (native + js imports mode) clean or triaged
- [ ] build parity: re-run `go run ./cmd/build all`, `git status` empty
- [ ] SRI present and single (not duplicated) in served HTML
- [ ] new fetched artifacts covered by signing or SRI
