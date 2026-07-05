# Wire Property & Event Vocabulary — DRAFT v0 for review

Per D10, nothing on the wire is positional: every value travels under a
property name, so the names below ARE protocol surface. This document
is the deliberate registry of those names, grounded in the current
widget APIs. **Status: draft for project-owner review** — nothing here
is frozen; naming questions are collected at the end.

## Conventions (proposed)

- **Type system (D17)** — every value is exactly one of:
  - `flag` — bare name / `!name` / `?name` (D12/D16)
  - `enum` — unquoted word from the property's declared vocabulary
  - `numeric` — int or float lexically; each property declares its
    domain (units are integer-valued; ratios are floats)
  - `identifier` — unquoted reference token: object IDs, key paths,
    command IDs, template names
  - `{}` — a collection of objects/definitions (D13)
  - `"string"` — **quotes required**; a bare token is never a string

  Consequence: quoting alone separates references from text —
  `parent=k1.sk1` is an identifier (resolved), `caption="k1.sk1"` is
  text. No reference sigil needed (closes the item parked under D15).
  Flags/enums/identifiers are lexically similar bare tokens typed per
  property, so the tokenizer is schema-free; only interpretation
  consults the vocabulary.
- **Case namespaces (D18):** system names (properties, widget types,
  verbs, enum values) begin lowercase; **user-defined templates and
  aliases MUST begin uppercase**. The two namespaces are disjoint by
  construction, so the system vocabulary can grow forever without
  colliding with any client's definitions (the HTML custom-elements
  move, done with case). Correlation keys are unconstrained: they
  occupy statement-key/value positions, never property-name position.
  Enforcement is the interpreter's job; the parser stays schema-free.
- Names are `lower_snake_case`, singular where sensible.
- All coordinates/sizes are units in the **container's denomination**
  (D8/D8′); rows/columns appear only where the concept is genuinely
  grid-based (e.g. terminal surfaces).
- Enum values are lowercase strings: `align=center`, `state=maximized`.
- **Flags (D12, extended by D16):** booleans travel as bare names —
  `name` = true, `!name` = false, `?name` = **asserted indeterminate**
  (a real value, deliberately set), and absence = *unsaid*: default at
  creation, **unchanged on update**. `?name` is a value; absence never
  is. The grammar admits `?` on any flag; each property declares
  whether indeterminate is *meaningful* (e.g. checkbox `checked`
  accepts it; `visible` rejects it under the standard invalid-property
  policy). Long forms `name=true`/`name=false`/`name=mixed` are
  accepted on input for generic tooling; flag forms are canonical.
  `!` and `?` never begin property names. Flags compose with aliases
  (`alias c="checked"` → `c` / `!c` / `?c`). Genuinely multi-valued
  switches (more than three states) remain enums.
- `id` values are ObjectIDs (server-assigned; see Correlation Keys).
- `action` values are command IDs — unquoted identifiers per D17
  (`action=file.open`, auto-assigned `cmd.auto.N`).
- **Alias targets are strings, not identifiers** (owner ruling): an
  alias is a lexical macro applied before interpretation, with no
  object to validate against — `alias c="caption"`. Template targets
  ARE identifiers (`template MyBtn=button`) — they name a type that
  must exist. Lexical mechanism ⇒ string; semantic mechanism ⇒
  identifier.
- Key nomenclature is D3 throughout, carried as strings:
  `shortcut="^N"`, `key="M-Tab"`.
- **Colors (answered 2026-07-05):** named colors are bare enum words
  (`fg=red`, `bg=bright_blue`; the 16 ANSI names with `bright_`
  prefixes, plus `default`); RGB is a quoted string `bg="#3366ff"`
  (bare `#` starts a comment, and quoting-is-for-text loses nothing
  here since an RGB literal is data, not a reference). Implemented as
  common `fg=`/`bg=` properties over the widget style override.
- **Verbs (D19, answered 2026-07-05):** `new`, `set`, `destroy`,
  `sub`, `unsub` (+ declarations `alias`, `template`). Targets are
  key paths or bare numeric ObjectIDs — `set root.status
  caption="…"`, `destroy 1042`, `sub wcb toggle change`, `unsub all`.
  Keys are session-persistent (D11 refined); surfacing also registers
  the surfaced name as a key. `set` accepts everything `new` accepts
  including `children={}` (appends).
- **Event flow (D20, answered 2026-07-05):** default-closed except
  `command`. `sub <target>|all [events…]` opens flows (no events
  listed = all from that target); `unsub` closes them; `command`
  events and registry dispatch are unconditional. Wire-initiated
  mutations (`new`, `set`) never echo — no state events, no action
  firing.
- **Structure encoding (D13):** subtrees build with inline children
  blocks — `new panel children={new button; new button}` — and the
  same construct encodes every list-like structure: combo `items`,
  `tabs`, tree `nodes`, and menu trees are children blocks of typed
  items. Block order = layout/z order. `parent=` remains for later
  additions/reparenting. Correlation keys stay flat per request at any
  nesting depth.
- **Templates (D14):** `template MyBtn=button align=right caption="…"`
  then `new MyBtn` — connection-scoped macros expanded at parse time
  (never live restyling). Template properties first, instance
  overrides win; D12 flags can explicitly un-set (`!visible`).
  Templates may contain children (component definitions). Builtins are
  lowercase; templates CamelCase by convention.

## Identity, creation, correlation

| Concept | Form | Notes |
|---|---|---|
| Object identity | `id` | Server-assigned ObjectID; never client-invented |
| Object type | `new <type> …` | `button`, `label`, `checkbox`, … |
| Parent | `parent=<id or key>` | Containment at creation or reparent |
| Correlation key (D11) | `key1=new button …` → reply `key1=<id>` | Request-scoped temporary names; only requests that need IDs back carry keys; many creates batch into one request |
| Forward reference (D11 proposal) | `key1=new window …` / `new button parent=key1 …` | Later lines in the same batch may reference earlier keys — whole trees in one burst; **pending owner review** |
| Scoped keys (D15) | `k1=new thing children={sk1=new subthing}` → path `k1.sk1` | Keys inside a block are block-local, addressable externally as paths through the enclosing key; unkeyed parents keep child keys internal |
| Surfacing (D15) | `mine=k1.sk1` → reply `mine=<id>` | Reply reports surfaced names + top-level keys only — reply size is app-controlled. Template-instantiated children are addressed this way (`k1.input`) |

## Common properties (all widgets)

| Property | Type | Notes |
|---|---|---|
| `id` | identifier | Read-only identity |
| `name` | string | Human label for debugging/tooling; NOT identity |
| `enabled` | flag | |
| `visible` | flag | |
| `x`, `y`, `width`, `height` | numeric (units) | Bounds in parent denomination; usually layout-managed |
| `min_width`, `min_height` | numeric (units) | |
| `max_width`, `max_height` | numeric (units) | |
| `size_policy_h`, `size_policy_v` | enum | `fixed`, `minimum`, `maximum`, `preferred`, `expanding`, `ignored` |
| `stretch` | numeric | Layout-item stretch factor |
| `align` | enum | Layout-item alignment: `fill`, `left`, `center`, `right`, `top`, `middle`, `bottom` |
| `font` | string | Family name; `font_size` (int), `font_style` (flags: `bold`, `italic`, `underline`, …) |
| `column_units`, `row_units` | numeric (units) | CellMetrics override (D8): how many units one column/row represents; unset = inherit |
| `scheme` | identifier | Color scheme selector |
| `background` | color | Explicit background; unset = inherit |
| `acc_name`, `acc_role`, `acc_description` | string | Accessibility |

## Per-widget properties

### button
| Property | Type | Notes |
|---|---|---|
| `caption` | string | Display text (`&` accelerator markup) |
| `icon` | string | Icon identifier |
| `action` | identifier | **Optional** — links the button to a command; when set, click dispatches the command. `click` events fire regardless |
| `default` | flag | Default-button styling/Enter behavior |

### label
| Property | Type | Notes |
|---|---|---|
| `caption` | string | Displayed text (may contain `\n`) |
| `wrap` | flag | Word wrap (enables height-for-width) |
| `text_align` | enum | `left`, `center`, `right` — TEXT alignment within the label. Renamed from `align` (2026-07-05): it shadowed the common layout-item `align` hint; distinct concepts get distinct names. |

### checkbox
| Property | Type | Notes |
|---|---|---|
| `caption` | string | |
| `checked` | flag | Tri-capable: `checked` / `!checked` / `?checked` (D16) |
| `tristate` | flag | UI cycling behavior only (does clicking pass through mixed); wire representability comes from D16 regardless |
| `wrap` | flag | Opt-in word wrap (D9: indicator is chrome) |
| `action` | identifier | Optional, as with button |

### radiobutton
| Property | Type | Notes |
|---|---|---|
| `caption` | string | |
| `checked` | flag | |
| `group` | identifier | Radio group membership |
| `wrap` | flag | |

### textinput
| Property | Type | Notes |
|---|---|---|
| `text` | string | The editable content (server-authoritative) |
| `placeholder` | string | |
| `cursor` | numeric | Caret position (rune index) |
| `selection_start`, `selection_end` | numeric | |
| `readonly` | flag | |
| `mask` | flag or string | Password-style echo: bare = default mask char; `mask="*"` = explicit |

### combobox
| Property | Type | Notes |
|---|---|---|
| children of `item` | {} | Children block of shared `item`s (D13); items cannot nest here |
| `selected` | numeric | Selected index (−1 = none) |
| `editable` | flag | |
| `placeholder` | string | |
| `max_visible` | numeric | Dropdown row cap |

### listview / treeview *(registered 2026-07-05)*
| Property | Type | Notes |
|---|---|---|
| children of `item` | {} | Children block (D13); the shared virtual `item` carries `caption` + `expanded` and nests for trees — one type serves combobox rows, list rows, and tree nodes |
| `selected` | numeric | Selection (visible-row index for trees) |
| `alternate_rows` | flag | listview |
| `indent_width` | numeric | treeview |
| `multi_select` | flag | (future) |

**Item identity (2026-07-05):** items are first-class wire objects —
each carries an ObjectID from the same allocation space as widgets,
and the ordinary correlation-key machinery names them (nothing
item-specific was invented): `fruit=new item …` inside a keyed tree
registers `tree.fruit`; `set tree.fruit caption="…" !expanded`,
`set tree.fruit children={new item …}` (grows the live subtree), and
`destroy tree.fruit` all work after construction, and a live tree
updates in place. Tree events report the identity as `item=<id>`:
`change`/`activate` carry `item=` + `selected=` (visible-row index),
and `expand item=<id> expanded`/`!expanded` reports user
expand/collapse. Go-side, `TreeItem.ID` is auto-assigned at
construction, so imperatively built trees carry identity too.
Listview/combobox rows have IDs as well but are not yet routed for
set/destroy (flat rows; rebuild is cheap — join a later slice if
needed).

### progress
| Property | Type | Notes |
|---|---|---|
| `value`, `minimum`, `maximum` | numeric | |
| `caption` | string | Optional overlay text |

### tabs *(registered 2026-07-05; type name `tabs`)*
| Property | Type | Notes |
|---|---|---|
| children of `tab` | {} | Virtual `tab`: `caption` + exactly one content child widget |
| `selected` | numeric | Active tab (after the tabs exist) |
| `position` | enum | `top`, `bottom`, `left`, `right` |
| `movable`, `closable` | flag | |

Events: `change selected=`.

### splitter
| Property | Type | Notes |
|---|---|---|
| `orientation` | enum | `horizontal`, `vertical` |
| `position` | numeric | 0.0–1.0 ratio (denomination-free by design) |
| `caption` | string | Optional divider title |

### scrollarea *(registered 2026-07-05)*
| Property | Type | Notes |
|---|---|---|
| children | {} | Exactly one content widget (wrap several in a panel) |
| `scroll_x`, `scroll_y` | numeric | Scroll offsets |
| `resizable` | flag | Content tracks viewport width |
| `h_bar`, `v_bar` | enum | `auto`, `always`, `never` (future) |

### panel
| Property | Type | Notes |
|---|---|---|
| `border` | flag | |
| `border_style` | enum | `single`, `double`, `rounded`, `heavy`, `ascii` |
| `layout` | enum | `vbox`, `hbox`, `grid`, `none` |
| `spacing` | numeric (units) | Layout spacing |

### separator / spacer
| Property | Type | Notes |
|---|---|---|
| `caption` | string | Separator title (optional) |
| `orientation` | enum | |
| (spacer) `width`, `height` | numeric (units) | Explicit size; unset = 1×1 cell |

### terminal (PurfecTerm surface)
| Property | Type | Notes |
|---|---|---|
| `columns`, `rows` | numeric | Grid size (genuinely cell-based) |
| `feed` | bulk | Byte/cell stream — needs the bulk escape (O6) |

### canvas *(deferred, D7)*
Reserved: `mode` (`commands`/`pixels`), plus its command stream — designed
when the widget is built.

## Window properties

*(registered 2026-07-05 — `window` type in the window package)*

| Property | Type | Notes |
|---|---|---|
| `title` | string | |
| `x`, `y`, `width`, `height` | numeric (units) | Desktop denomination |
| `state` | enum | `normal`, `minimized`, `maximized` (future) |
| `frameless`, `no_title`, `no_resize`, `no_move`, `no_close`, `no_minimize`, `no_maximize`, `modal`, `stays_on_top`, `tool` | flag | Individual flags per D12 — no bitsets on the wire (`new window frameless modal`) |
| children | {} | Exactly one content widget (wrap several in a panel) |
| `min_width`, `min_height` | numeric (units) | Via common properties |
| `font`, `column_units`, `row_units` | | Per-window overrides (D8) |
| `native` | flag | G4 dual-mode: request an OS window when available (future) |

Events: `window_closed window=<id>` (after close completes); moved/
resized/state events land when the window grows those callbacks.
`destroy` on a window closes it.

## Menu structures

Menus are data trees (G6): `menu` has `caption` and items; each item:

| Property | Type | Notes |
|---|---|---|
| `caption` | string | `&` accelerator markup |
| `action` | identifier | THE dispatch identity (slice 1) |
| `shortcut` | string | D3 nomenclature (`"^N"`) |
| `enabled`, `checkable`, `checked` | flag | |
| `separator` | flag | |
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
| `change` | `widget`, `text` \| `value` \| `selected` (+ `item` on trees) | Content/value/selection changed (textinput, combobox, progress-consumer, list, tree) |
| `activate` | `widget`, `selected` (+ `item` on trees) | Item chosen (combobox selection committed, list/tree double-activation) |
| `expand` | `widget`, `item`, `expanded` flag | Tree node expanded (`expanded`) or collapsed (`!expanded`) by the user |
| `focus_in` / `focus_out` | `widget` | |
| `key` | `widget`, `key` | D3 string; only when subscribed (raw-key mode) |
| `window_moved` / `window_resized` | `window`, `x`, `y`, `width`, `height` | |
| `window_state` | `window`, `state` | minimized/maximized/normal |
| `window_closed` | `window` | After close completes |
| `window_active` | `window`, `active` | Activation changes |
| `session` | `phase` | `attached`, `detached`, … (D4) |

## Open questions for review

1. ✅ **Answered:** keep the split — `caption` (label on a control),
   `text` (editable content), `title` (windows). Do not unify.
2. ✅ **Answered:** `selected` is the standard for selection indices
   (combobox, tabs, lists, trees) — standardized throughout this doc.
   (The Go API's `CurrentIndex` naming may be aligned later; the wire
   name is settled.)
3. ✅ **Answered (2026-07-05):** layout-item properties (`stretch`,
   `align`) live **on the child** — `new spacer stretch=1`,
   `new button caption="OK" align=right`. The hints travel with the
   widget (`WidgetBase.SetLayoutStretch`/`SetLayoutAlignment`) and
   the parent's layout manager consults them at attach time; also
   fixed generally: a widget whose cross-axis policy is Expanding
   fills its allocation regardless of default alignment.
4. ✅ **Answered:** `column_units` / `row_units` — the names state the
   denomination relationship directly (`row_units=32` reads "a row is
   represented by 32 units", matching D8′'s language).
5. Forward references from correlation keys within a batch (D11
   extension) — approve? *(left open by owner for now)*
6. ✅ **Answered:** keep `change` as the event name; no split.
7. ~~Addressing template-instantiated children~~ — **resolved by D15**
   (hierarchical key scoping): template-body keys are namespaced by
   the instance key (`k1.input`, `k2.input`); surface explicitly to
   get IDs in the reply. Remaining syntax-phase item (O6):
   distinguishing key paths from dotted string values in value
   position (type-directed resolution vs a reference sigil like
   `parent=@k1.sk1`).
