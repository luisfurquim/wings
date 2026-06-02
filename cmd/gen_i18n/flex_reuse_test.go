package main

import "testing"

// resetFlexState clears the package-level flex accumulators so each test runs
// against a clean catalog.
func resetFlexState() {
	flexBlocks = nil
	flexKeys = nil
	flexKeyIdx = map[string]int32{}
	flexOccurrences = map[int32][]Occurrence{}
	flexContent = map[int32]string{}
	flexDefs = map[string]flexDef{}
}

// A `=name` definition followed by a `#name` reuse: the pre-pass registers the
// name, the definer rewrites to its own runtime form and captures its content,
// and the reuse site rewrites to the same rule index with its control sigils
// merged over the definer's (here %qt added, *engine inherited, @g inherited).
func TestRewriteReuse(t *testing.T) {
	resetFlexState()

	// Pre-pass: register the =name definition (mirrors collectFlexNames).
	scanFlexNames("{{=greeting *motor @g Olá ~$nome}}")
	if _, ok := flexDefs["greeting"]; !ok {
		t.Fatalf("flexDefs missing greeting after scan")
	}

	// The definer rewrites to {{@g *motor #0}} and stores its content phrase.
	got, changed := rewriteFlexBlocks("{{=greeting *motor @g Olá ~$nome}}", nil)
	if !changed || got != "{{@g *motor #0}}" {
		t.Errorf("definer rewrite = %q (changed=%v); want {{@g *motor #0}}", got, changed)
	}
	if flexContent[0] != "Olá ~$nome" {
		t.Errorf("flexContent[0] = %q; want %q", flexContent[0], "Olá ~$nome")
	}

	// The reuse site rewrites to the same rule index, %qt merged in.
	got, changed = rewriteFlexBlocks("{{#greeting %qt}}", nil)
	if !changed || got != "{{@g %qt *motor #0}}" {
		t.Errorf("reuse rewrite = %q (changed=%v); want {{@g %%qt *motor #0}}", got, changed)
	}
}

// The reuse site overrides per slot: a *engine declared at the site replaces
// the definer's engines wholesale, and a @var at the site wins over the
// definer's.
func TestRewriteReuseOverride(t *testing.T) {
	resetFlexState()
	scanFlexNames("{{=msg *engA @gdef ~$x}}")
	rewriteFlexBlocks("{{=msg *engA @gdef ~$x}}", nil)

	got, _ := rewriteFlexBlocks("{{#msg *engB @gsite}}", nil)
	if got != "{{@gsite *engB #0}}" {
		t.Errorf("override rewrite = %q; want {{@gsite *engB #0}}", got)
	}
}

// A `#name` reached before its `=name` in walk order (forward reference) still
// resolves, because the pre-pass registered the definition. The reuse site gets
// the index assigned on first reference; the later definition reuses it.
func TestRewriteReuseForwardRef(t *testing.T) {
	resetFlexState()
	// Pre-pass sees both; the reuse is rewritten before the definition is.
	scanFlexNames("{{=later *motor ~$x}}")

	got, _ := rewriteFlexBlocks("{{#later %qt}}", nil)
	if got != "{{%qt *motor #0}}" {
		t.Errorf("forward reuse = %q; want {{%%qt *motor #0}}", got)
	}
	// The definition, walked later, reuses the same index 0.
	got, _ = rewriteFlexBlocks("{{=later *motor ~$x}}", nil)
	if got != "{{*motor #0}}" {
		t.Errorf("definition after forward ref = %q; want {{*motor #0}}", got)
	}
}

// A `#name` with no matching definition is left verbatim (visible degradation).
func TestRewriteReuseUnknown(t *testing.T) {
	resetFlexState()
	got, changed := rewriteFlexBlocks("{{#missing %qt}}", nil)
	if changed || got != "{{#missing %qt}}" {
		t.Errorf("unknown reuse = %q (changed=%v); want it left verbatim", got, changed)
	}
}
