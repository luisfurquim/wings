package main

import (
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
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
// no-store so the browser always re-fetches the freshly rebuilt wasm. Directory
// listings are disabled (see noListFS) so the server never exposes the app's
// source tree. If the webroot has no index.html the server still starts (the
// build/watch loop keeps running) but warns, since the browser will get a 404.
func startEmbeddedServer(cfg *devConfig) error {
	if _, err := os.Stat(filepath.Join(cfg.WebRoot, "index.html")); err != nil {
		devLogf("warning: no index.html in %s — the browser will get 404 until one exists (WINGS_WEBROOT?)", cfg.WebRoot)
	}
	fsv := http.FileServer(noListFS{http.Dir(cfg.WebRoot)})
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Defense in depth: even with WINGS_WEBROOT pointed at the app source,
		// never serve dotfiles (.env, .git/…) or private keys by direct path.
		if isSensitivePath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
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

// isSensitivePath reports whether a request path must never be served: anything
// with a dot-prefixed segment (.env, .git/…) or a private-key extension. This is
// a small, unambiguous denylist — such files are never legitimate web assets,
// and a dev webroot is often the app source root where they live.
func isSensitivePath(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if strings.HasPrefix(seg, ".") && seg != "." && seg != ".." {
			return true
		}
	}
	base := strings.ToLower(path.Base(p))
	return strings.HasSuffix(base, ".key") || strings.HasSuffix(base, ".pem")
}

// noListFS wraps an http.FileSystem to disable directory listings: a request for
// a directory that has no index.html returns "not found" instead of dumping the
// directory's contents. Without this the dev server would expose the app's whole
// source tree as a browsable index — including .env files and signing keys.
type noListFS struct{ fs http.FileSystem }

func (n noListFS) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if info.IsDir() {
		index, ierr := n.fs.Open(strings.TrimSuffix(name, "/") + "/index.html")
		if ierr != nil {
			f.Close()
			return nil, os.ErrNotExist
		}
		index.Close()
	}
	return f, nil
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
