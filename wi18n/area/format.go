//go:build js && wasm

package area

import (
	"fmt"

	"github.com/luisfurquim/wprana/wi18n"
)

// Format implements wi18n.Numerical. formatName selects the display unit
// ("m2", "km2", "cm2", "mm2", "ha", "mi2", "ft2", "yd2", "in2", "ac");
// empty formatName uses the locale default.
func (a Area) Format(locale, formatName string) (string, error) {
	unitName := formatName
	if unitName == "" {
		if u, ok := wi18n.MeasureDefault("area", locale); ok {
			unitName = u
		} else {
			unitName = DefaultUnit(locale)
		}
	}
	val, sym, err := a.Convert(unitName)
	if err != nil {
		return "", err
	}
	decimals := DefaultDecimals(unitName)
	if d, ok := wi18n.UnitDecimals(locale, unitName); ok {
		decimals = d
	}
	return fmt.Sprintf("%.*f %s", decimals, val, sym), nil
}

var _ wi18n.Numerical = Area{}
