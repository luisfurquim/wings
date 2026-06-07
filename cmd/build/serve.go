package main

import (
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// startServer brings up the dev web server. When WINGS_HTTPD is empty it starts
// the embedded static server; otherwise it spawns the webdev's own command once
// (cwd = app root, the inherited environment already carries the WINGS_* vars).
// Either way it returns immediately — the build/watch loop runs independently.
func startServer(cfg *devConfig) error {
	if cfg.Httpd != "" {
		startCustomHTTPD(cfg)
		return nil
	}
	return startEmbeddedServer(cfg)
}

// startEmbeddedServer serves cfg.WebRoot over HTTP, mirroring live-demo/serve.go:
// it tags .wasm responses with the correct MIME type, and adds Cache-Control:
// no-store so the browser always re-fetches the freshly rebuilt wasm.
func startEmbeddedServer(cfg *devConfig) error {
	fsv := http.FileServer(http.Dir(cfg.WebRoot))
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".wasm") {
			w.Header().Set("Content-Type", "application/wasm")
		}
		w.Header().Set("Cache-Control", "no-store")
		fsv.ServeHTTP(w, r)
	})
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: mux}
	devLogf("serving %s on http://localhost:%s", cfg.WebRoot, cfg.Port)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			devLogf("server error: %v", err)
		}
	}()
	return nil
}

// startCustomHTTPD runs WINGS_HTTPD once in the app root via the shell, wiring
// its output to ours. It is supervised only to report if it exits — the dev loop
// does not restart it (a custom backend owns its own lifecycle).
func startCustomHTTPD(cfg *devConfig) {
	devLogf("starting custom httpd: %s", cfg.Httpd)
	go func() {
		cmd := exec.Command("sh", "-c", cfg.Httpd)
		cmd.Dir = cfg.AppRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			devLogf("custom httpd exited: %v", err)
		}
	}()
}
