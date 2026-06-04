//go:build js && wasm

package wings

import "testing"

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

func TestRegisterCheckOverwrite(t *testing.T) {
	resetChecks(t)
	RegisterCheck("x", func(CheckCtx) (bool, string) { return false, "first" })
	RegisterCheck("x", func(CheckCtx) (bool, string) { return true, "second" })
	pass, detail, found := RunCheck("x", CheckCtx{})
	if !found || !pass || detail != "second" {
		t.Errorf("overwrite: found=%v pass=%v detail=%q, want true/true/second", found, pass, detail)
	}
}
