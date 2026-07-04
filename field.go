package wings

import "fmt"

// FieldCodec lets a bound value own the string<->value conversion for a
// two-way `&value` binding. When the value stored in a parent component's data
// map implements FieldCodec, the binding calls FromString instead of replacing
// the value with a raw string — so the value's concrete type is never lost.
//
// String() (from fmt.Stringer) projects the value back to the DOM on sync; the
// solver already renders Stringer values via fmt's %v, so no extra plumbing is
// needed for display.
//
// This interface is intentionally pure (no syscall/js): types implementing it
// can be unit-tested under the native toolchain even though the rest of package
// wings only builds for js/wasm.
type FieldCodec interface {
	fmt.Stringer
	FromString(string)
}

// Validator is an optional companion to FieldCodec. After FromString ingests a
// new value, the two-way binding calls Validate. The empty string means valid;
// any other string is the *id* of an error message — not the message text — to
// be resolved by the widget against its translated message slots. Returning an
// id rather than literal text keeps validation messages inside the HTML i18n
// sweep (gen_i18n), so they are translated like any other on-screen text.
type Validator interface {
	Validate() string
}
