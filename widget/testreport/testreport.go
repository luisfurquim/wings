//go:build js && wasm

// Package testreport provides the <w-test-report> custom element: a button that
// collects the full page test report — every <w-test> card (including the
// human-judged visual ones) plus every check declared by mounted Testabler
// modules (see wings.RunReport) — renders it as JSON, and fires a "report"
// trigger carrying that JSON.
//
// wings produces the report only. What to do with it — POST it to a server,
// write it to a file, diff it in CI — is the app's call; wire it via
// <w-test-report @report="fn"> and handle the JSON string in fn.
package testreport

import (
	_ "embed"
	"encoding/json"
	"syscall/js"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
)

const elementTag = "w-test-report"

// G is the logger for this module.
var G goose.Alert

//go:embed testreport.html
var htmlContent string

//go:embed design.css
var designCSS string

// New creates a new Report instance.
func New() *Report { return &Report{} }

func init() {
	G.Set(3)
	wings.Register(
		elementTag,
		htmlContent,
		designCSS,
		func() wings.PranaMod { return &Report{} },
	)
	G.Logf(3, "w-test-report: module registered\n")
}

// Report implements wings.PranaMod for the w-test-report custom element.
type Report struct {
	obj *wings.PranaObj
}

func (r *Report) InitData() map[string]any {
	return map[string]any{
		"json":  "",
		"count": 0,
		"ran":   false,
	}
}

func (r *Report) Render(obj *wings.PranaObj) {
	r.obj = obj
	btns := dom.Query(obj.Dom, ".wtr-run")
	if len(btns) == 0 {
		return
	}
	dom.AddEvent(btns[0], "click", func(_ js.Value, _ []js.Value) any {
		r.run()
		return nil
	}, false, false)
}

// run collects the full page report — every <w-test> card plus every module
// self-test — shows the JSON, and fires the report event so the app can
// transport/persist it however it likes.
func (r *Report) run() {
	results := wings.RunReport()
	b, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		G.Logf(1, "w-test-report: marshal failed: %v\n", err)
		return
	}
	r.obj.This.Set("count", len(results))
	r.obj.This.Set("ran", true)
	r.obj.This.Set("json", string(b))
	r.obj.Trigger("report", string(b))
}
