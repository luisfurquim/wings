//go:build js && wasm

package wi18n

import (
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/luisfurquim/wprana"
)

// fmtPrinter is installed into wprana.FmtPrinter at init time. It renders a
// resolved %var value into its locale-appropriate form, dispatching on the
// Go type of the value.
//
// Order of the type switch matters:
//  1. nil → empty string.
//  2. Native integer types → Intl.NumberFormat (no decimals).
//  3. Native float types → Intl.NumberFormat (default precision).
//  4. time.Time → Intl.DateTimeFormat via epoch ms.
//  5. js.Value pointing at a JS Date → Intl.DateTimeFormat directly.
//  6. Numerical interface → value.Format(locale, formatName) — the app
//     extension point. Any Go-side type that wants locale-aware behavior
//     implements Numerical; Currency is wi18n's built-in example.
//     A non-nil error stops rendering: the string is discarded and ""
//     is returned after logging with locale/formatName context.
//  7. default → fmt-style fallback via wprana.NoFmtFmtPrinter.
func fmtPrinter(val any, locale, formatName string) string {
	if val == nil {
		return ""
	}
	if formatName != "" {
		if s, ok := applyNamedFormat(val, locale, formatName); ok {
			return s
		}
	}
	switch v := val.(type) {
	case int:
		return formatInt(int64(v), locale)
	case int8:
		return formatInt(int64(v), locale)
	case int16:
		return formatInt(int64(v), locale)
	case int32:
		return formatInt(int64(v), locale)
	case int64:
		return formatInt(v, locale)
	case uint:
		return formatUint(uint64(v), locale)
	case uint8:
		return formatUint(uint64(v), locale)
	case uint16:
		return formatUint(uint64(v), locale)
	case uint32:
		return formatUint(uint64(v), locale)
	case uint64:
		return formatUint(v, locale)
	case float32:
		return formatFloat(float64(v), locale)
	case float64:
		return formatFloat(v, locale)
	case time.Time:
		return formatTime(v, locale)
	case js.Value:
		return formatJSDate(v, locale)
	case Numerical:
		s, err := v.Format(locale, formatName)
		if err != nil {
			G.Printf(1, "wi18n: FmtPrinter: %v (locale=%q formatName=%q)\n", err, locale, formatName)
			return ""
		}
		return s
	}
	return wprana.NoFmtFmtPrinter(val, locale, formatName)
}

// formatInt renders an int64 through Intl.NumberFormat with no fractional
// digits. Falls back to strconv on any JS-side failure.
func formatInt(n int64, locale string) string {
	if out, ok := formatNumber(numberFmtKey{locale: locale, fracDigits: 0}, float64(n)); ok {
		return out
	}
	return strconv.FormatInt(n, 10)
}

// formatUint mirrors formatInt for unsigned values. Values above 2^53 lose
// precision through float64; the fallback path preserves the exact digits.
func formatUint(n uint64, locale string) string {
	if n <= 1<<53 {
		if out, ok := formatNumber(numberFmtKey{locale: locale, fracDigits: 0}, float64(n)); ok {
			return out
		}
	}
	return strconv.FormatUint(n, 10)
}

// formatFloat renders a float64 through Intl.NumberFormat. The fractional
// digit count is taken from the input value's own minimal representation
// (via strconv) instead of Intl's default of 3, so callers see the
// precision they passed in. Capped at 20 because the ECMAScript spec
// limits maximumFractionDigits to that range.
func formatFloat(f float64, locale string) string {
	if out, ok := formatNumber(numberFmtKey{locale: locale, fracDigits: floatDecimals(f)}, f); ok {
		return out
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// floatDecimals returns the number of fractional digits in the minimal
// decimal representation of f, clamped to [0, 20] (Intl's spec limit).
func floatDecimals(f float64) int {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	i := strings.IndexByte(s, '.')
	if i < 0 {
		return 0
	}
	n := len(s) - i - 1
	if n > 20 {
		n = 20
	}
	return n
}

// formatTime renders a Go time.Time via Intl.DateTimeFormat using the
// locale's default date/time style. Named formats are handled by
// applyNamedFormat before this function is reached.
func formatTime(t time.Time, locale string) string {
	dateCtor := js.Global().Get("Date")
	if !dateCtor.Truthy() {
		return t.Format(time.RFC3339)
	}
	jsDate := dateCtor.New(t.UnixMilli())
	if out, ok := formatDate(dateFmtKey{locale: locale}, jsDate); ok {
		return out
	}
	return t.Format(time.RFC3339)
}

// formatJSDate handles the case where the app stored a js.Value holding a
// browser-native Date object. Any other JS value is stringified with
// String() to match the default-branch behaviour.
func formatJSDate(v js.Value, locale string) string {
	if !v.Truthy() {
		return ""
	}
	dateProto := js.Global().Get("Date")
	if dateProto.Truthy() && v.InstanceOf(dateProto) {
		if out, ok := formatDate(dateFmtKey{locale: locale}, v); ok {
			return out
		}
	}
	return v.String()
}

// applyNamedFormat tries to render val using a named Intl format entry from
// <lang>.fmt.json. Returns ("", false) when the formatName is not configured
// for the current locale, or when val cannot be coerced to the format's
// required input type. The caller falls through to the normal type switch on
// false.
func applyNamedFormat(val any, locale, formatName string) (string, bool) {
	fmtType, opts, ok := NamedFmt(locale, formatName)
	if !ok {
		return "", false
	}
	switch fmtType {
	case "number":
		if f, ok2 := scalarToFloat64(val); ok2 {
			if out, ok3 := formatNumberNamed(locale, formatName, fmtType, opts, f); ok3 {
				return out, true
			}
		}
	case "date":
		dateCtor := js.Global().Get("Date")
		switch v := val.(type) {
		case time.Time:
			if dateCtor.Truthy() {
				jsDate := dateCtor.New(v.UnixMilli())
				if out, ok2 := formatDateNamed(locale, formatName, fmtType, opts, jsDate); ok2 {
					return out, true
				}
			}
		case js.Value:
			if dateCtor.Truthy() && v.InstanceOf(dateCtor) {
				if out, ok2 := formatDateNamed(locale, formatName, fmtType, opts, v); ok2 {
					return out, true
				}
			}
		}
	}
	return "", false
}

// scalarToFloat64 coerces a native numeric value to float64 for use with
// named Intl.NumberFormat entries. Returns (0, false) for non-numeric types.
func scalarToFloat64(val any) (float64, bool) {
	switch v := val.(type) {
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	}
	return 0, false
}
