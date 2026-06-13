---
name: sec-fuzzing
description: Continuous fuzzing with Go native fuzzing for any parser/decoder YOU add to a WINGS app — keeping parseable logic in portable (non-js) packages so it's fuzzable, writing targets, the properties to assert, corpus management. Use when you add or change any string/byte parser, custom wire format, or decoder in your app.
---

# Fuzzing the parsers you add

djb minimized parsers because parsers are where bugs live; Apollo verified
exhaustively because there was no patching after launch. Fuzzing serves both —
machine-generated hostile input, continuously, against every parser.

**First question: do you even have a parser?** WINGS deliberately parses *for*
you at build time (templates, flex syntax → `gen_i18n`) and verifies catalogs at
load, so a typical app has none — and `sec-minimal-trusted-code` says keep it
that way. But the moment your app grows its *own* string/byte parser — a custom
backend wire format, a URL-scheme handler, a CSV/CSP/query parser, a binary
decoder — it becomes the place bugs concentrate, and it's yours to fuzz.

## Make your parser fuzzable

Go native fuzzing runs on **native targets only** (not `GOOS=js`). So the same
rule that keeps logic lintable also keeps it fuzzable: put the parser in a
package that builds everywhere — a pure function over `string`/`[]byte`, not one
that takes a `js.Value`. Pull the bytes off the `js.Value` at the boundary
(`sec-hostile-input`), then hand a plain Go value to the parser the fuzzer can
reach.

## Writing a target

```go
func FuzzParseMyFormat(f *testing.F) {
    f.Add("id=42;name=alpha")            // seeds: real inputs from your docs/tests
    f.Add("id=0;name=")
    f.Fuzz(func(t *testing.T, s string) {
        v, err := myformat.Parse(s)      // property 1: never panics
        if err != nil {
            return                       // rejecting bad input is correct
        }
        _ = v.Render()                   // property 2: accepted input is usable
    })
}
```

Properties worth asserting beyond "no panic":

- **Round-trip:** `parse → serialize ≍ input` (modulo documented normalization).
- **Fail-closed:** for anything you verify (a signature, a checksum), flipping
  any byte of a known-good input must yield an error — never a false accept.
- **Idempotence:** running a transform twice equals running it once.
- **No index out of range:** any length/offset/index taken from the input is
  bounds-checked and returns an error, never indexes past the slice.

## Corpus & continuity

- Seed with **real** inputs (`f.Add`), not only random ones: examples from your
  docs, fixtures, and actual backend responses.
- Crashers are auto-saved to `testdata/fuzz/<FuzzName>/` — **commit them.** They
  become permanent regression tests that plain `go test ./...` replays for free.
- Continuous = budgeted, not heroic: `go test -fuzz=FuzzX -fuzztime=60s` per
  target in a periodic job. Day-to-day `go test ./...` already replays the
  whole corpus at no cost.
- A fuzz finding is a finding **about your grammar**, not just one bug: ask
  first whether the format can be made simpler/dumber
  (`sec-minimal-trusted-code`, `sec-hostile-input`) before patching the parser.

## Definition of done for any parser you add

- [ ] Parser lives in a portable (non-`js`) package so the fuzzer can reach it
- [ ] Fuzz target exists, seeded with real inputs
- [ ] Never-panic property holds for 60s+ of local fuzzing
- [ ] Corpus committed; each crasher triaged into a fix + regression seed
- [ ] Error paths return typed errors (feeds the `sec-hostile-input` contract)

Sibling skills: `sec-hostile-input` (don't parse if you can avoid it),
`sec-minimal-trusted-code` (simplify the grammar first), `sec-wasm-go`
(why parsing logic stays in portable packages).
