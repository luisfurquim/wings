// Package cooking provides locale-aware cooking measurement types for wprana
// templates.
//
// Two separate types cover the two axes of cooking measurement:
//   - [CookingVolume] for liquid/dry-volume measures (cup, tbsp, tsp, mL, L…)
//   - [CookingWeight] for mass measures (g, kg, oz, lb)
//
// Store values in their respective SI-adjacent canonical form (litres for
// volume, kilograms for weight) and bind them with %var:unit in templates:
//
//	{{%milk:cup}}   {{%flour:g}}
//
// When the unit suffix is omitted the locale default is used: cup/oz for
// en-US, mL/g for all others. Apps may override via wi18n.SetConfig
// (wprana.json) using quantity keys "cooking_volume" and "cooking_weight".
//
// Unit names safe for template use:
//
//	Volume: L  mL  cup  tbsp  tsp  floz
//	Weight: kg  g  lb   oz
package cooking

import "fmt"

// CookingVolume stores a cooking volume in litres.
type CookingVolume struct{ Liters float64 }

// CookingWeight stores a cooking mass in kilograms.
type CookingWeight struct{ Kilograms float64 }

// usGallon is the exact US liquid gallon in litres.
const usGallon = 3.785411784

// lbKg is the exact pound-to-kilogram conversion.
const lbKg = 0.45359237

type volUnit struct {
	sym    string
	factor float64 // Liters * factor = value-in-unit
}

var volUnits = map[string]volUnit{
	"L":    {"L", 1},
	"mL":   {"mL", 1000},
	"cup":  {"cup", 16.0 / usGallon},    // 1 US cup = gallon/16
	"tbsp": {"tbsp", 256.0 / usGallon},  // 1 US tbsp = gallon/256
	"tsp":  {"tsp", 768.0 / usGallon},   // 1 US tsp  = gallon/768
	"floz": {"fl oz", 128.0 / usGallon}, // 1 US fl oz = gallon/128
}

type wgtUnit struct {
	sym    string
	factor float64 // Kilograms * factor = value-in-unit
}

var wgtUnits = map[string]wgtUnit{
	"kg": {"kg", 1},
	"g":  {"g", 1000},
	"lb": {"lb", 1.0 / lbKg},
	"oz": {"oz", 16.0 / lbKg},
}

// Convert returns the volume in the named unit and its display symbol.
func (cv CookingVolume) Convert(unitName string) (val float64, sym string, err error) {
	u, ok := volUnits[unitName]
	if !ok {
		return 0, "", fmt.Errorf("wi18n/cooking: unrecognized volume unit %q", unitName)
	}
	return cv.Liters * u.factor, u.sym, nil
}

// Convert returns the mass in the named unit and its display symbol.
func (cw CookingWeight) Convert(unitName string) (val float64, sym string, err error) {
	u, ok := wgtUnits[unitName]
	if !ok {
		return 0, "", fmt.Errorf("wi18n/cooking: unrecognized weight unit %q", unitName)
	}
	return cw.Kilograms * u.factor, u.sym, nil
}

// DefaultVolumeUnit returns the built-in display unit for cooking volumes.
// en-US defaults to cup; all others default to mL.
func DefaultVolumeUnit(locale string) string {
	if locale == "en-US" {
		return "cup"
	}
	return "mL"
}

// DefaultWeightUnit returns the built-in display unit for cooking weights.
// en-US defaults to oz; all others default to g.
func DefaultWeightUnit(locale string) string {
	if locale == "en-US" {
		return "oz"
	}
	return "g"
}

// DefaultVolumeDecimals returns the built-in decimal precision for a volume unit.
func DefaultVolumeDecimals(unitName string) int {
	if unitName == "mL" {
		return 0
	}
	return 2 // L, cup, tbsp, tsp, floz
}

// DefaultWeightDecimals returns the built-in decimal precision for a weight unit.
func DefaultWeightDecimals(unitName string) int {
	if unitName == "g" {
		return 0
	}
	return 2 // kg, lb, oz
}
