package wi18n

import "encoding/json"

type wpranaMeasureEntry struct {
	Defaults map[string]string `json:"defaults"`
}

type wpranaCfgJSON struct {
	Measures map[string]wpranaMeasureEntry `json:"measures"`
}

var measureDefaults map[string]map[string]string

// SetConfig parses a wprana.json configuration file and applies the measure
// unit overrides. Safe to call multiple times; each call replaces the
// previous configuration.
//
// Typical usage with go:embed:
//
//	//go:embed wprana.json
//	var wpranaCfg []byte
//
//	func init() { wi18n.SetConfig(wpranaCfg) }
func SetConfig(data []byte) error {
	var cfg wpranaCfgJSON
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	md := make(map[string]map[string]string, len(cfg.Measures))
	for qty, entry := range cfg.Measures {
		md[qty] = entry.Defaults
	}
	measureDefaults = md
	return nil
}

// MeasureDefault returns the display unit override for the given physical
// quantity (e.g. "length", "temperature") and BCP 47 locale tag. Returns
// ("", false) when no override is configured — callers fall back to their
// built-in locale defaults.
func MeasureDefault(quantity, locale string) (unit string, ok bool) {
	if measureDefaults == nil {
		return "", false
	}
	d, ok := measureDefaults[quantity]
	if !ok {
		return "", false
	}
	u, ok := d[locale]
	return u, ok
}
