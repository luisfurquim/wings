package temperature

import (
	"math"
	"testing"
)

func TestConvert(t *testing.T) {
	const eps = 1e-9
	cases := []struct {
		kelvin  float64
		unit    string
		wantSym string
		wantVal float64
	}{
		{273.15, "c", "°C", 0},
		{273.15, "celsius", "°C", 0},
		{373.15, "c", "°C", 100},
		{273.15, "f", "°F", 32},
		{373.15, "f", "°F", 212},
		{0, "k", "K", 0},
		{300, "k", "K", 300},
		{300, "kelvin", "K", 300},
		{273.15, "r", "°R", 273.15 * 9 / 5},
		{0, "r", "°R", 0},
	}
	for _, c := range cases {
		val, sym, err := Temperature{c.kelvin}.Convert(c.unit)
		if err != nil {
			t.Errorf("Convert(%g K, %q): unexpected error: %v", c.kelvin, c.unit, err)
			continue
		}
		if sym != c.wantSym {
			t.Errorf("Convert(%g K, %q): sym=%q, want %q", c.kelvin, c.unit, sym, c.wantSym)
		}
		if math.Abs(val-c.wantVal) > eps {
			t.Errorf("Convert(%g K, %q): val=%g, want %g", c.kelvin, c.unit, val, c.wantVal)
		}
	}
}

func TestConvertUnknownUnit(t *testing.T) {
	_, _, err := Temperature{300}.Convert("planck")
	if err == nil {
		t.Error("Convert with unknown unit: expected error, got nil")
	}
}

func TestDefaultUnit(t *testing.T) {
	cases := []struct{ locale, want string }{
		{"en-US", "f"},
		{"en-BS", "f"},
		{"en-BZ", "f"},
		{"en-KY", "f"},
		{"pt-BR", "c"},
		{"de-DE", "c"},
		{"fr-FR", "c"},
		{"en-GB", "c"},
		{"en-AU", "c"},
		{"zh-CN", "c"},
	}
	for _, c := range cases {
		if got := DefaultUnit(c.locale); got != c.want {
			t.Errorf("DefaultUnit(%q) = %q, want %q", c.locale, got, c.want)
		}
	}
}

func TestDefaultDecimals(t *testing.T) {
	cases := []struct {
		unit string
		want int
	}{
		{"k", 2},
		{"kelvin", 2},
		{"c", 1},
		{"celsius", 1},
		{"f", 1},
		{"fahrenheit", 1},
		{"r", 1},
		{"rankine", 1},
	}
	for _, c := range cases {
		if got := DefaultDecimals(c.unit); got != c.want {
			t.Errorf("DefaultDecimals(%q) = %d, want %d", c.unit, got, c.want)
		}
	}
}
