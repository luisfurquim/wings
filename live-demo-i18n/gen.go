//go:build generate

// This file exists only to host the //go:generate directive for the i18n
// pipeline. It is gated behind the "generate" build tag so that normal
// builds (which target js/wasm) never include it, and so that `go generate`
// can run on the host OS without GOOS=js GOARCH=wasm.
//
// Invoke with:
//
//	go generate -tags generate ./...

package main

//go:generate go run github.com/luisfurquim/wprana/cmd/gen_i18n -path . -deflang en-US
