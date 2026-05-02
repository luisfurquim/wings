// Package area provides a locale-aware area type for wprana templates.
//
// Store areas as [Area]{SquareMeters: v} and bind them with %var:unit
// (e.g. {{%plot:ac}}). When the unit suffix is omitted the locale default
// is used (ft² for en-US, m² for all others).
//
// Unit names safe for template use:
//
//	m2  → m²     mi2 → mi²
//	km2 → km²    ft2 → ft²
//	cm2 → cm²    yd2 → yd²
//	mm2 → mm²    in2 → in²
//	ha  → ha     ac  → ac (acre)
package area

import "fmt"

// Area stores an area in square metres (SI canonical unit).
type Area struct{ SquareMeters float64 }

type unit struct {
	sym    string
	factor float64 // SquareMeters * factor = value-in-unit
}

var units = map[string]unit{
	"m2":  {"m²", 1},
	"km2": {"km²", 1e-6},
	"cm2": {"cm²", 1e4},
	"mm2": {"mm²", 1e6},
	"ha":  {"ha", 1e-4},
	"mi2": {"mi²", 1.0 / (1609.344 * 1609.344)},
	"ft2": {"ft²", 1.0 / (0.3048 * 0.3048)},
	"yd2": {"yd²", 1.0 / (0.9144 * 0.9144)},
	"in2": {"in²", 1.0 / (0.0254 * 0.0254)},
	"ac":  {"ac", 1.0 / 4046.8564224}, // 1 acre = 4840 yd² exactly
}

// Convert returns the area in the named unit and its display symbol.
// Returns a non-nil error for unrecognized unit names.
func (a Area) Convert(unitName string) (val float64, sym string, err error) {
	u, ok := units[unitName]
	if !ok {
		return 0, "", fmt.Errorf("wi18n/area: unrecognized unit %q", unitName)
	}
	return a.SquareMeters * u.factor, u.sym, nil
}

// DefaultUnit returns the built-in display unit for the given BCP 47 locale.
// en-US defaults to ft²; all others default to m².
func DefaultUnit(locale string) string {
	if locale == "en-US" {
		return "ft2"
	}
	return "m2"
}

// DefaultDecimals returns the built-in decimal precision for a unit.
func DefaultDecimals(unitName string) int {
	switch unitName {
	case "m2", "cm2", "mm2", "ft2":
		return 0
	case "yd2", "in2":
		return 1
	}
	return 2 // km2, ha, mi2, ac
}
