// Package field provides ready-made FieldCodec/Validator implementations for
// wings form widgets (w-input and future text controls). Bind a pointer to one
// of these in a component's data map and the widget validates it natively,
// resolving the returned message id against the webdev's translated
// <span slot="errors" id="..."> nodes.
//
//	// parent component
//	"email": field.NewEmail("email-format"),
//
//	<w-input type="email" required="true" &value="{{email}}">
//	  <span slot="errors" id="email-format">Invalid email address</span>
//	</w-input>
//
// All types treat the empty string as valid — use the native `required`
// attribute to flag empty fields — and are pure Go (no syscall/js), so they can
// be unit-tested under the native toolchain.
package field

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/luisfurquim/wings"
)

// Text is a trimmed string field with no validation. It exists to give a value
// a stable type across a two-way binding (so it is never replaced by a raw
// string) without imposing any rule.
type Text struct{ v string }

// NewText returns an empty Text field.
func NewText() *Text { return &Text{} }

func (t *Text) FromString(s string) { t.v = strings.TrimSpace(s) }
func (t *Text) String() string      { return t.v }
func (t *Text) Validate() string    { return "" }

// Email validates that the value looks like an email address. invalidID is the
// message id returned (for the widget to resolve) when it does not.
type Email struct {
	v         string
	invalidID string
}

// NewEmail returns an Email field that reports invalidID when the value is not
// a well-formed address.
func NewEmail(invalidID string) *Email { return &Email{invalidID: invalidID} }

func (e *Email) FromString(s string) { e.v = strings.TrimSpace(s) }
func (e *Email) String() string      { return e.v }
func (e *Email) Validate() string {
	if e.v == "" || emailRE.MatchString(e.v) {
		return ""
	}
	return e.invalidID
}

var emailRE = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Int validates that the value is an integer within [Min, Max]. notIntID is
// reported for a value that does not parse as an integer, outOfRangeID for an
// integer outside the range — distinct messages, so "type a number" and
// "between 0 and 120" can be worded separately. The parsed number is available
// via Int().
type Int struct {
	v            string
	n            int
	ok           bool
	min, max     int
	notIntID     string
	outOfRangeID string
}

// NewInt returns an Int field constrained to [min, max], reporting notIntID
// when the value is not an integer and outOfRangeID when it is an integer
// outside the range.
func NewInt(min, max int, notIntID, outOfRangeID string) *Int {
	return &Int{min: min, max: max, notIntID: notIntID, outOfRangeID: outOfRangeID}
}

func (i *Int) FromString(s string) {
	i.v = strings.TrimSpace(s)
	n, err := strconv.Atoi(i.v)
	i.n, i.ok = n, err == nil
}
func (i *Int) String() string { return i.v }

// Int returns the parsed value and whether the current text parsed cleanly.
func (i *Int) Int() (int, bool) { return i.n, i.ok }

func (i *Int) Validate() string {
	if i.v == "" {
		return ""
	}
	if !i.ok {
		return i.notIntID
	}
	if i.n < i.min || i.n > i.max {
		return i.outOfRangeID
	}
	return ""
}

// Pattern validates the value against a regular expression. invalidID is
// reported on no match.
type Pattern struct {
	v         string
	re        *regexp.Regexp
	invalidID string
}

// NewPattern returns a Pattern field that reports invalidID when the value does
// not match re.
func NewPattern(re *regexp.Regexp, invalidID string) *Pattern {
	return &Pattern{re: re, invalidID: invalidID}
}

func (p *Pattern) FromString(s string) { p.v = strings.TrimSpace(s) }
func (p *Pattern) String() string      { return p.v }
func (p *Pattern) Validate() string {
	if p.v == "" || p.re.MatchString(p.v) {
		return ""
	}
	return p.invalidID
}

// Compile-time checks that every type satisfies the wings binding contract.
// Package wings builds natively with only its pure files (field.go), so these
// assertions — and this package's tests — run under the native toolchain.
var (
	_ wings.FieldCodec = (*Text)(nil)
	_ wings.Validator  = (*Text)(nil)
	_ wings.FieldCodec = (*Email)(nil)
	_ wings.Validator  = (*Email)(nil)
	_ wings.FieldCodec = (*Int)(nil)
	_ wings.Validator  = (*Int)(nil)
	_ wings.FieldCodec = (*Pattern)(nil)
	_ wings.Validator  = (*Pattern)(nil)
)
