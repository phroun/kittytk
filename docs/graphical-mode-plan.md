# Graphical Mode Plan — Decision Record

Status: **planning — no implementation yet.**
This document records the decisions made for adding true graphical (GUI)
rendering to tuitk, alongside the open questions still to be settled. It
builds on the analysis in `adding-true-gui-rendering.md` and the
architecture in `multi-app-desktop-plan.md`.

## Goals

1. Any window can optionally be created as a real graphical (OS-native)
   window when running under an OS that supports it.
2. The same application code runs as either a graphical or a text (TUI)
   version of the app — one codebase, two presentations.
3. Mixed use is supported: terminal-style containers and cell-grid
   rendering remain available *inside* a graphical window (PurfecTerm
   depends on this; it is also a product feature in its own right).
4. In graphical mode, the app integrates with the user's real desktop
   where possible — native OS windows, and native system/desktop menus
   when the platform allows — rather than living inside a single-window
   "desktop environment" as the current TUI demo does.

## Decisions

### D1 — Widgets are mode-aware and own both renderings  *(decided 2026-07-05)*

Every widget knows whether it is painting to a graphical or a text-mode
target and behaves accordingly. The **programmer-facing interface of a
widget is identical in both modes**; only its rendering activity differs.

- All current cell-idiom visuals — TabWidget's trapezoid tabs built from
  `/ \ _ < >` runes, Button's `▄ ▀` half-block shadows, `░`/`█` rune
  scrollbars, per-cell selection restyling, the overline→underline
  attribute hack in the TUI backend's `EndFrame` — are **TUI-specific
  rendering material**. In graphical mode they dissolve into a different
  implementation that the widget itself fosters (vector borders, real
  shadows, pixel scrollbars, highlight rectangles, real overlines).
- This supersedes both options sketched in `adding-true-gui-rendering.md`
  Decision 1: rendering is neither pushed down into the backend as
  per-widget draw calls, nor split into backend-specific widget classes.
  The widget hosts both paint paths and selects by target mode.

Implications:

- The `Painter` (or backend) must expose the rendering mode so a widget's
  `Paint` can branch (e.g. a `Mode()`/`IsGraphical()` query).
- The `Painter`/backend needs graphical primitives alongside the cell
  primitives: color fills, lines/strokes, rectangles (incl. rounded),
  real text runs with real fonts. Cell primitives (`DrawCell`, rune
  fills, box-drawing borders) remain for TUI mode and for cell-grid
  containers inside graphical windows.
- Layout and measurement logic must be shared between the two paths, so
  text measurement has to come from the backend (see groundwork G1) —
  a widget cannot assume 1 char = 1 cell = 8×16 units in graphical mode.

## Groundwork required regardless of open decisions

These follow from the code survey (2026-07-05) and are needed no matter
how the open decisions land. All are refactors verifiable against the
existing TUI demo with pixel-identical output.

### G1 — Backend-driven text measurement and metrics hygiene

- `Font.MeasureText` is currently a static, cell-quantized function
  (multiples of 8/16 units) and `Font.LineHeight` is hardcoded to 16.
  Measurement must route through the backend (a `TextMeasurer` provided
  by the rendering target) so a real font engine can answer in graphical
  mode.
- Widgets that call `core.DefaultCellMetrics()` directly (button,
  textinput, scrollarea, purfecterm, others) must ask the painter/backend
  instead.
- Remaining char-count measurements must be eliminated: `DrawBox` title
  truncation by rune count, Label word-wrap's hardcoded `charWidth := 1`,
  scrollbar range math done in cells via `CharsForWidth`.

### G2 — Split the backend into Platform + Surface

`core.RenderBackend` models exactly one drawing target and one event
queue. Native mode needs:

- **Platform** — owns the OS event loop, window creation, clipboard,
  screens/DPI, native menus, native dialogs.
- **Surface** — a per-window render target with per-window size, frames,
  damage/invalidations, and input events.

The TUI backend becomes a Platform with exactly one Surface (the
terminal). Multi-monitor awareness lives at the Platform level.

### G3 — Event-loop inversion

`Desktop.Run()`'s poll → dispatch → full-frame-render loop cannot drive
native toolkits, which own their main loop, require main-thread affinity
(`runtime.LockOSThread`), and deliver events per-window via callbacks.
Control inverts to `platform.Run(app)` calling back into tuitk dispatch.
Rendering becomes per-window and damage-driven rather than every-iteration
full-frame. The TUI backend adapts easily (its input is already a
goroutine feeding a channel).

### G4 — Dual-mode Window

`Window` gains an explicit mode:

- **Native top-level** — the OS owns chrome, coordinates, move/resize,
  minimize, activation; `Window.Paint` renders content only; `SetBounds`
  maps to native window geometry.
- **In-surface child** — current behavior: self-drawn chrome, drag,
  keyboard move/resize, desktop/MDI coordinates.

The in-surface path is permanent, not legacy: MDIPane embeds child
windows with the current chrome even inside a fully graphical app, and
the TUI desktop always uses it. `WindowManager` scopes down to managing
in-surface windows (TUI desktop, MDI panes); in native mode the OS window
server replaces its z-order/drag/cascade/maximize/minimize/M-Tab roles,
with native activation callbacks driving `activeWindow` and the
active-app/menu-bar switch. The dock likewise yields to the OS taskbar
for native windows but remains for TUI/MDI.

### G5 — Popups become real windows in native mode

Menu dropdowns and combobox popups are currently overlays painted onto
the single desktop surface (`PopupController`/`PopupOverlay`/
`MapToScreen`). With native windows they must become borderless popup OS
windows (or native menu APIs), positioned in screen coordinates. Modal
dialogs port most cleanly — they are already `Window`s and map to native
dialogs/sheets.

### G6 — Menu data model ports; menu presentation is per-platform

The `Menu`/`MenuItem` tree (title, items, shortcut, checked/enabled,
submenu, callback) maps nearly 1:1 to `NSMenu`/`HMENU`, and the Desktop's
single global bar with per-active-app content swap is already the macOS
model. Known porting snags:

- Shortcuts use terminal key-strings (`"^N"`, `"M-x"`) and need
  translation to native key equivalents.
- Items dispatch by closure with no stable command ID (Win32 `WM_COMMAND`
  wants IDs).
- Windows/Linux use per-window menu bars, so the active app's bar must be
  replicated onto each native window there.
- The Ψ system menu needs a mapping convention per platform (e.g. the
  macOS application menu).

## Open decisions

### O1 — GUI substrate

Portable windowing layer vs per-OS native shells. Leading options:

- **Gio** — pure Go, real multi-window, text shaping, HiDPI, clipboard,
  IME; no native menus (per-OS cgo add-ons needed for those regardless).
- **SDL2/3** — proven, one `SDL_Window` per window; cgo everywhere, no
  text shaping (needs FreeType + shaper), no native menus.
- **Per-OS native (Cocoa/Win32)** — the only route to first-class system
  menu integration; the most work. Likely later phases, not the
  foundation.

Because no portable layer provides native menus, the working shape is:
portable substrate for surfaces/input/pixels + a `PlatformIntegration`
capability interface (menus, dialogs, clipboard) that falls back to the
existing rendered `MenuBar` when no native implementation exists, with
per-OS native menu modules added over time.

### O2 — Unit semantics in graphical mode

What is 1 unit in a graphical window? Working proposal: 1 unit = 1
device-independent pixel, with HiDPI scaling handled by the Surface.
Note: today no widget generates sub-cell coordinates (everything is a
multiple of 8/16 units), so graphical layouts will initially sit on a
chunky grid until measurement (G1) and mode-aware painting (D1) land.

### O3 — Backend selection mechanism

Build tags vs runtime selection. Lean: graphical backends behind build
tags so TUI-only builds stay cgo-free and trivially cross-compilable;
within a graphics-enabled build, mode/window kind is a runtime choice.

### O4 — Bring-up sequencing of graphical rendering

Whether to use a whole-window glyph-grid presenter (existing cell
rendering rastered through a monospace font into a pixel window) as an
interim bring-up milestone before widgets' graphical paint paths exist.
The glyph-grid renderer is needed permanently either way — for PurfecTerm
and for terminal-style containers inside graphical windows (Goal 3) — the
open question is only whether it also serves as the first end-to-end
milestone for whole windows.

### O5 — Scope deferrals to confirm

- Native accessibility bridging (NSAccessibility/UIA) — the existing
  accessibility-label system is the seed, but the native bridge is a
  large separate effort. Propose: explicitly deferred, tracked.
- IME/composition input for graphical text entry — required for real
  GUI text input eventually; decide which phase.
- Drag-and-drop with the host desktop — propose deferred.
- Real clipboard support (TUI backend's is a stub) — needed early in
  graphical mode; cheap via the substrate.

## Proposed phase order (draft, pending O1–O4)

1. **G1** metrics/measurement hygiene (pure refactor, TUI-verified).
2. **G2 + G3** Platform/Surface split and event-loop inversion; TUI
   reimplemented as a one-surface Platform.
3. **G4** dual-mode Window; WindowManager scoped to in-surface use.
4. First graphical backend on the chosen substrate (O1): native windows,
   input, DPI; rendering path per O4.
5. **G5 + G6** native popups and `PlatformIntegration` for menus/dialogs/
   clipboard with rendered fallback; native macOS menus first.
6. **D1 rollout** widget-by-widget graphical paint paths with real fonts.

## Decision log

| # | Date | Decision |
|---|------|----------|
| D1 | 2026-07-05 | Widgets are mode-aware; same API, per-mode rendering owned by the widget. TUI cell idioms are TUI-only rendering material. |
