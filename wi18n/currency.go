//go:build js && wasm

package wi18n

import (
	"strconv"
)

// Currency is wi18n's locale-aware monetary amount. The amount is stored in
// the currency's minor unit (centavos, cents, yen-units) as a signed int64,
// avoiding the rounding pitfalls of float64 for financial data. Code is the
// ISO 4217 alphabetic code (BRL, USD, JPY, BHD). Format consults
// currencyDecimals(Code) to know where to place the decimal point.
//
// Multi-currency templates use []Currency directly; each element carries its
// own Code. Applications with a single fixed currency typically wrap
// Currency in a helper (e.g., BRL(n int64) Currency) or a custom type that
// implements Numerical and delegates to Currency internally.
type Currency struct {
	Amount int64
	Code   string
}

// Format implements Numerical. It renders the amount with the locale's
// decimal and grouping conventions and the currency's ISO symbol, using the
// browser's Intl.NumberFormat (cached) for correctness across locales.
//
// Currency ignores formatName — it always uses the currency style. If a
// caller needs a different rendering of the same amount it should wrap
// Currency in a custom type that implements Numerical.
//
// When Code is empty or the browser rejects the locale/currency pair,
// Format falls back to a plain decimal rendering so the page remains
// readable. Currency.Format never returns a non-nil error.
func (c Currency) Format(locale, _ string) (string, error) {
	decimals := currencyDecimals(c.Code)
	if c.Code == "" {
		return formatDecimal(c.Amount, decimals), nil
	}
	key := numberFmtKey{
		locale:     locale,
		style:      "currency",
		currency:   c.Code,
		fracDigits: decimals,
	}
	if out, ok := formatNumber(key, c.amountAsFloat(decimals)); ok {
		return out, nil
	}
	return formatDecimal(c.Amount, decimals) + " " + c.Code, nil
}

// amountAsFloat shifts the integer minor-unit amount into the major-unit
// value expected by Intl.NumberFormat. The shift is an exact power-of-ten
// division; for the currencies supported (≤4 decimals) float64 holds the
// result without loss for amounts up to ~9e11 in the major unit.
func (c Currency) amountAsFloat(decimals int) float64 {
	if decimals == 0 {
		return float64(c.Amount)
	}
	div := 1.0
	for i := 0; i < decimals; i++ {
		div *= 10
	}
	return float64(c.Amount) / div
}

// formatDecimal is a locale-agnostic fallback: it places the decimal point
// at `decimals` positions from the right, with no grouping separator. Used
// when Intl is unavailable or when the Currency has no Code.
func formatDecimal(amount int64, decimals int) string {
	if decimals == 0 {
		return strconv.FormatInt(amount, 10)
	}
	neg := amount < 0
	if neg {
		amount = -amount
	}
	s := strconv.FormatInt(amount, 10)
	for len(s) <= decimals {
		s = "0" + s
	}
	cut := len(s) - decimals
	whole, frac := s[:cut], s[cut:]
	if neg {
		return "-" + whole + "." + frac
	}
	return whole + "." + frac
}

// Ensure Currency satisfies Numerical at compile time.
var _ Numerical = Currency{}
