//go:build js && wasm

package cooking

import (
	"fmt"

	"github.com/luisfurquim/wings/wi18n"
)

// Format implements wi18n.Numerical for cooking volumes. formatName selects
// the display unit ("L", "mL", "cup", "tbsp", "tsp", "floz"); empty
// formatName uses the locale default from wprana.json ("cooking_volume") or
// the built-in table.
func (cv Volume) Format(locale, formatName string) (string, error) {
	unitName := formatName
	if unitName == "" {
		if u, ok := wi18n.MeasureDefault("cooking_volume", locale); ok {
			unitName = u
		} else {
			unitName = DefaultVolumeUnit(locale)
		}
	}
	val, sym, err := cv.Convert(unitName)
	if err != nil {
		return "", err
	}
	decimals := DefaultVolumeDecimals(unitName)
	if d, ok := wi18n.UnitDecimals(locale, unitName); ok {
		decimals = d
	}
	return fmt.Sprintf("%.*f %s", decimals, val, sym), nil
}

// Format implements wi18n.Numerical for cooking weights. formatName selects
// the display unit ("kg", "g", "lb", "oz"); empty formatName uses the locale
// default from wprana.json ("cooking_weight") or the built-in table.
func (cw Weight) Format(locale, formatName string) (string, error) {
	unitName := formatName
	if unitName == "" {
		if u, ok := wi18n.MeasureDefault("cooking_weight", locale); ok {
			unitName = u
		} else {
			unitName = DefaultWeightUnit(locale)
		}
	}
	val, sym, err := cw.Convert(unitName)
	if err != nil {
		return "", err
	}
	decimals := DefaultWeightDecimals(unitName)
	if d, ok := wi18n.UnitDecimals(locale, unitName); ok {
		decimals = d
	}
	return fmt.Sprintf("%.*f %s", decimals, val, sym), nil
}

var _ wi18n.Numerical = Volume{}
var _ wi18n.Numerical = Weight{}
