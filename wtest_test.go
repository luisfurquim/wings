//go:build js && wasm

package wings

import (
	"syscall/js"
	"testing"
)

// resetChecks clears the registry so tests don't leak into each other.
func resetChecks(t *testing.T) {
	t.Helper()
	checks = map[string]CheckFunc{}
	t.Cleanup(func() { checks = map[string]CheckFunc{} })
}

func TestRegisterAndRunCheck(t *testing.T) {
	resetChecks(t)

	RegisterCheck("sawSave", func(ctx CheckCtx) (bool, string) {
		for _, e := range ctx.Events {
			if e.Name == "save" {
				return true, "save fired"
			}
		}
		return false, "no save event"
	})

	// Not registered → found == false (config error, not a failed assertion).
	if _, _, found := RunCheck("ghost", CheckCtx{}); found {
		t.Error("RunCheck(ghost) found = true, want false for unregistered check")
	}

	// Registered but the event log lacks "save" → ran, failed.
	pass, detail, found := RunCheck("sawSave", CheckCtx{Events: []CheckEvent{{Name: "cancel"}}})
	if !found {
		t.Fatal("RunCheck(sawSave) found = false, want true")
	}
	if pass {
		t.Errorf("sawSave with only cancel: pass = true, want false")
	}
	if detail != "no save event" {
		t.Errorf("detail = %q, want %q", detail, "no save event")
	}

	// Log contains "save" → ran, passed.
	pass, detail, found = RunCheck("sawSave", CheckCtx{Events: []CheckEvent{{Name: "save", Args: []any{1}}}})
	if !found || !pass {
		t.Errorf("sawSave with save event: found=%v pass=%v, want true/true", found, pass)
	}
	if detail != "save fired" {
		t.Errorf("detail = %q, want %q", detail, "save fired")
	}
}

// fakeTestable is a PranaMod that also implements Testabler, for exercising the
// module-declared self-test registry without a real custom element.
type fakeTestable struct{ m map[string]CheckFunc }

func (fakeTestable) InitData() map[string]any         { return nil }
func (fakeTestable) Render(*PranaObj)                 {}
func (f fakeTestable) Testable() map[string]CheckFunc { return f.m }

// plainMod implements only PranaMod (no Testabler) — registerTestable must skip it.
type plainMod struct{}

func (plainMod) InitData() map[string]any { return nil }
func (plainMod) Render(*PranaObj)         {}

func TestRunReportTestables(t *testing.T) {
	liveTestables, wtestCards = nil, nil
	t.Cleanup(func() { liveTestables, wtestCards = nil, nil })

	// A plain module declares nothing.
	registerTestable(js.Undefined(), "plain", plainMod{})
	if len(liveTestables) != 0 {
		t.Fatalf("plainMod registered %d testables, want 0", len(liveTestables))
	}

	// A Testabler's checks are recorded and run, sorted by name.
	registerTestable(js.Undefined(), "w-foo", fakeTestable{m: map[string]CheckFunc{
		"b-check": func(CheckCtx) (bool, string) { return true, "ok-b" },
		"a-check": func(CheckCtx) (bool, string) { return false, "bad-a" },
	}})

	got := RunReport()
	if len(got) != 2 {
		t.Fatalf("RunReport returned %d results, want 2", len(got))
	}
	// Sorted by name within a module: a-check before b-check.
	if got[0].Kind != "testable" || got[0].Label != "w-foo/a-check" || got[0].State != "fail" || got[0].Detail != "bad-a" {
		t.Errorf("result[0] = %+v, want {testable w-foo/a-check fail bad-a}", got[0])
	}
	if got[1].Label != "w-foo/b-check" || got[1].State != "pass" || got[1].Detail != "ok-b" {
		t.Errorf("result[1] = %+v, want {testable w-foo/b-check pass ok-b}", got[1])
	}

	// Disconnect drops the entries.
	unregisterTestable(js.Undefined())
	if got := RunReport(); len(got) != 0 {
		t.Errorf("after unregister, RunReport returned %d, want 0", len(got))
	}
}

func TestRunReportIncludesCards(t *testing.T) {
	liveTestables, wtestCards = nil, nil
	t.Cleanup(func() { liveTestables, wtestCards = nil, nil })

	// A visual card left pending must still appear in the report.
	RegisterWTest(js.Undefined(), func() (string, string, string) {
		return "visual card", "pending", ""
	})
	got := RunReport()
	if len(got) != 1 || got[0].Kind != "w-test" || got[0].Label != "visual card" || got[0].State != "pending" {
		t.Errorf("report = %+v, want one pending w-test card", got)
	}
}

func TestRegisterCheckOverwrite(t *testing.T) {
	resetChecks(t)
	RegisterCheck("x", func(CheckCtx) (bool, string) { return false, "first" })
	RegisterCheck("x", func(CheckCtx) (bool, string) { return true, "second" })
	pass, detail, found := RunCheck("x", CheckCtx{})
	if !found || !pass || detail != "second" {
		t.Errorf("overwrite: found=%v pass=%v detail=%q, want true/true/second", found, pass, detail)
	}
}
