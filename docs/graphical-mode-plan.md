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

### D2 — Widget-level client/server protocol: apps compile independent of the renderer  *(decided 2026-07-05)*

Applications are compilable independent of the rendering environment
and talk to a rendering/desktop process over a boundary (unix socket,
IP, or ssh-forwarded), similar in spirit to the X Window System. A
running TUI (or graphical) desktop becomes a real desktop-environment
binary; separately compiled apps connect to it and request their
rendering needs. In-process operation remains supported: the same
app-facing API is implemented either directly (current single-binary
mode) or by a client library speaking the wire protocol.

Chosen boundary: a **widget-level protocol**. The server (desktop
process) owns widget instances, layout, rendering (mode-aware per D1),
and hit-testing. Apps manipulate widgets through proxy objects exposing
the same programmer interface as today (satisfying D1's "identical
API" guarantee), and receive **semantic events** (clicked, text
changed, selection changed, window closed) rather than raw input.

Why widget-level (supersedes the earlier primitive-level proposal):

- Best possible remote latency: hover, scrolling, text-edit echo,
  drag feedback, menu navigation all happen server-side with zero
  round-trips. Only meaningful state changes cross the wire.
- Apps are freed from layout and text measurement entirely — those
  live with the renderer, where the fonts are. G1's "mirrorable
  metrics" constraint dissolves for apps (it still applies *inside*
  the server between widgets and render backends).
- Matches the vision: the desktop is an environment that serves apps,
  not a dumb framebuffer.

Costs accepted, with mitigations:

- **The widget API becomes wire contract.** Every widget's properties
  and events are protocol surface. Requires versioning + capability
  negotiation at connect, and API-design discipline (additive changes).
- **Custom widgets need an escape hatch.** Apps that draw things the
  server has no widget for get a client-rendered surface widget:
  (a) a cell-grid surface (app streams cell diffs — also the natural
  transport for terminal content), and (b) later, a pixel surface for
  graphical custom rendering. These are the "canvas" widgets of the
  protocol.
- **State ownership must be explicit.** The server-side widget owns
  interactive state (text buffer contents, scroll position, selection,
  checked state) and emits change events; the client library keeps a
  replicated cache so app-side property *reads* stay synchronous-
  looking. App-side *writes* are async messages. In-process and remote
  modes must be behaviorally identical, so the API is designed against
  the cached-replica model from the start.

Design constraints this imposes on the groundwork (cheap to honor now,
expensive to retrofit):

- **IDs, not pointers; data + events, not closures.** Widgets, windows,
  menus, popups, dock entries get stable IDs across the seam. Menu
  items dispatch as "item ID triggered" events (extends G6's command-ID
  requirement). Callbacks (`OnClick`, `OnTriggered`, `PopupOverlay`'s
  `Paint func`) become event subscriptions keyed by ID.
- **No synchronous app→server queries** in any hot path; reads come
  from the replicated client cache, updated by server events.
- **Flow control and lifecycle:** back-pressure so a stalled client
  cannot wedge the server; the server cleans up all widgets/windows of
  a disconnected client (app crash safety — a side benefit no
  in-process design offers); reconnection semantics defined.

Sub-decisions still open (see O6).

### D3 — Unified key nomenclature everywhere  *(decided 2026-07-05)*

The direct-key-handler key-naming scheme (`^N`, `M-x`, `S-Tab`, `F10`,
and so on) is retained across **all** implementations — TUI, graphical,
and the display protocol — for now. It is the single internal
representation for key events, shortcut definitions, and shortcut
matching, and it is presented to the programmer, and to the user as far
as practical, unchanged on every platform.

Rationale: this is a specialized system — a technology demo and a
programming learning environment, deliberately dabbling in
WordStar-like heritage. The nomenclature carries real complexity and is
part of the project's identity; unification outweighs platform
convention for now.

Boundary rule: if native system menus (NSMenu key equivalents, Win32
accelerators) require per-platform normalized forms to be implemented,
translation happens **only at that boundary**, as a display/registration
concern inside the platform integration layer — never as a change to the
internal or app-facing representation. This may be revisited later; any
such revisit is a new decision, not an erosion of this one.

Effect on G6: the "shortcut translation" snag listed there is scoped to
a one-way mapping (key-string → native key equivalent) living in the
native menu module; wire protocol (D2) and widget APIs carry key-strings
verbatim.

### Context: PurfecTerm is already multi-frontend  *(noted 2026-07-05)*

PurfecTerm predates tuitk as an independent project and already has
**three working frontends in a single codebase: this TUI implementation,
GTK, and Qt.** Consequences for this plan:

- D1's mode-aware-widget pattern is already proven in production by
  PurfecTerm; porting the widget into the new system is expected to be
  easy.
- Its GTK/Qt renderers are existing graphical cell-grid renderers —
  directly relevant to Goal 3 (terminal containers inside graphical
  windows) and to the D2 cell-grid surface widget.
- Its GTK/Qt experience is an input to the substrate choice (O1).

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

- Shortcuts use key-strings (`"^N"`, `"M-x"`) which per D3 remain the
  internal and app-facing representation; native menus get a one-way
  key-string → native-key-equivalent mapping inside the platform menu
  module only.
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

### O6 — Display-protocol sub-decisions (under D2)

- **Wire format:** custom binary framing vs protobuf/flatbuffers vs gob.
  Requirements: compact property/event messages, cheap cell-diff
  encoding for surface widgets, pipelined (no round-trips in hot
  paths), capability/version negotiation at connect.
- **Protocol versioning discipline:** how widget properties/events are
  declared and evolved (additive-only? feature flags per widget?), so
  server and app binaries of different vintages interoperate.
- **Transports for v1:** unix socket with peer credentials is the
  default. For remote use, lean on ssh forwarding (the X11 answer)
  rather than building TLS+auth immediately? Direct IP+TLS later.
- **Where terminal emulation lives:** for a remote app hosting a PTY,
  does the app stream raw bytes to a server-side PurfecTerm widget
  (thin client, server does emulation), or run the emulator app-side
  and stream cell diffs to a cell-grid surface widget? Both are
  possible with the existing purfecterm library; pick the v1 shape.
- **Pixel escape-hatch timing:** the cell-grid surface widget is needed
  early; when does the pixel surface widget (arbitrary client-rendered
  graphical content) land?
- **Reconnection semantics:** do clients survive a server restart
  (re-attach and replay state) or terminate? Proposal for v1: terminate;
  design IDs/handshake so re-attach can be added.
- **App launching:** does the desktop server spawn client apps (menu of
  installed apps), or are apps launched externally and connect? Both
  eventually; which first?

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

1. **G1** metrics/measurement hygiene (pure refactor, TUI-verified) —
   server-internal under D2: widgets query the render backend, never
   static cell assumptions.
2. **D2 API shape** — restructure the app-facing API against the
   proxy/replica model (stable IDs, event subscriptions instead of
   closures, cached reads, async writes), still entirely in-process.
   This is the protocol's dress rehearsal with no serialization yet.
3. **G2 + G3** Platform/Surface split and event-loop inversion inside
   the server; TUI reimplemented as a one-surface Platform.
4. **G4** dual-mode Window; WindowManager scoped to in-surface use.
5. **D2 transport** — client library + wire format + server connection
   handling (unix socket, client lifecycle, cell-grid surface widget).
   The TUI desktop-as-server is the first target: separate app binaries
   connecting to a running TUI desktop.
6. First graphical backend on the chosen substrate (O1): native windows,
   input, DPI; rendering path per O4.
7. **G5 + G6** native popups and `PlatformIntegration` for menus/dialogs/
   clipboard with rendered fallback; native macOS menus first.
8. **D1 rollout** widget-by-widget graphical paint paths with real fonts.

## Decision log

| # | Date | Decision |
|---|------|----------|
| D1 | 2026-07-05 | Widgets are mode-aware; same API, per-mode rendering owned by the widget. TUI cell idioms are TUI-only rendering material. |
| D2 | 2026-07-05 | Apps compile independent of the renderer and talk to a desktop/render server over a socket (X-style). Boundary = **widget-level protocol**: server owns widgets/layout/rendering/hit-testing, apps drive proxies with the same API and receive semantic events. In-process stays as a direct implementation. Cell-grid + (later) pixel surface widgets are the custom-rendering escape hatch. |
| — | 2026-07-05 | Context: PurfecTerm is an independent pre-existing project with TUI, GTK, and Qt frontends in one codebase — proof of the D1 pattern, source of graphical cell-grid rendering, input to O1. |
| D3 | 2026-07-05 | The direct-key-handler key nomenclature (`^N`, `M-x`, `S-Tab`, …) stays the unified internal, app-facing, and (as far as practical) user-facing key representation on all platforms and in the wire protocol. Native-menu key equivalents, if required, are a one-way mapping at the platform-integration boundary only. Revisitable later as a new decision. |
