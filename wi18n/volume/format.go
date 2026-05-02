//go:build js && wasm

package volume

import (
	"fmt"

	"github.com/luisfurquim/wprana/wi18n"
)

// Format implements wi18n.Numerical. formatName selects the display unit
// ("L", "mL", "dL", "m3", "floz", "pt", "qt", "gal", "galimp"); empty
// formatName uses the locale default.
func (v Volume) Format(locale, formatName string) (string, error) {
	unitName := formatName
	if unitName == "" {
		if u, ok := wi18n.MeasureDefault("volume", locale); ok {
			unitName = u
		} else {
			unitName = DefaultUnit(locale)
		}
	}
	val, sym, err := v.Convert(unitName)
	if err != nil {
		return "", err
	}
	decimals := DefaultDecimals(unitName)
	if d, ok := wi18n.UnitDecimals(locale, unitName); ok {
		decimals = d
	}
	return fmt.Sprintf("%.*f %s", decimals, val, sym), nil
}

var _ wi18n.Numerical = Volume{}
