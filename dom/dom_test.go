//go:build js && wasm

package dom

import (
	"syscall/js"
	"testing"
)

// fakeEl returns a plain JS object standing in for a DOM element. The wasm test
// harness has no real document (see testsupport/dom_shim.cjs), so these stubs
// give AddEvent/RmEvent/RmEventsUnder just the methods they call —
// addEventListener, removeEventListener, contains — without needing live DOM.
// contains() answers from an "_underRoot" flag the test sets on each element.
func fakeEl(underRoot bool) js.Value {
	el := js.Global().Get("Object").New()
	el.Set("addEventListener", js.FuncOf(func(js.Value, []js.Value) any { return nil }))
	el.Set("removeEventListener", js.FuncOf(func(js.Value, []js.Value) any { return nil }))
	el.Set("shadowRoot", js.Null())
	el.Set("_underRoot", underRoot)
	return el
}

// fakeRoot returns a stub element whose contains(x) reports x._underRoot.
func fakeRoot() js.Value {
	root := js.Global().Get("Object").New()
	root.Set("shadowRoot", js.Null())
	root.Set("contains", js.FuncOf(func(this js.Value, args []js.Value) any {
		return args[0].Get("_underRoot").Truthy()
	}))
	return root
}

func noop(js.Value, []js.Value) any { return nil }

// TestRmEventsUnder verifies that disconnect-time cleanup removes exactly the
// listeners under the given root, leaves the others, and stays idempotent.
// This is the regression test for the listener leak that sec-wasm-go skill
// validation surfaced.
func TestRmEventsUnder(t *testing.T) {
	eventRegistry = map[int64]*eventEntry{} // isolate from other tests

	inside := fakeEl(true)
	outside := fakeEl(false)

	idIn := AddEvent(inside, "input", noop, false, false)
	idOut := AddEvent(outside, "click", noop, false, false)

	if len(eventRegistry) != 2 {
		t.Fatalf("registry size = %d, want 2", len(eventRegistry))
	}

	RmEventsUnder(fakeRoot())

	if _, ok := eventRegistry[idIn]; ok {
		t.Error("listener under root was not removed")
	}
	if _, ok := eventRegistry[idOut]; !ok {
		t.Error("listener outside root was wrongly removed")
	}

	// Idempotent: re-running cleanup and manually removing the gone ID must be
	// no-ops, not panics or double releases.
	RmEventsUnder(fakeRoot())
	RmEvent(idIn)
	if _, ok := eventRegistry[idOut]; !ok {
		t.Error("outside listener lost after idempotent re-cleanup")
	}
}

// TestRmEventIdempotent locks the standalone idempotency contract RmEventsUnder
// relies on: removing an ID twice is a no-op.
func TestRmEventIdempotent(t *testing.T) {
	eventRegistry = map[int64]*eventEntry{}

	id := AddEvent(fakeEl(false), "click", noop, false, false)
	RmEvent(id)
	if _, ok := eventRegistry[id]; ok {
		t.Fatal("listener still registered after RmEvent")
	}
	RmEvent(id) // second call must not panic
}
