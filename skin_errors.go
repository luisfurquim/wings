//go:build js && wasm

package wings

import (
	"fmt"
	"strings"
)

// SkinNotRegisteredError is returned by ApplySkin and DeactivateSkin when
// the named skin has not been registered via RegisterSkin.
type SkinNotRegisteredError struct {
	Name string
}

func (e *SkinNotRegisteredError) Error() string {
	return fmt.Sprintf("wings: skin %q is not registered", e.Name)
}

// SkinConflictError is returned by ApplySkin when activating a skin would
// overlap one or more categories already covered by an active skin. The
// caller can inspect Conflicts (the colliding active skin names) and
// ConflictingCategories (the bits that overlap) to surface a precise
// message in the UI without re-deriving the comparison.
type SkinConflictError struct {
	Name                  string       // the skin that failed to activate
	Categories            SkinCategory // its declared categories
	Conflicts             []string     // names of active skins that share bits
	ConflictingCategories SkinCategory // the OR of all colliding bits
}

func (e *SkinConflictError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "wings: skin %q (%s) conflicts on categories %s with active skin",
		e.Name, e.Categories, e.ConflictingCategories)
	if len(e.Conflicts) == 1 {
		fmt.Fprintf(&sb, " %q", e.Conflicts[0])
	} else {
		fmt.Fprintf(&sb, "s %s", strings.Join(quoteAll(e.Conflicts), ", "))
	}
	return sb.String()
}

func quoteAll(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = fmt.Sprintf("%q", n)
	}
	return out
}
