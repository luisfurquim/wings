package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// watch observes the app source tree and rebuilds on every relevant save. It
// watches each directory recursively (fsnotify is per-directory on Linux, so new
// dirs are added as they appear), filters events by WINGS_WATCH_EXT, and
// debounces bursts of saves into a single rebuild. It blocks until the watcher
// errors out, which only happens on an unrecoverable failure.
func watch(cfg *devConfig, wingsDir string) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()
	addDirsRecursive(w, cfg.AppRoot, cfg)
	devLogf("watching for changes (ext: %s) — Ctrl-C to stop", extList(cfg))

	var mu sync.Mutex
	var timer *time.Timer
	debounce := time.Duration(cfg.Debounce) * time.Millisecond
	rebuild := func() {
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(debounce, func() {
			devLogf("change detected — rebuilding…")
			if err := buildOnce(cfg, wingsDir); err != nil {
				devLogf("build failed: %v", err)
			}
		})
	}

	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			// Pick up directories created after startup so the watch stays
			// recursive (e.g. a new component package).
			if ev.Op&fsnotify.Create != 0 && isDir(ev.Name) && !ignoredDir(ev.Name, cfg) {
				addDirsRecursive(w, ev.Name, cfg)
			}
			if cfg.WatchExt[strings.ToLower(filepath.Ext(ev.Name))] {
				rebuild()
			}
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			devLogf("watch error: %v", err)
		}
	}
}

// addDirsRecursive adds root and all its subdirectories to the watcher, skipping
// the ignored ones (the webroot — where the build writes — plus VCS/vendor dirs)
// so the build's own output never triggers a rebuild loop.
func addDirsRecursive(w *fsnotify.Watcher, root string, cfg *devConfig) {
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil //nolint:nilerr // unreadable dirs are skipped, not fatal
		}
		if ignoredDir(path, cfg) {
			return filepath.SkipDir
		}
		_ = w.Add(path)
		return nil
	})
}

// ignoredDir reports whether a directory should be excluded from watching: the
// resolved webroot (only when it is a strict subdirectory of the app root —
// otherwise the whole app would be skipped), plus common noise directories.
func ignoredDir(path string, cfg *devConfig) bool {
	if cfg.WebRoot != cfg.AppRoot && (path == cfg.WebRoot || strings.HasPrefix(path, cfg.WebRoot+string(filepath.Separator))) {
		return true
	}
	switch filepath.Base(path) {
	case ".git", "node_modules", ".idea", ".vscode":
		return true
	}
	return false
}

// isDir reports whether path is an existing directory.
func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// extList renders the watched extensions as a stable, comma-joined string for
// the startup log.
func extList(cfg *devConfig) string {
	exts := make([]string, 0, len(cfg.WatchExt))
	for e := range cfg.WatchExt {
		exts = append(exts, strings.TrimPrefix(e, "."))
	}
	sort.Strings(exts)
	return strings.Join(exts, ",")
}
