//go:build js && wasm

package wings

// CustomFlex is the contract for programmable inflection inside a flex block.
//
// The built-in flex pipeline (gender × CLDR-plural cells, filled at build time
// by gen_i18n) covers the zero-config case. CustomFlex is the opt-in escape
// hatch for inflection that cannot be precomputed — arbitrary selection axes
// beyond gender/count, or dynamic values inflected at runtime (possibly via a
// backend call). A value reachable from the data context through a `*var`
// (engine/selector) or `~$var` (flexbind) sigil must implement CustomFlex.
//
// Roles inside a block:
//   - As the elected engine, Flex is called once per word to inflect (each
//     static `~word`, and each `~$var` value), receiving every other
//     participant as a FlexSelector. The engine is elected by Priority (see
//     Prioritized); the implicit catalog engine wins only when no CustomFlex
//     value is present.
//   - As a (losing) selector, the value still contributes String() at its
//     position and is passed to the winning engine, which may consult it.
//
// Contract notes:
//   - Flex MUST be synchronous. For async sources (REST, etc.) return a cached
//     value or a placeholder immediately and trigger a re-sync when the real
//     value arrives (the sync model re-runs Flex on the next pass) — blocking
//     on a fetch deadlocks the JS event loop, exactly as for SetLang.
//   - Flex MUST read wings.Locale at call time (never cache the locale): a
//     SetLang switches Locale and re-syncs in place.
//   - A non-nil error renders the input word verbatim plus a log entry, so a
//     failure stays visible rather than blanking the node.
type CustomFlex interface {
	// Flex returns the inflected form of word given the other block
	// participants. The order of selectors is NOT guaranteed — identify each
	// by Name/Sigil, never by position.
	Flex(word string, selectors ...FlexSelector) (string, error)
	// String returns the text this value emits at its own position in the
	// block ("" = pure selector, emits nothing; e.g. a count selector returns
	// the formatted number).
	String() string
}

// Prioritized is an optional companion to CustomFlex. When a flex block holds
// more than one engine-capable CustomFlex value, the one with the highest
// Priority is elected the engine. A CustomFlex that does not implement
// Prioritized counts as priority 0. The implicit catalog engine is below every
// user value, so any single user engine wins; a tie between user engines is an
// error (logged, with a verbatim fallback).
type Prioritized interface {
	Priority() uint
}

// FlexSelector is one resolved participant of a flex block, handed to
// CustomFlex.Flex as context. Sigil is the originating marker ('@', '%', '*',
// or '$'); Name is the variable path as written in the template (e.g.
// "gender", "user.tier", "cart[i].qty"); Value is the value resolved from the
// live data context.
type FlexSelector struct {
	Sigil byte
	Name  string
	Value any
}
