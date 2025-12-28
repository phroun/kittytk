# Menu Bar and MDI Architecture Considerations

This document captures design discussions and considerations for implementing context-sensitive menu bars and MDI (Multiple Document Interface) support in the TUI toolkit.

## Overview

The goal is to support Mac-like context-sensitive menus where the menu bar content can change based on which window or document is active, while also providing flexible MDI support that isn't tightly coupled to a specific container type.

## MDIPane Widget

### Motivation

The Window class should remain focused on being a window (frame, title bar, buttons). Container semantics for managing floating child windows should live in a separate widget.

### Proposed Design

An `MDIPane` widget that:

1. **Is a regular widget** - can be placed anywhere (in a Window, TabWidget tab, Panel, Splitter, etc.)

2. **Has background content** - accepts child widgets with layout managers for the area behind floating windows

3. **Manages floating windows** - maintains a z-ordered list of Window children that float above the content

4. **Handles input routing**:
   - Hit-test floating windows first (front to back)
   - If no window hit, route to background content
   - Keyboard goes to focused MDI child or falls through to content

5. **Provides the MDI API**:
   - `AddWindow(w *Window)`, `RemoveWindow(w *Window)`
   - `FocusedWindow()`, `SetFocusedWindow(w *Window)`
   - `FocusNextWindow()`, `FocusPrevWindow()`
   - `Windows() []*Window` for external UI to query
   - Callbacks: `OnWindowAdded`, `OnWindowRemoved`, `OnWindowFocusChanged`

6. **Doesn't prescribe UI** - external components (menus, docks, sidebars) use the API

### Usage Patterns

```go
// Classic MDI - pane fills a window
mainWindow.SetContent(mdiPane)

// MDI in a tab
tabWidget.AddTab("Documents", mdiPane)

// Split view - MDI pane on left, tools on right
splitter.AddChild(mdiPane)
splitter.AddChild(toolPanel)

// MDI with built-in toolbar above
panel.AddChild(toolbar)
panel.AddChild(mdiPane) // fills remaining space
```

### Desktop Refactoring

Desktop could be refactored to use MDIPane internally, becoming a composition of:
- MenuBar (top)
- MDIPane (fills middle)
- DockRow (above status bar)
- StatusBar (bottom)

## Context-Sensitive Menu Bars

### The Problem

Rather than having a static menu bar that owns all content, we want:

1. **Context sensitivity** - menu bar content changes based on which window/application is active
2. **Window-driven population** - windows find and populate their designated menu bar, not the reverse
3. **Inheritance** - dialog boxes and tool windows share menus with their parent/owner
4. **Flexibility** - support for multiple menu bars, merged menus, etc.

### Conceptual Model

#### Menu Bar as Display Slot

The MenuBar becomes more of a "display slot" or "projection surface" than an owner of content:

- **MenuBarSlot**: A physical location/widget that displays menu bar content
- **MenuBarProvider**: An interface that windows/widgets implement to supply menu content
- **Focus-driven activation**: When focus changes, the system finds the appropriate provider and displays its content

#### Inheritance Chain

When looking for a MenuBarProvider:
1. Start from the focused widget
2. Walk up the parent chain
3. Dialogs and tool windows that don't implement MenuBarProvider inherit from their parent/owner window

### Open Questions

1. **Widget vs Data**: Is menu bar content a widget instance, or declarative data that gets rendered?

2. **Menu Merging**: Can providers merge menus (base app menus + document-specific additions)?

3. **Binding Model**: How explicit is the slot-to-provider binding? Automatic via ancestry, or explicitly registered?

4. **Multiple Slots**: Could a window provide different menus for different slots (main menu bar vs local toolbar menu)?

## Prior Art

### Qt

- Separates `QMenuBar` (display) from `QAction` (commands)
- Actions are abstract - same action appears in menu, toolbar, context menu, shortcut
- `QMdiArea` is an explicit container widget for MDI children
- On macOS, can automatically move menu bar to system location
- Actions have "menu roles" for macOS integration (About, Preferences, Quit go to app menu)

### GTK (3/4)

- `GAction` + `GMenu` pattern - menus are declarative data, not widgets
- `GMenu` is a model, `GtkMenuBar` renders it
- Actions live on `GtkApplication` or `GtkApplicationWindow`
- Clear separation: define actions once, present them in multiple UI locations

### Cocoa (macOS native)

- **Responder chain**: Menu actions walk up from focused view to find a handler
- `validateMenuItem:` - objects enable/disable menu items dynamically based on current state
- Menu bar is app-global, but *which object handles each action* depends on focus
- First responder determines context without swapping menu bar content

### Electron

- Menus defined as JSON/data structures
- Mapped to native menus on macOS, in-window on other platforms
- Click handlers bound at definition time

## Key Patterns to Consider

### 1. Actions as First-Class Objects

Define a command once (name, shortcut, icon, handler), use it in menu bar, toolbar, context menu:

```go
type Action struct {
    ID          string
    Text        string
    Shortcut    string
    Icon        string
    Enabled     func() bool
    Triggered   func()
}
```

### 2. Responder Chain

Rather than swapping menu content, the *same* menu items route to *different handlers* based on focus. "Save" always exists in the menu, but the focused document handles it.

This avoids the complexity of swapping menu bars while still achieving context sensitivity.

### 3. Menu as Model, Not Widget

Separate the menu definition (model/data) from the display (widget):

```go
// Menu model - just data
type MenuModel struct {
    Items []MenuItemModel
}

// Multiple widgets can render the same model
menuBar.SetModel(model)
contextMenu.SetModel(model)
```

### 4. Chrome Widgets Find Their Context

Dock, MenuBar, StatusBar could look up their parent chain to find the nearest MDIPane (or other context provider) and interact with it:

```go
func (w *DockRow) findMDIPane() *MDIPane {
    for parent := w.Parent(); parent != nil; parent = parent.Parent() {
        if mdi, ok := parent.(*MDIPane); ok {
            return mdi
        }
    }
    return nil
}
```

## StatusBar Considerations

StatusBar could follow a similar pattern to MenuBar:

- **StatusBarSlot**: Display location
- **StatusBarProvider**: Interface for windows to provide status content
- **Focus-driven**: Active window's status content is displayed

## Recommendations

### Phase 1: MDIPane Foundation

1. Create `MDIPane` widget with basic functionality
2. Implement window management (add, remove, focus, z-order)
3. Implement input routing (mouse, keyboard)
4. Add lifecycle callbacks

### Phase 2: Desktop Refactoring

1. Refactor Desktop to use MDIPane internally
2. Ensure backward compatibility
3. Chrome widgets (Dock, etc.) look up MDIPane via ancestry

### Phase 3: Action System

1. Define Action type as first-class object
2. Actions can be used in menus, toolbars, shortcuts
3. Enable/disable based on context

### Phase 4: Context-Sensitive Menus

1. Implement MenuBarProvider interface
2. Implement focus-driven provider discovery
3. Consider responder chain for action handling

## Notes

- The responder chain pattern (Cocoa-style) may be more elegant than explicit menu swapping
- Start with simpler MDIPane work before tackling the full menu architecture
- Keep the API flexible enough to support different application patterns
