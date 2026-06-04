//go:build js && wasm

package wings

import (
	"reflect"
	"testing"
)

// newDetachedRD builds a ReactiveData with no PranaState. triggerSync is a
// no-op when state == nil, so the mutators exercise only their pure data-map /
// slice logic — no DOM sync is attempted.
func newDetachedRD(m map[string]any) *ReactiveData {
	return &ReactiveData{M: m}
}

func TestReactiveDataGetSetDelete(t *testing.T) {
	rd := newDetachedRD(map[string]any{"a": 1})
	if rd.Get("a") != 1 {
		t.Errorf("Get(a) = %v, want 1", rd.Get("a"))
	}
	if rd.Get("missing") != nil {
		t.Errorf("Get(missing) = %v, want nil", rd.Get("missing"))
	}
	rd.Set("b", "x")
	if rd.Get("b") != "x" {
		t.Errorf("after Set: Get(b) = %v, want x", rd.Get("b"))
	}
	rd.Delete("a")
	if rd.Get("a") != nil {
		t.Errorf("after Delete: Get(a) = %v, want nil", rd.Get("a"))
	}
}

func TestReactiveDataAppend(t *testing.T) {
	rd := newDetachedRD(map[string]any{"list": []any{1, 2}})
	rd.Append("list", 3)
	if got := rd.Get("list"); !reflect.DeepEqual(got, []any{1, 2, 3}) {
		t.Errorf("Append to existing: got %v, want [1 2 3]", got)
	}

	// Appending to an absent key creates a fresh single-element slice.
	rd.Append("new", "first")
	if got := rd.Get("new"); !reflect.DeepEqual(got, []any{"first"}) {
		t.Errorf("Append to absent key: got %v, want [first]", got)
	}

	// Appending to a non-slice value replaces it with a fresh slice (does not panic).
	rd.Set("scalar", 42)
	rd.Append("scalar", "x")
	if got := rd.Get("scalar"); !reflect.DeepEqual(got, []any{"x"}) {
		t.Errorf("Append to non-slice: got %v, want [x]", got)
	}
}

func TestReactiveDataDeleteAt(t *testing.T) {
	rd := newDetachedRD(map[string]any{"list": []any{"a", "b", "c"}})
	rd.DeleteAt("list", 1) // remove "b"
	if got := rd.Get("list"); !reflect.DeepEqual(got, []any{"a", "c"}) {
		t.Errorf("DeleteAt(1): got %v, want [a c]", got)
	}

	// Out-of-range and non-slice are silent no-ops.
	before := rd.Get("list")
	rd.DeleteAt("list", 99)
	rd.DeleteAt("list", -1)
	if got := rd.Get("list"); !reflect.DeepEqual(got, before) {
		t.Errorf("DeleteAt out-of-range mutated slice: %v", got)
	}
	rd.Set("scalar", 1)
	rd.DeleteAt("scalar", 0) // not []any → no-op, no panic
	if rd.Get("scalar") != 1 {
		t.Errorf("DeleteAt on non-slice changed value: %v", rd.Get("scalar"))
	}
}

func TestReactiveDataSetAt(t *testing.T) {
	rd := newDetachedRD(map[string]any{"list": []any{"a", "b", "c"}})
	rd.SetAt("list", 1, "B")
	if got := rd.Get("list"); !reflect.DeepEqual(got, []any{"a", "B", "c"}) {
		t.Errorf("SetAt in range: got %v, want [a B c]", got)
	}

	// Setting beyond the end grows the slice, padding with nil.
	rd.SetAt("list", 5, "z")
	want := []any{"a", "B", "c", nil, nil, "z"}
	if got := rd.Get("list"); !reflect.DeepEqual(got, want) {
		t.Errorf("SetAt past end: got %v, want %v", got, want)
	}

	// Non-slice target is a no-op.
	rd.Set("scalar", 7)
	rd.SetAt("scalar", 0, "x")
	if rd.Get("scalar") != 7 {
		t.Errorf("SetAt on non-slice changed value: %v", rd.Get("scalar"))
	}
}
