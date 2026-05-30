// Package temperature provides a locale-aware temperature type for wings
// templates.
//
// Store temperatures as [Temperature]{Kelvin: v} and bind them with
// %var:unit in templates (e.g. {{%temp:c}}). When the unit suffix is omitted,
// the locale default is used (Fahrenheit for en-US and a handful of other
// locales, Celsius for all others). Apps may override the per-locale default
// via wi18n.SetConfig (wings.json).
//
// Canonical storage unit is Kelvin (SI). Conversions to Celsius, Fahrenheit,
// and Rankine are exact arithmetic; no approximations beyond float64.
package temperature

import "fmt"

// Temperature stores a temperature in Kelvin (SI canonical unit).
type Temperature struct{ Kelvin float64 }

// unit bundles the display symbol and the conversion function from Kelvin.
type unit struct {
	sym     string
	convert func(k float64) float64
}

var units = map[string]unit{
	"k":          {"K", func(k float64) float64 { return k }},
	"kelvin":     {"K", func(k float64) float64 { return k }},
	"c":          {"°C", func(k float64) float64 { return k - 273.15 }},
	"celsius":    {"°C", func(k float64) float64 { return k - 273.15 }},
	"f":          {"°F", func(k float64) float64 { return (k-273.15)*9/5 + 32 }},
	"fahrenheit": {"°F", func(k float64) float64 { return (k-273.15)*9/5 + 32 }},
	"r":          {"°R", func(k float64) float64 { return k * 9 / 5 }},
	"rankine":    {"°R", func(k float64) float64 { return k * 9 / 5 }},
}

// fromUnits maps unit names to their inverse conversion: input value → Kelvin.
var fromUnits = map[string]func(float64) float64{
	"k":          func(v float64) float64 { return v },
	"kelvin":     func(v float64) float64 { return v },
	"c":          func(v float64) float64 { return v + 273.15 },
	"celsius":    func(v float64) float64 { return v + 273.15 },
	"f":          func(v float64) float64 { return (v-32)*5/9 + 273.15 },
	"fahrenheit": func(v float64) float64 { return (v-32)*5/9 + 273.15 },
	"r":          func(v float64) float64 { return v * 5 / 9 },
	"rankine":    func(v float64) float64 { return v * 5 / 9 },
}

// New constructs a Temperature from a value in unitName, converting to Kelvin.
// Example: temperature.New(98.6, "f") returns Temperature{Kelvin: 310.15}.
func New(val float64, unitName string) (Temperature, error) {
	fn, ok := fromUnits[unitName]
	if !ok {
		return Temperature{}, fmt.Errorf("wi18n/temperature: unrecognized unit %q", unitName)
	}
	return Temperature{Kelvin: fn(val)}, nil
}

// Convert returns the temperature expressed in the named unit and its display
// symbol. Returns a non-nil error for unrecognized unit names.
func (t Temperature) Convert(unitName string) (val float64, sym string, err error) {
	u, ok := units[unitName]
	if !ok {
		return 0, "", fmt.Errorf("wi18n/temperature: unrecognized unit %q", unitName)
	}
	return u.convert(t.Kelvin), u.sym, nil
}

// DefaultUnit returns the built-in display unit for the given BCP 47 locale
// tag. en-US, en-BS, en-BZ, and en-KY default to Fahrenheit; all others
// default to Celsius. Apps may override this via wi18n.SetConfig (wings.json).
func DefaultUnit(locale string) string {
	switch locale {
	case "en-US", "en-BS", "en-BZ", "en-KY":
		return "f"
	}
	return "c"
}

// DefaultDecimals returns the built-in decimal precision for a unit.
func DefaultDecimals(unitName string) int {
	switch unitName {
	case "k", "kelvin":
		return 2
	}
	return 1 // c, celsius, f, fahrenheit, r, rankine
}
