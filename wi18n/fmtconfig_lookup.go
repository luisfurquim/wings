//go:build js && wasm

package wi18n

// UnitDecimals returns the decimal precision configured for the given unit
// name in the named locale's fmt config. Returns (0, false) when no
// configuration exists for that locale/unit combination; callers provide their
// own default.
//
// This reads the per-locale bundle state (bundles, populated at SetLang time),
// so it stays GOOS=js only; the pure parser in fmtconfig.go is native-testable.
func UnitDecimals(locale, formatName string) (int, bool) {
	bundleMu.RLock()
	b, ok := bundles[locale]
	bundleMu.RUnlock()
	if !ok || b.fmtCfg == nil {
		return 0, false
	}
	d, ok := b.fmtCfg.units[formatName]
	return d, ok
}

// NamedFmt returns the Intl format type and options for a named scalar format
// in the given locale. Returns ("", nil, false) when not configured.
func NamedFmt(locale, formatName string) (fmtType string, opts map[string]any, ok bool) {
	bundleMu.RLock()
	b, found := bundles[locale]
	bundleMu.RUnlock()
	if !found || b.fmtCfg == nil {
		return "", nil, false
	}
	e, ok := b.fmtCfg.named[formatName]
	return e.FmtType, e.Options, ok
}
