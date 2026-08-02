//go:build js && wasm

package wi18n

import (
	"sync"
	"syscall/js"

	"github.com/luisfurquim/wings/internal/jsguard"
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
	// Named date/time styles use namedFmtKey + namedFmtCache; this key
	// covers only the locale-default rendering path.
}

// namedFmtKey identifies a formatter built from a named entry in
// <lang>.fmt.json. The locale+name pair is unique within a session because
// the fmtConfig for a given locale is immutable after load.
type namedFmtKey struct {
	locale string
	name   string
}

var (
	numberFmtMu    sync.RWMutex
	numberFmtCache = map[numberFmtKey]js.Value{}

	dateFmtMu    sync.RWMutex
	dateFmtCache = map[dateFmtKey]js.Value{}

	namedFmtMu    sync.RWMutex
	namedFmtCache = map[namedFmtKey]js.Value{}
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

func buildNumberFormatter(key numberFmtKey) js.Value {
	return guardedFormatter("Intl.NumberFormat", func() js.Value {
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
	})
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

func buildDateFormatter(key dateFmtKey) js.Value {
	return guardedFormatter("Intl.DateTimeFormat", func() js.Value {
		intl := js.Global().Get("Intl")
		if !intl.Truthy() {
			return js.Undefined()
		}
		return intl.Get("DateTimeFormat").New(key.locale)
	})
}

// formatNumber renders value through the cached formatter for key. Returns
// ("", false) when the formatter is unavailable or .format() throws.
func formatNumber(key numberFmtKey, value float64) (string, bool) {
	return guardedFormat("Intl.NumberFormat.format", func() (string, bool) {
		nf := getNumberFormatter(key)
		if !nf.Truthy() {
			return "", false
		}
		return nf.Call("format", value).String(), true
	})
}

// formatDate renders value through the cached date formatter for key.
// Returns ("", false) when the formatter is unavailable or .format() throws.
func formatDate(key dateFmtKey, value js.Value) (string, bool) {
	return guardedFormat("Intl.DateTimeFormat.format", func() (string, bool) {
		df := getDateFormatter(key)
		if !df.Truthy() {
			return "", false
		}
		return df.Call("format", value).String(), true
	})
}

// getNamedFormatter returns the Intl formatter built from a named entry in
// <lang>.fmt.json. fmtType is "number" or "date"; opts are the raw Intl
// options from the entry. On first call for a (locale, name) pair the
// formatter is constructed and cached; subsequent calls return the cached
// value regardless of fmtType/opts (the entry is immutable after load).
func getNamedFormatter(locale, name, fmtType string, opts map[string]any) js.Value {
	key := namedFmtKey{locale: locale, name: name}
	namedFmtMu.RLock()
	nf, ok := namedFmtCache[key]
	namedFmtMu.RUnlock()
	if ok {
		return nf
	}
	nf = buildNamedFormatter(locale, fmtType, opts)
	namedFmtMu.Lock()
	namedFmtCache[key] = nf
	namedFmtMu.Unlock()
	return nf
}

func buildNamedFormatter(locale, fmtType string, opts map[string]any) js.Value {
	return guardedFormatter("Intl named "+fmtType, func() js.Value {
		intl := js.Global().Get("Intl")
		if !intl.Truthy() {
			return js.Undefined()
		}
		jsOpts := js.Global().Get("Object").New()
		for k, v := range opts {
			jsOpts.Set(k, v)
		}
		switch fmtType {
		case "number":
			return intl.Get("NumberFormat").New(locale, jsOpts)
		case "date":
			return intl.Get("DateTimeFormat").New(locale, jsOpts)
		}
		return js.Undefined()
	})
}

// formatNumberNamed renders value through the named Intl.NumberFormat for
// (locale, name). Returns ("", false) on any failure.
func formatNumberNamed(locale, name, fmtType string, opts map[string]any, value float64) (string, bool) {
	return guardedFormat("Intl named number format", func() (string, bool) {
		nf := getNamedFormatter(locale, name, fmtType, opts)
		if !nf.Truthy() {
			return "", false
		}
		return nf.Call("format", value).String(), true
	})
}

// formatDateNamed renders value through the named Intl.DateTimeFormat for
// (locale, name). Returns ("", false) on any failure.
func formatDateNamed(locale, name, fmtType string, opts map[string]any, value js.Value) (string, bool) {
	return guardedFormat("Intl named date format", func() (string, bool) {
		df := getNamedFormatter(locale, name, fmtType, opts)
		if !df.Truthy() {
			return "", false
		}
		return df.Call("format", value).String(), true
	})
}

// guardedFormatter builds an Intl formatter under jsguard, reporting a
// failure instead of swallowing it.
//
// Every one of these used to recover inline and return undefined in
// SILENCE, so a locale or an option the browser refused left numbers and
// dates rendering wrong with nothing anywhere to say why — the exact
// failure shape this project keeps having to hunt down. An undefined
// formatter is still the fallback; it is just no longer a secret.
func guardedFormatter(op string, build func() js.Value) js.Value {
	v, err := jsguard.Value(op, build)
	if err != nil {
		G.Logf(1, "wi18n: %v; that value falls back to unformatted\n", err)
		return js.Undefined()
	}
	return v
}

// guardedFormat runs a formatter call under jsguard. The (string, bool)
// shape is the caller's contract — false means "render it unformatted".
func guardedFormat(op string, format func() (string, bool)) (string, bool) {
	type result struct {
		text string
		ok   bool
	}
	r, err := jsguard.Value(op, func() result {
		text, ok := format()
		return result{text, ok}
	})
	if err != nil {
		G.Logf(1, "wi18n: %v; that value falls back to unformatted\n", err)
		return "", false
	}
	return r.text, r.ok
}
