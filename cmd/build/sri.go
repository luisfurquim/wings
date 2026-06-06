package main

import (
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
)

// sriHash returns the Subresource Integrity hash (sha384-<base64>) for a file.
func sriHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha512.Sum384(data)
	return "sha384-" + base64.StdEncoding.EncodeToString(sum[:]), nil
}

// injectSRI rewrites the <script src="name"> reference in htmlPath so it carries
// a fresh integrity+crossorigin pair hashing fileForHash. It is idempotent: an
// existing integrity/crossorigin trio is replaced rather than duplicated. Only
// the matched src attribute is touched, so the rest of the hand-authored
// index.html is left byte-for-byte intact (no full-document reserialization).
func injectSRI(htmlPath, fileForHash, name string) error {
	hash, err := sriHash(fileForHash)
	if err != nil {
		return err
	}
	html, err := os.ReadFile(htmlPath)
	if err != nil {
		return err
	}
	re := regexp.MustCompile(
		`src="` + regexp.QuoteMeta(name) + `"( integrity="[^"]*")?( crossorigin="[^"]*")?`)
	if !re.Match(html) {
		return fmt.Errorf("%s: no <script src=%q> to add SRI to", htmlPath, name)
	}
	repl := []byte(fmt.Sprintf(`src="%s" integrity="%s" crossorigin="anonymous"`, name, hash))
	out := re.ReplaceAllFunc(html, func([]byte) []byte { return repl })
	return os.WriteFile(htmlPath, out, 0o644)
}
