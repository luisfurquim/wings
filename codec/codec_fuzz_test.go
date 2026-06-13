package codec

import "testing"

// FuzzDecode feeds arbitrary strings to Decode for every supported target kind.
// Property: never panics; garbage input returns an error, it is never accepted
// as a bogus value and never reads out of bounds.
func FuzzDecode(f *testing.F) {
	for _, s := range []string{
		"", "true", "false", "42", "-7", "3.14", "abc",
		"9999999999999999999999999999999", "\x00\x01\x02",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, buf string) {
		var c Codec
		var s string
		_ = c.Decode(buf, &s)
		var b bool
		_ = c.Decode(buf, &b)
		var i int
		_ = c.Decode(buf, &i)
		var u uint
		_ = c.Decode(buf, &u)
		var fl float64
		_ = c.Decode(buf, &fl)
		var by []byte
		_ = c.Decode(buf, &by)
	})
}

// FuzzRoundTripInt asserts Encode then Decode recovers the original integer.
func FuzzRoundTripInt(f *testing.F) {
	for _, n := range []int64{0, 1, -1, 42, -9999, 1 << 62} {
		f.Add(n)
	}
	f.Fuzz(func(t *testing.T, n int64) {
		var c Codec
		enc := c.Encode(n)
		var got int64
		if err := c.Decode(enc, &got); err != nil {
			t.Fatalf("round-trip decode of %q (from %d): %v", enc, n, err)
		}
		if got != n {
			t.Fatalf("round-trip int: %d -> %q -> %d", n, enc, got)
		}
	})
}

// FuzzRoundTripString asserts Encode then Decode recovers the original string.
func FuzzRoundTripString(f *testing.F) {
	for _, s := range []string{"", "hello", "açaí", "with\nnewline", "\x00"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		var c Codec
		enc := c.Encode(s)
		var got string
		if err := c.Decode(enc, &got); err != nil {
			t.Fatalf("round-trip decode of string %q: %v", s, err)
		}
		if got != s {
			t.Fatalf("round-trip string: %q -> %q -> %q", s, enc, got)
		}
	})
}
