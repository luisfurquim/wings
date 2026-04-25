//go:build js && wasm

package wi18n

import (
	"sync"
	"syscall/js"
)

// Intl instances are reused across calls because their construction is
// expensive (each `new Intl.NumberFormat(locale, opts)` crosses the
// syscall/js boundary and parses CLDR locale data internally) while the
// .format() method is essentially free. Cache keys capture every parameter
// that influences the formatter's output; values are the constructed
// js.Value (or js.Undefined() if construction failed).
//
// Cache entries are NOT invalidated on SetLang. Switching back and forth
// between locales reuses warm formatters, which matches the design
// rationale: switching is rare but happens, and old formatters are still
// useful when the user toggles back.

type numberFmtKey struct {
	locale     string
	style      string // "" for plain decimal, "currency" for monetary
	currency   string // ISO 4217 code; empty when style != "currency"
	fracDigits int    // maximumFractionDigits (and minimum, when currency)
}

type dateFmtKey struct {
	locale string
	// Reserved for future named date/time styles; today only the locale
	// varies, so a single field suffices.
}

var (
	numberFmtMu    sync.RWMutex
	numberFmtCache = map[numberFmtKey]js.Value{}

	dateFmtMu    sync.RWMutex
	dateFmtCache = map[dateFmtKey]js.Value{}
)

// getNumberFormatter returns the Intl.NumberFormat instance matching key,
// constructing and caching it on first request. The returned value is
// js.Undefined() (Truthy()==false) when Intl is unavailable or the
// constructor threw — callers must check before using.
func getNumberFormatter(key numberFmtKey) js.Value {
	numberFmtMu.RLock()
	nf, ok := numberFmtCache[key]
	numberFmtMu.RUnlock()
	if ok {
		return nf
	}
	nf = buildNumberFormatter(key)
	numberFmtMu.Lock()
	numberFmtCache[key] = nf
	numberFmtMu.Unlock()
	return nf
}

func buildNumberFormatter(key numberFmtKey) (out js.Value) {
	defer func() {
		if r := recover(); r != nil {
			out = js.Undefined()
		}
	}()
	intl := js.Global().Get("Intl")
	if !intl.Truthy() {
		return js.Undefined()
	}
	opts := js.Global().Get("Object").New()
	if key.style == "currency" {
		opts.Set("style", "currency")
		opts.Set("currency", key.currency)
		opts.Set("minimumFractionDigits", key.fracDigits)
		opts.Set("maximumFractionDigits", key.fracDigits)
	} else {
		opts.Set("maximumFractionDigits", key.fracDigits)
	}
	return intl.Get("NumberFormat").New(key.locale, opts)
}

// getDateFormatter mirrors getNumberFormatter for Intl.DateTimeFormat.
func getDateFormatter(key dateFmtKey) js.Value {
	dateFmtMu.RLock()
	df, ok := dateFmtCache[key]
	dateFmtMu.RUnlock()
	if ok {
		return df
	}
	df = buildDateFormatter(key)
	dateFmtMu.Lock()
	dateFmtCache[key] = df
	dateFmtMu.Unlock()
	return df
}

func buildDateFormatter(key dateFmtKey) (out js.Value) {
	defer func() {
		if r := recover(); r != nil {
			out = js.Undefined()
		}
	}()
	intl := js.Global().Get("Intl")
	if !intl.Truthy() {
		return js.Undefined()
	}
	return intl.Get("DateTimeFormat").New(key.locale)
}

// formatNumber renders value through the cached formatter for key. Returns
// ("", false) when the formatter is unavailable or .format() throws.
func formatNumber(key numberFmtKey, value float64) (out string, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			out, ok = "", false
		}
	}()
	nf := getNumberFormatter(key)
	if !nf.Truthy() {
		return "", false
	}
	return nf.Call("format", value).String(), true
}

// formatDate renders value through the cached date formatter for key.
// Returns ("", false) when the formatter is unavailable or .format() throws.
func formatDate(key dateFmtKey, value js.Value) (out string, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			out, ok = "", false
		}
	}()
	df := getDateFormatter(key)
	if !df.Truthy() {
		return "", false
	}
	return df.Call("format", value).String(), true
}
