//go:build js && wasm

package wi18n

import "encoding/json"

// fmtConfig holds the parsed <lang>.fmt.json for one locale.
// Two entry types coexist in the same file:
//   - Unit-precision entries (no "type" field): {"km": {"decimals": 1}}
//   - Named scalar formats  (with "type" field): {"compact": {"type": "number", "notation": "compact"}}
type fmtConfig struct {
	units map[string]int           // formatName → decimal places
	named map[string]namedFmtEntry // formatName → Intl options
}

// namedFmtEntry is a named scalar format entry from <lang>.fmt.json.
type namedFmtEntry struct {
	FmtType string         // "number" or "date"
	Options map[string]any // remaining Intl options (notation, style, etc.)
}

// parseFmtConfig parses the JSON body of a <lang>.fmt.json file. Returns nil
// on JSON parse error; individual malformed entries are silently skipped.
func parseFmtConfig(body string) *fmtConfig {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return nil
	}
	cfg := &fmtConfig{
		units: make(map[string]int),
		named: make(map[string]namedFmtEntry),
	}
	for name, v := range raw {
		var entry map[string]any
		if err := json.Unmarshal(v, &entry); err != nil {
			continue
		}
		if t, ok := entry["type"].(string); ok {
			opts := make(map[string]any, len(entry)-1)
			for k, val := range entry {
				if k != "type" {
					opts[k] = val
				}
			}
			cfg.named[name] = namedFmtEntry{FmtType: t, Options: opts}
		} else if d, ok := entry["decimals"]; ok {
			if f, ok := d.(float64); ok {
				cfg.units[name] = int(f)
			}
		}
	}
	return cfg
}

// UnitDecimals returns the decimal precision configured for the given unit
// name in the named locale's fmt config. Returns (0, false) when no
// configuration exists for that locale/unit combination; callers provide their
// own default.
func UnitDecimals(locale, formatName string) (int, bool) {
	bundleMu.RLock()
	b, ok := bundles[locale]
	bundleMu.RUnlock()
	if !ok || b.fmtCfg == nil {
		return 0, false
	}
	d, ok := b.fmtCfg.units[formatName]
	return d, ok
}

// NamedFmt returns the Intl format type and options for a named scalar format
// in the given locale. Returns ("", nil, false) when not configured.
func NamedFmt(locale, formatName string) (fmtType string, opts map[string]any, ok bool) {
	bundleMu.RLock()
	b, found := bundles[locale]
	bundleMu.RUnlock()
	if !found || b.fmtCfg == nil {
		return "", nil, false
	}
	e, ok := b.fmtCfg.named[formatName]
	return e.FmtType, e.Options, ok
}
