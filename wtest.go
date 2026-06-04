//go:build js && wasm

package wings

import "syscall/js"

// ── In-web test check registry ──────────────────────────────────────────────
//
// A "check" is a named Go assertion the <w-test> widget runs against the
// subject it wraps. The widget captures every event the subject fires (via the
// @all spy channel) and, on each event plus on demand, runs the named check
// against a CheckCtx — the subject element, its shadow/light DOM, and the
// recorded event log — yielding a pass/fail seal and a human-readable detail.
//
// Register checks from init() (or app setup), mirroring RegisterSkin. A
// <w-test> with no check= attribute is a manual, human-judged visual test.

// CheckEvent is one event the subject fired, as recorded by <w-test>. Name is
// the event name (e.g. "save"); Args are the trigger arguments minus the event
// name that the @all channel prepends.
type CheckEvent struct {
	Name string
	Args []any
}

// CheckCtx is what a CheckFunc inspects: the wrapped subject element, the DOM
// node to query against, and the ordered log of events the subject fired.
type CheckCtx struct {
	Subject js.Value     // the slotted element <w-test> wraps
	Dom     js.Value     // node to query (subject's light DOM / shadow root)
	Events  []CheckEvent // events captured so far, in fire order
}

// CheckFunc is a registered assertion. It returns whether the test passes and a
// short detail string shown next to the seal (the reason on failure, or a note
// on success).
type CheckFunc func(CheckCtx) (pass bool, detail string)

var checks = map[string]CheckFunc{}

// RegisterCheck registers a named assertion for <w-test check="name">.
// Re-registering an existing name overwrites the previous entry and logs a
// level-1 warning.
func RegisterCheck(name string, fn CheckFunc) {
	if _, exists := checks[name]; exists {
		G.Logf(1, "RegisterCheck: check %q already registered, overwriting\n", name)
	}
	checks[name] = fn
}

// RunCheck runs the named check against ctx. found is false when no check is
// registered under name (the <w-test> widget treats that as a configuration
// error, distinct from a check that ran and failed).
func RunCheck(name string, ctx CheckCtx) (pass bool, detail string, found bool) {
	fn, ok := checks[name]
	if !ok {
		return false, "", false
	}
	pass, detail = fn(ctx)
	return pass, detail, true
}
