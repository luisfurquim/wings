//go:build js && wasm

package wi18n

import "github.com/luisfurquim/goose"

var G goose.Alert = goose.Alert(2)

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
// comes from the `%var:formatName` template syntax (empty string when the
// template uses bare `%var`).
//
// Error semantics: returning a non-nil error signals that rendering must
// stop — FmtPrinter will discard the returned string, log the error with
// template context, and emit an empty string for the binding. Returning
// ("", nil) produces an empty string without triggering the error path.
// The implementation is free to log domain-level detail before returning
// the error; FmtPrinter adds the locale/formatName context on top.
type Numerical interface {
	Format(locale, formatName string) (string, error)
}
