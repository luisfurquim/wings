package wi18n

import "testing"

// FuzzParseFmtConfig feeds arbitrary strings to the <lang>.fmt.json parser.
// Property: never panics; garbage yields nil (parse error) and a well-formed
// object always has both maps initialized, never nil-map writes downstream.
func FuzzParseFmtConfig(f *testing.F) {
	for _, s := range []string{
		`{"km":{"decimals":1}}`,
		`{"compact":{"type":"number","notation":"compact"}}`,
		`{"km":{"decimals":1},"compact":{"type":"number"}}`,
		`{}`,
		``,
		`{"bad":`,
		`{"x":{"decimals":"notanumber"}}`,
		`[1,2,3]`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, body string) {
		cfg := parseFmtConfig(body)
		if cfg == nil {
			return
		}
		if cfg.units == nil || cfg.named == nil {
			t.Fatalf("non-nil cfg with nil map: units=%v named=%v", cfg.units, cfg.named)
		}
	})
}
