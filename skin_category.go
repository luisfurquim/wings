//go:build js && wasm

package wings

import (
	"errors"
	"strconv"
	"strings"
)

// SkinCategory is a bitmask describing which design dimensions a skin
// touches. Each skin declares its set of categories at registration time;
// two skins can be active simultaneously only when their bitmasks are
// disjoint (the AND is zero). This lets a "complete theme" (e.g. mushroom
// — cores + geometria + profundidade + …) compose with a focused skin
// (e.g. glass — apenas atmosfera).
//
// The 64-bit width leaves room for new categories without breaking the
// API. The bit space is partitioned like IANA address space: the low bits
// (0–8 used today) are the official, framework-owned categories and grow
// upward as new built-ins are appended; the high 16 bits (48–63) are
// permanently reserved for application- or library-defined private
// categories — see CategoryUserMask, UserCategory and RegisterCategoryName.
// The middle bits (9–47) are headroom for future built-ins. A built-in
// category will never be assigned a bit in the user-reserved range, so
// private categories stay forward-compatible across WINGS upgrades.
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

// User-reserved category range. The high 16 bits (48–63) are never used by
// a built-in category, so applications and component libraries can define
// their own orthogonal design dimensions that participate in the same
// disjoint-mask conflict-detection contract as the built-ins.
const (
	userCategoryShift = 48 // lowest bit of the user-reserved range
	userCategoryCount = 16 // number of user-reserved bits (48–63)
)

// CategoryUserMask is the set of all user-reserved category bits (48–63).
// Test whether a mask carries any private category with
// `c & wings.CategoryUserMask != 0`.
const CategoryUserMask SkinCategory = ((1 << userCategoryCount) - 1) << userCategoryShift

// Errors returned by the user-category API.
var (
	// ErrUserCategoryRange is returned by UserCategory when the index is
	// outside [0, 16).
	ErrUserCategoryRange = errors.New("wings: user category index out of range [0,16)")
	// ErrNotUserCategory is returned by RegisterCategoryName when the bit is
	// not a single category within the user-reserved range.
	ErrNotUserCategory = errors.New("wings: not a single user-reserved category bit")
)

// userCategoryNames holds optional display names for user-reserved category
// bits, populated by RegisterCategoryName.
var userCategoryNames = map[SkinCategory]string{}

// UserCategory returns the n-th user-reserved category bit, with n in the
// range [0, 16). The single-bit mask it returns is guaranteed never to
// collide with a built-in category, so it is the safe way to mint a private
// design dimension. Returns ErrUserCategoryRange if n >= 16; the caller
// decides whether to treat that as fatal.
//
//	func init() {
//	    brand, err := wings.UserCategory(0)
//	    if err != nil { panic(err) } // the app's call, not the library's
//	    wings.RegisterCategoryName(brand, "Brand")
//	    wings.RegisterSkin("acme", brand, acmeCSS)
//	}
func UserCategory(n uint) (SkinCategory, error) {
	if n >= userCategoryCount {
		return 0, ErrUserCategoryRange
	}
	return SkinCategory(1) << (userCategoryShift + n), nil
}

// RegisterCategoryName assigns a human-readable name to a user-reserved
// category bit (one returned by UserCategory). The name then appears in
// SkinCategory.Names()/String() and therefore in the <skin-switcher> UI.
// Calling it again for the same bit overwrites the previous name. Returns
// ErrNotUserCategory if bit is not a single bit within the user-reserved
// range.
func RegisterCategoryName(bit SkinCategory, name string) error {
	if bit == 0 || bit&(bit-1) != 0 || bit&^CategoryUserMask != 0 {
		return ErrNotUserCategory
	}
	userCategoryNames[bit] = name
	return nil
}

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
	// User-reserved categories (bits 48–63): show the registered name, or a
	// stable "User<n>" fallback so an un-named private bit still surfaces.
	for n := uint(0); n < userCategoryCount; n++ {
		bit := SkinCategory(1) << (userCategoryShift + n)
		if c&bit == 0 {
			continue
		}
		if name, ok := userCategoryNames[bit]; ok {
			out = append(out, name)
		} else {
			out = append(out, "User"+strconv.FormatUint(uint64(n), 10))
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
