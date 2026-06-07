# WINGS dev container — rebuild-on-save for your own wings app.
#
# Multi-stage so the final image stays lean:
#   - `tools`  installs the orchestrator + dictbuild from the module proxy.
#   - `dicts`  (only when WINGS_DICT_LANGS is set) brings in a C++ toolchain to
#              compile Unitex and bake the requested <lang>.db dictionaries.
#   - final    carries just the Go toolchain (to compile your app's wasm), the
#              `wings-dev` binary, and the baked dictionaries.
#
# Your app's source stays OUTSIDE the image: bind-mount it at /app via
# docker-compose. WINGS_VERSION should match the github.com/luisfurquim/wings
# version your app pins in go.mod.

# ---- Stage: tool binaries from the module proxy -----------------------------
# CGO disabled so both binaries are statically linked and portable — dictbuild
# is copied into the Debian-based dicts stage below.
FROM golang:1.25-alpine AS tools
ARG WINGS_VERSION=v0.16.1-alpha
ENV CGO_ENABLED=0
RUN go install "github.com/luisfurquim/wings/cmd/build@${WINGS_VERSION}" \
 && go install "github.com/luisfurquim/wings/cmd/dictbuild@${WINGS_VERSION}"

# ---- Stage: Unitex dictionaries (baked at build time) -----------------------
# Compiling Unitex from source needs git + a C++ toolchain; both live here and
# never reach the final image. We use Debian with an OLDER GCC on purpose:
# unitex-core still uses the `int x = NULL;` idiom, which recent GCC (≥13, as
# shipped by alpine) rejects because NULL is std::nullptr_t and won't convert to
# int. GCC 10 (bullseye) compiles it as upstream intended (a warning, not an
# error). With WINGS_DICT_LANGS empty this stage is a no-op producing an empty
# /opt/wings/dicts.
FROM debian:bullseye-slim AS dicts
ARG WINGS_DICT_LANGS=
COPY --from=tools /go/bin/dictbuild /usr/local/bin/dictbuild
RUN mkdir -p /opt/wings/dicts \
 && if [ -n "$WINGS_DICT_LANGS" ]; then \
      apt-get update \
   && apt-get install -y --no-install-recommends git make g++ cmake ca-certificates \
   && rm -rf /var/lib/apt/lists/* \
   && for lang in $(echo "$WINGS_DICT_LANGS" | tr ',' ' '); do \
        echo "building Unitex dictionary: $lang" \
     && dictbuild -lang "$lang" -out /opt/wings/dicts ; \
      done ; \
    fi

# ---- Final: dev runtime ------------------------------------------------------
FROM golang:1.25-alpine
COPY --from=tools /go/bin/build /usr/local/bin/wings-dev
COPY --from=dicts /opt/wings/dicts /opt/wings/dicts
WORKDIR /app
EXPOSE 8080
# `dev` reads its WINGS_* config from the environment (docker-compose .env).
ENTRYPOINT ["wings-dev", "dev"]
