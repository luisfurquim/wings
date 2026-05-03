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

func TestNew(t *testing.T) {
	cases := []struct{ val float64; unit string }{{1,"L"},{500,"mL"},{5,"dL"},{0.001,"m3"},{8,"floz"},{2,"pt"},{1,"qt"},{1,"gal"},{1,"galimp"}}
	for _, c := range cases {
		v, err := New(c.val, c.unit)
		if err != nil {
			t.Errorf("New(%g, %q): unexpected error: %v", c.val, c.unit, err)
			continue
		}
		got, _, err := v.Convert(c.unit)
		if err != nil {
			t.Errorf("New(%g, %q).Convert: unexpected error: %v", c.val, c.unit, err)
			continue
		}
		if math.Abs(got-c.val) > 1e-9 {
			t.Errorf("New(%g, %q) round-trip = %g, want %g", c.val, c.unit, got, c.val)
		}
	}
}

func TestNewUnknownUnit(t *testing.T) {
	if _, err := New(1, "firkin"); err == nil {
		t.Error("New with unknown unit: expected error, got nil")
	}
}
