# D2 API-Shape Phase — Tracker

Phase 2 of `graphical-mode-plan.md`: restructure the app-facing API
around the display protocol's proxy/replica model **while everything is
still in one process** — no serialization, no transport. This is the
protocol's dress rehearsal: every breaking API change happens here,
verified against the TUI demo, so the transport phase later is "just"
a second implementation of already-shaped seams.

Constraints honored throughout (from D2/D4): stable IDs instead of
pointer identity; data + events instead of closures crossing the seam;
reads served replica-style (synchronous-looking, cache-backed);
sessions separable from connections.

Additional guardrail (2026-07-05): **the app-side client library is
instance-scoped, never a singleton.** One app must be able to hold N
connections to N display services simultaneously (console + GUI +
remote — the Emacs-daemon scenario), which works by construction as
long as connections are first-class handles, event delivery and
command dispatch carry their originating connection, and nothing
app-side assumes "the one display." ObjectIDs, aliases, templates, and
sessions are already per-connection by earlier decisions; this
guardrail keeps the client library from undoing that.

## Slices

1. **Menu command identity & registry dispatch** — ✅ done 2026-07-05
   (below).
2. **Stable IDs for windows and widgets** — ✅ done 2026-07-05 (below).
3. **Event-subscription formalization** — widget callbacks
   (`OnClick`, `OnToggled`, `OnStateChanged`, …) become subscriptions
   keyed by widget ID under the hood; the public setter API keeps its
   current feel.
4. **Replicated-read discipline audit** — classify every public getter
   as replica-safe (served from app-side cached state) or
   server-authoritative; restructure the exceptions.

The protocol's wire shape is deliberately NOT decided here, with one
principle now fixed (D10, 2026-07-05): **nothing positional — every
value travels under a property name**, with sender-declared,
connection-scoped alias dictionaries for wire efficiency. Consequence
for this phase: the property and event names formalized in slices 3–4
are wire vocabulary — choose them deliberately and keep a vocabulary
list. (Also validated by the D10 sketch: buttons bind to command IDs
via an `action` property — slice 1's registry extends to buttons in
slice 3.)

## Milestone P0 — the demo runs on the protocol, in-process

Owner-directed goal (2026-07-05): tuitk and its demo operating on the
D10–D17 protocol basis within a single process — commands and events
as real protocol records, no sockets yet. Steps:

1. ✅ **Protocol core package** (`protocol/`) — parser + AST for the
   full command language: flags (`!`/`?`), quoted strings, numerics,
   identifiers, `{}` blocks, keyed statements, alias/template forms,
   surfacing references. The tokenizer is schema-free per D17; alias
   application and template expansion belong to the layers above.
   Provisional pending O6: `#` comments; string escapes `\\ \" \n \t
   \r`. Eight tests cover the canonical corpus from D10–D17.
2. **Vocabulary binding + builder** — machine-readable property
   registry (the twin of `property-vocabulary.md`) mapping names to
   typed setters per widget type; interpreter executing
   `new`/`alias`/`template` against real widgets; scoped-key
   resolution and correlation replies. (Introduces the `set` verb and
   friends — verb inventory is a small O6 item to settle with the
   owner.)
3. **Event records + subscriptions** (absorbs slice 3) — events
   emitted as protocol records (`event change widget=17 selected=3`)
   through an in-process channel; app-side dispatch by ObjectID.
4. **Demo on this basis** — a demo window defined as protocol text
   executed at startup in-process, interactions flowing back as event
   records into registry/subscription handlers. Full API purity (the
   Go client-library veneer) comes after the milestone.

## Slice 1 — Menu command identity & dispatch  *(done 2026-07-05)*

What exists now:

- **`core.CommandRegistry`** — handlers keyed by stable string command
  ID; `Register` / `Unregister` / `Has` / `Dispatch`. The in-process
  half of the dispatch seam: under the protocol, the display service
  emits "command <ID> triggered" events and the app-side client
  library dispatches through exactly this shape.
- **Every `MenuItem` has a stable ID** — auto-assigned
  (`cmd.auto.N`) at construction; override with `SetID` for semantic,
  run-stable IDs (`"file.open"` — the `core.StandardActions`
  vocabulary predates this and fits directly).
- **`Menu.BindCommands(reg)`** walks a menu tree (submenus included),
  registers each item's handler under its ID, and routes all future
  triggers through the registry. `MenuItem.Trigger()` dispatches by ID
  when bound, falling back to the direct closure when not (standalone
  menus keep working).
- **Wiring:** `Application.SetMenuBarContent` binds automatically into
  the app's registry (`Application.Commands()` accessor); the Desktop
  binds its system menu into a desktop-level registry. The shortcut
  path (`checkMenuItemShortcuts`) now goes through `Trigger()` like
  every other activation path — closures are no longer invoked
  directly anywhere.
- Behavior notes: shortcut activation of a checkable item now toggles
  its checked state, consistent with clicking (previously it bypassed
  the toggle). `SetOnTriggered` after binding refreshes the
  registration.

Public API unchanged: `NewMenuItem("&Open").SetOnTriggered(fn)` works
exactly as before; `SetID` is additive.

Deferred within this slice (tracked, not forgotten):

- Standard items injected by `createAppMenuWithStandardItems`
  (Hide/Quit) use the closure fallback; bind them to the desktop
  registry when that merge path is next touched.
- `core.Action` integration: `MenuItem` and `Action` should
  eventually converge (an item constructed *from* an action inherits
  ID/shortcut/enabled/checkable) — slice 3 territory.
- Dock entries (`OnClick`) and `PopupRequest` callbacks are the other
  closure-crossing surfaces; they join in slices 2–3.

## Slice 2 — Stable object identity  *(done 2026-07-05)*

What exists now:

- **`core.ObjectID`** (uint64) — the stable identity of a UI object.
  Allocated from a process-wide counter at `NewWidgetBase()`, so every
  widget, window, panel, and the desktop itself carries one from
  birth: `w.ObjectID()`. Immutable after construction.
- **Deliberately NO process-global ID→object registry.** The object
  table belongs to the display service's per-session connection state
  (created at attach, released at detach, per D4's session model).
  A global registry now would bake in the wrong lifecycle and leak
  discarded widgets; the transport phase builds the real table.
- **First consumers converted (both fixed latent identity bugs):**
  - Dock entries carry `WindowID`; minimize/restore wiring (Desktop
    and Application) adds/removes by ID. Previously keyed by window
    *title* — two same-titled windows corrupted the dock.
    `RemoveEntryByTitle` is deprecated but kept.
  - ComboBox popup IDs derive from `ObjectID()`. Previously
    `"combobox-" + Name()` — unnamed comboboxes collided.
- Distinction now explicit in the codebase: **ObjectID is identity;
  `Name()` is a human label; command IDs are semantic verbs.** Three
  different things, no longer substitutable.

Next: slice 3 (event-subscription formalization) keys widget-event
subscriptions by ObjectID — the "widget 17 was clicked" half of the
event stream, joining slice 1's "command file.open triggered" half.
