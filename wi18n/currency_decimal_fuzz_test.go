package wi18n

import (
	"strconv"
	"strings"
	"testing"
)

// FuzzFormatDecimal hammers the locale-agnostic decimal fallback. Properties:
// never panics; and the output is a faithful representation — stripping the
// single decimal point and parsing the digits back yields the original amount
// (this is the property that exposes the MinInt64 sign-negation overflow).
//
// decimals is clamped to its real contract (0..50; callers pass currencyDecimals,
// range 0..4) so we test the function as actually used, not impossible inputs.
func FuzzFormatDecimal(f *testing.F) {
	seeds := []struct {
		amount   int64
		decimals int
	}{
		{0, 0}, {5, 2}, {-5, 2}, {1234, 2}, {-1, 4},
		{1<<63 - 1, 2},  // MaxInt64
		{-(1 << 62), 3}, // large negative
	}
	for _, s := range seeds {
		f.Add(s.amount, s.decimals)
	}
	f.Fuzz(func(t *testing.T, amount int64, decimals int) {
		if decimals < 0 {
			decimals = -decimals
		}
		decimals %= 51 // keep within 0..50 (the real range is 0..4)

		got := formatDecimal(amount, decimals)

		recon := strings.Replace(got, ".", "", 1)
		n, err := strconv.ParseInt(recon, 10, 64)
		if err != nil {
			t.Fatalf("formatDecimal(%d,%d)=%q: digits not parseable: %v", amount, decimals, got, err)
		}
		if n != amount {
			t.Fatalf("formatDecimal(%d,%d)=%q reconstructs to %d", amount, decimals, got, n)
		}
	})
}
