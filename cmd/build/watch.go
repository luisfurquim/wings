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
	"time"

	"github.com/fsnotify/fsnotify"
)

// rebuildTriggerName is the file (at the app root) whose touch fires the rebuild
// in on-demand watch mode. Only the event matters; its content is ignored.
const rebuildTriggerName = "REBUILD"

// watch observes the app source tree and rebuilds on every relevant save. It
// watches each directory recursively (fsnotify is per-directory on Linux, so new
// dirs are added as they appear), filters events by WINGS_WATCH_EXT, and
// debounces bursts of saves into a single rebuild. Every event is decided by
// content hash: each watched file's hash is recorded when its event is seen
// (before the rebuild it triggers) and the build's own outputs are recorded
// after each build, so an event whose file still hashes to a value already
// accounted for changed nothing real — the build's echo, or a no-op save — and
// is ignored, while a genuine edit (including one made mid-build) rebuilds. It
// blocks until the watcher errors out, which only happens on an unrecoverable
// failure.
//
// In on-demand mode (WINGS_WATCH_MODE=on-demand) a source change is only
// logged; the rebuild fires when the user touches <app root>/REBUILD. The
// trigger matches Write, Create and Chmod events because `touch` on an existing
// file reaches inotify as an attribute update, not a write.
func watch(cfg *devConfig, wingsDir string) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()
	addDirsRecursive(w, cfg.AppRoot, cfg)
	onDemand := cfg.WatchMode == watchOnDemand
	trigger := filepath.Join(cfg.AppRoot, rebuildTriggerName)
	if onDemand {
		devLogf("watching for changes (ext: %s) — on-demand mode: touch %s to rebuild — Ctrl-C to stop", extList(cfg), rebuildTriggerName)
	} else {
		devLogf("watching for changes (ext: %s) — Ctrl-C to stop", extList(cfg))
	}

	var mu sync.Mutex
	var timer *time.Timer
	// seen[path] is the content hash already accounted for: each source as last
	// detected (hashed when its event arrives, before the rebuild it triggers)
	// and the build's own outputs (recorded after each build). An event whose
	// file still hashes to seen[path] changed nothing real — the build's echo or
	// a no-op save — so it is ignored. No time window is needed: a stale entry
	// can only ever match identical content, which would not warrant a rebuild.
	seen := snapshotOutputs(cfg) // account for the initial build's outputs
	// building serializes builds; because the tree is in flux while one runs, its
	// events are buffered into pending and re-examined once it finishes, so an
	// edit saved mid-build is re-checked rather than lost. Both are guarded by mu
	// (building is read and cleared in the same critical section that drains
	// pending, so an event can't slip between the two and be stranded).
	building := false
	pending := map[string]struct{}{}
	// pendingTrigger remembers a REBUILD touch that arrived mid-build (on-demand
	// mode), so the request is honored once the running build settles.
	pendingTrigger := false
	debounce := time.Duration(cfg.Debounce) * time.Millisecond

	// logChange announces an edit in on-demand mode, where it does not rebuild.
	logChange := func(path string) {
		if rel, err := filepath.Rel(cfg.AppRoot, path); err == nil {
			path = rel
		}
		devLogf("change detected: %s", path)
	}

	// changed reports whether path's content differs from what we last accounted
	// for, recording the new hash when it does. A read error (e.g. a deleted
	// file) counts as a change.
	changed := func(path string) bool {
		h, err := hashFile(path)
		clean := filepath.Clean(path)
		mu.Lock()
		defer mu.Unlock()
		if err == nil && seen[clean] == h {
			return false
		}
		if err != nil {
			delete(seen, clean)
		} else {
			seen[clean] = h
		}
		return true
	}

	// rebuild is recursive: a build may surface edits that landed while it ran
	// and reschedule itself, so it is declared before assignment.
	var rebuild func()
	rebuild = func() {
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(debounce, func() {
			mu.Lock()
			building = true
			mu.Unlock()
			if onDemand {
				devLogf("rebuild requested — rebuilding…")
			} else {
				devLogf("change detected — rebuilding…")
			}
			if err := buildOnce(cfg, wingsDir); err != nil {
				devLogf("build failed: %v", err)
			}
			// Account for what the build wrote (so its echoes are ignored), take
			// the events buffered while it ran, and reopen the gate — all under
			// one lock so no event slips through unclassified.
			mu.Lock()
			for g, h := range snapshotOutputs(cfg) {
				seen[g] = h
			}
			drain := pending
			pending = map[string]struct{}{}
			retrigger := pendingTrigger
			pendingTrigger = false
			building = false
			mu.Unlock()
			// A buffered event whose file no longer matches what we accounted
			// for is a real edit made mid-build — rebuild once more (or, in
			// on-demand mode, just announce it and wait for the next trigger).
			for p := range drain {
				if !changed(p) {
					continue
				}
				if onDemand {
					logChange(p)
					continue
				}
				rebuild()
				break
			}
			if retrigger {
				rebuild()
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
			// On-demand trigger: any touch of <app root>/REBUILD requests a
			// build, no hash check — the file's content is irrelevant.
			if onDemand && filepath.Clean(ev.Name) == trigger &&
				ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Chmod) != 0 {
				mu.Lock()
				inBuild := building
				if inBuild {
					pendingTrigger = true
				}
				mu.Unlock()
				if !inBuild {
					rebuild()
				}
				continue
			}
			if cfg.WatchExt[strings.ToLower(filepath.Ext(ev.Name))] {
				mu.Lock()
				inBuild := building
				if inBuild {
					// Tree in flux — classify after the build settles.
					pending[filepath.Clean(ev.Name)] = struct{}{}
				}
				mu.Unlock()
				if inBuild {
					continue
				}
				if changed(ev.Name) {
					if onDemand {
						logChange(ev.Name)
					} else {
						rebuild()
					}
				}
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
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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

// snapshotOutputs fingerprints the watched files a build *writes* — the gen_i18n
// catalogs under <ModuleDir>/<I18nPath>/i18n (where the *.inflections.json also
// land) and the *.i18n.html template outputs next to their sources. The
// wasm/helpers/published copies land in the ignored webroot, so they never
// surface here. Source files are deliberately excluded: recording their hash
// after a build would absorb an edit made mid-build and silently drop it. It
// returns path → content hash.
func snapshotOutputs(cfg *devConfig) map[string]string {
	catalogs := filepath.Clean(filepath.Join(cfg.ModuleDir, cfg.I18nPath, "i18n"))
	out := map[string]string{}
	_ = filepath.WalkDir(cfg.AppRoot, func(path string, d fs.DirEntry, err error) error {
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
		clean := filepath.Clean(path)
		inCatalogs := strings.HasPrefix(clean, catalogs+string(filepath.Separator))
		if !inCatalogs && !strings.HasSuffix(clean, ".i18n.html") {
			return nil // a source the build reads, not an output it writes
		}
		if h, err := hashFile(path); err == nil {
			out[clean] = h
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
