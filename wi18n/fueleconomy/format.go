//go:build js && wasm

package fueleconomy

import (
	"fmt"

	"github.com/luisfurquim/wprana/wi18n"
)

// Format implements wi18n.Numerical. formatName selects the display unit
// ("l100km", "mpg", "mpgimp", "kml"); empty formatName uses the locale
// default.
func (fe FuelEconomy) Format(locale, formatName string) (string, error) {
	unitName := formatName
	if unitName == "" {
		if u, ok := wi18n.MeasureDefault("fueleconomy", locale); ok {
			unitName = u
		} else {
			unitName = DefaultUnit(locale)
		}
	}
	val, sym, err := fe.Convert(unitName)
	if err != nil {
		return "", err
	}
	decimals := DefaultDecimals(unitName)
	if d, ok := wi18n.UnitDecimals(locale, unitName); ok {
		decimals = d
	}
	return fmt.Sprintf("%.*f %s", decimals, val, sym), nil
}

var _ wi18n.Numerical = FuelEconomy{}
