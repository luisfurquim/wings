// Package volume provides a locale-aware volume type for wprana templates.
//
// Store volumes as [Volume]{Liters: v} and bind them with %var:unit
// (e.g. {{%capacity:mL}}). When the unit suffix is omitted the locale
// default is used (US gallons for en-US, litres for all others).
//
// Unit names safe for template use:
//
//	L      → L (litre)     gal    → gal (US gallon)
//	mL     → mL            galimp → gal (Imperial)
//	dL     → dL            pt     → pt  (US pint)
//	m3     → m³            qt     → qt  (US quart)
//	floz   → fl oz (US)
package volume

import "fmt"

// Volume stores a volume in litres (canonical unit — pragmatic SI exception;
// m³ is impractical for everyday quantities).
type Volume struct{ Liters float64 }

// usGallon is the exact US liquid gallon in litres (NIST SP 811).
const usGallon = 3.785411784

type unit struct {
	sym    string
	factor float64 // Liters * factor = value-in-unit
}

var units = map[string]unit{
	"L":      {"L", 1},
	"mL":     {"mL", 1000},
	"dL":     {"dL", 10},
	"m3":     {"m³", 0.001},
	"floz":   {"fl oz", 128.0 / usGallon},
	"pt":     {"pt", 2.0 / usGallon * (usGallon / 2)}, // US liquid pint = gallon/8
	"qt":     {"qt", 4.0 / usGallon * (usGallon / 4)}, // initialised below
	"gal":    {"gal", 1.0 / usGallon},
	"galimp": {"gal", 1.0 / 4.54609}, // Imperial gallon (exact)
}

func init() {
	// US liquid pint = gallon/8, US quart = gallon/4
	units["pt"] = unit{"pt", 8.0 / usGallon}
	units["qt"] = unit{"qt", 4.0 / usGallon}
}

// Convert returns the volume in the named unit and its display symbol.
// Returns a non-nil error for unrecognized unit names.
func (v Volume) Convert(unitName string) (val float64, sym string, err error) {
	u, ok := units[unitName]
	if !ok {
		return 0, "", fmt.Errorf("wi18n/volume: unrecognized unit %q", unitName)
	}
	return v.Liters * u.factor, u.sym, nil
}

// DefaultUnit returns the built-in display unit for the given BCP 47 locale.
// en-US defaults to US gallons; all others default to litres.
func DefaultUnit(locale string) string {
	if locale == "en-US" {
		return "gal"
	}
	return "L"
}

// DefaultDecimals returns the built-in decimal precision for a unit.
func DefaultDecimals(unitName string) int {
	switch unitName {
	case "mL", "m3":
		return 0
	case "dL", "floz":
		return 1
	}
	return 2 // L, gal, galimp, pt, qt
}
