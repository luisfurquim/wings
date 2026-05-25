//go:build js && wasm

package wprana

import "strings"

// SkinCategory is a bitmask describing which design dimensions a skin
// touches. Each skin declares its set of categories at registration time;
// two skins can be active simultaneously only when their bitmasks are
// disjoint (the AND is zero). This lets a "complete theme" (e.g. mushroom
// — cores + geometria + profundidade + …) compose with a focused skin
// (e.g. glass — apenas atmosfera).
//
// The 64-bit width leaves room for new categories without breaking the
// API. Only the lowest 9 bits are currently defined.
type SkinCategory uint64

// Categories. Each bit identifies one design dimension. New categories
// MUST append at the end so existing bitmasks keep their meaning.
//
// The split between adjacent categories is deliberate: any token whose
// VALUE is a colour belongs to the chromatic categories (Identity,
// Lighting, Interaction). Tokens whose value is a metric (px, ms,
// ratio, transform) belong to the structural/temporal categories
// (Geometry, Spacing, Depth, Motion). Shadows are split: the shape
// (offsets+blur+spread) is Depth; the colour rgba is Identity (via
// `--wings-shadow-color-*`). Widgets read the composed `--wings-shadow-*`
// produced by Depth skins via `var()`.
const (
	CategoryIdentity    SkinCategory = 1 << iota // colors, surfaces, text, primary, secondary, borders, button colors, shadow-color
	CategoryGeometry                             // radius scale, border width/style
	CategoryDepth                                // shadow shapes (offsets/blur/spread); the colour comes from Identity
	CategoryMotion                               // transition durations/easing, hover-lift, active-scale
	CategoryInteraction                          // focus-ring (chromatic feedback)
	CategoryTypography                           // font-family, font-size, font-weight (reserved)
	CategorySpacing                              // padding/gap density
	CategoryLighting                             // gradients, glows, gradient-shadow
	CategoryAtmosphere                           // glass-opacity, surface-blur, surface-noise
)

// CategoryNone is the empty bitmask (no categories).
const CategoryNone SkinCategory = 0

// CategoryAll is the OR of every category currently defined. New
// categories appended above are automatically included.
const CategoryAll = CategoryIdentity | CategoryGeometry | CategoryDepth |
	CategoryMotion | CategoryInteraction | CategoryTypography |
	CategorySpacing | CategoryLighting | CategoryAtmosphere

// IdentitySkinCategories is the conventional bitmask for a chromatic
// theme — one that defines colors, gradients, focus-ring and shadow-color
// tokens. The eight built-in themes (light, dark, autumn, …) all use
// this mask, so they are mutually exclusive among themselves but
// coexist with focused skins covering orthogonal categories
// (Geometry, Depth, Motion, Spacing, Atmosphere).
const IdentitySkinCategories = CategoryIdentity | CategoryLighting | CategoryInteraction

// GeometrySkinCategories is the bitmask for skins controlling shape and
// density: corner radius, border width/style, padding, gap. The three
// built-in geometry skins (sharp / classic / soft) all share this mask.
const GeometrySkinCategories = CategoryGeometry | CategorySpacing

// DepthSkinCategories is the bitmask for skins controlling the metric
// component of shadows (offsets/blur/spread). Built-in: flat, lifted,
// floating. The colour rgba of every shadow comes from the active
// Identity skin via `--wings-shadow-color-*`.
const DepthSkinCategories = CategoryDepth

// MotionSkinCategories is the bitmask for skins controlling motion:
// transition durations/easing, hover-lift, active-scale. Built-in:
// gentle, calm, brisk.
const MotionSkinCategories = CategoryMotion

// categoryNames maps each single-bit category to its human-readable name.
// Order matters: it controls the listing order produced by Names()/String().
var categoryNames = []struct {
	bit  SkinCategory
	name string
}{
	{CategoryIdentity, "Identity"},
	{CategoryGeometry, "Geometry"},
	{CategoryDepth, "Depth"},
	{CategoryMotion, "Motion"},
	{CategoryInteraction, "Interaction"},
	{CategoryTypography, "Typography"},
	{CategorySpacing, "Spacing"},
	{CategoryLighting, "Lighting"},
	{CategoryAtmosphere, "Atmosphere"},
}

// Has reports whether c contains every bit set in other.
func (c SkinCategory) Has(other SkinCategory) bool {
	return c&other == other
}

// Conflicts reports whether c and other share any bit (i.e. they cannot
// be active simultaneously).
func (c SkinCategory) Conflicts(other SkinCategory) bool {
	return c&other != 0
}

// Names returns the names of the categories present in c, in declaration
// order. Unknown bits (set but not named) are skipped.
func (c SkinCategory) Names() []string {
	out := make([]string, 0, len(categoryNames))
	for _, e := range categoryNames {
		if c&e.bit != 0 {
			out = append(out, e.name)
		}
	}
	return out
}

// String returns "Identity|Geometry|…" — the names of the categories
// present in c joined by "|". Returns "None" when c is empty.
func (c SkinCategory) String() string {
	if c == CategoryNone {
		return "None"
	}
	return strings.Join(c.Names(), "|")
}
