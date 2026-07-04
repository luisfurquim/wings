//go:build js && wasm

package dom

import "syscall/js"

// observerEntry holds the data needed to disconnect a MutationObserver.
type observerEntry struct {
	target js.Value
	obs    js.Value
	fn     js.Func
}

// observerRegistry maps IDs of observers registered via Observe.
var (
	observerRegistry       = map[int64]*observerEntry{}
	nextObserverID   int64 = 1
)

// Observe creates a MutationObserver running handler and starts observing
// target with the given options (the MutationObserverInit dictionary, e.g.
// map[string]any{"attributes": true, "attributeFilter": []any{"disabled"}}).
// Returns an ID that can be passed to RmObserver to stop the observer.
//
// Prefer this over building a MutationObserver by hand: the js.Func behind a
// hand-built observer pins its Go closure forever unless released, and the
// observer itself keeps watching a detached node. Observers registered here
// are freed automatically when the enclosing component disconnects (via
// RmObserversUnder), mirroring the AddEvent / RmEventsUnder lifecycle.
func Observe(target js.Value, options map[string]any, handler func(this js.Value, args []js.Value) any) int64 {
	fn := js.FuncOf(handler)
	obs := js.Global().Get("MutationObserver").New(fn)
	obs.Call("observe", target, options)

	id := nextObserverID
	nextObserverID++
	observerRegistry[id] = &observerEntry{
		target: target,
		obs:    obs,
		fn:     fn,
	}
	return id
}

// RmObserver disconnects the observer registered with the ID returned by
// Observe and releases its Go callback. It is idempotent: removing an ID that
// is already gone is a no-op, so it is safe to call after RmObserversUnder has
// already released the same observer.
func RmObserver(id int64) {
	entry, ok := observerRegistry[id]
	if !ok {
		return
	}
	entry.obs.Call("disconnect")
	entry.fn.Release()
	delete(observerRegistry, id)
}

// RmObserversUnder releases every observer registered with Observe whose
// target is root or sits inside root — in root's light DOM or its shadow root.
// wings calls this from elementDisconnected, alongside RmEventsUnder, so that
// observers a widget wired in Render are freed when the component leaves the
// DOM. It reuses RmObserver, so it inherits its idempotency.
func RmObserversUnder(root js.Value) {
	if root.IsNull() || root.IsUndefined() {
		return
	}
	shadow := root.Get("shadowRoot")
	var ids []int64
	for id, e := range observerRegistry {
		if root.Call("contains", e.target).Bool() ||
			(shadow.Truthy() && shadow.Call("contains", e.target).Bool()) {
			ids = append(ids, id)
		}
	}
	for _, id := range ids {
		RmObserver(id)
	}
}
