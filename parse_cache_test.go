//go:build js && wasm

package wings

import "testing"

func TestCachedParseText(t *testing.T) {
	// Use a string unlikely to be cached by another test.
	const src = "literal {{user.name}} tail"

	segs1, err := cachedParseText(src)
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	if len(segs1) == 0 {
		t.Fatal("expected at least one segment")
	}

	// Second call must hit the cache and return the identical slice (same
	// backing array), proving memoisation rather than a re-parse.
	segs2, err := cachedParseText(src)
	if err != nil {
		t.Fatalf("second parse: %v", err)
	}
	if &segs1[0] != &segs2[0] {
		t.Error("cache miss: second call returned a freshly parsed slice")
	}

	// The cache map must hold the entry under the source string key.
	parseCacheMu.RLock()
	_, ok := parseCache[src]
	parseCacheMu.RUnlock()
	if !ok {
		t.Error("parseCache not populated for the source string")
	}
}

// A parse-equivalent of a plain literal yields one non-ref segment.
func TestCachedParseTextLiteral(t *testing.T) {
	segs, err := cachedParseText("just text, no refs")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(segs) != 1 || segs[0].IsRef {
		t.Errorf("literal should be a single non-ref segment, got %+v", segs)
	}
}
