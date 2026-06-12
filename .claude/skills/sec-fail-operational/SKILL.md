---
name: sec-fail-operational
description: Apollo AGC philosophy (restart protection, priority shedding, bounded everything, NASA/JPL Power of 10) applied to wings runtime and tooling. Use when writing loops, goroutines, recovery paths, watchers, schedulers, or any code that must keep working when something else fails.
---

# Fail-Operational Design (Apollo AGC + NASA Power of 10)

The Apollo 11 landing survived the 1201/1202 program alarms because the AGC
executive was designed around one assumption: **things WILL go wrong in
flight**. Its answers, visible in the published AGC source, plus NASA/JPL's
later "Power of 10" rules, give a runtime discipline:

1. **Priority shedding, not total failure.** Under overload the AGC dropped
   low-priority jobs and kept guidance running. A wings module that fails
   (catalog rejected, fetch failed, engine errored) must degrade — render
   source language, render placeholder verbatim, skip the widget — while the
   rest of the app keeps flying. One feature's failure never escalates to a
   blank page. The CustomFlex async path (placeholder verbatim → re-sync when
   data arrives) is the house pattern.
2. **Restart protection: make work resumable and idempotent.** AGC restart
   tables let a reboot resume mid-burn. Our equivalents: builds are
   re-runnable byte-for-byte (`cmd/build` parity), the dev watcher decides by
   content hash so a rebuild echo or an interrupted cycle converges instead
   of looping. New tooling steps must be safe to re-run from any point.
3. **Bounded everything** (Power of 10 rule 2/3): every loop has a provable
   upper bound; no unbounded recursion; sizes of accepted inputs have explicit
   limits checked at the boundary. In wasm there is no OOM killer protecting
   anyone — an unbounded loop freezes the page (single thread).
4. **Assert invariants; check every return.** P10 rules 5 and 7. Anything
   "impossible" gets a check that logs loudly (goose level 1) and takes the
   degraded path. Never discard an `error`; never assume a `js.Value` is the
   type you expect. In library code prefer log+degrade over panic — in wasm a
   panic kills the entire app (see sec-wasm-go), the opposite of shedding.
5. **Irreversible actions need explicit confirmation.** The AGC required the
   PRO key before executing a burn. UI equivalent already in wings: w-dialog
   overwrite confirmation in wlate. Destructive operations (overwrite
   translations, clear catalogs, delete files in tooling) get a confirm path
   or an explicit `--force`.
6. **Comments record assumptions, not mechanics.** The AGC source comments
   state intent and constraints ("TEMPORARY, I HOPE HOPE HOPE" marks a known
   debt). Write the constraint the code can't show; nothing else.
7. **Simple control flow** (P10 rule 1): shallow nesting, early returns,
   functions short enough to verify by eye — this is also the gocyclo target
   (<15) and the repo's 1-func-per-file convention.

## Checklist for runtime/tooling code

- [ ] What happens if this fetch/parse/engine FAILS? Trace the degraded path
      end to end — does the rest of the app still render?
- [ ] Is every loop bounded by input size or an explicit constant?
- [ ] Is this step idempotent (safe to re-run after interruption)?
- [ ] Errors: logged with context at the failure site, returned typed to the
      caller, and never both swallowed and silent?
- [ ] Goroutines: who owns the channel, what guarantees it can't block
      forever? (`select{}`-deadlock class — use buffered/`go SetLang(...)`
      patterns.)
- [ ] Anything irreversible behind a confirm or `--force`?

## Anti-patterns

- `if err != nil { return err }` chains that lose context until the top
  logs "error" with no file/lang/index.
- Retry loops without a bound or backoff.
- A watcher/scheduler that can be wedged by one bad file forever.
- Recovering from a panic to hide a bug (recover only at a top-level boundary,
  and log it as a bug).
