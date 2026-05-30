//go:build js && wasm

package temperature

import (
	"fmt"

	"github.com/luisfurquim/wings/wi18n"
)

// Format implements wi18n.Numerical. formatName selects the display unit
// ("k"/"kelvin", "c"/"celsius", "f"/"fahrenheit", "r"/"rankine"); an empty
// formatName uses the locale default from wprana.json or the built-in table.
func (t Temperature) Format(locale, formatName string) (string, error) {
	unitName := formatName
	if unitName == "" {
		if u, ok := wi18n.MeasureDefault("temperature", locale); ok {
			unitName = u
		} else {
			unitName = DefaultUnit(locale)
		}
	}
	val, sym, err := t.Convert(unitName)
	if err != nil {
		return "", err
	}
	decimals := DefaultDecimals(unitName)
	if d, ok := wi18n.UnitDecimals(locale, unitName); ok {
		decimals = d
	}
	return fmt.Sprintf("%.*f %s", decimals, val, sym), nil
}

// Compile-time assertion: Temperature must satisfy wi18n.Numerical.
var _ wi18n.Numerical = Temperature{}
