---
name: sec-minimal-trusted-code
description: djb's trusted-code minimization applied to YOUR WINGS app — keep parsing at build time, design misuse-resistant APIs, add the fewest deps and config knobs, don't evaluate user strings at runtime. Use when designing a new component/package/public helper, adding a dependency or config option, or reviewing a diff that grows your app.
---

# Minimal trusted code, for your app

Source: D. J. Bernstein, "Some thoughts on security after ten years of qmail
1.0." Security holes are bugs; the bug rate scales with the amount of code that
must be *correct* for security to hold. Three levers, strongest first:

1. **Eliminate code.** The most secure line is the one never written. Before
   adding a feature, weigh what it costs in lines-that-must-be-right.
2. **Eliminate bugs in what remains.** Small functions, explicit data flow, no
   hidden global state, every branch handled.
3. **Eliminate *trusted* code.** Code can exist and still be untrusted — move
   risky work to where a bug *cannot* breach security.

## How this maps to a WINGS app

- **Keep parsing at build time — that's the qmail pattern, and WINGS already
  does it for you.** `gen_i18n` parses all your HTML and flex syntax at build
  time; the wasm runtime just consumes pre-indexed JSON. Don't undo that by
  building a runtime template/expression evaluator that interprets
  user-supplied strings in the browser. If a feature tempts you to "parse rich
  syntax at runtime," push it to build time instead.
- **Design misuse-resistant APIs (NaCl style).** One call that is correct by
  default beats a toolkit of sharp primitives. WINGS gives you the shape to
  imitate: `wi18n.SetCatalogPublicKey` (one call → all-or-nothing,
  fail-closed signature enforcement), `ApplySkin` (returns a typed error
  instead of panicking). The helpers and components you write should have **no
  "insecure but convenient" mode** — no flag that disables escaping,
  validation, or verification.
- **Dependencies are trusted code you didn't write.** Each one must justify
  itself against "could 30 lines of our own do it?" (see `sec-supply-chain`).
- **Config options multiply states.** Every env var or settings key roughly
  doubles your test matrix and adds a way to be configured insecurely. Prefer
  one good default over a knob; if you must add a knob, make the *default* the
  safe value.

## Checklist for a new feature / API / component

- [ ] Could this live at build time instead of runtime?
- [ ] Single entry point that is safe by default — no flag that turns off
      validation, escaping, or verification?
- [ ] Public helpers return `error` instead of panicking? (A panic is total in
      wasm — see `sec-wasm-go`.)
- [ ] Did security-relevant code shrink or grow? If it grew, is the growth
      isolated in one small package with a narrow interface?
- [ ] Zero new dependencies — or a written justification?

## Anti-patterns

- A "flexible" template/expression feature that evaluates user-supplied strings
  in the browser at runtime.
- A helper that accepts `interface{}`/`js.Value` and "figures it out" — that's a
  parser in disguise; type it at the boundary (see `sec-hostile-input`).
- The same validation duplicated across N call sites instead of one choke point
  (one validator = one place to audit, one place to get it right).

Sibling skills: `sec-hostile-input` (boundary typing), `sec-supply-chain`
(dependency cost), `sec-wasm-go` (why panics are fatal here).
