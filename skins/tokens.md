# wprana skin tokens

This file is the canonical contract between **skins** (which define CSS
custom properties at `:root`) and **widgets** (which reference them with a
fallback). Every token name listed here is reserved for that purpose.

> **Naming.** All tokens use the `--wings-*` prefix. The project will be
> renamed to **WINGS — Web IN Go Sphere**; the prefix is adopted ahead of
> the rename to avoid a second migration. Third-party widgets are expected
> to use their own prefix.

## Authoring rules

- **Skins** define tokens once at `:root { ... }`. A skin's CSS is injected
  by `wprana.ApplySkin(name)` into a single `<style id="wprana-skin">`
  element appended to `document.head`.
- **Widgets** reference tokens with a fallback so they remain functional
  without an active skin:

  ```css
  border: 1px solid var(--wings-border, #ccc);
  ```

  Widgets MUST NOT redefine tokens with `:host { --wings-X: ...; }` — that
  shadows the global value inside the widget's shadow tree and breaks
  skin override. Per-widget defaults belong in the `var(name, fallback)`
  fallback position, not in `:host` blocks.
- **Adding a token**: add the entry below first (with a one-line semantic
  description), then update every registered skin in `./skins/*` so they
  remain coherent. Widgets may then reference the new token.

## Core tokens (cross-widget)

These are the shared design language: colors, surfaces, focus ring, common
button affordances. Every skin SHOULD define all of them.

| Token                          | Semantics                                        |
|--------------------------------|--------------------------------------------------|
| `--wings-bg`                   | Page background (the outermost surface)          |
| `--wings-surface`              | Elevated surface above the page bg (panels, cards, dialogs) |
| `--wings-text`                 | Primary foreground text colour                   |
| `--wings-text-muted`           | Intermediate dimmed text (less emphasis than primary, more than light) |
| `--wings-text-light`           | Secondary/very dimmed text (helper labels, captions) |
| `--wings-primary`              | Brand accent — links, focused input text, etc.   |
| `--wings-border`               | Default 1px border colour for inputs/cards       |
| `--wings-border-focus`         | Border colour when an input is focused           |
| `--wings-btn-bg`               | Neutral button background                        |
| `--wings-btn-hover-bg`         | Neutral button background on hover               |
| `--wings-btn-hover-color`      | Neutral button text colour on hover              |
| `--wings-btn-hover-shadow`     | Box-shadow applied to neutral buttons on hover   |
| `--wings-secondary`            | Secondary accent — draws attention, complements primary (e.g. status indicators, unrevised items) |
| `--wings-quiet`                | Quiet counterpart to secondary — settled, no action needed (e.g. resolved/done indicators) |
| `--wings-primary-pale`         | Very pale/dark tint of the primary colour (for tinted backgrounds, panel accents) |
| `--wings-gradient`             | Optional accent gradient string (e.g. `linear-gradient(...)`) for branded buttons/tabs |
| `--wings-gradient-color`       | Text colour to use on `--wings-gradient` backgrounds |
| `--wings-gradient-shadow`      | Glow / drop-shadow for elements using `--wings-gradient` |

## Widget-namespaced tokens

Widgets that need finer-grained tokens than the core set MAY register their
own under the `--wings-<widget>-*` namespace. Skins MAY define them; if
absent the widget falls back to its embedded default.

### `w-dialog`

| Token                                  | Semantics                                |
|----------------------------------------|------------------------------------------|
| `--wings-dialog-bg`                    | Dialog panel background                  |
| `--wings-dialog-shadow`                | Dialog panel drop shadow                 |
| `--wings-dialog-border-radius`         | Dialog panel corner radius               |
| `--wings-dialog-padding`               | Inner padding of the dialog panel        |
| `--wings-dialog-overlay-bg`            | Backdrop overlay colour (rgba)           |
| `--wings-dialog-button-bg`             | Default button surface                   |
| `--wings-dialog-button-hover-bg`       | Default button surface on hover          |
| `--wings-dialog-button-active-bg`      | Default button surface on active/click   |
| `--wings-dialog-button-border`         | Default button border                    |
| `--wings-dialog-button-border-hover`   | Default button border on hover           |
| `--wings-dialog-button-padding`        | Button inner padding                     |
| `--wings-dialog-button-gap`            | Gap between buttons in the action row    |
| `--wings-dialog-primary-bg`            | Primary action button surface            |
| `--wings-dialog-primary-color`         | Primary action button text colour        |
| `--wings-dialog-primary-hover-bg`      | Primary action button surface on hover   |

### `w-navbar`

`w-navbar` consumes only core tokens at present. If finer control becomes
necessary it will register `--wings-navbar-*`.

### `w-combobox`

| Token                                       | Semantics                                  |
|---------------------------------------------|--------------------------------------------|
| `--wings-combobox-tag-bg`                   | Selected-tag background                    |
| `--wings-combobox-tag-color`                | Selected-tag text colour                   |
| `--wings-combobox-tag-border`               | Selected-tag border colour                 |
| `--wings-combobox-rm-color`                 | Tag remove-button (×) colour               |
| `--wings-combobox-rm-hover-bg`              | Tag remove-button background on hover      |
| `--wings-combobox-rm-hover-color`           | Tag remove-button colour on hover          |
| `--wings-combobox-input-border`             | Text-input border                          |
| `--wings-combobox-input-focus-border`       | Text-input border on focus                 |
| `--wings-combobox-input-focus-shadow`       | Focus-ring rgba (3px outer shadow)         |
| `--wings-combobox-input-bg`                 | Text-input background                      |
| `--wings-combobox-drop-bg`                  | Dropdown panel background                  |
| `--wings-combobox-drop-border`              | Dropdown panel border                      |
| `--wings-combobox-drop-shadow`              | Dropdown panel drop-shadow                 |
| `--wings-combobox-scroll-thumb`             | Dropdown scrollbar thumb colour            |
| `--wings-combobox-opt-color`                | Option text colour                         |
| `--wings-combobox-opt-hover-bg`             | Option background on hover                 |
| `--wings-combobox-opt-hover-color`          | Option text colour on hover                |
| `--wings-combobox-opt-active-bg`            | Option background on active/click          |
| `--wings-combobox-empty-color`              | "No results" empty-state text colour       |

## Status

- API: `wprana.RegisterSkin(name, css)` and `wprana.ApplySkin(name)` are
  in place (see `skin.go`).
- Skins shipped: `light`, `dark`, `darkblueberry`, `darkforest`,
  `lightblueberry`, `mushroom`, `vividforest`.
  All define the full core token set including `--wings-secondary`,
  `--wings-quiet`, `--wings-primary-pale`, and `--wings-gradient*`.

  Activate with `_ "github.com/luisfurquim/wprana/skins/<name>"` plus
  `wprana.ApplySkin("<name>")` before `wprana.Main()`.
- Widget migration:
  - **`w-dialog`** ✅ migrated: `design.css` references
    `var(--wings-dialog-*, fallback)`; `vars.css` is empty.
  - **`w-navbar`** ✅ migrated: `design.css` references `var(--wings-*, fallback)`
    for the core token set; `vars.css` is empty.
  - **`w-combobox`** ✅ migrated: `design.css` references
    `var(--wings-combobox-*, fallback)`; `vars.css` is empty.
