//go:build js && wasm

package length

import (
	"fmt"

	"github.com/luisfurquim/wprana/wi18n"
)

// Format implements wi18n.Numerical. formatName selects the display unit
// (e.g. "km", "mi", "ft", "in"); an empty formatName uses the locale default
// from wprana.json or the built-in table.
//
// The returned string uses "." as the decimal separator regardless of locale;
// number localisation (grouping, decimal sign) can be layered on top by
// wrapping Length in a custom type whose Format delegates here and post-
// processes the result.
func (l Length) Format(locale, formatName string) (string, error) {
	unitName := formatName
	if unitName == "" {
		if u, ok := wi18n.MeasureDefault("length", locale); ok {
			unitName = u
		} else {
			unitName = DefaultUnit(locale)
		}
	}
	val, sym, err := l.Convert(unitName)
	if err != nil {
		return "", err
	}
	decimals := DefaultDecimals(unitName)
	if d, ok := wi18n.UnitDecimals(locale, unitName); ok {
		decimals = d
	}
	return fmt.Sprintf("%.*f %s", decimals, val, sym), nil
}

// Compile-time assertion: Length must satisfy wi18n.Numerical.
var _ wi18n.Numerical = Length{}
