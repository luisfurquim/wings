# wings skin tokens

This file is the canonical contract between **skins** (which define CSS
custom properties at `:root`) and **widgets** (which reference them with a
fallback). Every token name listed here is reserved for that purpose.

> **Naming.** All tokens use the `--wings-*` prefix. The project will be
> renamed to **WINGS — Web IN Go Sphere**; the prefix is adopted ahead of
> the rename to avoid a second migration. Third-party widgets are expected
> to use their own prefix.

## Authoring rules

- **Skins** declare a `SkinCategory` bitmask at `wings.RegisterSkin(name,
  categories, css)`. The CSS is injected by `wings.ApplySkin(name)` into
  a `<style id="wings-skin-<name>" data-wings-skin="<name>">` element
  appended to `document.head`. Multiple skins can be active simultaneously
  if their bitmasks are disjoint.
- **Skins are decomposed by orthogonal axis**. The shipped families are:
  - **Identity** (chromatic): cores, gradient, focus-ring, shadow colour
  - **Geometry+Spacing** (forma/densidade): radius, border, padding, gap
  - **Depth** (sombra métrica): shadow shape (offsets/blur/spread)
  - **Motion** (ritmo): transitions, hover-lift, active-scale
  - **Atmosphere** (efeito): glass-blur, surface-saturate
  Cada família tem 1–8 alternativas. As do mesmo eixo são mutuamente
  exclusivas; com eixos diferentes compõem livremente.
- **Widgets** reference tokens with a fallback so they remain functional
  without an active skin:

  ```css
  border: 1px solid var(--wings-border, #ccc);
  ```

  Widgets MUST NOT redefine tokens with `:host { --wings-X: ...; }` — that
  shadows the global value inside the widget's shadow tree and breaks
  skin override.
- **Adding a token**: place it under the category that semantically owns
  it. Then update every skin that declares that category so they remain
  coherent. Widgets may then reference the new token.

## Categories

A skin declares which design dimensions it owns by combining the
`wings.SkinCategory` constants below with `|`. Two skins coexist iff
their bitmasks have no bit in common.

| Constant                | Bit | Domain                                                      |
|-------------------------|-----|-------------------------------------------------------------|
| `CategoryIdentity`      | 0   | Colors, surfaces, text, primary, secondary, button colors   |
| `CategoryGeometry`      | 1   | Radius scale, border-width, border-style                    |
| `CategoryDepth`         | 2   | Shadow scale, elevation, drop-shadows                       |
| `CategoryMotion`        | 3   | Transition durations/easing, hover-lift                     |
| `CategoryInteraction`   | 4   | Active-scale, focus-ring, click feedback                    |
| `CategoryTypography`    | 5   | Font family, sizes, weights *(reserved — no tokens yet)*    |
| `CategorySpacing`       | 6   | Padding/gap density *(reserved — no tokens yet)*            |
| `CategoryLighting`      | 7   | Gradients, glows, gradient-shadow                           |
| `CategoryAtmosphere`    | 8   | Glass-opacity, surface-blur, surface-noise                  |

Built-in bitmask helpers (in `skin_category.go`):

- `wings.IdentitySkinCategories` = `Identity | Lighting | Interaction` —
  used by the eight chromatic themes (`light`, `dark`, `autumn`,
  `darkblueberry`, `darkforest`, `lightblueberry`, `mushroom`,
  `vividforest`).
- `wings.GeometrySkinCategories` = `Geometry | Spacing` — used by
  `sharp`, `classic`, `soft`.
- `wings.DepthSkinCategories` = `Depth` — used by `flat`, `lifted`,
  `floating`.
- `wings.MotionSkinCategories` = `Motion` — used by `gentle`, `calm`,
  `brisk`.

Each helper's family is internally mutually exclusive; the four helpers
are pairwise disjoint, so up to one skin from each family can stack with
each other (and with `glass` from the Atmosphere axis).

---

## Identity (`CategoryIdentity`)

Color semantics that define the visual brand. Tokens are grouped by
**functional role** (what the element does in the UI), not by widget —
so a chip in a combobox and a chip in a future tag-list reuse the same
`--wings-tiny-element-*` tokens. A widget that needs finer control over
its own surface only must reach for the role group whose semantics match.

### Core palette
| Token                          | Semantics                                        |
|--------------------------------|--------------------------------------------------|
| `--wings-bg`                   | Page background (the outermost surface)          |
| `--wings-surface`              | Elevated surface above the page bg (panels, cards, dialogs) |
| `--wings-text`                 | Primary foreground text colour                   |
| `--wings-text-muted`           | Intermediate dimmed text                         |
| `--wings-text-light`           | Secondary/very dimmed text (helper labels)       |
| `--wings-primary`              | Brand accent — primary CTA bg, links, focused input text |
| `--wings-primary-color`        | Foreground colour to use **on** `--wings-primary` (CTA text) |
| `--wings-primary-hover-bg`     | `--wings-primary` darkened/shifted for hover state |
| `--wings-primary-pale`         | Pale tint of the primary colour                  |
| `--wings-secondary`            | Secondary accent (warm complement to primary)    |
| `--wings-quiet`                | Quiet counterpart to secondary                   |
| `--wings-border`               | Default border colour for inputs/cards           |
| `--wings-border-focus`         | Border colour when an input is focused           |
| `--wings-btn-bg`               | Inline neutral button background                 |
| `--wings-btn-hover-bg`         | Inline neutral button background on hover        |
| `--wings-btn-hover-color`      | Inline neutral button text colour on hover       |
| `--wings-btn-hover-shadow`     | Box-shadow on inline neutral button hover        |

### Tiny elements — chips, tags, badges
| Token                          | Semantics                              |
|--------------------------------|----------------------------------------|
| `--wings-tiny-element-bg`      | Background fill                        |
| `--wings-tiny-element-color`   | Text colour                            |
| `--wings-tiny-element-border`  | Border colour                          |

### Remover (×) — close/dismiss/remove buttons inside chips and rows
| Token                          | Semantics                              |
|--------------------------------|----------------------------------------|
| `--wings-remover-color`        | × colour at rest                       |
| `--wings-remover-hover-bg`     | × hover background                     |
| `--wings-remover-hover-color`  | × hover colour (typically a danger hue)|

### Text input — single-line text fields
| Token                          | Semantics                              |
|--------------------------------|----------------------------------------|
| `--wings-input-bg`             | Field background                       |
| `--wings-input-border`         | Default border                         |
| `--wings-input-focus-border`   | Border when focused                    |
| `--wings-input-focus-shadow`   | Focus ring rgba                        |

### List box — dropdowns, popovers, scrollable surfaces
| Token                          | Semantics                              |
|--------------------------------|----------------------------------------|
| `--wings-list-box-bg`          | Panel background                       |
| `--wings-list-box-border`      | Panel border                           |
| `--wings-list-box-shadow`      | Drop shadow override (falls back to `--wings-shadow-md`) |
| `--wings-scroll-thumb`         | Custom scrollbar thumb colour          |
| `--wings-empty-list-color`     | "No results" / placeholder text colour |

### List item — option/menu rows
| Token                            | Semantics                            |
|----------------------------------|--------------------------------------|
| `--wings-list-item-color`        | Text at rest                         |
| `--wings-list-item-hover-bg`     | Background on hover                  |
| `--wings-list-item-hover-color`  | Text on hover                        |
| `--wings-list-item-active-bg`    | Background on press/active           |

### Button — block button (w-button, dialog actions, toolbars)
| Token                              | Semantics                                          |
|------------------------------------|----------------------------------------------------|
| `--wings-button-bg`                | At rest                                            |
| `--wings-button-hover-bg`          | Hover                                              |
| `--wings-button-active-bg`         | Active/pressed                                     |
| `--wings-button-border`            | Border at rest (full shorthand: `1px solid X`)     |
| `--wings-button-border-hover`      | Border on hover (full shorthand)                   |
| `--wings-button-font-weight`       | Label font weight (default 500)                    |
| `--wings-button-disabled-opacity`  | Opacity when `[disabled]` (default 0.5)            |

### Semantic state colours — danger / success
Used by `w-button` variants and any component that needs semantic colouring.

| Token                     | Semantics                                        |
|---------------------------|--------------------------------------------------|
| `--wings-danger`          | Danger/destructive action bg (default `#dc3545`) |
| `--wings-danger-hover`    | Danger hover bg (default `#bd2130`)              |
| `--wings-success`         | Success action bg (default `#28a745`)            |
| `--wings-success-hover`   | Success hover bg (default `#218838`)             |

### Input — text input field (w-input)
| Token                              | Semantics                                          |
|------------------------------------|----------------------------------------------------|
| `--wings-input-bg`                 | Field background                                   |
| `--wings-input-bg-filled`          | Field background in `variant="filled"`             |
| `--wings-input-color`              | Input text colour                                  |
| `--wings-input-placeholder-color`  | Placeholder text colour                            |
| `--wings-input-border`             | Field border shorthand at rest                     |
| `--wings-input-border-focus`       | Field border on focus                              |
| `--wings-input-border-error`       | Field border when `[data-invalid]`                 |
| `--wings-input-label-color`        | Label text colour                                  |
| `--wings-input-label-color-focus`  | Label colour when field is focused                 |
| `--wings-input-label-color-error`  | Label colour when field is invalid                 |
| `--wings-input-helper-color`       | Helper text colour                                 |
| `--wings-input-error-color`        | Error message colour                               |
| `--wings-input-count-color`        | Character count colour                             |
| `--wings-input-prefix-color`       | Prefix / suffix icon colour                        |
| `--wings-input-clear-color`        | Clear button (×) colour at rest                    |
| `--wings-input-disabled-opacity`   | Opacity when `[disabled]` (default 0.5)            |

### Box — modal/panel containers
| Token                            | Semantics                            |
|----------------------------------|--------------------------------------|
| `--wings-box-overlay-bg`         | Backdrop overlay (rgba)              |
| `--wings-box-padding`            | Inner padding                        |

---

## Geometry (`CategoryGeometry`)

Shape language: corner radius and border style. Skins shift the whole UI
between sharp and rounded uniformly through these tokens.

| Token                  | Suggested value | Semantics                                  |
|------------------------|-----------------|--------------------------------------------|
| `--wings-radius-xs`    | `2px`           | Subtle rounding (input chips, badges)      |
| `--wings-radius-sm`    | `4px`           | Tags, small inline elements                |
| `--wings-radius-md`    | `6px`           | Buttons, tab buttons, inputs               |
| `--wings-radius-lg`    | `8px`           | Cards, panels, dialogs                     |
| `--wings-radius-pill`  | `9999px`        | Pills, fully-round badges                  |
| `--wings-border-width` | `1px`           | Default border thickness                   |
| `--wings-border-style` | `solid`         | Default border style                       |

---

## Depth (`CategoryDepth`)

Shadow geometry. **Sombras são compostas em duas camadas**: a Depth
skin define a forma (offsets/blur/spread) e a Identity skin define a
cor (rgba). A Depth skin então compõe ambas no token consumível
`--wings-shadow-*`. Widgets sempre leem só o composto.

| Token                       | Defined by    | Semantics                              |
|-----------------------------|---------------|----------------------------------------|
| `--wings-shadow-sm-shape`   | Depth skin    | Offsets+blur+spread para tier sm       |
| `--wings-shadow-md-shape`   | Depth skin    | idem, tier md                          |
| `--wings-shadow-lg-shape`   | Depth skin    | idem, tier lg                          |
| `--wings-shadow-inset-shape`| Depth skin    | idem, tier inset                       |
| `--wings-shadow-color-sm`   | Identity skin | Cor rgba para tier sm                  |
| `--wings-shadow-color-md`   | Identity skin | Cor rgba para tier md                  |
| `--wings-shadow-color-lg`   | Identity skin | Cor rgba para tier lg                  |
| `--wings-shadow-color-inset`| Identity skin | Cor rgba para tier inset               |
| `--wings-shadow-sm`         | Depth (composto) | shape + color final                  |
| `--wings-shadow-md`         | Depth (composto) | shape + color final                  |
| `--wings-shadow-lg`         | Depth (composto) | shape + color final                  |
| `--wings-shadow-inset`      | Depth (composto) | shape + color final                  |
| `--wings-list-box-shadow`   | (opcional)    | Override em dropdown; falls back to `--wings-shadow-md` |

Sem Identity ou Depth ativos, o widget cai no fallback hard-coded em
`var(--wings-shadow-md, fallback)`.

---

## Motion (`CategoryMotion`)

Animation and click-feedback transforms (no colour). The `active-scale`
moved here from Interaction so a Motion skin owns every kinetic
behaviour while Interaction holds only chromatic feedback.

| Token                       | Suggested value           | Semantics                              |
|-----------------------------|---------------------------|----------------------------------------|
| `--wings-transition-fast`   | `120ms ease`              | Hover/focus reactions                  |
| `--wings-transition-normal` | `200ms ease`              | Standard property changes              |
| `--wings-transition-slow`   | `350ms ease`              | Panels expanding, drawer/dialog open   |
| `--wings-hover-lift`        | `translateY(-1px)`        | Hover raise transform                  |
| `--wings-active-scale`      | `scale(0.97)`             | Click-feedback transform               |

Built-in skins: `gentle` (slow + still), `calm` (default), `brisk`
(fast + pronounced).

---

## Interaction (`CategoryInteraction`)

Chromatic feedback for input focus. Currently a single token; the
shipped Identity skins all declare it because the colour is brand-tied.

| Token                  | Suggested value                | Semantics                              |
|------------------------|--------------------------------|----------------------------------------|
| `--wings-focus-ring`   | `0 0 0 3px rgba(0,123,255,.3)` | 3px focus halo (boxshadow form)        |

---

## Typography (`CategoryTypography`) — reserved

No tokens yet. Reserved for `--wings-font-body`, `--wings-font-heading`,
`--wings-font-mono`, `--wings-font-size-base`, `--wings-line-height`,
`--wings-letter-spacing`, `--wings-font-weight`.

---

## Spacing (`CategorySpacing`) — partial

A full `--wings-space-{xs,sm,md,lg,xl}` scale is reserved. Today the
core skins already declare the following spacing tokens (currently
shipped under Identity in the eight built-in themes; they will migrate
to Spacing-only skins as that category becomes useful for composition):

| Token                   | Suggested value | Semantics                       |
|-------------------------|-----------------|---------------------------------|
| `--wings-button-padding`| `8px 16px`      | Block-button inner padding      |
| `--wings-button-gap`    | `8px`           | Gap between buttons in a row    |
| `--wings-box-padding`   | `24px`          | Modal/panel inner padding       |

---

## Lighting (`CategoryLighting`)

Gradient accents and emissive halos.

| Token                          | Semantics                                        |
|--------------------------------|--------------------------------------------------|
| `--wings-gradient`             | Optional accent gradient string                  |
| `--wings-gradient-color`       | Text colour to use on gradient backgrounds       |
| `--wings-gradient-shadow`      | Glow/drop-shadow for elements using `gradient`   |

---

## Atmosphere (`CategoryAtmosphere`)

Translucency and material effects (glass-morphism, blur, noise).

| Token                       | Suggested value | Semantics                                  |
|-----------------------------|-----------------|--------------------------------------------|
| `--wings-glass-opacity`     | `0.72`          | Glass surface alpha (used in color-mix)    |
| `--wings-surface-blur`      | `14px`          | `backdrop-filter: blur(...)` value         |
| `--wings-surface-saturate`  | `1.4`           | `backdrop-filter: saturate(...)` value     |
| `--wings-glass-border`      | `rgba(255,255,255,0.18)` | Glass-edge border colour          |
| `--wings-glass-noise`       | `none`          | Optional noise texture URL                 |

---

## Composing skins

Two registered skins can be active simultaneously when their category
bitmasks are disjoint. The five orthogonal axes (Identity, Geometry,
Depth, Motion, Atmosphere) compose freely; pick at most one skin per
axis. Example stacks:

| Identity   | Geometry | Depth     | Motion  | Atmosphere | Result                                    |
|------------|----------|-----------|---------|------------|-------------------------------------------|
| `light`    | `classic`| `lifted`  | `calm`  | —          | ✅ Default look                           |
| `mushroom` | `soft`   | `floating`| `gentle`| `glass`    | ✅ Romantic dreamy mushroom               |
| `dark`     | `sharp`  | `flat`    | `brisk` | —          | ✅ Brutalist dark mode                    |
| `light`    | `soft`   | `floating`| `gentle`| `glass`    | ✅ Apple-ish frosted-glass UI             |
| `light` + `dark`             | —    | —    | —    | ❌ Identity collision                       |
| `sharp` + `soft`             | —    | —    | —    | ❌ Geometry collision                       |

Programmatic activation:

```go
_ = wings.ApplySkin("mushroom")
if err := wings.ApplySkin("glass"); err != nil {
    var conflict *wings.SkinConflictError
    if errors.As(err, &conflict) {
        log.Printf("conflict on %s with %v", conflict.ConflictingCategories, conflict.Conflicts)
    }
}
```

`wings.ActiveCategories()` returns the OR of every active skin; useful
for diagnostic UIs (e.g. the skin-switcher widget displays which
categories are currently covered).

---

## Status

- API: `wings.RegisterSkin(name, categories, css)`,
  `wings.ApplySkin(name) error`, `wings.DeactivateSkin(name) error`,
  `wings.ClearSkins()`, `wings.ListSkinInfos()`,
  `wings.ActiveSkins()`, `wings.ActiveCategories()`,
  `wings.OnSkinChange(fn)`. See `skin.go`.
- Identity skins (`IdentitySkinCategories`): `light`, `dark`, `autumn`,
  `darkblueberry`, `darkforest`, `lightblueberry`, `mushroom`,
  `vividforest`.
- Geometry+Spacing skins (`GeometrySkinCategories`): `sharp`, `classic`,
  `soft`.
- Depth skins (`DepthSkinCategories`): `flat`, `lifted`, `floating`.
- Motion skins (`MotionSkinCategories`): `gentle`, `calm`, `brisk`.
- Atmosphere skins (focused): `glass` (`CategoryAtmosphere`).
- Widget migration:
  - `w-dialog`, `w-navbar`, `w-combobox`, `w-tabs`, `w-tabbutton`,
    `w-tab` — all consume only documented tokens via
    `var(--wings-*, fallback)`.
  - `w-dialog` and `w-combobox` opt into the Atmosphere category by
    using `backdrop-filter: blur(var(--wings-surface-blur, 0))` so
    `glass` produces a visible effect without coupling.
