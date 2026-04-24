//go:build js && wasm

package wi18n

import (
	"strconv"
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
//  7. default → fmt-style fallback via wprana.NoFmtFmtPrinter.
func fmtPrinter(val any, locale, formatName string) string {
	if val == nil {
		return ""
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
		return v.Format(locale, formatName)
	}
	return wprana.NoFmtFmtPrinter(val, locale, formatName)
}

// formatInt renders an int64 through Intl.NumberFormat with no fractional
// digits. Falls back to strconv on any JS-side failure.
func formatInt(n int64, locale string) string {
	out, ok := safeNumberIntegerFormat(locale, float64(n))
	if !ok {
		return strconv.FormatInt(n, 10)
	}
	return out
}

// formatUint mirrors formatInt for unsigned values. Values above 2^53 lose
// precision through float64; the fallback path preserves the exact digits.
func formatUint(n uint64, locale string) string {
	if n <= 1<<53 {
		if out, ok := safeNumberIntegerFormat(locale, float64(n)); ok {
			return out
		}
	}
	return strconv.FormatUint(n, 10)
}

// formatFloat renders a float64 through Intl.NumberFormat with the locale's
// default fractional-digit behaviour (typically up to 3).
func formatFloat(f float64, locale string) string {
	intl := js.Global().Get("Intl")
	if !intl.Truthy() {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	out, ok := safeNumberFormatCall(locale, js.Null(), f)
	if !ok {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return out
}

// formatTime renders a Go time.Time via Intl.DateTimeFormat. Uses the
// locale's default date/time style; named formats will route here with a
// non-empty formatName in a later iteration.
func formatTime(t time.Time, locale string) string {
	intl := js.Global().Get("Intl")
	if !intl.Truthy() {
		return t.Format(time.RFC3339)
	}
	dateCtor := js.Global().Get("Date")
	if !dateCtor.Truthy() {
		return t.Format(time.RFC3339)
	}
	jsDate := dateCtor.New(t.UnixMilli())
	out, ok := safeDateFormatCall(locale, js.Null(), jsDate)
	if !ok {
		return t.Format(time.RFC3339)
	}
	return out
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
		if out, ok := safeDateFormatCall(locale, js.Null(), v); ok {
			return out
		}
	}
	return v.String()
}

// safeNumberIntegerFormat wraps Intl.NumberFormat with integer options.
// Returns (output, true) on success and ("", false) on any exception.
func safeNumberIntegerFormat(locale string, value float64) (string, bool) {
	intl := js.Global().Get("Intl")
	if !intl.Truthy() {
		return "", false
	}
	opts := js.Global().Get("Object").New()
	opts.Set("maximumFractionDigits", 0)
	return safeNumberFormatCall(locale, opts, value)
}

// safeNumberFormatCall invokes Intl.NumberFormat(locale, opts).format(value),
// trapping any JS exception via recover. opts may be js.Null() to request
// the locale's default number formatting.
func safeNumberFormatCall(locale string, opts js.Value, value float64) (out string, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			out, ok = "", false
		}
	}()
	nf := js.Global().Get("Intl").Get("NumberFormat").New(locale, opts)
	return nf.Call("format", value).String(), true
}

// safeDateFormatCall invokes Intl.DateTimeFormat(locale, opts).format(date),
// trapping any JS exception via recover.
func safeDateFormatCall(locale string, opts, date js.Value) (out string, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			out, ok = "", false
		}
	}()
	df := js.Global().Get("Intl").Get("DateTimeFormat").New(locale, opts)
	return df.Call("format", date).String(), true
}
