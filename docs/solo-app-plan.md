# Solo App Plan

## Standing rule: everything goes through the protocol

**We always develop client/server functionality.** No application built on
this toolkit should reach the UI except through the display protocol
(D10-D22): it builds and drives its interface as protocol text over a
connection, whether that connection is in-process (`client.NewInProcess`)
or a socket to a display service (`client.Dial`). New capabilities are
added as protocol vocabulary (properties, verbs, handshake options) plus
the host-side handling that services them - never as an in-process-only
widget API an app calls directly. When a feature needs the app to reach
something only the host owns (the focused widget, window arrangement, the
desktop), it becomes a protocol verb/property, as the display app-verbs
(`cut`, `tile`, `theme`, `rawkey`, ...) and window properties (`main`,
`tearable`, `font`) already do.

## Overview

Let an application be "the whole thing": its main window replaces the
desktop entirely and fills the display, rendered in the same self-contained
form a main window already takes when it is torn off - its own menu bar and
status bar, no desktop wallpaper, dock, or system menu behind it. This is
**solo mode**.

The torn-off state is already the visual target. A torn main window is a
`platform.SurfaceHandler` (`window.TearOffHost`) that paints a
window-with-chrome and handles its own input, popups, cursor and clipboard.
Solo mode is that same picture made the root, with no desktop hosting it.

## Settled decisions

- **No Psi / system menu.** A solo app owns 100% of the chrome; there is no
  desktop system menu. (The empty-desktop rule - only Psi when an app has
  neither menus nor windows - is a step toward "no system furniture unless
  earned"; solo mode drops Psi entirely.)
- **No redock handle.** The main window is not tearable in solo mode (there
  is nothing to dock back to), so it shows no `%`/`#` handle.
- **Additional windows.** A solo app may still open more windows. Each runs
  like another solo surface (its own OS surface) or lives in an MDI pane -
  the app's choice. They are peers of the main window, not children on a
  desktop.
- **Lifecycle.** Additional windows/apps do not outlive the root: when the
  first (root) solo app exits, everything it spawned dies with it.
- **Protocol-driven.** Solo is declared over the wire (see below); it is not
  an in-process-only mode. Per the standing rule, the same declaration works
  in-process and remote.
- **Desktop may return later.** Nothing here forbids re-introducing a
  desktop *inside* a solo app someday (a desktop is a widget); it is simply
  not needed now.

## Protocol surface

Solo is a property of the whole connection/app, known at connect time, so it
rides the handshake:

```
hello version=1 app="My App" solo
```

- `client.DialSolo(path, appName, dispatch)` (and an in-process equivalent)
  sends the `solo` flag.
- The display records it on the connection. When that connection adopts a
  top-level window marked `main` (the `main` window property), the display
  puts its desktop into solo mode bound to that window.
- The root connection's disconnect quits the host (all spawned surfaces die).

## Two implementation paths

- **Path A - solo mode on the existing Desktop (first).** Keep Desktop as
  the host but strip it: no wallpaper, no dock, no Psi menu; the `main`
  window is maximized to the full surface and non-tearable; its menu/status
  render as the (Psi-less) bar. Reuses the event loop, window manager,
  timers and cursor wiring already in place. A now-invisible Desktop still
  sits underneath - hidden, not replaced - which is acceptable for now.
- **Path B - extract a `Shell` host (later).** Factor the platform services
  both `Desktop` and `TearOffHost` consume (surface/backend, event loop,
  timer system, `CursorController`, clipboard, global pointer) into one host
  type. Then a window+chrome runs directly on a `Shell` as root with no
  Desktop. Torn window = "a Shell hosting one window on a secondary
  surface"; solo app = "... on the primary surface"; Desktop = "a Shell that
  also has wallpaper/dock/WM/multiple apps." This makes "the app replaces the
  desktop" literally true. Build A first and let it reveal exactly which
  services B must extract.

## Milestones

1. **Solo handshake + solo desktop mode (Path A).** `solo` in the handshake;
   `client.DialSolo`; the display flips its desktop into solo mode when the
   root app's `main` window is adopted: suppress Psi/dock/wallpaper, maximize
   and de-tear the main window, quit on root disconnect. Headless test:
   connect solo, assert no Psi menu, main window maximized and non-tearable,
   host quits when the root connection closes.
2. **Additional windows as solo surfaces.** A solo app's non-main windows get
   their own surfaces (reuse the tear-off host path) or an MDI pane, per the
   app.
3. **Shell extraction (Path B).** Unify torn / solo / desktop under one host.
