// Package fueleconomy provides a locale-aware fuel economy type for wprana
// templates.
//
// Store fuel consumption as [FuelEconomy]{LitersPer100km: v} and bind with
// %var:unit (e.g. {{%economy:mpg}}). When the unit suffix is omitted the
// locale default is used (mpg for en-US, mpg-imp for en-GB, L/100km for
// all others).
//
// IMPORTANT — mpg and kml are INVERSELY proportional to L/100km:
// a more efficient car has a smaller L/100km but a larger mpg value.
//
// Unit names safe for template use:
//
//	l100km  → L/100km (canonical)
//	mpg     → mpg (US miles per gallon)
//	mpgimp  → mpg (Imperial miles per gallon)
//	kml     → km/L
package fueleconomy

import "fmt"

// FuelEconomy stores fuel consumption in litres per 100 kilometres.
// This is the SI-adjacent canonical form (canonical SI would be m², but
// L/100km is the universal practical standard outside the US/UK).
type FuelEconomy struct{ LitersPer100km float64 }

// mpgUS and mpgImp are the exact conversion constants:
//
//	L/100km = factor / mpg   →   mpg = factor / (L/100km)
const (
	mpgUS  = 100.0 * 3.785411784 / 1.609344 // ≈ 235.2145833
	mpgImp = 100.0 * 4.54609 / 1.609344     // ≈ 282.4809363
)

// Convert returns the fuel economy in the named unit and its display symbol.
// Returns a non-nil error for unrecognized unit names or when LitersPer100km
// is ≤ 0 for inverse units (mpg, mpgimp, kml).
func (fe FuelEconomy) Convert(unitName string) (val float64, sym string, err error) {
	x := fe.LitersPer100km
	switch unitName {
	case "l100km":
		return x, "L/100km", nil
	case "mpg":
		if x <= 0 {
			return 0, "", fmt.Errorf("wi18n/fueleconomy: LitersPer100km must be > 0 for mpg conversion, got %g", x)
		}
		return mpgUS / x, "mpg", nil
	case "mpgimp":
		if x <= 0 {
			return 0, "", fmt.Errorf("wi18n/fueleconomy: LitersPer100km must be > 0 for mpgimp conversion, got %g", x)
		}
		return mpgImp / x, "mpg", nil
	case "kml":
		if x <= 0 {
			return 0, "", fmt.Errorf("wi18n/fueleconomy: LitersPer100km must be > 0 for kml conversion, got %g", x)
		}
		return 100.0 / x, "km/L", nil
	default:
		return 0, "", fmt.Errorf("wi18n/fueleconomy: unrecognized unit %q", unitName)
	}
}

// DefaultUnit returns the built-in display unit for the given BCP 47 locale.
// en-US → mpg, en-GB → mpgimp, all others → l100km.
func DefaultUnit(locale string) string {
	switch locale {
	case "en-US":
		return "mpg"
	case "en-GB":
		return "mpgimp"
	}
	return "l100km"
}

// DefaultDecimals returns the built-in decimal precision for a unit.
func DefaultDecimals(_ string) int { return 1 }
