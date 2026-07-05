# Wire Property & Event Vocabulary — DRAFT v0 for review

Per D10, nothing on the wire is positional: every value travels under a
property name, so the names below ARE protocol surface. This document
is the deliberate registry of those names, grounded in the current
widget APIs. **Status: draft for project-owner review** — nothing here
is frozen; naming questions are collected at the end.

## Conventions (proposed)

- Names are `lower_snake_case`, singular where sensible.
- All coordinates/sizes are units in the **container's denomination**
  (D8/D8′); rows/columns appear only where the concept is genuinely
  grid-based (e.g. terminal surfaces).
- Enum values are lowercase strings: `align=center`, `state=maximized`.
- Booleans: `true`/`false`.
- `id` values are ObjectIDs (server-assigned; see Correlation Keys).
- `action` values are command IDs (semantic strings like `file.open`,
  or auto-assigned `cmd.auto.N`).
- Key nomenclature is D3 throughout: `shortcut="^N"`, `key="M-Tab"`.
- List/structure encoding (menu trees, combo items) is deliberately
  TBD until the syntax phase; the vocabulary below names the
  properties regardless of encoding.

## Identity, creation, correlation

| Concept | Form | Notes |
|---|---|---|
| Object identity | `id` | Server-assigned ObjectID; never client-invented |
| Object type | `new <type> …` | `button`, `label`, `checkbox`, … |
| Parent | `parent=<id or key>` | Containment at creation or reparent |
| Correlation key (D11) | `key1=new button …` → reply `key1=<id>` | Request-scoped temporary names; only requests that need IDs back carry keys; many creates batch into one request |
| Forward reference (D11 proposal) | `key1=new window …` / `new button parent=key1 …` | Later lines in the same batch may reference earlier keys — whole trees in one burst; **pending owner review** |

## Common properties (all widgets)

| Property | Type | Notes |
|---|---|---|
| `id` | id | Read-only identity |
| `name` | string | Human label for debugging/tooling; NOT identity |
| `enabled` | bool | |
| `visible` | bool | |
| `x`, `y`, `width`, `height` | units | Bounds in parent denomination; usually layout-managed |
| `min_width`, `min_height` | units | |
| `max_width`, `max_height` | units | |
| `size_policy_h`, `size_policy_v` | enum | `fixed`, `minimum`, `maximum`, `preferred`, `expanding`, `ignored` |
| `stretch` | int | Layout-item stretch factor |
| `align` | enum | Layout-item alignment: `fill`, `left`, `center`, `right`, `top`, `middle`, `bottom` |
| `font` | string | Family name; `font_size` (int), `font_style` (flags: `bold`, `italic`, `underline`, …) |
| `grid_width`, `grid_height` | units | CellMetrics override: units per column/row (D8); unset = inherit |
| `scheme` | string/int | Color scheme selector |
| `background` | color | Explicit background; unset = inherit |
| `acc_name`, `acc_role`, `acc_description` | string | Accessibility |

## Per-widget properties

### button
| Property | Type | Notes |
|---|---|---|
| `caption` | string | Display text (`&` accelerator markup) |
| `icon` | string | Icon identifier |
| `action` | command-id | **Optional** — links the button to a command; when set, click dispatches the command. `click` events fire regardless |
| `default` | bool | Default-button styling/Enter behavior |

### label
| Property | Type | Notes |
|---|---|---|
| `caption` | string | Displayed text (may contain `\n`) |
| `wrap` | bool | Word wrap (enables height-for-width) |
| `align` | enum | Text alignment within bounds |

### checkbox
| Property | Type | Notes |
|---|---|---|
| `caption` | string | |
| `checked` | enum | `off`, `on`, `mixed` (tri-state) |
| `tristate` | bool | |
| `wrap` | bool | Opt-in word wrap (D9: indicator is chrome) |
| `action` | command-id | Optional, as with button |

### radiobutton
| Property | Type | Notes |
|---|---|---|
| `caption` | string | |
| `checked` | bool | |
| `group` | id | Radio group membership |
| `wrap` | bool | |

### textinput
| Property | Type | Notes |
|---|---|---|
| `text` | string | The editable content (server-authoritative) |
| `placeholder` | string | |
| `cursor` | int | Caret position (rune index) |
| `selection_start`, `selection_end` | int | |
| `readonly` | bool | |
| `mask` | bool/rune | Password-style echo |

### combobox
| Property | Type | Notes |
|---|---|---|
| `items` | list | Encoding TBD |
| `current` | int | Selected index (−1 = none) |
| `editable` | bool | |
| `placeholder` | string | |
| `max_visible` | int | Dropdown row cap |

### listview / treeview
| Property | Type | Notes |
|---|---|---|
| `items` / `nodes` | list/tree | Encoding TBD; tree nodes carry `caption`, `expanded`, children |
| `current` | int/path | Selection |
| `multi_select` | bool | (future) |

### progress
| Property | Type | Notes |
|---|---|---|
| `value`, `minimum`, `maximum` | int | |
| `caption` | string | Optional overlay text |

### tabwidget
| Property | Type | Notes |
|---|---|---|
| `tabs` | list | Each tab: `caption`, content `id` |
| `current` | int | Active tab |
| `tab_position` | enum | `top`, `bottom`, `left`, `right` |

### splitter
| Property | Type | Notes |
|---|---|---|
| `orientation` | enum | `horizontal`, `vertical` |
| `position` | float | 0.0–1.0 ratio (denomination-free by design) |
| `caption` | string | Optional divider title |

### scrollarea
| Property | Type | Notes |
|---|---|---|
| `scroll_x`, `scroll_y` | int | Scroll offsets (cells of own denomination) |
| `h_bar`, `v_bar` | enum | `auto`, `always`, `never` |
| `content` | id | Scrolled child |

### panel
| Property | Type | Notes |
|---|---|---|
| `border` | bool | |
| `border_style` | enum | `single`, `double`, `rounded`, `heavy`, `ascii` |
| `layout` | enum | `vbox`, `hbox`, `grid`, `none` |
| `spacing` | units | Layout spacing |

### separator / spacer
| Property | Type | Notes |
|---|---|---|
| `caption` | string | Separator title (optional) |
| `orientation` | enum | |
| (spacer) `width`, `height` | units | Explicit size; unset = 1×1 cell |

### terminal (PurfecTerm surface)
| Property | Type | Notes |
|---|---|---|
| `columns`, `rows` | int | Grid size (genuinely cell-based) |
| `feed` | bulk | Byte/cell stream — needs the bulk escape (O6) |

### canvas *(deferred, D7)*
Reserved: `mode` (`commands`/`pixels`), plus its command stream — designed
when the widget is built.

## Window properties

| Property | Type | Notes |
|---|---|---|
| `title` | string | |
| `x`, `y`, `width`, `height` | units | Desktop denomination |
| `state` | enum | `normal`, `minimized`, `maximized` |
| `flags` | flags | `frameless`, `no_title`, `no_resize`, `modal`, `passive`, … |
| `content` | id | Content widget |
| `min_width`, `min_height` | units | |
| `font`, `grid_width`, `grid_height` | | Per-window overrides (D8) |
| `native` | bool | G4 dual-mode: request an OS window when available |

## Menu structures

Menus are data trees (G6): `menu` has `caption` and items; each item:

| Property | Type | Notes |
|---|---|---|
| `caption` | string | `&` accelerator markup |
| `action` | command-id | THE dispatch identity (slice 1) |
| `shortcut` | string | D3 nomenclature (`"^N"`) |
| `enabled`, `checkable`, `checked` | bool | |
| `separator` | bool | |
| `submenu` | menu | |
| `icon` | string | |

## Events (display service → app)

Envelope: `event <type>` plus named fields; `widget=<id>` names the
source where applicable. Apps subscribe per widget/event (slice 3).

| Event | Fields | Notes |
|---|---|---|
| `command` | `action` | Menu/button/shortcut dispatch — the slice-1 seam |
| `click` | `widget`, `x`, `y`, `button` | Positions in the widget's denomination |
| `toggle` | `widget`, `checked` | Checkbox/radio state after the change |
| `change` | `widget`, `text` \| `value` \| `current` | Content/value/selection changed (textinput, combobox, progress-consumer, list) |
| `activate` | `widget`, `current` | Item chosen (combobox selection committed, list double-activation) |
| `focus_in` / `focus_out` | `widget` | |
| `key` | `widget`, `key` | D3 string; only when subscribed (raw-key mode) |
| `window_moved` / `window_resized` | `window`, `x`, `y`, `width`, `height` | |
| `window_state` | `window`, `state` | minimized/maximized/normal |
| `window_closed` | `window` | After close completes |
| `window_active` | `window`, `active` | Activation changes |
| `session` | `phase` | `attached`, `detached`, … (D4) |

## Open questions for review

1. **`caption` vs `text`** — proposed split: `caption` = display label
   ON a control (button, checkbox, label, tab, menu item);
   `text` = editable/content text (textinput). Window uses `title`.
   Agree, or unify on one name?
2. `current` vs `selected` for selection indices (combobox, tabs,
   lists) — `current` proposed for consistency with existing API.
3. Should layout-item properties (`stretch`, `align`) live on the
   child (as here) or as arguments of an attach/add operation?
4. `grid_width`/`grid_height` naming for the CellMetrics override —
   or `units_per_column`/`units_per_row` (more literal, longer)?
5. Forward references from correlation keys within a batch (D11
   extension) — approve?
6. Event names: is `change` too generic — split into `text_changed`,
   `value_changed`, `selection_changed`?
