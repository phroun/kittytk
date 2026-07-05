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

### D4 — X-direction rendezvous; sessions are separate from connections  *(decided 2026-07-05)*

Connection topology for the D2 protocol follows the X model: the
**display service (desktop) listens on a well-known endpoint** (env
var, unix socket by default) and **apps dial in**. Rationale: the
ephemeral party must announce itself to the durable, well-known party —
discovery is one env var, launching from the desktop is fork/exec with
connect-back, cleanup on socket drop is unambiguous, one socket to
secure with peer credentials, and ssh forwarding works like `ssh -X`.

**Protocol invariant adopted:** a *session* (an app's entire UI state)
is a first-class protocol object distinct from the *connection* that
carries it. Under the widget-level protocol this is cheap — the app-side
client library already replicates the widget tree — and it buys
tmux-grade capabilities in the X-style topology: reattach after a
display-service restart, attach to a different display service
(replay the tree, resubscribe), and potentially multiple simultaneous
viewers later (input ownership then becomes a policy question).

Also adopted:

- **Naming discipline:** the desktop process is the *display service*
  (or desktop); apps are *apps*. Client/server language is reserved for
  describing individual connections, avoiding X's naming confusion.
- **Reverse attachment is a possible later mode, not the foundation:**
  daemon-style apps that listen for a display service to dial in can be
  added later — the post-handshake protocol is identical, only the
  connector differs (as with LSP transports / gdbserver reverse
  connections). Nothing in the wire format depends on who dialed.

This resolves the reconnection item in O6 in principle (v1 may still
ship terminate-on-disconnect, but IDs/handshake are designed for
session reattach from the start).

### D5 — Dual graphical substrates: Gio and SDL, neutrally  *(decided 2026-07-05)*

The graphical Platform (G2) gets **two substrate implementations, Gio
and SDL**, behind one substrate-neutral interface — the same discipline
PurfecTerm already applies to GTK/Qt. A third implementation of the
Platform interface (alongside the TUI Platform) is what keeps the
boundary honest and fossil-free, and it insures against substrate risk
(Gio API churn; SDL cgo dependency).

**Condition that makes this sound — the shared text engine:** text
shaping, measurement, and rasterization are pulled *out* of the
substrates into one tuitk-owned font module (go-text/typesetting is the
leading candidate), used identically by all graphical backends. Gio's
built-in text stack goes deliberately unused. This is mandatory, not
stylistic: under D2, layout is server-side and must be deterministic —
text measuring differently on two substrates would be a correctness
bug. The shared engine doubles as G1's server-side `TextMeasurer`, so
measurement and painting can never disagree.

The substrate contract is correspondingly small:

- window/surface creation and lifecycle, DPI scale factor
- input events, translated into D3 key nomenclature at the boundary
- vector primitives: fills, strokes, clips; glyph and image blitting
- clipboard; capability flags (IME, etc.)

Threading rule: Gio runs an event loop per window (goroutine each); SDL
has one main-thread global queue. The Platform delivers all events into
a **single tuitk dispatch goroutine** (channel fan-in), keeping widget
code single-threaded on both substrates (pins down G3's model).

Sequencing rule: substrates are brought up serially — define the
interface, land one substrate, then land the second **before the
interface is declared stable**, as a validation pass. Which substrate
goes first remains open (see O1): SDL is the easy glyph-grid bring-up
target; Gio is the nicer pure-Go distribution story.

Each substrate sits behind its own build tag (interacts with O3).

### D6 — Pango-class text is an available capability, at shaped-paragraph altitude  *(decided 2026-07-05, scope clarified same day)*

The shared text engine (D5) must make the full modern text model
**available to any widget that needs it** — full Unicode, OpenType
shaping (ligatures via GSUB, combining-mark positioning via GPOS — e.g.
Hebrew niqqud), bidirectional text (UAX #9, e.g. mixed Hebrew/Latin),
font fallback, and standard line/grapheme segmentation (UAX #14/#29).
We are building the architectural role Pango plays; a text model that
*cannot* express this is explicitly rejected.

**Scope clarification — capability, not mandate:**

- Not all UI text must go through the full pipeline. The engine exposes
  tiers behind one roof: a **fast simple path** (single-font,
  single-direction glyph runs — button labels, menu items, titles) and
  the **full shaped-paragraph path**, chosen per widget need. Same
  engine, same fonts, same metrics source, so D5's substrate-
  independence and D2's layout determinism hold on both tiers.
- **Terminal-style regions are a carve-out: PurfecTerm keeps its own
  text handling.** PurfecTerm's graphical text rendering is already
  sophisticated, customized, and proven in its GTK/Qt frontends, and it
  is retained for all terminal-style regions tuitk incorporates. The
  shared engine has no jurisdiction inside those regions; the boundary
  is the widget border. A terminal region's external layout contract
  (columns × cell size) is trivially deterministic, satisfying D2/D5
  without touching the shared engine.

**The protective decision is the interface altitude.** The engine's
contract is the shaped paragraph, not the measured string:

- Input: attributed text (font/style spans), available width,
  paragraph direction.
- Output: lines of shaped glyph runs — positioned glyphs with a bidi
  level per run — plus the **cluster map** (byte-range ↔ glyph-range),
  which is what makes caret movement, selection, and hit-testing
  correct in RTL text and inside ligatures.

Widgets' graphical paint paths (D1) consume shaped runs and cluster
maps — never per-rune arithmetic. With this contract the implementation
is swappable without touching widgets, layout, or protocol.

Implementation direction: **go-text/typesetting** as the reference
implementation (a Go transliteration of HarfBuzz's shaper — real
GSUB/GPOS execution; used by Gio but standalone), with
`x/text/unicode/bidi` (UAX #9), go-text's segmenter, and
`go-text/fontscan` for fontconfig-style discovery/fallback. A cgo
HarfBuzz/FreeType (or Pango) backend remains possible behind the same
interface if fidelity gaps appear; known soft spot in pure Go is
rasterization hinting quality (a swappable back-end concern, not
architectural).

Consequences recorded:

- **D2 synergy:** shaping lives entirely in the display service, where
  layout already is. Apps send logical text and never shape; cluster
  maps never cross the wire. (A primitive-level protocol would have
  forced both.)
- **Accepted asymmetry — TUI mode is constrained by the terminal.** A
  character grid cannot position niqqud or render ligatures; the TUI
  paint path does what terminals can (grapheme clusters, wide chars,
  the terminal's own bidi behavior). Same widget, same stored text,
  same API; full fidelity appears in the graphical path. This is D1
  working as intended, not a defect.

### D7 — A Canvas widget is the pixel escape hatch; development deferred  *(decided 2026-07-05)*

There will be a widget akin to HTML5's canvas: the escape hatch for
apps with image and drawing needs the stock widget set cannot express.
It follows the PurfecTerm pattern — app-owned content streaming into a
server-composited region, with input events forwarded raw — but for
pixels/drawing instead of character cells. **Development is deferred**
to a future to-be-developed widget; the groundwork only needs to keep
the slot open (it is one more widget type in the D2 protocol, so
nothing structural depends on its internals).

Design questions to answer when it is built (noted now, not decided):

- **Command-based vs pixel-buffer, or both** (HTML5 canvas is
  command-based with `putImageData` bolted on). Command-based remotes
  well — compact, and the display service can redraw on expose/resize
  without app round-trips. Pixel-buffer is the truly universal hatch
  but is bandwidth-heavy remotely and wants a shared-memory fast path
  locally. Likely both modes, command-based first.
- Coordinate space and DPI behavior (ties to O2).
- Input forwarding contract and frame synchronization/damage.
- Behavior in TUI mode (unavailable? degraded cell rendering? app's
  choice?).

### D9 — Height-for-width protocol; text-flow tiers; chrome vs text  *(decided 2026-07-05)*

A `core.HeightForWidther` optional interface (HasHeightForWidth /
HeightForWidth) lets widgets whose height depends on allocated width
(wrapped text) report their real height during layout, when widths are
known. `SizeHint` remains the width-independent preference. BoxLayout
consults it; Panel propagates it upward; ScrollArea/Splitter/Window are
absorbers where propagation stops. (A WidthForHeight transpose is
acknowledged but not built — nothing needs it.)

Text-flow tiers (which widgets flow text is a toolkit design decision,
and under D2 it is protocol surface):

- **Label** — wrap is core purpose; wraps + height-for-width.
- **Checkbox / RadioButton** — wrap is **opt-in** (`SetWordWrap`),
  default single-line. **The `[x]`/`(*)` indicator is CHROME, not
  text**: it stays anchored to the top line, and wrapped lines hang
  under the text column, never under the indicator.
- **Button, tabs, list rows, menu items** — deliberately single-line
  for now; overflow handled by other means (ellipsis, scrolling).
- **DockRow** — already a hand-rolled height-for-width widget
  (RequiredHeight); deliberately NOT migrated to the interface —
  it is specific for other reasons and will be considered separately
  rather than replaced on confidence alone.
- **MessageBox** — future adopter (wrap message via Label, auto-size
  height); pending, ties to the dialog.go:179 C-site.

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

### G1 — Metrics hygiene: two concepts, two sources  *(model decided 2026-07-05; full call-site audit in `g1-metrics-audit.md`)*

There are two distinct measurement concepts, which coincide numerically
in default TUI mode and diverge everywhere else. G1's job is giving each
its proper source and stopping them impersonating each other:

- **Grid metrics** (CellMetrics) — a deliberate **layout vocabulary**,
  not a TUI artifact: the application decides how many native units a
  virtual row/column occupies per container, for placement detail and
  density — including in true graphical windows, where lines/columns
  remain available for aesthetic layout decoupled from the proportional
  font. **Clarified: metrics are a coordinate denomination, like DPI —
  they change what unit values mean, never how big things look.**
  Row/column-denominated sizes are visually invariant under
  re-denomination; only explicit numeric unit values reinterpret.
  Requires denomination scaling at container boundaries in the
  paint/input path (see `g1-metrics-audit.md`, "Denomination model"). **Decided model:** grid metrics are *inherited from the
  container chain* (mirroring the existing `FontProvider`/
  `FindEffectiveFont` pattern), overridable per window/container in all
  modes including TUI, rooted at the display service's default — which
  derives from the system default font size in its settings. The
  hygiene defect is that ~133 call sites grab the global
  `DefaultCellMetrics()` constructor instead of their container's
  definition (no container metrics storage exists yet — the inheritance
  mechanism must be built; fonts provide the blueprint).
- **Text metrics** — "how much space does this text in this font occupy
  on this render target?" Target-dependent: the same text has different
  correct widths on a TUI surface and a graphical window coexisting in
  one display service. `Font.MeasureText` as a static function cannot
  answer this; measurement routes through the render context
  (TextMeasurer per target; the TUI measurer returns today's
  cell-quantized answers, the graphical one delegates to the D6 text
  engine). The Tuesday font exists precisely as a proportional test
  case (letters/digits 16 units, punctuation 8) and already exposes
  text-questions-answered-with-grid-math (Label `wrapText`,
  tab-bar width, menu clock width).

Audit summary (see `g1-metrics-audit.md` for the per-site checklist):
133 sites — 102 grid questions needing the container walk, 14 text
questions in disguise, 10 legitimate root-default sites (backend init +
Desktop chrome, to be re-rooted on a stored desktop default), 6
PurfecTerm cell-grid sites, 1 trivial painter swap, and one structural
dead-end (`NewSpacer` sizing in its constructor). Verified by
byte-identical demo rendering, except deliberate bug fixes at the
text-in-disguise sites (reproducible via the Selection-tab wrap row
under Tuesday).

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

### O1 — Substrate bring-up order  *(narrowed by D5)*

The substrate question itself is resolved by D5 (both Gio and SDL,
neutrally, serially). Remaining open: **which substrate lands first.**
SDL is the trivially easy target for glyph-grid bring-up (cell grid →
texture-atlas blit); Gio is the nicer pure-Go distribution story and
exercises more of the interface sooner.

Still applicable regardless: no portable layer provides native menus,
so a `PlatformIntegration` capability interface (menus, dialogs) falls
back to the existing rendered `MenuBar` when no native implementation
exists, with per-OS native menu modules (Cocoa/Win32) added over time.

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

- **Wire format — direction set by D10 (2026-07-05):** self-describing
  **named-property records** (nothing positional), text-oriented, with
  **sender-declared alias dictionaries** for wire efficiency. Sketch
  (syntax illustrative, not final):

  ```
  new button caption="Caption Here" action="action_id_here"
  alias c="caption" a="action"
  new button c/Caption Here/ a/other_action/ some_float_prop=4.2
  ```

  Design notes recorded with it: alias tables are
  **connection-scoped, not session-scoped** (encoding state resets on
  reattach; session replay stays purely semantic, per D4), and
  **independent per direction** (each sender declares its own
  outbound aliases). Named properties + additive-only evolution +
  capability advertisement is the versioning story. Likely needs a
  length-prefixed bulk/binary escape within the text framing for
  cell-diff streams (PurfecTerm) — TBD. Remaining open: exact syntax,
  framing, quoting/escaping, the bulk escape, negotiation handshake.
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
- **Pixel escape-hatch timing:** resolved by D7 — a Canvas widget will
  exist and is explicitly deferred; the cell-grid surface widget is
  still needed early.
- **Reconnection semantics:** resolved in principle by D4 (sessions are
  first-class and separable from connections). Remaining detail: does
  v1 ship terminate-on-disconnect with reattach-ready IDs/handshake, or
  implement session reattach immediately?
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

1. ✅ **G1** metrics/measurement hygiene — **complete 2026-07-05**
   (inheritance chain, denomination exchange with visual invariance,
   full call-site sweep, C-site resolution; residuals tracked in
   `g1-metrics-audit.md`). Delivered beyond original scope: the
   denomination model (D8′) implemented and live-verified, plus the
   height-for-width protocol (D9) and four layout-contract fixes.
2. **D2 API shape** — restructure the app-facing API against the
   proxy/replica model (stable IDs, event subscriptions instead of
   closures, cached reads, async writes), still entirely in-process.
   This is the protocol's dress rehearsal with no serialization yet.
3. **G2 + G3** Platform/Surface split and event-loop inversion inside
   the server; TUI reimplemented as a one-surface Platform.
4. **G4** dual-mode Window; WindowManager scoped to in-surface use.
5. **D2 transport** — client library + wire format + display-service
   connection handling (unix socket rendezvous per D4, client
   lifecycle, cell-grid surface widget). The TUI desktop-as-display-
   service is the first target: separate app binaries connecting to a
   running TUI desktop.
6. **Shared text engine** (D5/D6) — Pango-class shaped-paragraph module
   (shaping, bidi, fallback, segmentation, rasterization); also serves
   as G1's server-side TextMeasurer for graphical mode.
7. First graphical substrate (order per O1): native windows, input,
   DPI; rendering path per O4. Second substrate lands before the
   substrate interface is declared stable.
8. **G5 + G6** native popups and `PlatformIntegration` for menus/dialogs/
   clipboard with rendered fallback; native macOS menus first.
9. **D1 rollout** widget-by-widget graphical paint paths with real fonts.

## Decision log

| # | Date | Decision |
|---|------|----------|
| D1 | 2026-07-05 | Widgets are mode-aware; same API, per-mode rendering owned by the widget. TUI cell idioms are TUI-only rendering material. |
| D2 | 2026-07-05 | Apps compile independent of the renderer and talk to a desktop/render server over a socket (X-style). Boundary = **widget-level protocol**: server owns widgets/layout/rendering/hit-testing, apps drive proxies with the same API and receive semantic events. In-process stays as a direct implementation. Cell-grid + (later) pixel surface widgets are the custom-rendering escape hatch. |
| — | 2026-07-05 | Context: PurfecTerm is an independent pre-existing project with TUI, GTK, and Qt frontends in one codebase — proof of the D1 pattern, source of graphical cell-grid rendering, input to O1. |
| D7 | 2026-07-05 | A Canvas widget (HTML5-canvas-like: PurfecTerm pattern, but for images/drawing) is the pixel escape hatch. Committed to exist; development deferred to a future widget. Likely command-based + pixel-buffer modes, command-based first. |
| D8 | 2026-07-05 | Grid-metrics model: CellMetrics is a per-container layout vocabulary (app chooses units per virtual row/column for placement density), inherited through the container chain like fonts, overridable per window in all modes including TUI, rooted at the display service's default derived from its system default font size. Text measurement is a separate, per-render-target question. G1 implements this model; call-site audit in `g1-metrics-audit.md`. |
| D9 | 2026-07-05 | Height-for-width: `core.HeightForWidther` optional interface, consulted by layouts at layout time, propagated by containers, absorbed by ScrollArea/Splitter/Window. Text-flow tiers: Label wraps; Checkbox/RadioButton wrap opt-in with the indicator as top-line-anchored chrome and lines hanging under the text; buttons/tabs/list rows/menu items stay single-line; DockRow deliberately not migrated (considered separately); MessageBox a future adopter. Implemented same day (slices 1+2). |
| D10 | 2026-07-05 | Wire discipline: **nothing positional — every value travels under a property name**, with sender-declared, connection-scoped alias dictionaries for efficiency (HPACK-style, but explicit). Text-oriented spirit; exact syntax deliberately open. Consequence: property/event names formalized during the D2 API-shape phase ARE wire vocabulary — maintained deliberately from slice 3 onward. |
| D8′ | 2026-07-05 | **D8 clarified:** CellMetrics is a coordinate *denomination* (units per row/column, like DPI), not a spacing knob. Row/column-denominated sizes are visually invariant under re-denomination; only explicit numeric unit values reinterpret. Implies denomination scaling at container boundaries (paint + input; `Transform.ScaleX/Y` was built for this) and container-denominated text metrics (font.go's 8/16 are DefaultCellMetrics in disguise). Demo's grid toggle is the acceptance test: must become a visual no-op for row-denominated content. |
| D3 | 2026-07-05 | The direct-key-handler key nomenclature (`^N`, `M-x`, `S-Tab`, …) stays the unified internal, app-facing, and (as far as practical) user-facing key representation on all platforms and in the wire protocol. Native-menu key equivalents, if required, are a one-way mapping at the platform-integration boundary only. Revisitable later as a new decision. |
| D4 | 2026-07-05 | X-direction rendezvous: the display service listens on a well-known endpoint, apps dial in. Sessions are first-class protocol objects separable from connections (enables reattach/multi-viewer without inverting topology). Reverse attachment is a possible later mode. Naming: "display service" and "apps". |
| D5 | 2026-07-05 | Two graphical substrates, Gio and SDL, behind one neutral Platform interface (PurfecTerm-style discipline). Mandatory condition: one shared tuitk-owned text engine (shaping/measurement/rasterization) outside the substrates, so layout is substrate-independent; it doubles as the server-side TextMeasurer. Substrates land serially; second lands before the interface is declared stable. |
| D6 | 2026-07-05 | Pango-class text (full Unicode, OpenType shaping incl. ligatures and niqqud mark positioning, bidi, fallback, UAX segmentation) is an **available capability**, not a universal mandate: the engine offers a fast simple-run tier and a full shaped-paragraph tier, chosen per widget need. Interface at shaped-paragraph altitude (attributed text in → shaped runs + cluster maps out); go-text/typesetting reference, cgo HarfBuzz/Pango swappable. Terminal-style regions are a carve-out: PurfecTerm keeps its own proven graphical text handling. TUI-mode fidelity limits remain an accepted asymmetry. |
