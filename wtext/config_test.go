package wtext

import (
	"errors"
	"strings"
	"testing"
)

// samplePlugin is a minimal ConfigPlugin for exercising the contract.
type samplePlugin struct{ title string }

func (p samplePlugin) ConfigSections() []ConfigSection {
	return []ConfigSection{{
		ID: "epub", Label: "epub-label",
		Fields: []ConfigField{
			ConfigText{ID: "title", Label: "t", Default: p.title},
			ConfigChoice{ID: "orient", Label: "o", Default: "portrait",
				Options: []Option{{Value: "portrait"}, {Value: "landscape"}}},
		},
	}}
}

func TestConfigDefaultsChain(t *testing.T) {
	defs := configDefaults(Profile{Config: []ConfigPlugin{samplePlugin{title: "Meu Livro"}}})
	core := &fakeCore{cfgDefs: defs}

	if got := core.Config("epub.title"); got != "Meu Livro" {
		t.Errorf("default read = %q", got)
	}
	if got := core.Config("epub.orient"); got != "portrait" {
		t.Errorf("choice default = %q", got)
	}
	if got := core.Config("epub.missing"); got != "" {
		t.Errorf("unknown key = %q, want empty", got)
	}
	if err := core.SetConfig("epub.title", "Editado"); err != nil {
		t.Fatal(err)
	}
	if got := core.Config("epub.title"); got != "Editado" {
		t.Errorf("stored read = %q", got)
	}
	// Empty value deletes: reads fall back to the default again.
	if err := core.SetConfig("epub.title", ""); err != nil {
		t.Fatal(err)
	}
	if got := core.Config("epub.title"); got != "Meu Livro" {
		t.Errorf("read after delete = %q, want the default back", got)
	}
}

func TestConfigValidation(t *testing.T) {
	core := &fakeCore{}
	for _, bad := range []string{"", "com espaço", "aspas\"aqui", "tab\tkey",
		strings.Repeat("k", MaxConfigKeyLen+1)} {
		if err := core.SetConfig(bad, "v"); !errors.Is(err, ErrConfigKey) {
			t.Errorf("SetConfig(%q) = %v, want ErrConfigKey", bad, err)
		}
	}
	if err := core.SetConfig("k", strings.Repeat("v", MaxConfigValueLen+1)); !errors.Is(err, ErrConfigValue) {
		t.Errorf("oversized value = %v, want ErrConfigValue", err)
	}
	for i := 0; i < MaxConfigKeys; i++ {
		if err := core.SetConfig(ConfigKey("s", string(rune('a'+i%26))+strings.Repeat("x", i/26+1)), "v"); err != nil {
			t.Fatalf("fill %d: %v", i, err)
		}
	}
	if err := core.SetConfig("s.overflow", "v"); !errors.Is(err, ErrConfigFull) {
		t.Errorf("overflow = %v, want ErrConfigFull", err)
	}
	// Overwriting an EXISTING key must still work at capacity.
	if err := core.SetConfig(ConfigKey("s", "ax"), "v2"); err != nil {
		t.Errorf("overwrite at capacity = %v", err)
	}
}

func TestConfigKey(t *testing.T) {
	if got := ConfigKey("epub", "title"); got != "epub.title" {
		t.Errorf("ConfigKey = %q", got)
	}
}
