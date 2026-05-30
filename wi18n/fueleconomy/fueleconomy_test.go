package fueleconomy

import (
	"math"
	"testing"
)

func TestConvert(t *testing.T) {
	const eps = 1e-4
	cases := []struct {
		l100km  float64
		unit    string
		wantSym string
		wantVal float64
	}{
		{8, "l100km", "L/100km", 8},
		{mpgUS, "mpg", "mpg", 1},     // exactly 1 mpg at mpgUS L/100km
		{mpgImp, "mpgimp", "mpg", 1}, // exactly 1 mpg-imp
		{100, "kml", "km/L", 1},      // 100 L/100km → 1 km/L
		{10, "kml", "km/L", 10},      // 10 L/100km → 10 km/L
	}
	for _, c := range cases {
		val, sym, err := FuelEconomy{c.l100km}.Convert(c.unit)
		if err != nil {
			t.Errorf("Convert(%g L/100km, %q): unexpected error: %v", c.l100km, c.unit, err)
			continue
		}
		if sym != c.wantSym {
			t.Errorf("Convert(%g, %q): sym=%q, want %q", c.l100km, c.unit, sym, c.wantSym)
		}
		if math.Abs(val-c.wantVal) > eps {
			t.Errorf("Convert(%g, %q): val=%g, want %g", c.l100km, c.unit, val, c.wantVal)
		}
	}
}

func TestConvertInverseZero(t *testing.T) {
	for _, unit := range []string{"mpg", "mpgimp", "kml"} {
		_, _, err := FuelEconomy{0}.Convert(unit)
		if err == nil {
			t.Errorf("Convert(0, %q): expected error, got nil", unit)
		}
	}
}

func TestConvertUnknownUnit(t *testing.T) {
	_, _, err := FuelEconomy{10}.Convert("furlongs-per-hogshead")
	if err == nil {
		t.Error("expected error for unknown unit, got nil")
	}
}

func TestDefaultUnit(t *testing.T) {
	cases := []struct{ locale, want string }{
		{"en-US", "mpg"},
		{"en-GB", "mpgimp"},
		{"pt-BR", "l100km"},
		{"de-DE", "l100km"},
		{"fr-FR", "l100km"},
	}
	for _, c := range cases {
		if got := DefaultUnit(c.locale); got != c.want {
			t.Errorf("DefaultUnit(%q) = %q, want %q", c.locale, got, c.want)
		}
	}
}

func TestNew(t *testing.T) {
	cases := []struct {
		val  float64
		unit string
	}{
		{8, "l100km"}, {30, "mpg"}, {25, "mpgimp"}, {12, "kml"},
	}
	for _, c := range cases {
		fe, err := New(c.val, c.unit)
		if err != nil {
			t.Errorf("New(%g, %q): unexpected error: %v", c.val, c.unit, err)
			continue
		}
		got, _, err := fe.Convert(c.unit)
		if err != nil {
			t.Errorf("New(%g, %q).Convert: unexpected error: %v", c.val, c.unit, err)
			continue
		}
		if math.Abs(got-c.val) > 1e-9 {
			t.Errorf("New(%g, %q) round-trip = %g, want %g", c.val, c.unit, got, c.val)
		}
	}
}

func TestNewInverseZero(t *testing.T) {
	for _, unit := range []string{"mpg", "mpgimp", "kml"} {
		if _, err := New(0, unit); err == nil {
			t.Errorf("New(0, %q): expected error for zero value, got nil", unit)
		}
	}
}

func TestNewUnknownUnit(t *testing.T) {
	if _, err := New(0, "furlongs-per-hogshead"); err == nil {
		t.Error("New with unknown unit: expected error, got nil")
	}
}
