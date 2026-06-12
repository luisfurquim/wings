---
name: sec-fuzzing
description: Continuous fuzzing discipline for wings using Go native fuzzing — what to fuzz (expr parsers, gen_i18n, codec, signature verification), how to write targets, corpus management, CI budget. Use when adding/changing any parser or decoder, or when asked to improve test coverage of input handling.
---

# Continuous Fuzzing (Go native)

djb minimized parsers because parsers are where bugs live; Apollo verified
exhaustively because there was no patching after launch. Fuzzing is the
modern tool that serves both: machine-generated hostile input, continuously,
against every parser that survived minimization.

## Where the parsers are (priority order)

| Target | Why | Harness note |
|---|---|---|
| `expr.ParseFlexBlock`, `expr.TokenizeFlexContent`, `expr.ParseReference`, `splitSymbols` | hand-written string parsers; webdev-authored input; the leniency/passthrough contract must hold for ALL inputs | pure Go, fuzz natively |
| `cmd/gen_i18n` catalog align/remap (`align.go`: exact map + anchored Levenshtein) | merges old+new catalogs; corruption here silently destroys translations | fuzz the pure functions; fixture HTML via `f.Add` seeds |
| `codec` | de/serialization | classic round-trip target |
| `wi18n` catalog load + `verify.go` | runtime-fetched JSON + ed25519 `.sig`; must refuse, never panic, on garbage | structure as portable funcs if needed |
| `wi18n/fmt*`, currency parsing | locale data → formatting | |

Go fuzzing runs on native targets only — another reason parsing logic stays
in portable packages (`expr/` already is; see sec-wasm-go).

## Writing targets

```go
func FuzzParseFlexBlock(f *testing.F) {
    f.Add("{{@gender %qt #0}}")            // seeds: real syntax from docs/demo
    f.Add("{{%qt *flexer ~$produto}}")
    f.Add("{{=cesta @gender %n}}")
    f.Fuzz(func(t *testing.T, s string) {
        blk, err := expr.ParseFlexBlock(s)  // property 1: never panics
        if err != nil { return }
        _ = blk.Render(...)                 // property 2: accepted input renders
    })
}
```

Properties worth asserting beyond "no panic":

- **Round-trip:** tokenize → reassemble ≍ input (modulo documented space
  collapse in `TokenizeFlexContent`).
- **Fail-closed:** for verify.go, a mutated body or sig NEVER verifies;
  flipping any byte of a known-good pair must yield an error.
- **Idempotence:** running align/remap twice equals running it once
  (the restart-protection property from sec-fail-operational, made testable).
- **No index out of range:** any catalog index arriving from fuzzed JSON is
  bounds-checked, returns error.

## Corpus & continuity

- Seeds: real catalogs (`live-demo/mod/i18n/*.json`), real flex blocks from
  the demo and README examples — add via `f.Add`, not only files.
- Crashers are auto-saved to `testdata/fuzz/<FuzzName>/`; **commit them** —
  they become permanent regression tests run by plain `go test`.
- Continuous = budgeted, not heroic: `go test -fuzz=FuzzX -fuzztime=60s`
  per target in a periodic job (or a `cmd/build` `fuzz` target looping the
  Fuzz* functions). Plain `go test ./...` always replays the corpus for free.
- A fuzz finding in a parser is a *finding about the grammar*, not just a
  bug: ask first whether the syntax can be simplified (sec-minimal-trusted-code)
  before patching the parser.

## Definition of done for any new/changed parser

- [ ] Fuzz target exists with real-syntax seeds
- [ ] Never-panic property holds for 60s+ of local fuzzing
- [ ] Corpus committed; crashers triaged into fixes + regression seeds
- [ ] Error paths return typed errors (feeds sec-hostile-input contract)
