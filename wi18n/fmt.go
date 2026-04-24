//go:build js && wasm

package wi18n

// Numerical is the extension point for locale-aware formatting of values
// whose type is not known to wi18n.
//
// wi18n ships direct support for Go's native numeric types (int, int64,
// float64, …), time.Time, and js.Value dates. For any other value, the
// FmtPrinter type switch dispatches through this interface — the type
// implementing Numerical is responsible for producing its own
// locale-appropriate string.
//
// An application that wants locale-specific behavior for a domain type
// implements Numerical directly on that type (or on a wrapper around a
// wi18n-provided type such as Currency). There is no registration step:
// satisfying the interface is the registration.
//
// locale is the BCP 47 tag currently in effect (e.g. "pt-BR"). formatName
// is reserved for the future `%var:formatName` template syntax — in this
// release it is always the empty string; implementations should treat any
// value they do not recognize as equivalent to the default.
type Numerical interface {
	Format(locale, formatName string) string
}
