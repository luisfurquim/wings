// Package length provides a locale-aware length type for wprana templates.
//
// Store distances as [Length]{Meters: v} and bind them with %var:unit in
// templates (e.g. {{%dist:km}}). When the unit suffix is omitted, the locale's
// default unit is used (miles for en-US/en-GB, meters for all others). Apps
// may override the per-locale default via wi18n.SetConfig (wprana.json).
//
// Canonical storage unit is the metre (SI). All conversions are exact
// rational factors; no floating-point rounding beyond float64 arithmetic.
package length

import "fmt"

// Length stores a distance in metres (SI canonical unit).
type Length struct{ Meters float64 }

// unit bundles the conversion factor (metres → unit) and its display symbol.
type unit struct {
	sym    string
	factor float64 // Meters * factor = value-in-unit
}

var units = map[string]unit{
	"m":      {"m", 1},
	"km":     {"km", 1e-3},
	"cm":     {"cm", 1e2},
	"mm":     {"mm", 1e3},
	"mi":     {"mi", 1 / 1609.344},
	"ft":     {"ft", 1 / 0.3048},
	"yd":     {"yd", 1 / 0.9144},
	"in":     {"in", 1 / 0.0254},
	"nmi":    {"nmi", 1 / 1852.0},
	"league": {"lea", 1 / 4828.032},
}

// New constructs a Length from a value expressed in unitName, converting it
// to the canonical SI storage unit (metres). Returns an error for unrecognized
// unit names. Example: length.New(5, "mi") returns Length{Meters: 8046.72}.
func New(val float64, unitName string) (Length, error) {
	u, ok := units[unitName]
	if !ok {
		return Length{}, fmt.Errorf("wi18n/length: unrecognized unit %q", unitName)
	}
	return Length{Meters: val / u.factor}, nil
}

// Convert returns the distance expressed in the named unit and its display
// symbol. Returns a non-nil error for unrecognized unit names.
func (l Length) Convert(unitName string) (val float64, sym string, err error) {
	u, ok := units[unitName]
	if !ok {
		return 0, "", fmt.Errorf("wi18n/length: unrecognized unit %q", unitName)
	}
	return l.Meters * u.factor, u.sym, nil
}

// DefaultUnit returns the built-in display unit for the given BCP 47 locale
// tag. en-US, en-GB, en-LR and my-MM default to miles; all others default to
// metres. Apps may override this via wi18n.SetConfig (wprana.json).
func DefaultUnit(locale string) string {
	switch locale {
	case "en-US", "en-GB", "en-LR", "my", "my-MM":
		return "mi"
	}
	return "m"
}

// DefaultDecimals returns the built-in decimal precision for a unit. The
// value is used when no precision is configured in <lang>.fmt.json.
func DefaultDecimals(unitName string) int {
	switch unitName {
	case "m", "cm", "mm":
		return 0
	case "km", "ft", "yd", "in":
		return 1
	}
	return 2 // mi, nmi, league, unknown
}
