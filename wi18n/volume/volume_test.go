package volume

import (
	"math"
	"testing"
)

func TestConvert(t *testing.T) {
	const eps = 1e-6
	cases := []struct {
		liters  float64
		unit    string
		wantSym string
		wantVal float64
	}{
		{1, "L", "L", 1},
		{1, "mL", "mL", 1000},
		{1, "dL", "dL", 10},
		{1000, "m3", "m³", 1},
		{usGallon, "gal", "gal", 1},
		{4.54609, "galimp", "gal", 1},
		{usGallon / 8, "pt", "pt", 1},
		{usGallon / 4, "qt", "qt", 1},
		{usGallon / 128, "floz", "fl oz", 1},
	}
	for _, c := range cases {
		val, sym, err := Volume{c.liters}.Convert(c.unit)
		if err != nil {
			t.Errorf("Convert(%g L, %q): unexpected error: %v", c.liters, c.unit, err)
			continue
		}
		if sym != c.wantSym {
			t.Errorf("Convert(%g, %q): sym=%q, want %q", c.liters, c.unit, sym, c.wantSym)
		}
		if math.Abs(val-c.wantVal) > eps {
			t.Errorf("Convert(%g, %q): val=%g, want %g", c.liters, c.unit, val, c.wantVal)
		}
	}
}

func TestConvertUnknownUnit(t *testing.T) {
	_, _, err := Volume{1}.Convert("barrel")
	if err == nil {
		t.Error("expected error for unknown unit, got nil")
	}
}

func TestDefaultUnit(t *testing.T) {
	cases := []struct{ locale, want string }{
		{"en-US", "gal"},
		{"pt-BR", "L"},
		{"de-DE", "L"},
		{"en-GB", "L"},
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
		{"mL", 0},
		{"m3", 0},
		{"dL", 1},
		{"floz", 1},
		{"L", 2},
		{"gal", 2},
		{"pt", 2},
		{"qt", 2},
	}
	for _, c := range cases {
		if got := DefaultDecimals(c.unit); got != c.want {
			t.Errorf("DefaultDecimals(%q) = %d, want %d", c.unit, got, c.want)
		}
	}
}
