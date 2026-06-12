---
name: sec-minimal-trusted-code
description: Apply djb's trusted-code minimization when adding features, dependencies, APIs, or config options to wings. Use when designing new packages, public APIs, build steps, or when reviewing a diff that grows the codebase.
---

# Minimal Trusted Code (djb)

Source philosophy: D. J. Bernstein, "Some thoughts on security after ten years
of qmail 1.0". Security holes are bugs; the bug rate scales with the amount of
code that must be correct for security to hold. Three levers, in order of power:

1. **Eliminate code.** The most secure line is the one never written. Before
   adding a feature, ask what it costs in lines-that-must-be-right, not just
   in lines.
2. **Eliminate bugs in the code that remains.** Smaller functions, explicit
   data flow, no hidden global state, exhaustive handling of every branch.
3. **Eliminate *trusted* code.** Code may exist and still be untrusted: move
   risky work to where a bug cannot violate security.

## How this maps to wings

- **Build-time vs runtime split is the qmail pattern.** `gen_i18n` does all
  HTML/flex parsing at build time; the wasm runtime consumes pre-indexed JSON.
  Keep parsing OUT of the runtime. Any feature that tempts you to parse rich
  syntax inside the browser belongs in `cmd/gen_i18n` or `cmd/build` instead.
- **Misuse-resistant APIs (NaCl style).** One call that is correct by default
  beats a toolkit of sharp primitives. Existing good examples to imitate:
  `wi18n.SetCatalogPublicKey` (one call → all-or-nothing signature
  enforcement), `ApplySkin` returning typed errors instead of panicking.
  A new API should have no "insecure but convenient" mode.
- **Dependencies are trusted code you didn't write.** go.mod currently has 6
  direct requires. Every candidate dep must answer: what does it replace, how
  many transitive deps does it drag in (`go mod graph`), could 50 lines of our
  own code do it? (See sec-supply-chain.)
- **Config options multiply states.** Every `WINGS_*` env var or wings.json
  key doubles the test matrix. Prefer one good default over a knob.

## Checklist for a new feature/API

- [ ] Could this live at build time instead of runtime?
- [ ] Is there a single entry point that is safe by default (no flag that
      disables verification, escaping, or validation)?
- [ ] Does the public surface return `error` instead of panicking
      (input validation contract of this repo)?
- [ ] Net lines of *security-relevant* code: did it shrink or grow? If it
      grew, is the growth isolated in one package with a narrow interface?
- [ ] Zero new dependencies, or a written justification.

## Anti-patterns

- "Flexible" template/expression features evaluated at runtime from
  user-supplied strings.
- Helper that accepts `interface{}`/`js.Value` and "figures it out" — that is
  a parser in disguise; type it at the boundary.
- Duplicating validation in N call sites instead of funneling through one
  choke point (one validator = one place to audit).
