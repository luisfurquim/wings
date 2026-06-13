---
name: sec-fail-operational
description: Apollo AGC fail-operational design (priority shedding, bounded everything, restart protection, NASA Power of 10) applied to YOUR WINGS app — loops, goroutines, timers, fetch error paths, recovery. Use when writing component effects, background goroutines, polling/retry, or any code that must keep the app rendering when one part fails.
---

# Fail-operational design, for your app

The Apollo 11 landing survived the 1201/1202 alarms because the guidance
computer assumed things *would* go wrong in flight and shed low-priority work
instead of failing whole. In your WINGS app the stakes are a blank page, not a
crater — but the architecture is the same: one wasm module, one Go runtime, so
one feature's failure must not take the page down with it (see `sec-wasm-go`).

1. **Priority shedding, not total failure.** When a fetch fails, a component
   errors, or data is missing, that *part* degrades — show a placeholder, the
   last cached value, or an error state — while the rest of the app keeps
   rendering. WINGS does this for its own features (missing translation →
   source-language text; failed flex → literal passthrough); match it in your
   code. One widget's bad day never escalates to a white screen.
2. **Make work resumable and idempotent.** A component can be created,
   destroyed, and recreated; `Render` can run again. Your initialization and
   effects should be safe to re-run — re-fetching or re-subscribing on a second
   `Render` must not double-count or leak.
3. **Bound everything.** Every loop has a provable upper bound; no unbounded
   recursion; cap the size of anything you accept from outside. In wasm there
   is **no OOM killer and one thread** — an unbounded loop freezes the page.
   Background loops also need an exit: guard with
   `obj.Element.Get("isConnected").Bool()` since there is no teardown hook.
4. **Assert invariants; check every return.** Never discard an `error`; never
   assume a `js.Value` is the type you expect. For anything "impossible," check
   it, log loudly, and take the degraded path. In a component prefer
   **log + degrade over panic** — a panic kills the whole app, the opposite of
   shedding (`sec-wasm-go`). `recover` only at a deliberate top-level boundary,
   and log the recovery as the bug it is.
5. **Irreversible actions need explicit confirmation.** The AGC required the
   PRO key before a burn. In the UI, gate destructive actions (overwriting
   user data, clearing state, deleting) behind a confirmation — use the
   built-in `w-dialog` (`buttons="save,discard,overwrite,cancel"`) rather than
   hand-rolling one (see `wings-widgets`).
6. **Comments record assumptions, not mechanics.** Write down the constraint
   the code can't show ("backend caps this at 100; we don't re-check"), not a
   paraphrase of the next line.
7. **Simple control flow.** Shallow nesting, early returns, functions short
   enough to verify by eye. Easier to see the degraded path actually exists.

## Checklist for component effects / background work

- [ ] If this fetch/parse/effect FAILS, what renders? Trace the degraded path
      end to end — does the rest of the app still show?
- [ ] Is every loop bounded by input size or an explicit constant?
- [ ] Does a background goroutine exit when the element is gone
      (`isConnected`)? No teardown hook will do it for you.
- [ ] Is `Render` safe to run twice (no double fetch / double subscription)?
- [ ] Errors logged with context at the failure site AND surfaced to the user,
      never both swallowed and silent?
- [ ] Channels: who sends, and can a receive block forever? (`select{}`
      deadlock — use a buffered channel or `go`, never block the event loop.)
- [ ] Anything irreversible behind a `w-dialog` confirm?

## Anti-patterns

- `if err != nil { return err }` chains that lose context until something at
  the top logs a bare "error" with no idea which fetch/locale/index failed.
- Retry/poll loops with no bound and no backoff — they pin the single thread.
- A goroutine that blocks on a channel nobody will write to (deadlocks the app).
- `recover()` used to paper over a bug instead of fixing it.
- A `for {}` or `for range ticker.C` in `Render` with no `isConnected` exit.

Sibling skills: `sec-wasm-go` (panic = total, single thread),
`wings-component` (the async/teardown pattern), `wings-widgets` (`w-dialog`).
