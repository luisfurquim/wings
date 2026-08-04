# Contributing to WINGS

Thanks for looking. WINGS is browser UI written as **Go compiled to WASM** —
every component is a custom element whose logic is Go, not JavaScript.

New to the codebase? Read [`AGENTS.md`](AGENTS.md) first: it is the short
always-on guide to how a component is structured and the two mistakes everyone
makes. [`README.md`](README.md) is the long-form reference.

## Working language

**Everything written into the repository is in English** — commit messages,
code comments, documentation, issues and pull requests. Day-to-day conversation
among contributors happens in whatever language they share, but nothing that
lands in git should require Portuguese to read.

Much of the existing history is in Portuguese. That is legacy, not a template:
don't mirror the language of the commit you are copying the format from.

## Commit messages

One line, in the form `@{type}scope: what changed`:

```
@{feat}wtext: import an EPUB into the editor
@{fix}combobox: typing into a filterable box no longer erases itself
@{chore}live-demo: require wings v0.25.0 + wtextepub v0.5.0
```

**Types** — five, and only these:

| Type | For |
|---|---|
| `@{feat}` | new behaviour |
| `@{fix}` | a bug that was reachable by a user |
| `@{docs}` | README, CHANGELOG, skills, this file |
| `@{chore}` | dependency bumps, build plumbing, moves and renames |
| `@{release}` | stamping a version (see [Releasing](#releasing)) |

Older commits use `@{newfeature}`, `@{bugfix}` and `@{doc}`. Those are retired —
use the table above.

**Scope** is the package or widget touched (`wtext`, `combobox`, `live-demo`),
joined with `+` when a change genuinely spans several (`wtext+wtextepub`), or
omitted entirely for repo-wide changes (`@{release}: …`).

Keep the subject **short and about WHAT changed, not HOW**. The diff already
explains the implementation; the log should stay skimmable. A body is for the
rare thing a reader needs before opening the diff — a breaking change, a
migration step — in one or two lines, not a summary of the patch.

## Before you push

The CI runs the native pass; the `GOOS=js` half is easy to forget, so run both
locally. From the repo root:

```sh
go vet ./...
go test ./...                        # also replays the committed fuzz corpus
./run-wasm-tests.sh                  # the js/wasm tests, under Node
golangci-lint run ./...              # native
GOOS=js GOARCH=wasm golangci-lint run ./...
```

The `GOOS=js` runtime is excluded from the native passes by build constraints,
so a native-only run can be green while the browser code is broken. Both lint
passes must report **0 issues**.

To build:

```sh
go run ./cmd/build <lib|example|live-demo|wlate|all>
go run ./cmd/build dev               # dev server, rebuild-on-save
```

Use these (or the `build.sh` wrapper in a module's directory) rather than a bare
`go build` — the build orchestrator also lints templates for camelCase binding
names, which are silent no-ops at runtime, and regenerates the i18n catalogs.

Two extra targets are not part of `all` because they need network or time:
`vulncheck` (govulncheck over every module, run before a release) and `fuzz`
(runs each `Fuzz*` target for a budget).

## Repository layout

The root module is `github.com/luisfurquim/wings`. Several directories are
**nested Go modules** with their own `go.mod` — `wtextepub`, `live-demo`,
`example`, `helpers/wlate` — so an app pays only for what it imports.

Local development ties them together with `go.work` at the root. Do **not** add
`replace` directives to a sub-module's `go.mod`: the workspace override lives in
one place so that it does not travel when a module is copied out of the tree.
A copied module must resolve `wings` from the module proxy via its `require`.

## Releasing

**Never tag a commit whose `CHANGELOG.md` still says `## [Unreleased]`.** The
version has to be stamped into the documentation *before* the tag exists, not
after — a tag is permanent, so anyone who checks out exactly that tag reads a
changelog that does not name the release they are holding, and the only ways
back are moving a published tag or leaving it wrong forever. The stamp is
therefore step 1, always, and the tag never runs ahead of it.

Nested modules are tagged with a directory prefix (`wtextepub/v0.5.0`), and a
sub-module cannot build standalone until the `wings` contract it depends on has
been tagged. The order matters:

1. `@{release}` commit stamping the version in `CHANGELOG.md` (rename
   `## [Unreleased]` to `## [X.Y.Z] — YYYY-MM-DD`) and in README's "What's
   New". **This commit is what gets tagged.**
2. Tag and push `wings` (`vX.Y.Z`) — at the commit from step 1, never before it.
3. Bump the sub-module's `require` to that version; verify it builds with
   `GOWORK=off` (native **and** `GOOS=js`), which proves it no longer needs the
   workspace. Commit.
4. Tag and push the sub-module (`wtextepub/vA.B.C`).
5. Bump consumers (`live-demo`) to both versions. Verify by copying the module
   outside the tree and building it there — without `go.work` it must resolve
   from the proxy.

Between steps 2 and 3 the sub-module does not build standalone. That gap is
expected, not a regression.

A sub-module needs a tag of its own only when it CONSUMES contract that the new
`wings` release introduces. When its code is untouched, Go's minimal version
selection hands it the newer `wings` as soon as a consumer asks for one, and
re-tagging it would be noise. A consumer, on the other hand, always needs its
`require` bumped — MVS does not raise a version nobody asked for, so skipping
step 5 leaves the demo running the previous release.

## License

MPL 2.0 — see [`LICENSE`](LICENSE). Bundled third-party material keeps its own
terms under [`LICENSES/`](LICENSES). By contributing you agree your work is
released under the same license.
