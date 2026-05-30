//go:build generate

// Host-only file: carries the //go:generate directive for the i18n pipeline.
// Gated behind the "generate" tag so normal js/wasm builds skip it.
//
// Invoke with:
//
//	go generate -tags generate ./...

package main

//go:generate go run github.com/luisfurquim/wings/cmd/gen_i18n -path ./mod -deflang pt-BR
