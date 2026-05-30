//go:build js && wasm

package speed

import (
	"fmt"

	"github.com/luisfurquim/wings/wi18n"
)

// Format implements wi18n.Numerical. formatName selects the display unit
// ("ms", "kmh", "mph", "kn", "fps"); empty formatName uses the locale default.
func (s Speed) Format(locale, formatName string) (string, error) {
	unitName := formatName
	if unitName == "" {
		if u, ok := wi18n.MeasureDefault("speed", locale); ok {
			unitName = u
		} else {
			unitName = DefaultUnit(locale)
		}
	}
	val, sym, err := s.Convert(unitName)
	if err != nil {
		return "", err
	}
	decimals := DefaultDecimals(unitName)
	if d, ok := wi18n.UnitDecimals(locale, unitName); ok {
		decimals = d
	}
	return fmt.Sprintf("%.*f %s", decimals, val, sym), nil
}

var _ wi18n.Numerical = Speed{}
