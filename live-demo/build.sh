#!/bin/bash
# Thin wrapper around the Go build orchestrator (cmd/build), which lives in the
# wings module. We locate it via `go list -m` and run it there with `go -C`.
set -e
cd "$(dirname "$0")"
WINGS=$(go list -m -f '{{.Dir}}' github.com/luisfurquim/wings)
exec go -C "$WINGS" run ./cmd/build live-demo
