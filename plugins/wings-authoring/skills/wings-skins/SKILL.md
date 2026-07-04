---
name: wings-skins
description: Theme a WINGS app with --wings-* skin tokens — consume tokens in component CSS via var() with fallbacks, apply/compose built-in skins (ApplySkin), the ten disjoint skin categories and multi-skin composition, register your own skin, bundles (composed skins), and private categories. Use when styling components to be theme-aware, applying or composing skins, or building a theme.
---

# Theming with skins

A **skin** is a block of `--wings-*` CSS custom properties at `:root`, paired
with a category bitmask. Components read every token through
`var(--wings-X, fallback)`, so they look right with no skin active and restyle
globally the instant one is applied — there is no per-component theming step.

## Make a component theme-aware (consume tokens)

In your component CSS, reference tokens with a literal fallback so it works
unskinned:
```css
.card {
  background: var(--wings-surface, #fff);
  color: var(--wings-text, #222);
  border: var(--wings-border-width, 1px) var(--wings-border-style, solid)
          var(--wings-border, #ccc);
  border-radius: var(--wings-radius-md, 6px);
  box-shadow: var(--wings-shadow-md, 0 1px 3px rgba(0,0,0,.12));
  transition: background var(--wings-transition-fast, .15s);
}
.card button { background: var(--wings-primary, #0b6); color: var(--wings-primary-color, #fff); }
```
Always provide the fallback — never assume a skin is loaded. The full token
contract is `skins/tokens.md` in the wings repo, one section per category.

## Apply built-in skins

Blank-import each skin you use (its `init()` registers it), then activate:
```go
import (
	"github.com/luisfurquim/wings"
	_ "github.com/luisfurquim/wings/skins/light"
	_ "github.com/luisfurquim/wings/skins/classic"
	_ "github.com/luisfurquim/wings/skins/glass"
)

func main() {
	_ = wings.ApplySkin("light")   // returns error; handle, don't ignore in real code
	_ = wings.ApplySkin("classic") // composes (disjoint categories)
	_ = wings.ApplySkin("glass")
	wings.Main()
}
```

## Composition & the ten categories

Several skins can be active **only if their category bitmasks are disjoint**
(AND == 0). A *complete theme* (one category) composes with *focused* skins on
orthogonal dimensions. Activating an overlapping skin returns a
`*SkinConflictError` (it never silently double-sets a token).

Categories: `Identity` (colours/surfaces/text/borders/shadow-colour), `Geometry`
(radius/border), `Depth` (shadow shape), `Motion` (transitions/hover/active),
`Interaction` (focus-ring), `Typography` *(reserved)*, `Spacing` (padding/gap),
`Lighting` (gradients/glows), `Atmosphere` (glass blur/opacity),
`Material` (widget surface form: outlined / filled / underlined).

Built-ins (23) — pick at most one per exclusive column:
- Identity: `light dark autumn darkblueberry darkforest lightblueberry mushroom vividforest`
- Geometry: `sharp classic soft` · Depth: `flat lifted floating` · Motion: `gentle calm brisk`
- Atmosphere: `glass` · Lighting: `glasslighting` · Material: `outlined filled underlined`
- Bundles: `glassmorphism` = `glass` + `glasslighting` (claims Atmosphere|Lighting)

A full look is one from each, e.g. `light + classic + lifted + calm (+ filled)`.
Every built-in is importable at `github.com/luisfurquim/wings/skins/<name>`.

## Bundles — composed skins without CSS duplication

A **bundle** is a skin whose CSS concatenates two or more component skins. Each
component exports `CSS` and `Categories` vars so bundles can reuse them:

```go
// skins/glassmorphism/skin.go
import (
    "github.com/luisfurquim/wings/skins/glass"
    "github.com/luisfurquim/wings/skins/glasslighting"
)
func init() {
    wings.RegisterSkin("glassmorphism",
        glass.Categories|glasslighting.Categories,
        glass.CSS+glasslighting.CSS)
}
```

Apply the bundle for convenience, or the components individually for finer
control (e.g. `glasslighting` without `glass` on environments that lack
`backdrop-filter` support).

## Register your own skin

```go
//go:embed skin.css
var css string

func init() {
	// Declare exactly the dimensions your CSS defines (for composition/conflict).
	wings.RegisterSkin("midnight", wings.IdentitySkinCategories, css) // Identity|Lighting|Interaction
	// focused: wings.RegisterSkin("rounded", wings.GeometrySkinCategories, css)
	// material: wings.RegisterSkin("paper", wings.MaterialSkinCategories, css)
}
```
Convenience masks: `IdentitySkinCategories`, `GeometrySkinCategories`,
`DepthSkinCategories`, `MotionSkinCategories`, `MaterialSkinCategories`.

**Private categories** (your own orthogonal dimension, e.g. brand): mint with
`wings.UserCategory(n)` (n in [0,16), high bits 48–63 are permanently yours),
optionally `RegisterCategoryName`, then register skins against it — it joins the
same conflict detection and never collides with future built-ins. (The API
returns errors, never panics — handle them.)

## Material tokens (widget surface form)

The `Material` category controls how field borders and backgrounds are
structurally shaped. Widgets consume `--wings-input-material-*` tokens; when
none are set the widget defaults produce the `outlined` look.

| Token | Semantics |
|---|---|
| `--wings-input-material-border-top/right/bottom/left` | Individual border sides |
| `--wings-input-material-radius` | border-radius (accepts multi-value: `6px 6px 0 0`) |
| `--wings-input-material-bg` | Field background |
| `--wings-input-material-padding-x` | Horizontal inner padding |
| `--wings-input-material-focus-shadow` | Box-shadow on focus (`none` suppresses ring) |
| `--wings-input-material-focus-shadow-error` | Box-shadow on focused+invalid |

## API & UI

`ApplySkin` / `DeactivateSkin` / `ClearSkins`, `ActiveSkins` / `ActiveSkin`,
`ActiveCategories`, `ConflictsWith`, `ListSkins` / `ListSkinInfos`,
`SkinCategoriesOf`, `OnSkinChange`. The built-in `<skin-switcher>` widget exposes
all of it as a picker (see `wings-widgets`).

Deeper: README "Skins — Theming with --wings-* Tokens".
