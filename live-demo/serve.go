//go:build ignore

package main

import (
	"net/http"
	"strings"

	"github.com/luisfurquim/goose"
)

// G is this binary's goose alert.
var G goose.Alert = goose.Alert(2)

func main() {
	fs := http.FileServer(http.Dir("docs"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".wasm") {
			w.Header().Set("Content-Type", "application/wasm")
		}
		fs.ServeHTTP(w, r)
	})
	G.Logf(2, "Listening on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
