package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSnapshotOutputs_OnlyBuildWrites verifies that the watcher fingerprints the
// files a build writes (gen_i18n catalogs under <I18nPath>/i18n and the
// *.i18n.html template outputs) and never the sources it reads, nor anything in
// the ignored webroot. Fingerprinting a source would absorb an edit made
// mid-build and silently drop it — the bug this guards against.
func TestSnapshotOutputs_OnlyBuildWrites(t *testing.T) {
	root := t.TempDir()
	cfg := &devConfig{
		AppRoot:   root,
		ModuleDir: root,
		I18nPath:  "mod",
		WebRoot:   filepath.Join(root, "docs"),
		WatchExt:  parseExt("go,html,css,json"),
	}

	write := func(rel, body string) string {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return filepath.Clean(p)
	}

	// Build outputs — must be fingerprinted.
	catalog := write("mod/i18n/pt-BR.json", `{"a":1}`)
	infl := write("mod/i18n/pt-BR.inflections.json", `{}`)
	tmpl := write("mod/tabs/foo/foo.i18n.html", `<x>`)
	// Sources the build reads — must NOT be fingerprinted.
	src := write("mod/tabs/foo/foo.html", `<x data-i18n="a">`)
	gosrc := write("mod/app.go", "package mod\n")
	// A template output dropped into the ignored webroot — excluded despite the
	// matching suffix, because the whole webroot is skipped.
	published := write("docs/foo.i18n.html", `<x>`)

	snap := snapshotOutputs(cfg)

	for _, want := range []string{catalog, infl, tmpl} {
		if _, ok := snap[want]; !ok {
			t.Errorf("expected build output %s to be fingerprinted", want)
		}
	}
	for _, no := range []string{src, gosrc, published} {
		if _, ok := snap[no]; ok {
			t.Errorf("%s must not be fingerprinted (source or ignored webroot)", no)
		}
	}
}
