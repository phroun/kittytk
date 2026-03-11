# TUI Toolkit API Reference

Complete API for building tuitk applications.

## Package Structure

```
tuitk/
  app/       - Application lifecycle
  backend/   - Terminal rendering
  core/      - Fundamental types, widget interface, events
  layout/    - Layout managers
  style/     - Colors, themes, schemes
  widgets/   - UI components
  window/    - Window management
```

## Quick Start

```go
package main

import (
    "github.com/phroun/tuitk/app"
    "github.com/phroun/tuitk/backend"
    "github.com/phroun/tuitk/widgets"
    "github.com/phroun/tuitk/window"
)

func main() {
    desktop := widgets.NewDesktop()
    desktop.SetBackend(backend.NewTUIBackend(backend.DefaultTUIOptions()))

    application := app.New(nil)
    application.SetName("My App")
    desktop.AddApplication(application)

    desktop.SetOnStartup(func() {
        w := window.NewWindow("Hello")
        w.SetContent(widgets.NewLabel("Hello, World!"))
        application.AddWindow(w)
    })

    desktop.Run()
}
```

---

## Core Types

### Geometry (core/types.go)

```go
// Cell-based coordinates
type Point struct { X, Y int }
type Size struct { Width, Height int }
type Rect struct { X, Y, Width, Height int }
type Margins struct { Top, Right, Bottom, Left int }

// Abstract units (resolution-independent)
type Unit int
type UnitPoint struct { X, Y Unit }
type UnitSize struct { Width, Height Unit }
type UnitRect struct { X, Y, Width, Height Unit }
type UnitMargins struct { Top, Right, Bottom, Left Unit }
```

### CellMetrics

Converts between abstract units and character cells.

```go
metrics := core.DefaultCellMetrics()  // 8x16 units per cell
col, row := metrics.UnitsToCell(unitX, unitY)
```

### Enums

```go
// Alignment
AlignFill, AlignLeft, AlignCenter, AlignRight
AlignTop, AlignMiddle, AlignBottom

// Orientation
Horizontal, Vertical

// SizePolicy
SizeFixed, SizeMinimum, SizeMaximum, SizePreferred, SizeExpanding, SizeIgnored

// FocusPolicy
NoFocus, TabFocus, ClickFocus, StrongFocus, WheelFocus

// WindowState
WindowNormal, WindowStateMaximized, WindowStateMinimized

// MouseButton
NoButton, LeftButton, MiddleButton, RightButton, ScrollUp, ScrollDown
```

---

## Widget Interface

### core.Widget

Base interface for all UI elements.

```go
type Widget interface {
    // Identity
    Name() string
    SetName(string)
    Parent() Widget
    SetParent(Widget)

    // Geometry
    Bounds() UnitRect
    SetBounds(UnitRect)
    Size() UnitSize
    SetSize(UnitSize)
    MinimumSize() UnitSize
    SetMinimumSize(UnitSize)
    MaximumSize() UnitSize
    SetMaximumSize(UnitSize)
    SizeHint() UnitSize
    SizePolicy() SizePolicyPair
    SetSizePolicy(SizePolicyPair)

    // State
    IsVisible() bool
    SetVisible(bool)
    Show()
    Hide()
    IsEnabled() bool
    SetEnabled(bool)

    // Focus
    FocusPolicy() FocusPolicy
    SetFocusPolicy(FocusPolicy)
    HasFocus() bool
    SetFocus()
    ClearFocus()

    // Styling
    Style() *style.CellStyle
    SetStyle(*style.CellStyle)
    Scheme() style.SchemeID
    SetScheme(style.SchemeID)

    // Rendering
    Paint(p *Painter)
    Update()
    NeedsRepaint() bool

    // Events (return true to consume)
    HandleKeyPress(KeyPressEvent) bool
    HandleMousePress(MousePressEvent) bool
    HandleMouseRelease(MouseReleaseEvent) bool
    HandleMouseMove(MouseMoveEvent) bool
    HandleMouseWheel(MouseWheelEvent) bool
    HandleFocusIn(FocusEvent) bool
    HandleFocusOut(FocusEvent) bool
}
```

### core.Container

Extends Widget to hold children.

```go
type Container interface {
    Widget
    Children() []Widget
    AddChild(Widget)
    RemoveChild(Widget)
    ChildAt(UnitPoint) Widget
    LayoutManager() LayoutManager
    SetLayoutManager(LayoutManager)
}
```

### WidgetBase

Embed in custom widgets:

```go
type MyWidget struct {
    core.WidgetBase
    // custom fields
}

func NewMyWidget() *MyWidget {
    w := &MyWidget{}
    w.WidgetBase = *core.NewWidgetBase()
    w.Init(w)  // Pass outer reference
    return w
}
```

Key WidgetBase methods:
```go
BackgroundColor() *style.Color
SetBackgroundColor(*style.Color)
Font() *Font
SetFont(*Font)
EffectiveFont() *Font
```

---

## Events

```go
type KeyPressEvent struct {
    Key       string      // "a", "Enter", "^C", "Alt+F"
    Modifiers KeyModifiers
    Text      string      // Printable character
}

type MousePressEvent struct {
    X, Y   Unit
    Button MouseButton
}

type MouseMoveEvent struct {
    X, Y Unit
}

type MouseWheelEvent struct {
    X, Y      Unit
    Direction int  // positive=up, negative=down
}
```

---

## Style System

### Colors (style/style.go)

```go
// Standard 16 colors
ColorBlack, ColorRed, ColorGreen, ColorYellow
ColorBlue, ColorMagenta, ColorCyan, ColorWhite
ColorBrightBlack ... ColorBrightWhite
ColorDefault  // Terminal default

// 256-color palette
style.Color256(index)  // 0-255

// True color (24-bit)
style.RGB(r, g, b)  // 0-255 each
```

### CellStyle

```go
s := style.DefaultStyle()
s = s.WithFg(style.ColorRed)
s = s.WithBg(style.ColorBlack)
s = s.Bold()
s = s.Underline()
s = s.Reverse()
```

### TextStyle Attributes

```go
StyleNormal, StyleBold, StyleDim, StyleItalic
StyleUnderline, StyleBlink, StyleReverse
StyleStrikethrough, StyleOverline
```

### BorderStyle

```go
BorderNone, BorderSingle, BorderDouble
BorderRounded, BorderHeavy, BorderASCII
```

### Theme

Complete color definitions for all UI elements.

```go
theme := style.DefaultTheme()  // or DarkTheme(), ClassicTheme()
```

### Scheme

Color variants within a theme (default, modal, etc.)

```go
widget.SetScheme(style.SchemeDefault)
widget.SetScheme(style.SchemeModal)
```

---

## Fonts

```go
type Font struct {
    Name       string     // "Monday", "Tuesday"
    Style      FontStyle
    Size       int
    Foreground FontColor
    Background FontColor
}

// Predefined
core.FontMonday12   // Standard fixed-width (8 units/char)
core.FontTuesday12  // Double-width (16 units/char)

widget.SetFont(core.FontTuesday12)
```

---

## Widgets

### Label

```go
label := widgets.NewLabel("Hello")
label.SetText("Updated")
label.SetAlignment(core.AlignCenter)
label.SetWordWrap(true)
```

### Button

```go
btn := widgets.NewButton("Click Me")
btn.SetOnClick(func() { /* handler */ })

// Icon button
iconBtn := widgets.NewIconButton(&style.Icon{...})

// Checkable button
btn.SetCheckable(true)
btn.SetOnToggled(func(checked bool) { })
```

### Checkbox

```go
cb := widgets.NewCheckbox("Enable feature")
cb.SetChecked(true)
cb.SetOnToggled(func(checked bool) { })

// Tri-state
cb.SetTriState(true)
cb.SetCheckState(widgets.StatePartial)
```

### RadioButton

```go
group := widgets.NewRadioGroup()
r1 := widgets.NewRadioButton("Option A")
r2 := widgets.NewRadioButton("Option B")
group.AddButton(r1)
group.AddButton(r2)
r1.SetOnToggled(func(checked bool) { })
```

### TextInput

```go
input := widgets.NewTextInput()
input.SetPlaceholder("Enter name...")
input.SetText("default")
input.SetMaxLength(50)
input.SetEchoMode(widgets.EchoPassword)
input.SetReadOnly(true)
input.SetOnTextChanged(func(text string) { })
input.SetOnReturnPressed(func() { })
```

### ComboBox

```go
combo := widgets.NewComboBox()
combo.AddItem("First")
combo.AddItem("Second")
combo.SetCurrentIndex(0)
combo.SetEditable(true)
combo.SetOnCurrentChanged(func(index int) { })
```

### ListView

```go
list := widgets.NewListView()
list.AddItem(widgets.NewListItem("Item 1"))
list.AddTextItem("Item 2")
list.SetSelectionMode(widgets.MultiSelection)
list.SetOnItemActivated(func(index int) { })
list.SetOnSelectionChanged(func() { })

// Access items
item := list.Item(0)
selected := list.SelectedIndices()
```

### TreeView

```go
tree := widgets.NewTreeView()

root := widgets.NewTreeItem("Documents")
child := widgets.NewTreeItem("File.txt")
root.AddChild(child)
root.Expanded = true

tree.AddRootItem(root)
tree.SetOnItemActivated(func(item *TreeItem) { })
```

### TabWidget

```go
tabs := widgets.NewTabWidget()
tabs.AddTab("General", generalPanel)
tabs.AddTab("Advanced", advancedPanel)
tabs.SetCurrentIndex(0)
tabs.SetTabPosition(widgets.TabsTop)  // TabsBottom, TabsLeft, TabsRight
tabs.SetOnCurrentChanged(func(index int) { })
```

### ScrollArea

```go
scroll := widgets.NewScrollArea()
scroll.SetContent(largePanel)
scroll.SetVerticalScrollBarPolicy(widgets.ScrollBarAsNeeded)
scroll.SetHorizontalScrollBarPolicy(widgets.ScrollBarAlwaysOff)
```

### Panel

```go
panel := widgets.NewPanel()
panel.AddChild(label)
panel.AddChild(button)
panel.SetLayoutManager(layout.NewBoxLayout(core.Vertical))
panel.SetTitle("Settings")
panel.SetBorder(style.BorderSingle)
```

### ProgressBar

```go
progress := widgets.NewProgressBar()
progress.SetMaximum(100)
progress.SetValue(50)
progress.SetIndeterminate(true)  // Animated unknown progress
```

### Splitters

```go
// Vertical splitter (top/bottom)
vsplit := widgets.NewVSplitter()
vsplit.SetFirst(topPanel)
vsplit.SetSecond(bottomPanel)
vsplit.SetPosition(0.3)  // 30% top

// Horizontal splitter (left/right)
hsplit := widgets.NewHSplitter()
hsplit.SetFirst(leftPanel)
hsplit.SetSecond(rightPanel)
```

### Separator

```go
sep := widgets.NewLineSeparator(core.Horizontal)
sep.SetTitle("Section")  // Optional divider title
```

### Spacer

```go
spacer := widgets.NewSpacer()         // Expanding
fixed := widgets.NewFixedSpacer(16)   // Fixed size
```

### Menu System

```go
menu := widgets.NewMenu("&File")  // & marks accelerator

item := widgets.NewMenuItem("&Open...")
item.SetShortcut(core.NewShortcut("^O"))
item.SetOnTriggered(func() { })

checkItem := widgets.NewMenuItem("Show Toolbar")
checkItem.SetCheckable(true)
checkItem.SetChecked(true)

menu.AddItem(item)
menu.AddSeparator()
menu.AddItem(checkItem)
```

### Dialog / MessageBox

```go
dialog := widgets.NewMessageBox(
    "Confirm",
    "Are you sure?",
    widgets.ButtonYes | widgets.ButtonNo,
)
dialog.SetIcon(widgets.IconQuestion)
dialog.SetOnFinished(func(result widgets.DialogResult) {
    if result == widgets.ResultYes { /* ... */ }
})
application.AddWindow(&dialog.Window)
```

### PurfecTerm

Terminal emulator widget.

```go
term := widgets.NewPurfecTerm()
term.Start()  // Run default shell

// Or run specific command
term.StartCommand("vim", "file.txt")

// Debug callback
term.SetOnCellClicked(func(info widgets.CellDebugInfo) {
    // info.Col, info.Row, info.Char
    // info.FgType, info.BgType ("RGB", "256", "Std", "Def")
    // info.Bold, info.Underline, info.Reverse
})
```

---

## Layout System

### BoxLayout

Linear arrangement (horizontal or vertical).

```go
box := layout.NewBoxLayout(core.Vertical)
box.SetSpacing(8)
box.SetContentsMargins(core.UnitMargins{Top: 4, Right: 4, Bottom: 4, Left: 4})

panel.SetLayoutManager(box)
panel.AddChild(label)
panel.AddChild(button)
```

Shortcuts:
```go
layout.NewHBoxLayout()  // Horizontal
layout.NewVBoxLayout()  // Vertical
```

### FlexLayout

CSS Flexbox-style layout.

```go
flex := layout.NewFlexLayout()
flex.SetDirection(layout.FlexRow)  // FlexColumn, FlexRowReverse, FlexColumnReverse
flex.SetWrap(layout.FlexWrapNormal)
flex.SetJustifyContent(layout.FlexJustifySpaceBetween)
flex.SetAlignItems(layout.FlexAlignCenter)
```

### GridLayout

2D grid arrangement.

```go
grid := layout.NewGridLayout(3, 2)  // 3 columns, 2 rows
grid.SetColumnStretch(0, 1)         // Column 0 stretches
grid.SetRowStretch(1, 2)            // Row 1 stretches more
```

### LayoutManager Interface

```go
type LayoutManager interface {
    Layout(container Container, bounds UnitRect)
    SizeHint(container Container) UnitSize
    MinimumSize(container Container) UnitSize
    Spacing() int
    SetSpacing(int)
    ContentsMargins() UnitMargins
    SetContentsMargins(UnitMargins)
}
```

---

## Window System

### Window

```go
w := window.NewWindow("My Window")
w.SetSize(core.UnitSize{Width: 480, Height: 320})  // 60x20 cells
w.SetContent(panel)

// State
w.Maximize()
w.Minimize()
w.Restore()
w.Close()

// Flags
w.SetFlags(core.WindowNoResize | core.WindowModal)

// Position
w.SetBounds(core.UnitRect{X: 80, Y: 32, Width: 480, Height: 320})
w.Move(core.UnitPoint{X: 100, Y: 50})
```

### WindowManager

```go
wm := desktop.WindowManager()
wm.AddWindow(w)
wm.SetActiveWindow(w)
wm.TileWindows()
wm.CascadeWindows()

// Callbacks
wm.SetOnWindowAdded(func(w *window.Window) { })
wm.SetOnActiveWindowChanged(func(w *window.Window) { })
```

### MDIPane

Multi-document interface container (embeddable widget).

```go
mdi := widgets.NewMDIPane()
mdi.AddWindow(childWindow)
mdi.SetActiveWindow(childWindow)
mdi.TileWindows()
mdi.CascadeWindows()
mdi.NextWindow()
mdi.PrevWindow()

// Callbacks
mdi.SetOnWindowMinimized(func(w *window.Window) { })
mdi.SetOnWindowRestored(func(w *window.Window) { })
```

---

## Desktop

Root widget managing windows, menus, and status bar.

```go
desktop := widgets.NewDesktop()
desktop.SetBackend(backend.NewTUIBackend(backend.DefaultTUIOptions()))

// Applications
desktop.AddApplication(app)

// Startup
desktop.SetOnStartup(func() {
    // Create initial windows here
})

// Run event loop
desktop.Run()

// Components
menuBar := desktop.MenuBar()
statusBar := desktop.StatusBar()
dockRow := desktop.DockRow()
```

### StatusBar

```go
statusBar := desktop.StatusBar()
statusBar.SetText("Ready")

// Styled sections
statusBar.SetSections([]widgets.StatusSection{
    {Text: "Mode: Normal", Width: 120},
    {Text: "Line 42", Width: -1},  // -1 = stretch
})

// Styled text
statusBar.SetStyledText([]widgets.StatusTextSpan{
    {Text: "Ready - Press "},
    {Text: "F10", Style: &highlightStyle},
    {Text: " for menu"},
})
```

### DockRow

Minimized window dock.

```go
dock := desktop.DockRow()
dock.AddEntry(&widgets.DockEntry{
    Title: "Document 1",
    OnClick: func() { mdi.RestoreWindow(win) },
})
```

---

## Application

### app.Application

```go
application := app.New(nil)  // nil backend when Desktop owns it
application.SetName("My App")

// Windows
application.AddWindow(w)
windows := application.Windows()

// Menu/Status content (for multi-app desktops)
application.SetMenuBarContent([]*widgets.Menu{fileMenu, editMenu})
application.SetStatusBarContent([]widgets.StatusSection{{Text: "Ready"}})

// Lifecycle callbacks
application.SetOnActivate(func() { /* app gained focus */ })
application.SetOnDeactivate(func() { /* app lost focus */ })
```

### Secondary Applications

For multi-application desktops:

```go
secondary := app.New(nil)
secondary.SetName("Secondary App")
// Set up menus, status, windows...
desktop.AddApplication(secondary)
```

---

## Focus Management

```go
// Widget focus
widget.SetFocusPolicy(core.StrongFocus)
widget.SetFocus()
widget.ClearFocus()

// Focus manager
fm := window.FocusManager()
fm.NextWidget()      // Tab forward
fm.PreviousWidget()  // Tab backward
fm.SetWrapAround(true)

// Callback
fm.SetOnFocusChanged(func(old, new Widget) { })
```

---

## Accessibility

```go
// On widgets
widget.SetAccessibleRole(core.RoleButton)
widget.SetAccessibleName("Submit Form")
widget.SetAccessibleDescription("Click to submit the form")

// Announcements
am := desktop.AccessibilityManager()
am.AnnouncePolite("Document saved")
am.AnnounceAssertive("Error: Connection lost")
```

### Accessible Roles

```go
RoleButton, RoleLabel, RoleTextInput, RoleCheckBox
RoleRadioButton, RoleComboBox, RoleList, RoleListItem
RoleTree, RoleTreeItem, RoleTab, RoleTabPanel
RoleMenu, RoleMenuItem, RoleMenuBar, RoleToolBar
RoleScrollBar, RoleProgressBar, RoleSlider
RoleDialog, RoleAlert, RoleWindow, RoleTerminal
```

---

## Actions and Shortcuts

```go
action := core.NewAction("save", "Save")
action.SetShortcut(core.NewShortcut("^S"))
action.SetOnTriggered(func() { saveDocument() })

// Shortcut formats
"^S"        // Ctrl+S
"^+S"       // Ctrl+Shift+S
"Alt+F4"    // Alt+F4
"F1"        // Function key
```

---

## Backend

### TUI Backend

```go
opts := backend.DefaultTUIOptions()
opts.MouseEnabled = true
opts.AltScreen = true

tui := backend.NewTUIBackend(opts)
desktop.SetBackend(tui)
```

### RenderBackend Interface

For custom backends:

```go
type RenderBackend interface {
    Init() error
    Shutdown()
    Size() (width, height int)
    BeginFrame()
    EndFrame()
    Clear()
    DrawCell(x, y int, ch rune, style style.CellStyle)
    DrawText(x, y int, text string, style style.CellStyle, font *Font)
    FillRect(r Rect, ch rune, style style.CellStyle)
    SetClip(r Rect)
    PollEvent() Event
    SetCursorPosition(x, y int)
    SetCursorVisible(bool)
}
```

---

## Painter

High-level drawing API used in Paint() methods.

```go
func (w *MyWidget) Paint(p *core.Painter) {
    bounds := w.Bounds()
    s := style.DefaultStyle().WithFg(style.ColorWhite)

    p.FillRect(bounds, ' ', s)
    p.DrawText(bounds.X, bounds.Y, "Hello", s, nil)
    p.DrawBox(bounds, style.BorderSingle, "Title", s)
    p.DrawHLine(bounds.X, bounds.Y+16, 80, '-', s)
}
```

Key methods:
```go
DrawCell(x, y Unit, ch rune, style CellStyle)
DrawText(x, y Unit, text string, style CellStyle, font *Font)
DrawTextAligned(bounds UnitRect, text string, hAlign, vAlign Alignment, style CellStyle, font *Font)
FillRect(bounds UnitRect, ch rune, style CellStyle)
DrawRect(bounds UnitRect, border BorderStyle, style CellStyle)
DrawBox(bounds UnitRect, border BorderStyle, title string, style CellStyle)
DrawHLine(x, y, width Unit, ch rune, style CellStyle)
DrawVLine(x, y, height Unit, ch rune, style CellStyle)
SetClip(bounds UnitRect)
PushTransform(Transform)
PopTransform()
```

---

## Common Patterns

### Creating Custom Widgets

```go
type MyWidget struct {
    core.WidgetBase
    value int
    onChange func(int)
}

func NewMyWidget() *MyWidget {
    w := &MyWidget{}
    w.WidgetBase = *core.NewWidgetBase()
    w.Init(w)
    w.SetFocusPolicy(core.StrongFocus)
    return w
}

func (w *MyWidget) SizeHint() core.UnitSize {
    return core.UnitSize{Width: 80, Height: 16}
}

func (w *MyWidget) Paint(p *core.Painter) {
    // Custom rendering
}

func (w *MyWidget) HandleKeyPress(e core.KeyPressEvent) bool {
    if e.Key == "Enter" {
        w.value++
        if w.onChange != nil {
            w.onChange(w.value)
        }
        w.Update()
        return true
    }
    return false
}
```

### Event Filters

Process events before widgets:

```go
desktop.AddEventFilter(func(event core.Event) bool {
    if ke, ok := event.(core.KeyPressEvent); ok {
        if ke.Key == "F12" {
            showDebugPanel()
            return true  // Consume event
        }
    }
    return false  // Let event propagate
})
```

### Popup Overlays

```go
popup := &core.PopupRequest{
    ID:     "my-popup",
    Bounds: core.UnitRect{X: 100, Y: 50, Width: 200, Height: 100},
    Paint: func(p *core.Painter) {
        // Render popup content
    },
}
popupController.RegisterPopup(popup)
// Later: popupController.UnregisterPopup("my-popup")
```

---

## Unit System

Standard cell metrics: 8 units wide, 16 units tall per character cell.

```go
metrics := core.DefaultCellMetrics()

// Window sized for 60x20 cells
w.SetSize(core.UnitSize{
    Width:  60 * metrics.CellWidth,   // 480 units
    Height: 20 * metrics.CellHeight,  // 320 units
})

// Convert units to cells
col, row := metrics.UnitsToCell(x, y)

// Convert cells to units
x, y := metrics.CellsToUnits(col, row)
```
