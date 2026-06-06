#!/bin/bash
# Thin wrapper around the Go build orchestrator (cmd/build). Kept so the old
# `./build.sh` command still works; the real logic lives in cmd/build, which is
# pure Go (no sed/openssl/python3) and cross-platform.
set -e
cd "$(dirname "$0")"
exec go run ./cmd/build lib
