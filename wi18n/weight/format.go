//go:build js && wasm

package weight

import (
	"fmt"

	"github.com/luisfurquim/wings/wi18n"
)

// Format implements wi18n.Numerical. formatName selects the display unit
// ("kg", "g", "mg", "t", "lb", "oz", "st"); empty formatName uses the locale
// default.
func (w Weight) Format(locale, formatName string) (string, error) {
	unitName := formatName
	if unitName == "" {
		if u, ok := wi18n.MeasureDefault("weight", locale); ok {
			unitName = u
		} else {
			unitName = DefaultUnit(locale)
		}
	}
	val, sym, err := w.Convert(unitName)
	if err != nil {
		return "", err
	}
	decimals := DefaultDecimals(unitName)
	if d, ok := wi18n.UnitDecimals(locale, unitName); ok {
		decimals = d
	}
	return fmt.Sprintf("%.*f %s", decimals, val, sym), nil
}

var _ wi18n.Numerical = Weight{}
