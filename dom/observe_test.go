//go:build js && wasm

package dom

import (
	"syscall/js"
	"testing"
)

// obsEl returns a plain JS object standing in for an observed element,
// following the fakeEl pattern: only what Observe/RmObserversUnder touch.
// The shim's inert MutationObserver (testsupport/dom_shim.cjs) supplies
// observe/disconnect.
func obsEl(underRoot bool) js.Value {
	el := js.Global().Get("Object").New()
	el.Set("shadowRoot", js.Null())
	el.Set("_underRoot", underRoot)
	return el
}

// TestRmObserversUnder verifies that disconnect-time cleanup disconnects
// exactly the observers under the given root, leaves the others, and stays
// idempotent — the regression test for the MutationObserver leak (same class
// as the dom.AddEvent leak fixed in 0.16.14, missed on the observer path).
func TestRmObserversUnder(t *testing.T) {
	observerRegistry = map[int64]*observerEntry{} // isolate from other tests

	inside := obsEl(true)
	outside := obsEl(false)
	opts := map[string]any{"attributes": true}

	idIn := Observe(inside, opts, noop)
	idOut := Observe(outside, opts, noop)

	if len(observerRegistry) != 2 {
		t.Fatalf("registry size = %d, want 2", len(observerRegistry))
	}

	RmObserversUnder(fakeRoot())

	if _, ok := observerRegistry[idIn]; ok {
		t.Error("observer under root was not removed")
	}
	if _, ok := observerRegistry[idOut]; !ok {
		t.Error("observer outside root was wrongly removed")
	}

	// Idempotent: re-running cleanup and manually removing the gone ID must be
	// no-ops, not panics or double releases.
	RmObserversUnder(fakeRoot())
	RmObserver(idIn)
	if _, ok := observerRegistry[idOut]; !ok {
		t.Error("outside observer lost after idempotent re-cleanup")
	}
}

// TestRmObserverIdempotent locks the standalone idempotency contract
// RmObserversUnder relies on: removing an ID twice is a no-op.
func TestRmObserverIdempotent(t *testing.T) {
	observerRegistry = map[int64]*observerEntry{}

	id := Observe(obsEl(false), map[string]any{"attributes": true}, noop)
	RmObserver(id)
	if _, ok := observerRegistry[id]; ok {
		t.Fatal("observer still registered after RmObserver")
	}
	RmObserver(id) // second call must not panic
}
