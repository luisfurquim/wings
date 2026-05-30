#!/bin/bash
set -e

GOOS=js GOARCH=wasm go build -o wings.wasm .
