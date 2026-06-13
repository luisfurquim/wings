package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSignSecondaryCatalogs: inflections and fmt catalogs get verifiable .sig
// sidecars; meta files and the main <lang>.json (signed elsewhere) do not; a
// nil key is a no-op.
func TestSignSecondaryCatalogs(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"pt-BR.inflections.json":      `[]`,
		"pt-BR.inflections.meta.json": `{}`,
		"pt-BR.fmt.json":              `{}`,
		"pt-BR.json":                  `[]`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := signSecondaryCatalogs(nil, dir); err != nil {
		t.Fatalf("nil key must be a no-op, got %v", err)
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, "*.sig")); len(matches) != 0 {
		t.Fatalf("nil key wrote signatures: %v", matches)
	}

	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := signSecondaryCatalogs(key, dir); err != nil {
		t.Fatal(err)
	}

	for _, signed := range []string{"pt-BR.inflections.json", "pt-BR.fmt.json"} {
		sigB64, err := os.ReadFile(filepath.Join(dir, signed+".sig"))
		if err != nil {
			t.Fatalf("%s.sig missing: %v", signed, err)
		}
		sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigB64)))
		if err != nil {
			t.Fatalf("%s.sig is not base64: %v", signed, err)
		}
		if !ed25519.Verify(pub, []byte(files[signed]), sig) {
			t.Errorf("%s signature does not verify", signed)
		}
	}
	for _, unsigned := range []string{"pt-BR.inflections.meta.json", "pt-BR.json"} {
		if _, err := os.Stat(filepath.Join(dir, unsigned+".sig")); err == nil {
			t.Errorf("%s must not be signed by signSecondaryCatalogs", unsigned)
		}
	}
}
