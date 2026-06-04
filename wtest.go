//go:build js && wasm

package wings

import (
	"sort"
	"syscall/js"
)

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

// ── Module-declared self-tests (Testabler + <w-test-report>) ─────────────────
//
// A module can declare its own integration self-tests by implementing Testabler.
// The runtime discovers them per live instance: when an element whose module
// implements Testabler mounts, registerTestable records its checks against that
// element; unregisterTestable drops them on disconnect. <w-test-report> then
// runs them all via RunAllTestables and reports the results.
//
// Declare Testable() in a file gated by the wings_test build tag so the tests
// compile only into test builds, never production. "Test everything" is a
// composition concern, not a framework one: build a throwaway test app that
// imports and mounts all your modules flat, opened only in your dev pipeline.

// Testabler is the optional interface a module implements to expose named
// integration checks about itself. Each check sees a CheckCtx whose Subject and
// Dom are the live element (Events is empty — event-stream assertions belong in
// a <w-test> wrapper, which spies via the @all channel).
type Testabler interface {
	Testable() map[string]CheckFunc
}

// CheckResult is one entry in the page test report produced by RunReport and
// marshalled to JSON by <w-test-report>: either a <w-test> card (Kind "w-test",
// including human-judged visual cards) or a module-declared self-test (Kind
// "testable"). State is "pass", "fail", or "pending" (a visual card not yet
// judged). Label is the card title, or "tag/check" for a testable.
type CheckResult struct {
	Kind   string `json:"kind"`
	Label  string `json:"label"`
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
}

// liveTestable is one mounted Testabler instance and the checks it declared.
type liveTestable struct {
	tag     string
	subject js.Value
	checks  map[string]CheckFunc
}

var liveTestables []*liveTestable

// registerTestable is called by the runtime after a freshly mounted element's
// Render returns. If mod implements Testabler its checks are recorded against
// the live element. No-op otherwise — the cost on the mount path is a single
// type assertion, so a production build (no wings_test) pays effectively nothing.
func registerTestable(self js.Value, tag string, mod PranaMod) {
	t, ok := mod.(Testabler)
	if !ok {
		return
	}
	m := t.Testable()
	if len(m) == 0 {
		return
	}
	liveTestables = append(liveTestables, &liveTestable{tag: tag, subject: self, checks: m})
	G.Logf(3, "registerTestable: %s declared %d check(s)\n", tag, len(m))
}

// unregisterTestable drops any testables recorded for self (called on disconnect).
func unregisterTestable(self js.Value) {
	kept := liveTestables[:0]
	for _, lt := range liveTestables {
		if !lt.subject.Equal(self) {
			kept = append(kept, lt)
		}
	}
	liveTestables = kept
}

// ── <w-test> card registry (for the page report) ─────────────────────────────
//
// Each <w-test> card registers a probe so the page report can include it — most
// importantly the human-judged *visual* cards, whose seal a tester sets by eye.
// The point of the report is one-click delivery of everything tested on the
// page (auto + manual), so a tester never hand-writes "these 100 of 500 failed".

// WTestProbe reports a <w-test> card's current title, seal state ("pass",
// "fail", or "pending"), and detail. The widget registers one via RegisterWTest.
type WTestProbe func() (title, state, detail string)

type wtestCard struct {
	subject js.Value
	probe   WTestProbe
}

var wtestCards []wtestCard

// RegisterWTest registers a <w-test> card's probe so it appears in the page
// report. The entry is dropped automatically when the element disconnects.
func RegisterWTest(self js.Value, probe WTestProbe) {
	wtestCards = append(wtestCards, wtestCard{subject: self, probe: probe})
}

func unregisterWTest(self js.Value) {
	kept := wtestCards[:0]
	for _, c := range wtestCards {
		if !c.subject.Equal(self) {
			kept = append(kept, c)
		}
	}
	wtestCards = kept
}

// RunReport collects the full test result for the page: every <w-test> card
// (including the human-judged visual ones, in whatever state the tester left
// them) followed by every check declared by mounted Testabler modules. So a
// tester can run all the tests, judge the visual ones by eye, and deliver one
// report of what passed and what failed with a single click. wings produces the
// report only — transporting or persisting it (POST to a server, write a file,
// diff in CI) is the app's call, via the <w-test-report> "report" event.
func RunReport() []CheckResult {
	out := make([]CheckResult, 0, len(wtestCards)+len(liveTestables))
	for _, c := range wtestCards {
		title, state, detail := c.probe()
		out = append(out, CheckResult{Kind: "w-test", Label: title, State: state, Detail: detail})
	}
	for _, lt := range liveTestables {
		names := make([]string, 0, len(lt.checks))
		for name := range lt.checks {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			pass, detail := lt.checks[name](CheckCtx{Subject: lt.subject, Dom: lt.subject})
			state := "fail"
			if pass {
				state = "pass"
			}
			out = append(out, CheckResult{Kind: "testable", Label: lt.tag + "/" + name, State: state, Detail: detail})
		}
	}
	return out
}
