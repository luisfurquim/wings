# WINGS dev container

A zero-toolchain development loop for your own wings app: edit source on your
host, and the container recompiles `wings.wasm` and serves it on every save.

> **⚠️ Experimental (v0.16.0-alpha).** The underlying `cmd/build dev` loop is
> tested and stable when run natively (`go run … dev`). The **Docker image build
> itself has not yet been validated end-to-end** — for a chicken-and-egg reason:
> the image installs the dev tool with
> `go install …/cmd/build@${WINGS_VERSION}`, which needs that version published
> on the module proxy. Until `v0.16.0-alpha` is tagged and pushed, the proxy has
> no wings release that contains `dev` mode, so the image cannot build. Once the
> tag is published, set `WINGS_VERSION=v0.16.0-alpha` and the build works; the
> Docker path will be promoted to stable after that round-trip is verified. The
> native loop (below, "Without Docker") works today.

## Use

Copy three files into your app's root directory:

- `Dockerfile`
- `docker-compose.yml`
- `.env.example` → rename to `.env`

Then:

```sh
docker compose up
```

Open <http://localhost:8080>. Edit your `.go`/`.html`/`.css`/`.json` files on the
host; the container rebuilds and you reload the browser to see the change. Your
source never enters the image — it is bind-mounted at `/app`.

## Configuration (`.env`)

| Variable             | Default                | Purpose                                                        |
| -------------------- | ---------------------- | -------------------------------------------------------------- |
| `WINGS_VERSION`      | `v0.16.0-alpha`        | wings version installed for the dev tool; match your `go.mod`. |
| `WINGS_PORT`         | `8080`                 | Dev server port (also published by compose).                   |
| `WINGS_WEBROOT`      | `.`                    | Dir with `index.html`; `wings.wasm` + JS helpers are written here. |
| `WINGS_MAIN`         | `.`                    | Main package compiled to wasm (relative to the app root).      |
| `WINGS_HTTPD`        | *(empty)*              | Custom server command; empty uses the built-in static server.  |
| `WINGS_DEFLANG`      | *(empty)*              | If set (e.g. `pt-BR`), runs `gen_i18n` each build.             |
| `WINGS_GENI18N_ARGS` | *(empty)*              | Extra `gen_i18n` flags.                                        |
| `WINGS_BUILD_TAGS`   | *(empty)*              | Extra `-tags` for `go build`.                                  |
| `WINGS_WATCH_EXT`    | `go,html,css,json`     | File extensions that trigger a rebuild.                        |
| `WINGS_DEBOUNCE_MS`  | `200`                  | Coalesce window (ms) for bursts of saves.                      |

### Inflection dictionaries (`-auto-flex`)

Auto-filling plural/gender inflections needs Unitex `<lang>.db` dictionaries.
They are **baked into the image at build time** — compiling Unitex pulls a C++
toolchain into a throwaway build stage, so the *first* build is slow, but the
final image only carries the `.db` files.

| Variable          | Default            | Purpose                                                       |
| ----------------- | ------------------ | ------------------------------------------------------------- |
| `WINGS_DICT_LANGS`| *(empty)*          | **Build arg**: comma list of locales to bake, e.g. `pt-BR,en-US`. Empty = none. |
| `WINGS_AUTO_FLEX` | *(empty)*          | `1` passes `-auto-flex` to `gen_i18n` (needs `WINGS_DEFLANG`). |
| `WINGS_DICT_DIR`  | `/opt/wings/dicts` | Where the baked `.db` live (passed as `-dict-dir`).           |

Changing `WINGS_DICT_LANGS` requires a rebuild (`docker compose build`).

### Machine / LLM translation (`-auto-translate`)

Pre-fill empty catalog entries from a translation backend (flagged for human
review). These settings synthesize a `gen_i18n.json` — **only if your app does
not already have one**; a hand-authored `gen_i18n.json` always wins.

| Variable              | Default   | Purpose                                                  |
| --------------------- | --------- | -------------------------------------------------------- |
| `WINGS_AUTO_TRANSLATE`| *(empty)* | `1` passes `-auto-translate` (needs `WINGS_DEFLANG`).    |
| `WINGS_TR_BACKEND`    | *(empty)* | `openai` or `libretranslate`.                            |
| `WINGS_TR_URL`        | *(empty)* | Backend endpoint (LibreTranslate or OpenAI-compatible).  |
| `WINGS_TR_MODEL`      | *(empty)* | Model name (`openai` backend only).                      |
| `WINGS_TR_KEY`        | *(empty)* | API key, if the backend needs one.                       |
| `WINGS_TR_TIMEOUT`    | `60s`     | Per-call timeout.                                        |

### Bring your own server (`WINGS_HTTPD`)

The built-in server is a plain static file server. If your app needs a real
backend (API routes, SSR, TLS), set `WINGS_HTTPD` to the command that starts it,
e.g.:

```
WINGS_HTTPD=go run ./server
```

It runs once with the app root as the working directory and the `WINGS_*` vars in
its environment; the built-in server is then skipped, while build-and-watch keeps
running. The dev loop does not restart your server — it owns its own lifecycle.

## Without Docker

The same loop runs natively if you have Go installed — handy for quick iteration:

```sh
WINGS_DEFLANG=pt-BR go run github.com/luisfurquim/wings/cmd/build@latest dev
```

(or `go run ./cmd/build dev` from inside the wings repo).
