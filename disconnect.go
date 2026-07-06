//go:build js && wasm

package wings

import "syscall/js"

// Per-instance teardown hook. PranaMod has no destructor, and the built-in
// disconnect path only frees resources wired through dom.AddEvent /
// dom.Observe (auto-released because their targets sit under the element).
// A widget that owns resources reaching OUTSIDE its subtree — a
// document-level listener, a hand-built MutationObserver needing
// takeRecords, a timer — registers here to release them when the instance
// leaves the DOM. Mirrors OnRetranslate / OnFormReset.

var disconnectHooks = map[string]func(js.Value){}

// OnDisconnect registers fn to run when a live instance of tag is removed
// from the DOM, after the built-in dom.AddEvent/dom.Observe auto-release.
// A later call for the same tag replaces the previous hook. w-text uses it
// to Detach its Editor (document-level selectionchange listener, own
// MutationObserver).
func OnDisconnect(tag string, fn func(js.Value)) {
	disconnectHooks[tag] = fn
}

// elementDisconnectHook dispatches the disconnect callback to the tag's
// hook. Called from elementDisconnected.
func elementDisconnectHook(self js.Value) {
	if fn := disconnectHooks[self.Get("_pranaTag").String()]; fn != nil {
		fn(self)
	}
}
