package wi18n

import (
	"encoding/json"

	"github.com/luisfurquim/goose"
)

type wingsMeasureEntry struct {
	Defaults map[string]string `json:"defaults"`
}

type wingsCfgJSON struct {
	Measures   map[string]wingsMeasureEntry `json:"measures"`
	DebugLevel int                          `json:"debugLevel"`
	TraceOn    bool                         `json:"traceOn"`
}

var (
	measureDefaults map[string]map[string]string
	debugLevel      int
	traceEnabled    bool
)

// SetConfig parses a wings.json configuration file and applies the
// project-wide settings: measure unit overrides, goose debug level, and
// optional goose stack tracing. Safe to call multiple times; each call
// replaces the previous configuration.
//
// Typical usage with go:embed:
//
//	//go:embed wings.json
//	var wingsCfg []byte
//
//	func init() { wi18n.SetConfig(wingsCfg) }
//
// To propagate the debug level to a package-local goose.Alert, call
// ConfigureGoose(&pkg.G) after SetConfig.
func SetConfig(data []byte) error {
	var cfg wingsCfgJSON
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	md := make(map[string]map[string]string, len(cfg.Measures))
	for qty, entry := range cfg.Measures {
		md[qty] = entry.Defaults
	}
	measureDefaults = md
	debugLevel = cfg.DebugLevel
	traceEnabled = cfg.TraceOn
	if cfg.TraceOn {
		goose.TraceOn()
	}
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

// DebugLevel returns the goose debug level loaded from wings.json.
// Zero until SetConfig has been called with a non-zero "debugLevel" key.
func DebugLevel() int { return debugLevel }

// TraceEnabled reports whether the wings.json "traceOn" key was true.
// SetConfig already calls goose.TraceOn() in that case; this getter is
// for diagnostic display only.
func TraceEnabled() bool { return traceEnabled }

// ConfigureGoose applies the debug level loaded by SetConfig to g. Call
// this after SetConfig from any package that holds its own goose.Alert.
func ConfigureGoose(g *goose.Alert) {
	g.Set(debugLevel)
}
