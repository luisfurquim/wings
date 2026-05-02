// Package weight provides a locale-aware weight/mass type for wprana templates.
//
// Store masses as [Weight]{Kilograms: v} and bind them with %var:unit
// (e.g. {{%pkg:lb}}). When the unit suffix is omitted the locale default
// is used (pounds for en-US, kilograms for all others).
//
// Unit names safe for template use:
//
//	kg → kg     lb → lb (pound avoirdupois)
//	g  → g      oz → oz (ounce avoirdupois)
//	mg → mg     st → st (stone, 14 lb)
//	t  → t  (metric tonne)
package weight

import "fmt"

// Weight stores a mass in kilograms (SI canonical unit).
type Weight struct{ Kilograms float64 }

type unit struct {
	sym    string
	factor float64 // Kilograms * factor = value-in-unit
}

// lbKg is the exact conversion: 1 lb = 0.45359237 kg (NIST).
const lbKg = 0.45359237

var units = map[string]unit{
	"kg": {"kg", 1},
	"g":  {"g", 1000},
	"mg": {"mg", 1e6},
	"t":  {"t", 0.001},
	"lb": {"lb", 1.0 / lbKg},
	"oz": {"oz", 1.0 / (lbKg / 16)},
	"st": {"st", 1.0 / (lbKg * 14)},
}

// Convert returns the mass in the named unit and its display symbol.
// Returns a non-nil error for unrecognized unit names.
func (w Weight) Convert(unitName string) (val float64, sym string, err error) {
	u, ok := units[unitName]
	if !ok {
		return 0, "", fmt.Errorf("wi18n/weight: unrecognized unit %q", unitName)
	}
	return w.Kilograms * u.factor, u.sym, nil
}

// DefaultUnit returns the built-in display unit for the given BCP 47 locale.
// en-US defaults to pounds; all others default to kilograms.
func DefaultUnit(locale string) string {
	if locale == "en-US" {
		return "lb"
	}
	return "kg"
}

// DefaultDecimals returns the built-in decimal precision for a unit.
func DefaultDecimals(unitName string) int {
	switch unitName {
	case "g", "mg":
		return 0
	case "oz":
		return 1
	case "t":
		return 3
	}
	return 2 // kg, lb, st
}
