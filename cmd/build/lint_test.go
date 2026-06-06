package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBindingIdent(t *testing.T) {
	cases := map[string]string{
		"?showExtra":  "showExtra",
		"?show_extra": "show_extra",
		"?!isReady":   "isReady", // ?! boolean-not prefix
		"?cond^":      "cond",    // starts-with operator suffix
		"?cond!":      "cond",    // not-equal operator suffix
		"*items":      "items",   // array
		"*items:i":    "items",   // array with loop index
		"**rows":      "rows",    // span-less array
		"&value":      "value",   // attr sync
		"&dataFoo":    "dataFoo", // attr sync, camelCase
		"class":       "",        // plain attribute, not a binding
		"@click":      "",        // event handler — value-side, not linted
		"id":          "",        // plain attribute
		"{{x}}":       "",        // not an attribute name shape
	}
	for name, want := range cases {
		if got := bindingIdent(name); got != want {
			t.Errorf("bindingIdent(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestAttrNamesSkipsValues(t *testing.T) {
	// A sigil inside a quoted value must NOT be mistaken for an attribute name.
	raw := []byte(`<input &value="{{inputVal}}" placeholder="a *b and ?c" ?show_extra type="text" />`)
	got := attrNames(raw)
	want := map[string]bool{"&value": true, "placeholder": true, "?show_extra": true, "type": true}
	if len(got) != len(want) {
		t.Fatalf("attrNames = %v, want keys %v", got, want)
	}
	for _, n := range got {
		if !want[n] {
			t.Errorf("unexpected attribute name %q (value content leaked?)", n)
		}
	}
}

func TestLintFile(t *testing.T) {
	dir := t.TempDir()

	bad := filepath.Join(dir, "bad.html")
	mustWrite(t, bad, "<div class=\"x\">\n  <span ?showExtra>{{showExtra}}</span>\n</div>\n")
	v, err := lintFile(bad)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 1 || !strings.Contains(v[0], "bad.html:2: ?showExtra") {
		t.Errorf("lintFile(bad) = %v, want one hit at line 2 for ?showExtra", v)
	}

	good := filepath.Join(dir, "good.html")
	// camelCase only in text/value positions (immune) plus snake_case bindings.
	mustWrite(t, good, "<div ?show_extra><input &value=\"{{inputVal}}\" *items:i>{{camelCase}}</div>\n")
	v, err = lintFile(good)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 0 {
		t.Errorf("lintFile(good) = %v, want no violations", v)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
