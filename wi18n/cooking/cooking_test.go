package cooking

import (
	"math"
	"testing"
)

func TestConvertVolume(t *testing.T) {
	const eps = 1e-6
	cases := []struct {
		liters  float64
		unit    string
		wantSym string
		wantVal float64
	}{
		{1, "L", "L", 1},
		{0.001, "mL", "mL", 1},
		{usGallon / 16, "cup", "cup", 1},
		{usGallon / 256, "tbsp", "tbsp", 1},
		{usGallon / 768, "tsp", "tsp", 1},
		{usGallon / 128, "floz", "fl oz", 1},
	}
	for _, c := range cases {
		val, sym, err := Volume{c.liters}.Convert(c.unit)
		if err != nil {
			t.Errorf("Volume.Convert(%g L, %q): unexpected error: %v", c.liters, c.unit, err)
			continue
		}
		if sym != c.wantSym {
			t.Errorf("Volume.Convert(%g, %q): sym=%q, want %q", c.liters, c.unit, sym, c.wantSym)
		}
		if math.Abs(val-c.wantVal) > eps {
			t.Errorf("Volume.Convert(%g, %q): val=%g, want %g", c.liters, c.unit, val, c.wantVal)
		}
	}
}

func TestConvertWeight(t *testing.T) {
	const eps = 1e-9
	cases := []struct {
		kg      float64
		unit    string
		wantSym string
		wantVal float64
	}{
		{1, "kg", "kg", 1},
		{0.001, "g", "g", 1},
		{lbKg, "lb", "lb", 1},
		{lbKg / 16, "oz", "oz", 1},
	}
	for _, c := range cases {
		val, sym, err := Weight{c.kg}.Convert(c.unit)
		if err != nil {
			t.Errorf("Weight.Convert(%g kg, %q): unexpected error: %v", c.kg, c.unit, err)
			continue
		}
		if sym != c.wantSym {
			t.Errorf("Weight.Convert(%g, %q): sym=%q, want %q", c.kg, c.unit, sym, c.wantSym)
		}
		if math.Abs(val-c.wantVal) > eps {
			t.Errorf("Weight.Convert(%g, %q): val=%g, want %g", c.kg, c.unit, val, c.wantVal)
		}
	}
}

func TestConvertUnknownUnits(t *testing.T) {
	cv := Volume{Liters: 1}
	if _, _, err := cv.Convert("barrel"); err == nil {
		t.Error("Volume: expected error for unknown unit, got nil")
	}
	cw := Weight{Kilograms: 1}
	if _, _, err := cw.Convert("slug"); err == nil {
		t.Error("Weight: expected error for unknown unit, got nil")
	}
}

func TestDefaultVolumeUnit(t *testing.T) {
	cases := []struct{ locale, want string }{
		{"en-US", "cup"},
		{"pt-BR", "mL"},
		{"de-DE", "mL"},
		{"en-GB", "mL"},
	}
	for _, c := range cases {
		if got := DefaultVolumeUnit(c.locale); got != c.want {
			t.Errorf("DefaultVolumeUnit(%q) = %q, want %q", c.locale, got, c.want)
		}
	}
}

func TestDefaultWeightUnit(t *testing.T) {
	cases := []struct{ locale, want string }{
		{"en-US", "oz"},
		{"pt-BR", "g"},
		{"de-DE", "g"},
		{"en-GB", "g"},
	}
	for _, c := range cases {
		if got := DefaultWeightUnit(c.locale); got != c.want {
			t.Errorf("DefaultWeightUnit(%q) = %q, want %q", c.locale, got, c.want)
		}
	}
}

func TestNewVolume(t *testing.T) {
	cases := []struct{ val float64; unit string }{
		{1, "L"}, {250, "mL"}, {2, "cup"}, {4, "tbsp"}, {3, "tsp"}, {8, "floz"},
	}
	for _, c := range cases {
		v, err := NewVolume(c.val, c.unit)
		if err != nil {
			t.Errorf("NewVolume(%g, %q): unexpected error: %v", c.val, c.unit, err)
			continue
		}
		got, _, err := v.Convert(c.unit)
		if err != nil {
			t.Errorf("NewVolume(%g, %q).Convert: unexpected error: %v", c.val, c.unit, err)
			continue
		}
		if math.Abs(got-c.val) > 1e-9 {
			t.Errorf("NewVolume(%g, %q) round-trip = %g, want %g", c.val, c.unit, got, c.val)
		}
	}
}

func TestNewWeight(t *testing.T) {
	cases := []struct{ val float64; unit string }{
		{1, "kg"}, {500, "g"}, {2, "lb"}, {8, "oz"},
	}
	for _, c := range cases {
		w, err := NewWeight(c.val, c.unit)
		if err != nil {
			t.Errorf("NewWeight(%g, %q): unexpected error: %v", c.val, c.unit, err)
			continue
		}
		got, _, err := w.Convert(c.unit)
		if err != nil {
			t.Errorf("NewWeight(%g, %q).Convert: unexpected error: %v", c.val, c.unit, err)
			continue
		}
		if math.Abs(got-c.val) > 1e-9 {
			t.Errorf("NewWeight(%g, %q) round-trip = %g, want %g", c.val, c.unit, got, c.val)
		}
	}
}

func TestNewVolumeUnknownUnit(t *testing.T) {
	if _, err := NewVolume(1, "barrel"); err == nil {
		t.Error("NewVolume with unknown unit: expected error, got nil")
	}
}

func TestNewWeightUnknownUnit(t *testing.T) {
	if _, err := NewWeight(1, "stone"); err == nil {
		t.Error("NewWeight with unknown unit: expected error, got nil")
	}
}
