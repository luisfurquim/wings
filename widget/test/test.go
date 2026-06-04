//go:build js && wasm

// Package test provides the w-test custom element for wings: an in-web test
// harness. Wrap any prana widget (the "subject") in <w-test>; the element
// spies on every event the subject fires (via the @all channel), renders a
// live event log, and shows a pass/fail seal.
//
// # Usage
//
//	<w-test title="Dialog fires @save"
//	        expect="Clicking Save should fire the save event"
//	        check="dialogSawSave">
//	    <w-dialog buttons="save,cancel">Save your work?</w-dialog>
//	</w-test>
//
// With a check= attribute the seal is driven by a Go assertion registered via
// wings.RegisterCheck; the assertion runs on mount, after every captured event,
// and on the "Re-run" button. Without check= the test is manual: the seal is a
// human-toggled ✅/❌ for purely visual checks (e.g. skin aesthetics).
//
// The host carries data-wtest-state="pending|pass|fail" so a later headless
// runner (Playwright) can scrape seals without re-deriving them.
package test

import (
	_ "embed"
	"fmt"
	"syscall/js"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
)

const elementTag = "w-test"

// captureHandler is the handler name <w-test> stamps as @all on its subject and
// provides in its own reactive data, so the subject's events bubble back here.
const captureHandler = "__wtest_capture"

// G is the logger for this module.
var G goose.Alert

//go:embed test.html
var htmlContent string

//go:embed design.css
var designCSS string

// New creates a new Test instance.
func New() *Test { return &Test{} }

func init() {
	G.Set(3)
	wings.Register(
		elementTag,
		htmlContent,
		designCSS,
		func() wings.PranaMod { return &Test{} },
		"title", "expect", "check",
	)
	G.Logf(3, "w-test: module registered\n")
}

// Test implements wings.PranaMod for the w-test custom element.
type Test struct {
	obj       *wings.PranaObj
	subject   js.Value
	seals     js.Value // .wt-seals — pulsed on every seal update
	events    []wings.CheckEvent
	checkName string
	manual    bool
}

func (t *Test) InitData() map[string]any {
	return map[string]any{
		"title":        "",
		"expect":       "",
		"check":        "",
		"state":        "pending",
		"detail":       "",
		"seal_pending": true,
		"seal_pass":    false,
		"seal_fail":    false,
		"events":       []any{},
	}
}

func (t *Test) Render(obj *wings.PranaObj) {
	t.obj = obj
	t.checkName, _ = obj.This.Get("check").(string)
	t.manual = t.checkName == ""

	// The subject is the first light-DOM child of the host.
	t.subject = obj.Element.Get("children").Call("item", 0)
	if found := dom.Query(obj.Dom, ".wt-seals"); len(found) > 0 {
		t.seals = found[0]
	}

	if t.manual {
		obj.Element.Call("setAttribute", "data-wtest-manual", "")
		t.bindManualToggle()
	} else if t.subject.Truthy() {
		// Stamp the @all spy channel and provide its handler. buildTrigger reads
		// @all live at fire time, so stamping now captures every subsequent event.
		t.subject.Call("setAttribute", "@all", captureHandler)
		obj.This.M[captureHandler] = func(args ...any) { t.onCapture(args...) }
	}

	t.bindRerun()

	if t.manual {
		t.setSeal("pending", "")
	} else {
		t.runAndSeal()
	}
}

// onCapture records one event (the @all channel prepends the event name as the
// first arg) and re-runs the check.
func (t *Test) onCapture(args ...any) {
	name := ""
	if len(args) > 0 {
		name, _ = args[0].(string)
	}
	var eventArgs []any
	if len(args) > 1 {
		eventArgs = args[1:]
	}
	t.events = append(t.events, wings.CheckEvent{Name: name, Args: eventArgs})

	argStr := ""
	if len(eventArgs) > 0 {
		argStr = fmt.Sprintf("%v", eventArgs)
	}
	t.obj.This.Append("events", map[string]any{
		"seq":  len(t.events),
		"name": name,
		"args": argStr,
	})
	t.runAndSeal()
}

// runAndSeal runs the registered check against the current subject + event log
// and updates the seal. No-op in manual mode.
func (t *Test) runAndSeal() {
	if t.manual {
		return
	}
	ctx := wings.CheckCtx{Subject: t.subject, Dom: t.subject, Events: t.events}
	pass, detail, found := wings.RunCheck(t.checkName, ctx)
	if !found {
		t.setSeal("fail", "no check registered: "+t.checkName)
		return
	}
	if pass {
		t.setSeal("pass", detail)
	} else {
		t.setSeal("fail", detail)
	}
}

// setSeal updates the seal state across the bound flags, the detail line, and
// the host's data-wtest-state attribute (the headless-scraping hook).
func (t *Test) setSeal(state, detail string) {
	t.obj.This.Set("state", state)
	t.obj.This.Set("seal_pending", state == "pending")
	t.obj.This.Set("seal_pass", state == "pass")
	t.obj.This.Set("seal_fail", state == "fail")
	t.obj.This.Set("detail", detail)
	t.obj.Element.Call("setAttribute", "data-wtest-state", state)
	t.pulse()
}

// pulse plays a short scale animation on the seal so a re-run is perceptible
// even when the result is unchanged (Web Animations API replays on each call).
func (t *Test) pulse() {
	if !t.seals.Truthy() {
		return
	}
	frames := []any{
		map[string]any{"transform": "scale(1)"},
		map[string]any{"transform": "scale(1.35)"},
		map[string]any{"transform": "scale(1)"},
	}
	t.seals.Call("animate", js.ValueOf(frames), js.ValueOf(map[string]any{
		"duration": 220,
		"easing":   "ease-out",
	}))
}

func (t *Test) bindManualToggle() {
	if !t.seals.Truthy() {
		return
	}
	dom.AddEvent(t.seals, "click", func(_ js.Value, _ []js.Value) any {
		if cur, _ := t.obj.This.Get("state").(string); cur == "pass" {
			t.setSeal("fail", "")
		} else {
			t.setSeal("pass", "")
		}
		return nil
	}, false, false)
}

func (t *Test) bindRerun() {
	btns := dom.Query(t.obj.Dom, ".wt-rerun")
	if len(btns) == 0 {
		return
	}
	dom.AddEvent(btns[0], "click", func(_ js.Value, _ []js.Value) any {
		t.runAndSeal()
		return nil
	}, false, false)
}
