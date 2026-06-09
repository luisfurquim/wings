package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// buildEchoWindow is how long after a build its catalog fingerprints stay valid
// for echo detection: long enough to absorb fsnotify's asynchronous delivery of
// the build's own writes, short enough that a stale fingerprint never lingers.
const buildEchoWindow = 5 * time.Second

// watch observes the app source tree and rebuilds on every relevant save. It
// watches each directory recursively (fsnotify is per-directory on Linux, so new
// dirs are added as they appear), filters events by WINGS_WATCH_EXT, and
// debounces bursts of saves into a single rebuild. The build's own output
// (gen_i18n catalogs and *.i18n.html template outputs written back into the
// watched tree) is told apart from real edits by content: events during a build
// are ignored, and for a short window after, an event whose file still hashes to
// what the build wrote is its echo — while a genuine edit (different content)
// rebuilds immediately. It blocks until the watcher errors out, which only
// happens on an unrecoverable failure.
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
	// building is set while a build runs: gen_i18n rewrites catalogs (and
	// auto-flex writes *.inflections.json) into the watched i18n tree mid-build,
	// and those events must not arm another build.
	var building atomic.Bool
	// built fingerprints the files the last build wrote (path → content hash),
	// valid until builtUntil. fsnotify delivers those writes after the build
	// returns; an event whose file still hashes to what the build wrote is that
	// echo and is ignored, while a real edit (different content, or a file the
	// build never wrote) rebuilds at once — even inside the window.
	built := map[string]string{}
	var builtUntil time.Time
	debounce := time.Duration(cfg.Debounce) * time.Millisecond

	// isBuildEcho reports whether an event is a lingering write of the last
	// build's own output rather than a fresh edit.
	isBuildEcho := func(path string) bool {
		mu.Lock()
		defer mu.Unlock()
		if time.Now().After(builtUntil) {
			return false
		}
		want, ok := built[filepath.Clean(path)]
		if !ok {
			return false
		}
		got, err := hashFile(path)
		return err == nil && got == want
	}

	rebuild := func() {
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(debounce, func() {
			building.Store(true)
			devLogf("change detected — rebuilding…")
			if err := buildOnce(cfg, wingsDir); err != nil {
				devLogf("build failed: %v", err)
			}
			// Fingerprint what the build just wrote, then reopen the gate: the
			// trailing write events are now told apart from edits by content.
			snap := snapshotWatched(cfg)
			mu.Lock()
			built, builtUntil = snap, time.Now().Add(buildEchoWindow)
			mu.Unlock()
			building.Store(false)
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
				if building.Load() {
					continue // mid-build write — ignore
				}
				if isBuildEcho(ev.Name) {
					continue // trailing echo of the build's own output
				}
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

// snapshotWatched fingerprints every watched-ext file in the same tree the
// watcher observes (app root minus the ignored dirs — the webroot, where wasm
// and helpers land, is one of them). A build writes several kinds of file back
// into that tree: gen_i18n catalogs under <I18nPath>/i18n and the *.i18n.html
// template outputs scattered next to their sources. Hashing the whole watched
// set — rather than a single known dir — recognizes all of them (and any future
// output) as the build's own echo. It returns path → content hash.
func snapshotWatched(cfg *devConfig) map[string]string {
	out := map[string]string{}
	filepath.WalkDir(cfg.AppRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // unreadable entries are skipped, not fatal
		}
		if d.IsDir() {
			if ignoredDir(path, cfg) {
				return filepath.SkipDir
			}
			return nil
		}
		if !cfg.WatchExt[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		if h, err := hashFile(path); err == nil {
			out[filepath.Clean(path)] = h
		}
		return nil
	})
	return out
}

// hashFile returns the hex SHA-256 of a file's contents (catalogs are small, so
// a streamed read is cheap).
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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
