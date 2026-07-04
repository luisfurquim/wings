// dom_shim.cjs — minimal browser-global stubs for running the wings package's
// js/wasm tests under Node.
//
// The wings package init() (defs.go) reads window.location.hash and installs a
// "hashchange" listener at import time (hash.go). Node has no DOM, so without
// these stubs every test binary panics in init before any Test* runs. We stub
// only what package init actually touches — nothing here simulates real DOM
// behaviour; tests that need live DOM semantics must run in a browser.
//
// Deliberately NO `document` stub: the runtime guards every DOM mutation with
// `js.Global().Get("document").Truthy()` (see injectSkinCSS, removeSkinStyle),
// and an undefined document is exactly the "not available — unit test" path
// those guards are written for. Stubbing document as `{}` would be truthy yet
// lack the DOM methods, turning a graceful no-op into a panic.
//
// Loaded via NODE_OPTIONS=--require by testsupport/wasm_test_exec.sh, so it
// executes before Go's wasm_exec_node.js instantiates the module.
globalThis.location ??= { hash: "" };
globalThis.addEventListener ??= function () {};
globalThis.removeEventListener ??= function () {};
// Inert MutationObserver so dom.Observe/RmObserver lifecycle tests can run;
// callbacks never fire (no real DOM to mutate).
globalThis.MutationObserver ??= class {
	observe() {}
	disconnect() {}
};
