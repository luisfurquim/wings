package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type metaEntry struct {
	Context   string `json:"context"`
	Ctxdetail string `json:"ctxdetail"`
}

// generateMeta writes a placeholder <lang>.meta.json next to each data catalog
// in dir that lacks one — one empty {context,ctxdetail} entry per catalog entry
// — so the wlate dev server serves them without 404 noise. This replaces the
// python3 block in helpers/wlate/build.sh.
func generateMeta(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	for _, f := range files {
		if strings.Contains(filepath.Base(f), ".meta.") {
			continue
		}
		meta := strings.TrimSuffix(f, ".json") + ".meta.json"
		if _, err := os.Stat(meta); err == nil {
			continue
		}
		n, err := countEntries(f)
		if err != nil {
			return err
		}
		entries := make([]metaEntry, n)
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(meta, append(data, '\n'), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// countEntries returns the number of entries in a JSON-array catalog, or 0 if
// the file is not a JSON array (mirrors the python's isinstance(list) guard).
func countEntries(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		return 0, nil
	}
	return len(arr), nil
}
