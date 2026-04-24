//go:build js && wasm

package wi18n

// iso4217Decimals maps ISO 4217 currency codes to the number of fractional
// digits the currency uses. Curated from the ISO 4217 MA list.
//
// Most currencies have 2 decimal places; the entries below list the actual
// exceptions (0, 3, or 4 decimals). Lookup via currencyDecimals() returns 2
// as the default when a code is not in this map, which matches the vast
// majority of live currencies.
var iso4217Decimals = map[string]int{
	// Zero-decimal currencies
	"BIF": 0, // Burundian Franc
	"CLP": 0, // Chilean Peso
	"DJF": 0, // Djiboutian Franc
	"GNF": 0, // Guinean Franc
	"ISK": 0, // Icelandic Króna
	"JPY": 0, // Japanese Yen
	"KMF": 0, // Comorian Franc
	"KRW": 0, // South Korean Won
	"PYG": 0, // Paraguayan Guaraní
	"RWF": 0, // Rwandan Franc
	"UGX": 0, // Ugandan Shilling
	"UYI": 0, // Uruguay Peso en Unidades Indexadas
	"VND": 0, // Vietnamese Đồng
	"VUV": 0, // Vanuatu Vatu
	"XAF": 0, // Central African CFA Franc
	"XOF": 0, // West African CFA Franc
	"XPF": 0, // CFP Franc
	// Three-decimal currencies
	"BHD": 3, // Bahraini Dinar
	"IQD": 3, // Iraqi Dinar
	"JOD": 3, // Jordanian Dinar
	"KWD": 3, // Kuwaiti Dinar
	"LYD": 3, // Libyan Dinar
	"OMR": 3, // Omani Rial
	"TND": 3, // Tunisian Dinar
	// Four-decimal currencies (funds)
	"CLF": 4, // Unidad de Fomento
	"UYW": 4, // Unidad Previsional
}

// currencyDecimals returns the number of fractional digits for a given
// ISO 4217 code, defaulting to 2 when the code is unknown or empty.
func currencyDecimals(code string) int {
	if d, ok := iso4217Decimals[code]; ok {
		return d
	}
	return 2
}
