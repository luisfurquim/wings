// These tests cover the native (non-wasm) configuration surface of wi18n:
// wings.json parsing and the measure-default lookup. They run under the host
// toolchain because config.go and catalog.go carry no build constraints.
package wi18n

import (
	"encoding/json"
	"testing"
)

func TestSetConfigAndMeasureDefault(t *testing.T) {
	cfg := `{
		"measures": {
			"length":      {"defaults": {"en-US": "mi", "pt-BR": "km"}},
			"temperature": {"defaults": {"en-US": "F"}}
		},
		"debugLevel": 3,
		"traceOn": false
	}`
	if err := SetConfig([]byte(cfg)); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	if DebugLevel() != 3 {
		t.Errorf("DebugLevel() = %d, want 3", DebugLevel())
	}
	if TraceEnabled() {
		t.Error("TraceEnabled() = true, want false")
	}

	cases := []struct {
		quantity, locale string
		wantUnit         string
		wantOK           bool
	}{
		{"length", "en-US", "mi", true},
		{"length", "pt-BR", "km", true},
		{"temperature", "en-US", "F", true},
		{"length", "fr-FR", "", false},      // locale absent for known quantity
		{"temperature", "pt-BR", "", false}, // locale absent
		{"weight", "en-US", "", false},      // quantity absent entirely
	}
	for _, c := range cases {
		unit, ok := MeasureDefault(c.quantity, c.locale)
		if unit != c.wantUnit || ok != c.wantOK {
			t.Errorf("MeasureDefault(%q,%q) = (%q,%v), want (%q,%v)",
				c.quantity, c.locale, unit, ok, c.wantUnit, c.wantOK)
		}
	}
}

// SetConfig replaces, not merges: a second call with empty measures must clear
// the previously loaded defaults.
func TestSetConfigReplaces(t *testing.T) {
	if err := SetConfig([]byte(`{"measures":{"length":{"defaults":{"en-US":"mi"}}}}`)); err != nil {
		t.Fatal(err)
	}
	if _, ok := MeasureDefault("length", "en-US"); !ok {
		t.Fatal("setup: expected length/en-US to be present")
	}
	if err := SetConfig([]byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if _, ok := MeasureDefault("length", "en-US"); ok {
		t.Error("after replacing with empty config, length/en-US should be gone")
	}
	if DebugLevel() != 0 {
		t.Errorf("DebugLevel after empty config = %d, want 0", DebugLevel())
	}
}

func TestSetConfigInvalidJSON(t *testing.T) {
	if err := SetConfig([]byte(`{not json`)); err == nil {
		t.Error("SetConfig with malformed JSON: expected error, got nil")
	}
}

// catalog.go is wire-format schema. Verify the omitempty contract that keeps
// the browser bundle small: optional fields must not appear when zero-valued.
func TestEntryDataOmitempty(t *testing.T) {
	b, err := json.Marshal(EntryData{Content: "hi", Revised: true})
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if want := `{"content":"hi","revised":true}`; got != want {
		t.Errorf("EntryData JSON = %s, want %s", got, want)
	}

	// FlexEntryData: Cells/Content/Sources are all omitempty.
	b, err = json.Marshal(FlexEntryData{Label: "aluno", Revised: false})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"label":"aluno","revised":false}`; string(b) != want {
		t.Errorf("FlexEntryData JSON = %s, want %s", b, want)
	}
}

// Entry/FlexEntry embed the wire halves; a round-trip through their data half
// must preserve the fields the runtime reads.
func TestEntryDataRoundTrip(t *testing.T) {
	orig := EntryData{Content: "olá", Revised: true, Source: "llm:gemma4"}
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}
	var back EntryData
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back != orig {
		t.Errorf("round-trip EntryData = %+v, want %+v", back, orig)
	}
}
