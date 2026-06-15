# WINGS authoring

This project builds UI as **Go compiled to WASM** with the WINGS framework: each
component is a custom element whose logic is Go, not JavaScript. The canonical,
always-current authoring guide is [`AGENTS.md`](../AGENTS.md) at the repository
root — read and follow it; this file is only a short pointer plus the two
mistakes that bite hardest.

1. **Binding names in attributes must be snake_case.** The browser lowercases
   attribute names, so a camelCase binding name (`?isLoading`, `*myItems`,
   `&myValue`, `showExtra="{{...}}"`) silently becomes a no-op — wrong render,
   zero error. `go run ./cmd/build <target>` fails the build on any uppercase
   binding name, with `file:line`. `{{textBinding}}` in text content is exempt,
   but use snake_case everywhere anyway.
2. **Handlers that need `obj` can't be built in `InitData`** (`obj` isn't ready
   there). Put `wings.TriggerHandler(nil)` as a placeholder in `InitData`, then
   set the real `func(args ...any){...}` in `Render` via `obj.This.Set`.

Mental model in one breath: a module is three files with the same basename
(`name.go`/`name.html`/`name.css`); `name.go` calls `wings.Register(...)` in
`init()` and implements `PranaMod` (`InitData` + `Render`); state lives in the
reactive store `obj.This` and the DOM updates from data — there is no virtual
DOM and you never set `innerHTML`. All component files carry
`//go:build js && wasm`. Everything else — template syntax, i18n, skins,
built-in widgets, the build, and security — is in `AGENTS.md` and the
`wings-authoring` Claude Code plugin (see the README "AI-Assisted Development").
