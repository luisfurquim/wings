// Package speed provides a locale-aware speed type for wprana templates.
//
// Store speeds as [Speed]{MetersPerSecond: v} and bind them with %var:unit
// (e.g. {{%wind:kmh}}). When the unit suffix is omitted the locale default
// is used (mph for en-US/en-GB, km/h for all others).
//
// Unit names safe for template use (identifier characters only):
//
//	ms   → m/s    kn  → kn (knots)
//	kmh  → km/h   fps → ft/s
//	mph  → mph
package speed

import "fmt"

// Speed stores a speed in metres per second (SI canonical unit).
type Speed struct{ MetersPerSecond float64 }

type unit struct {
	sym    string
	factor float64 // MetersPerSecond * factor = value-in-unit
}

var units = map[string]unit{
	"ms":  {"m/s", 1},
	"kmh": {"km/h", 3.6},
	"mph": {"mph", 3600.0 / 1609.344},
	"kn":  {"kn", 3600.0 / 1852.0},
	"fps": {"ft/s", 1.0 / 0.3048},
}

// New constructs a Speed from a value in unitName, converting to m/s.
// Example: speed.New(60, "mph") returns Speed{MetersPerSecond: 26.8224}.
func New(val float64, unitName string) (Speed, error) {
	u, ok := units[unitName]
	if !ok {
		return Speed{}, fmt.Errorf("wi18n/speed: unrecognized unit %q", unitName)
	}
	return Speed{MetersPerSecond: val / u.factor}, nil
}

// Convert returns the speed in the named unit and its display symbol.
// Returns a non-nil error for unrecognized unit names.
func (s Speed) Convert(unitName string) (val float64, sym string, err error) {
	u, ok := units[unitName]
	if !ok {
		return 0, "", fmt.Errorf("wi18n/speed: unrecognized unit %q", unitName)
	}
	return s.MetersPerSecond * u.factor, u.sym, nil
}

// DefaultUnit returns the built-in display unit for the given BCP 47 locale.
// en-US, en-GB, en-LR and my-MM default to mph; all others default to km/h.
func DefaultUnit(locale string) string {
	switch locale {
	case "en-US", "en-GB", "en-LR", "my", "my-MM":
		return "mph"
	}
	return "kmh"
}

// DefaultDecimals returns the built-in decimal precision for a unit.
func DefaultDecimals(_ string) int { return 1 }
