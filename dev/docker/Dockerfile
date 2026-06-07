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
FROM golang:1.25-alpine AS tools
ARG WINGS_VERSION=v0.16.0-alpha
RUN go install "github.com/luisfurquim/wings/cmd/build@${WINGS_VERSION}" \
 && go install "github.com/luisfurquim/wings/cmd/dictbuild@${WINGS_VERSION}"

# ---- Stage: Unitex dictionaries (baked at build time) -----------------------
# Compiling Unitex from source needs git + a C++ toolchain; both live here and
# never reach the final image. With WINGS_DICT_LANGS empty this stage is a no-op
# and produces an empty /opt/wings/dicts.
FROM golang:1.25-alpine AS dicts
ARG WINGS_DICT_LANGS=
COPY --from=tools /go/bin/dictbuild /usr/local/bin/dictbuild
RUN mkdir -p /opt/wings/dicts \
 && if [ -n "$WINGS_DICT_LANGS" ]; then \
      apk add --no-cache git cmake make g++ libstdc++ \
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
