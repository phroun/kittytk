// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/platform"
	"github.com/phroun/tuitk/style"
	"github.com/phroun/tuitk/window"
)

// ApplicationProvider is the interface that applications must implement
// to integrate with Desktop. This allows multiple applications to run
// in the same desktop environment.
type ApplicationProvider interface {
	// Name returns the application name.
	Name() string

	// Windows returns all windows owned by this application.
	Windows() []*window.Window

	// AddWindow adds a window to this application.
	AddWindow(w *window.Window)

	// RemoveWindow removes a window from this application.
	RemoveWindow(w *window.Window)

	// MenuBarContent returns the menu bar content for this application.
	// Called when the application becomes active.
	MenuBarContent() []*Menu

	// StatusBarContent returns the status bar content for this application.
	StatusBarContent() []StatusSection

	// OnActivate is called when this application becomes the active one.
	OnActivate()

	// OnDeactivate is called when this application is no longer active.
	OnDeactivate()

	// SetDesktop sets the desktop that owns this application.
	// Called by Desktop.AddApplication().
	SetDesktop(desktop core.Widget)

	// PassNextKeyToWidget returns whether pass-next-key mode is active for this app.
	PassNextKeyToWidget() bool

	// ActivatePassNextKeyToWidget activates pass-next-key mode for this app.
	ActivatePassNextKeyToWidget()

	// ClearPassNextKeyToWidget clears pass-next-key mode for this app.
	ClearPassNextKeyToWidget()
}

// Desktop represents the application desktop (background behind windows).
// It can optionally display a menu bar at the top (Mac-style) and a
// status bar at the bottom. Desktop can serve as the top-level object
// managing multiple applications.
type Desktop struct {
	core.WidgetBase

	// graphicalFrames reports whether the backend paints rounded
	// window frames (core.RoundedRectDrawer); windows discover it via
	// core.FindGraphicalFrames to pick their client-area contract.
	graphicalFrames bool

	// resizeGrip is the graphical resize-handle thickness in units
	// (0 on cell frames, where the whole border cell is the grip).
	resizeGrip core.Unit

	// Graphical wallpaper (classic MacOS style): an 8x8 two-color
	// bitmap, each bit rendered as wallpaperChunkPx x wallpaperChunkPx
	// device pixels. Tune via SetWallpaperPattern/SetWallpaperChunk.
	wallpaperPattern [8]uint8
	wallpaperChunkPx int

	// Menu bar at the top (Mac-style)
	menuBar *MenuBar

	// System menu (always present, upper-left)
	systemMenu *Menu

	// Status bar at the bottom
	statusBar *StatusBar

	// Dock row for minimized windows (above status bar)
	dockRow *DockRow

	// Background pattern
	bgChar rune

	// Whether child windows include a virtual "blur" focus item that allows
	// keyboard users to exit the window and focus the menu bar. Default: true.
	keyboardBlurChildren bool

	// Content area (shown behind windows but below menu/status)
	content core.Widget

	// Multi-application support
	mu           sync.RWMutex
	applications []ApplicationProvider
	activeApp    ApplicationProvider

	// Backend for rendering (optional - used when Desktop.Run() is called)
	backend core.RenderBackend

	// Window manager (optional - used when Desktop.Run() is called)
	windowManager *window.WindowManager

	// Focus manager
	focusManager *core.GlobalFocusManager

	// Accessibility manager
	accessibilityManager *core.AccessibilityManager

	// Theme
	theme *style.Theme

	// Font (default font for all windows/widgets)
	font *core.Font

	// Inverted-loop residence (G3/D21): the platform whose main
	// thread runs us, and our one surface on it.
	platform platform.Platform
	surface  platform.Surface

	// Live tear-off drag driven from the desktop's event stream (the
	// desktop window owns the capture until the button is released).
	tornDrag *tornDrag

	// Every window currently torn off into its own surface: the
	// repaint tick drives their animation alongside the desktop's.
	tornHosts []*window.TearOffHost

	// Running state
	running atomic.Bool

	// Quit channel
	quitChan chan struct{}

	// Update request channel
	updateChan chan struct{}

	// Timer events
	timers     []*DesktopTimer
	timerMutex sync.Mutex

	// Callbacks
	onStartup  func()
	onShutdown func()

	// Event filters
	eventFilters []func(core.Event) bool

	// Command registry for desktop-level (system menu) commands,
	// keyed by stable command ID - the D2 dispatch seam.
	commands *core.CommandRegistry

	// Exit code
	exitCode int
}

// DesktopTimer represents a scheduled timer callback.
type DesktopTimer struct {
	ID       int
	Interval time.Duration
	Repeat   bool
	Callback func()
	nextFire time.Time
	stopped  bool
}

// Stop stops the timer.
func (t *DesktopTimer) Stop() {
	if t != nil {
		t.stopped = true
	}
}

// NewDesktop creates a new desktop widget.
func NewDesktop() *Desktop {
	d := &Desktop{
		bgChar:               '▓', // Default pattern (three-quarter shade block)
		dockRow:              NewDockRow(),
		quitChan:             make(chan struct{}),
		updateChan:           make(chan struct{}, 100),
		theme:                style.DefaultTheme(),
		keyboardBlurChildren: true, // Default to enabling keyboard blur
		// Classic MacOS-style wallpaper: 50% checkerboard dither,
		// each pattern bit 2x2 device pixels.
		wallpaperPattern: [8]uint8{0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55},
		wallpaperChunkPx: 2,
	}
	d.WidgetBase = *core.NewWidgetBase()
	d.Init(d)
	d.SetFocusPolicy(core.NoFocus)
	d.dockRow.SetParent(d)

	// Desktop-level command registry (system menu dispatch)
	d.commands = core.NewCommandRegistry()

	// Create system menu and bind it to the command registry
	d.systemMenu = d.createSystemMenu()
	d.systemMenu.BindCommands(d.commands)

	// Create menu bar (always present in Desktop)
	d.menuBar = NewMenuBar()
	d.menuBar.SetParent(d)
	d.menuBar.AddMenu(d.systemMenu)

	// Create status bar (always present in Desktop)
	d.statusBar = NewStatusBar()
	d.statusBar.SetParent(d)

	// Wire up Tab navigation between dock and menu bar
	d.dockRow.SetOnFocusMenuBar(func() {
		d.UnfocusDock()
		d.menuBar.HandleKeyPress(core.KeyPressEvent{Key: "F10"})
	})
	d.menuBar.SetOnFocusDock(func() {
		if !d.dockRow.IsEmpty() {
			d.FocusDock()
		}
	})

	return d
}

// createSystemMenu creates the always-present system menu (ψ).
func (d *Desktop) createSystemMenu() *Menu {
	menu := NewMenu("Ψ")
	menu.AddItem(NewMenuItem("&About Desktop").SetOnTriggered(func() {
		// Show about dialog
		// TODO: Implement about dialog
	}))
	menu.AddItem(NewSeparator())
	menu.AddItem(NewMenuItem("Desktop &Accessories").SetEnabled(false)) // Placeholder
	menu.AddItem(NewSeparator())

	// Exit Desktop - uses ActionExitDesktop keybinding
	exitItem := NewMenuItem("E&xit Desktop")
	if keys := core.DefaultKeyBindings.Keys(core.ActionExitDesktop); len(keys) > 0 {
		exitItem.SetShortcut(core.NewShortcut(keys[0]))
	}
	exitItem.SetOnTriggered(func() {
		d.Quit()
	})
	menu.AddItem(exitItem)

	return menu
}

// SetBackend sets the render backend and initializes related components.
func (d *Desktop) SetBackend(backend core.RenderBackend) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.backend = backend

	// The desktop roots the grid-metrics inheritance chain: seed its
	// override from the backend so every widget inherits the display
	// service's default unless a container overrides it.
	rootMetrics := backend.Metrics()
	d.WidgetBase.SetCellMetrics(&rootMetrics)

	// Measurement comes from the render target (G1): a pixel backend
	// answers from its shaping engine so measurement matches the
	// proportional render; the text-based system keeps the built-in
	// cell arithmetic (nil restores it).
	if tm, ok := backend.(core.TextMeasurer); ok {
		core.SetTextMeasurer(tm)
	} else {
		core.SetTextMeasurer(nil)
	}

	// Frame mode: when the backend paints rounded window frames, the
	// client-area contract changes (content extends to the window
	// edges; only the titlebar reserves a full row). Windows discover
	// this through the desktop via core.FindGraphicalFrames.
	_, d.graphicalFrames = backend.(core.RoundedRectDrawer)

	// Resize grip: on graphical frames only the outer sliver of a
	// window edge resizes - a quarter of a layout column, scaled by
	// the device scale so the physical grab target grows with the
	// zoom, and never thinner than 4 device pixels - so edge widgets
	// stay clickable.
	d.resizeGrip = 0
	if d.graphicalFrames {
		scale := 1
		if ds, ok := backend.(core.DeviceScaler); ok && ds.Scale() > 0 {
			scale = ds.Scale()
		}
		grip := rootMetrics.CellWidth / 4 * core.Unit(scale)
		if minUnits := core.Unit((4 + scale - 1) / scale); grip < minUnits {
			grip = minUnits
		}
		d.resizeGrip = grip
	}

	d.windowManager = window.NewWindowManager()
	d.windowManager.SetResizeGrip(d.resizeGrip)
	if sp, ok := backend.(core.SmoothPositioner); ok && sp.SmoothPositioning() {
		// Pixel surfaces place windows at unit granularity; cell-grid
		// surfaces keep the default snap-to-cell behavior.
		d.windowManager.SetSmoothPositioning(true)
	}
	d.windowManager.SetOnRepaintNeeded(func() {
		d.RequestUpdate()
	})
	d.windowManager.SetOnActiveChanged(func(win *window.Window) {
		d.windowFocusChanged(win)
	})
	d.focusManager = core.NewGlobalFocusManager()
	d.accessibilityManager = core.NewAccessibilityManager()
	d.focusManager.SetAccessibilityManager(d.accessibilityManager)

	// Connect each window's FocusManager to the AccessibilityManager
	d.windowManager.SetOnWindowAdded(func(win *window.Window) {
		if fm := win.FocusManager(); fm != nil {
			fm.SetAccessibilityManager(d.accessibilityManager)
		}
	})

	d.windowManager.SetDesktop(d)

	// Wire up dock row integration
	if d.dockRow != nil {
		d.windowManager.SetOnWindowMinimized(func(win *window.Window) {
			entry := &DockEntry{
				Title:    win.Title(),
				WindowID: win.ObjectID(),
				OnClick: func() {
					d.windowManager.RestoreWindow(win)
				},
			}
			d.dockRow.AddEntry(entry)
		})

		d.windowManager.SetOnWindowRestored(func(win *window.Window) {
			d.dockRow.RemoveEntryByID(win.ObjectID())
		})
	}

	// Wire up menu bar integration
	if d.menuBar != nil {
		d.menuBar.SetOnMenuOpen(func() {
			d.windowManager.DeactivateActiveWindow()
		})
		d.menuBar.SetOnMenuDismiss(func() {
			d.windowManager.RestorePreviousActiveWindow()
		})
	}
}

// SetWallpaperPattern sets the graphical wallpaper's 8x8 two-color
// bitmap (row-major, bit 7 leftmost; set bits use the desktop fill
// foreground, clear bits its background).
func (d *Desktop) SetWallpaperPattern(pattern [8]uint8) {
	d.mu.Lock()
	d.wallpaperPattern = pattern
	d.mu.Unlock()
	d.RequestUpdate()
}

// SetWallpaperChunk sets how many device pixels each wallpaper
// pattern bit covers (the pattern's "pixel size"; minimum 1).
func (d *Desktop) SetWallpaperChunk(px int) {
	if px < 1 {
		px = 1
	}
	d.mu.Lock()
	d.wallpaperChunkPx = px
	d.mu.Unlock()
	d.RequestUpdate()
}

// GraphicalResizeGrip implements core.ResizeGripProvider: the
// resize-handle thickness for graphical frames (0 on cell frames).
func (d *Desktop) GraphicalResizeGrip() core.Unit {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.resizeGrip
}

// GraphicalWindowFrames implements core.GraphicalFrameProvider: true
// when the backend paints rounded window frames, which switches the
// window client-area contract to edge-to-edge below the titlebar.
func (d *Desktop) GraphicalWindowFrames() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.graphicalFrames
}

// Backend returns the render backend.
func (d *Desktop) Backend() core.RenderBackend {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.backend
}

// WindowManager returns the window manager.
func (d *Desktop) WindowManager() *window.WindowManager {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.windowManager
}

// FocusManager returns the focus manager.
func (d *Desktop) FocusManager() *core.GlobalFocusManager {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.focusManager
}

// AccessibilityManager returns the accessibility manager.
func (d *Desktop) AccessibilityManager() *core.AccessibilityManager {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.accessibilityManager
}

// Theme returns the current theme.
func (d *Desktop) Theme() *style.Theme {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.theme
}

// SetTheme sets the current theme.
func (d *Desktop) SetTheme(theme *style.Theme) {
	d.mu.Lock()
	d.theme = theme
	d.mu.Unlock()
	d.RequestUpdate()
}

// Font returns the desktop's default font, or nil if using the system default.
func (d *Desktop) Font() *core.Font {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.font
}

// SetFont sets the desktop's default font.
// This font is inherited by all windows and widgets unless overridden.
// Set to nil to use the system default (Monday 12pt).
func (d *Desktop) SetFont(font *core.Font) {
	d.mu.Lock()
	d.font = font
	apps := make([]ApplicationProvider, len(d.applications))
	copy(apps, d.applications)
	d.mu.Unlock()

	// Recalculate layout for all windows since font affects widget sizes
	for _, app := range apps {
		for _, w := range app.Windows() {
			w.Layout()
		}
	}
	d.RequestUpdate()
}

// EffectiveFont returns the font to use for the desktop.
// Returns the set font, or the system default if none is set.
func (d *Desktop) EffectiveFont() *core.Font {
	d.mu.RLock()
	f := d.font
	d.mu.RUnlock()
	if f != nil {
		return f
	}
	return core.DefaultFont()
}

// AddApplication registers an application with the desktop.
func (d *Desktop) AddApplication(app ApplicationProvider) {
	d.mu.Lock()
	d.applications = append(d.applications, app)

	// If this is the first app, make it active
	shouldActivate := d.activeApp == nil
	if shouldActivate {
		d.activeApp = app
	}
	wm := d.windowManager
	d.mu.Unlock()

	// Set this desktop as the application's desktop
	app.SetDesktop(d)

	// Add any existing windows from the app to the WindowManager.
	// This handles the case where windows were added to the app before
	// it was registered with the desktop.
	if wm != nil {
		for _, win := range app.Windows() {
			wm.AddWindow(win)
		}
	}

	if shouldActivate {
		app.OnActivate()
		d.updateMenuBarContent()
		d.updateStatusBarContent()
	}
}

// RemoveApplication unregisters an application from the desktop.
func (d *Desktop) RemoveApplication(app ApplicationProvider) {
	d.mu.Lock()

	for i, a := range d.applications {
		if a == app {
			d.applications = append(d.applications[:i], d.applications[i+1:]...)
			break
		}
	}

	// If this was the active app, switch to another or none
	wasActive := d.activeApp == app
	var newActiveApp ApplicationProvider
	if wasActive {
		if len(d.applications) > 0 {
			d.activeApp = d.applications[0]
			newActiveApp = d.activeApp
		} else {
			d.activeApp = nil
		}
	}
	d.mu.Unlock()

	if wasActive {
		app.OnDeactivate()
		if newActiveApp != nil {
			newActiveApp.OnActivate()
		}
		d.updateMenuBarContent()
		d.updateStatusBarContent()
	}
}

// SetApplication sets a single application (for backward compatibility).
func (d *Desktop) SetApplication(app ApplicationProvider) {
	d.mu.Lock()
	// Clear existing applications
	for _, a := range d.applications {
		a.OnDeactivate()
	}
	d.applications = nil
	d.activeApp = nil
	d.mu.Unlock()

	d.AddApplication(app)
}

// ActiveApplication returns the currently active application.
func (d *Desktop) ActiveApplication() ApplicationProvider {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.activeApp
}

// Applications returns all registered applications.
func (d *Desktop) Applications() []ApplicationProvider {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]ApplicationProvider, len(d.applications))
	copy(result, d.applications)
	return result
}

// findApplicationForWindow finds which application owns a window.
func (d *Desktop) findApplicationForWindow(w *window.Window) ApplicationProvider {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, app := range d.applications {
		for _, win := range app.Windows() {
			if win == w {
				return app
			}
		}
	}
	return nil
}

// windowFocusChanged is called when window focus changes.
func (d *Desktop) windowFocusChanged(w *window.Window) {
	// When w is nil, it means the window was deactivated (e.g., menu bar took focus).
	// In this case, we should NOT change the active app or update menus - the user
	// is still interacting with the same app through its menu bar.
	if w == nil {
		return
	}

	owner := d.findApplicationForWindow(w)

	d.mu.Lock()
	if owner != d.activeApp {
		if d.activeApp != nil {
			d.activeApp.OnDeactivate()
		}
		d.activeApp = owner
		if d.activeApp != nil {
			d.activeApp.OnActivate()
		}
	}
	d.mu.Unlock()

	d.updateMenuBarContent()
	d.updateStatusBarContent()
}

// updateMenuBarContent updates the menu bar with the active app's menus.
// The first menu after the system menu automatically gets standard app items
// (Hide, Hide Others, Show All, Quit) appended.
func (d *Desktop) updateMenuBarContent() {
	if d.menuBar == nil {
		return
	}

	d.mu.RLock()
	activeApp := d.activeApp
	d.mu.RUnlock()

	// Clear existing menus
	d.menuBar.Clear()

	// Add system menu first
	if d.systemMenu != nil {
		d.menuBar.AddMenu(d.systemMenu)
	}

	// Add active app's menus with standard items injected into the first one
	if activeApp != nil {
		appMenus := activeApp.MenuBarContent()
		appName := activeApp.Name()

		if len(appMenus) > 0 {
			// Create a merged copy of the first menu with standard items
			firstMenu := appMenus[0]
			mergedMenu := d.createAppMenuWithStandardItems(firstMenu, appName)
			d.menuBar.AddMenu(mergedMenu)

			// Add remaining menus as-is
			for i := 1; i < len(appMenus); i++ {
				d.menuBar.AddMenu(appMenus[i])
			}
		} else {
			// App has no menus - create a standard app menu
			appMenu := d.createStandardAppMenu(appName)
			d.menuBar.AddMenu(appMenu)
		}
	}
}

// createAppMenuWithStandardItems creates a copy of the given menu with standard
// app items (Hide, Hide Others, Show All, Quit) appended.
func (d *Desktop) createAppMenuWithStandardItems(original *Menu, appName string) *Menu {
	// Create a new menu with the same title
	merged := NewMenu(original.Title())

	// Copy all items from the original menu
	for _, item := range original.Items() {
		merged.AddItem(item)
	}

	// Add standard app items
	d.appendStandardAppItems(merged, appName)

	return merged
}

// createStandardAppMenu creates a standard app menu when the app provides none.
// The menu is named after the app with the first letter as the accelerator.
func (d *Desktop) createStandardAppMenu(appName string) *Menu {
	// Create menu with app name, first letter as accelerator
	menuTitle := "&" + appName
	appMenu := NewMenu(menuTitle)

	// Add standard app items
	d.appendStandardAppItems(appMenu, appName)

	return appMenu
}

// appendStandardAppItems adds the standard app menu items to the given menu.
// Items added: separator, Hide [App], Hide Others, Show All, separator, Quit [App]
func (d *Desktop) appendStandardAppItems(menu *Menu, appName string) {
	menu.AddSeparator()

	// Hide [App Name]
	hideItem := NewMenuItem("&Hide " + appName)
	if keys := core.DefaultKeyBindings.Keys(core.ActionAppHide); len(keys) > 0 {
		hideItem.SetShortcut(core.NewShortcut(keys[0]))
	}
	hideItem.SetOnTriggered(func() {
		d.hideActiveApp()
	})
	menu.AddItem(hideItem)

	// Hide Others
	hideOthersItem := NewMenuItem("Hide &Others")
	if keys := core.DefaultKeyBindings.Keys(core.ActionAppHideOthers); len(keys) > 0 {
		hideOthersItem.SetShortcut(core.NewShortcut(keys[0]))
	}
	hideOthersItem.SetOnTriggered(func() {
		d.hideOtherApps()
	})
	menu.AddItem(hideOthersItem)

	// Show All
	showAllItem := NewMenuItem("&Show All")
	if keys := core.DefaultKeyBindings.Keys(core.ActionAppShowAll); len(keys) > 0 {
		showAllItem.SetShortcut(core.NewShortcut(keys[0]))
	}
	showAllItem.SetOnTriggered(func() {
		d.showAllApps()
	})
	menu.AddItem(showAllItem)

	menu.AddSeparator()

	// Quit [App Name] - quits only this application, not the entire desktop
	quitItem := NewMenuItem("&Quit " + appName)
	if keys := core.DefaultKeyBindings.Keys(core.ActionQuit); len(keys) > 0 {
		quitItem.SetShortcut(core.NewShortcut(keys[0]))
	}
	quitItem.SetOnTriggered(func() {
		d.quitActiveApp()
	})
	menu.AddItem(quitItem)
}

// hideActiveApp minimizes all windows of the active application.
func (d *Desktop) hideActiveApp() {
	d.mu.RLock()
	activeApp := d.activeApp
	d.mu.RUnlock()

	if activeApp == nil || d.windowManager == nil {
		return
	}

	for _, win := range activeApp.Windows() {
		if win != nil && win.IsVisible() && !win.IsMinimized() {
			d.windowManager.MinimizeWindow(win)
		}
	}
}

// hideOtherApps minimizes all windows of applications other than the active one.
func (d *Desktop) hideOtherApps() {
	d.mu.RLock()
	activeApp := d.activeApp
	apps := make([]ApplicationProvider, len(d.applications))
	copy(apps, d.applications)
	d.mu.RUnlock()

	if d.windowManager == nil {
		return
	}

	for _, app := range apps {
		if app != activeApp {
			for _, win := range app.Windows() {
				if win != nil && win.IsVisible() && !win.IsMinimized() {
					d.windowManager.MinimizeWindow(win)
				}
			}
		}
	}
}

// showAllApps restores all minimized windows of all applications.
func (d *Desktop) showAllApps() {
	d.mu.RLock()
	apps := make([]ApplicationProvider, len(d.applications))
	copy(apps, d.applications)
	d.mu.RUnlock()

	if d.windowManager == nil {
		return
	}

	for _, app := range apps {
		for _, win := range app.Windows() {
			if win != nil && win.IsMinimized() {
				d.windowManager.RestoreWindow(win)
			}
		}
	}
}

// quitActiveApp closes all windows of the active application and removes it.
// If this was the last application with windows, the desktop automatically exits.
func (d *Desktop) quitActiveApp() {
	d.mu.RLock()
	activeApp := d.activeApp
	d.mu.RUnlock()

	if activeApp == nil {
		return
	}

	// Close all windows of this application
	for _, win := range activeApp.Windows() {
		if win != nil {
			win.Close()
		}
	}

	// Remove the application from the desktop
	d.RemoveApplication(activeApp)

	// Check if there are any remaining windows across all applications
	d.mu.RLock()
	hasWindows := false
	for _, app := range d.applications {
		if len(app.Windows()) > 0 {
			hasWindows = true
			break
		}
	}
	d.mu.RUnlock()

	// If no windows remain, exit the desktop
	if !hasWindows {
		d.Quit()
	}
}

// updateStatusBarContent updates the status bar with the active app's content.
func (d *Desktop) updateStatusBarContent() {
	if d.statusBar == nil {
		return
	}

	d.mu.RLock()
	activeApp := d.activeApp
	d.mu.RUnlock()

	if activeApp != nil {
		d.statusBar.SetSections(activeApp.StatusBarContent())
	} else {
		d.statusBar.SetSections(nil)
	}
}

// RefreshStatusBar refreshes the status bar from the active app's content.
// This is called by applications when their status bar content changes.
func (d *Desktop) RefreshStatusBar() {
	d.updateStatusBarContent()
}

// SetOnStartup sets the startup callback.
func (d *Desktop) SetOnStartup(handler func()) {
	d.mu.Lock()
	d.onStartup = handler
	d.mu.Unlock()
}

// SetOnShutdown sets the shutdown callback.
func (d *Desktop) SetOnShutdown(handler func()) {
	d.mu.Lock()
	d.onShutdown = handler
	d.mu.Unlock()
}

// FocusedWidget returns the widget with keyboard focus in the active
// window, or nil. Window focus lives in each window's own focus
// manager (the desktop's GlobalFocusManager tracks scopes, not the
// per-window focused widget), so this reaches through the active
// window. Menu actions run after window focus is restored
// (Menu.onWillTrigger fires RestorePreviousActiveWindow before the
// action), so the active window is valid when a menu command asks.
func (d *Desktop) FocusedWidget() core.Widget {
	d.mu.RLock()
	wm := d.windowManager
	d.mu.RUnlock()
	if wm == nil {
		return nil
	}
	if aw := wm.ActiveWindow(); aw != nil {
		return aw.FocusManager().FocusedWidget()
	}
	return nil
}

// ActivatePassNextKeyToWidget activates pass-next-key-to-widget mode for the active app.
// The next keypress will bypass all global shortcut handling and go directly
// to the focused widget. This can be called from menu items or other UI elements.
func (d *Desktop) ActivatePassNextKeyToWidget() {
	d.mu.RLock()
	activeApp := d.activeApp
	d.mu.RUnlock()
	if activeApp != nil {
		activeApp.ActivatePassNextKeyToWidget()
	}
}

// AddEventFilter adds an event filter.
// Filters are called before normal event handling and can consume events.
// Return true to consume the event, false to let it propagate.
func (d *Desktop) AddEventFilter(filter func(core.Event) bool) {
	d.mu.Lock()
	d.eventFilters = append(d.eventFilters, filter)
	d.mu.Unlock()
}

// filterEvent runs the event through all filters.
// Returns true if the event was consumed.
func (d *Desktop) filterEvent(event core.Event) bool {
	d.mu.RLock()
	filters := d.eventFilters
	d.mu.RUnlock()

	for _, filter := range filters {
		if filter(event) {
			return true
		}
	}
	return false
}

// Run starts the desktop under the inverted loop (G3/D21): it wraps
// the polled backend in a one-surface Platform and hands control
// over. Kept as the compatible entry point; RunOn is the general
// form. Returns the exit code when the desktop quits.
func (d *Desktop) Run() int {
	d.mu.RLock()
	backend := d.backend
	d.mu.RUnlock()
	if backend == nil {
		return 1
	}
	return d.RunOn(platform.NewPolling(backend))
}

// RunOn runs the desktop as the content of one surface of the given
// platform. All dispatch, layout, and paint happen on the platform's
// main thread (D21); Platform.Post is the only cross-thread door.
func (d *Desktop) RunOn(p platform.Platform) int {
	d.mu.Lock()
	d.platform = p
	onStartup := d.onStartup
	wm := d.windowManager
	d.mu.Unlock()

	code := p.Run(func(pf platform.Platform) {
		surface, err := pf.CreateSurface(platform.SurfaceOptions{})
		if err != nil {
			pf.Quit(1)
			return
		}
		d.mu.Lock()
		d.surface = surface
		d.mu.Unlock()
		surface.SetHandler(&desktopSurfaceHandler{d: d})
		d.setupTearOff(pf, surface)

		size := surface.Size()
		wm.SetScreenBounds(core.UnitRect{Width: size.Width, Height: size.Height})

		d.running.Store(true)
		if onStartup != nil {
			onStartup()
		}
		surface.Invalidate(core.UnitRect{})
		d.scheduleTick(pf)
	})

	d.running.Store(false)

	d.mu.RLock()
	onShutdown := d.onShutdown
	d.mu.RUnlock()
	if onShutdown != nil {
		onShutdown()
	}
	return code
}

// scheduleTick keeps desktop timers firing and preserves the
// historical full-frame cadence while widgets migrate to precise
// invalidation. Self-reposting through the platform (D21: PostAfter
// is the timer primitive).
func (d *Desktop) scheduleTick(p platform.Platform) {
	p.PostAfter(50*time.Millisecond, func() {
		if !d.running.Load() {
			return
		}
		d.ProcessTimers()
		d.mu.RLock()
		s := d.surface
		torn := append([]*window.TearOffHost(nil), d.tornHosts...)
		d.mu.RUnlock()
		if s != nil {
			s.Invalidate(core.UnitRect{})
		}
		for _, h := range torn {
			h.Invalidate()
		}
		d.scheduleTick(p)
	})
}

// desktopSurfaceHandler adapts the Desktop to platform.SurfaceHandler.
type desktopSurfaceHandler struct {
	d *Desktop
}

// Frame paints the whole desktop (v1 full-frame contract).
func (h *desktopSurfaceHandler) Frame(painter *core.Painter) {
	d := h.d
	d.mu.RLock()
	wm := d.windowManager
	theme := d.theme
	s := d.surface
	d.mu.RUnlock()

	size := s.Size()
	painter.Clear(core.UnitRect{Width: size.Width, Height: size.Height}, theme.Normal)
	wm.Paint(painter)
}

// Resized reports the terminal/window size change.
func (h *desktopSurfaceHandler) Resized(size core.UnitSize) {
	d := h.d
	d.mu.RLock()
	wm := d.windowManager
	s := d.surface
	d.mu.RUnlock()
	wm.SetScreenBounds(core.UnitRect{Width: size.Width, Height: size.Height})
	if s != nil {
		s.Invalidate(core.UnitRect{})
	}
}

// Event dispatches one input event, then requests a frame (parity
// with the historical render-after-events loop).
func (h *desktopSurfaceHandler) Event(ev core.Event) bool {
	d := h.d
	handled := d.dispatchEvent(ev)
	d.mu.RLock()
	s := d.surface
	d.mu.RUnlock()
	if s != nil {
		s.Invalidate(core.UnitRect{})
	}
	return handled
}

// dispatchEvent routes one input event through the desktop: pass-key
// mode, event filters, shortcuts, focus, window manager. Runs on the
// platform thread (delivered by the surface handler).
func (d *Desktop) dispatchEvent(event core.Event) bool {
	d.mu.RLock()
	wm := d.windowManager
	fm := d.focusManager
	d.mu.RUnlock()

	// A live tear-off drag owns the pointer stream: the desktop keeps
	// the capture while the torn window follows the pointer.
	if d.handleTornDrag(event) {
		return true
	}

	// Check pass-next-key-to-widget mode FIRST, before any event filters.
	// This ensures the key goes directly to the widget without any interception.
	if keyEvent, isKey := event.(core.KeyPressEvent); isKey {
		d.mu.RLock()
		activeApp := d.activeApp
		d.mu.RUnlock()
		if activeApp != nil && activeApp.PassNextKeyToWidget() {
			activeApp.ClearPassNextKeyToWidget()
			// Skip ALL shortcut handling - send key directly to the active window's
			// focused widget, bypassing WindowManager's menu accelerator interception
			if wm != nil {
				if activeWin := wm.ActiveWindow(); activeWin != nil {
					activeWin.HandleKeyPress(keyEvent)
				}
			}
			return true
		}
	}

	// Run through event filters
	if d.filterEvent(event) {
		return true
	}

	// Handle event based on type
	switch e := event.(type) {
	case core.QuitEvent:
		d.QuitWithCode(0)
		return true

	case core.KeyPressEvent:
		// Pass-next-key mode is handled above, before event filters.
		// Check global shortcuts first
		if d.handleShortcut(e) {
			return true
		}
		// Try focus manager
		if fm != nil && fm.HandleKeyPress(e) {
			return true
		}
		// Pass to window manager
		return wm.HandleKeyPress(e)

	case core.FocusEvent:
		// The desktop's OS window gained or lost focus: its active
		// window's chrome follows, the same way a torn-off window's
		// chrome follows its own OS window. WM state is untouched -
		// re-focusing lights the same window back up.
		if aw := wm.ActiveWindow(); aw != nil {
			aw.SetActive(e.Focused)
		}
		return true

	case core.MousePressEvent:
		return wm.HandleMousePress(e)

	case core.MouseMoveEvent:
		core.WheelPointerMoved()
		return wm.HandleMouseMove(e)

	case core.MouseReleaseEvent:
		return wm.HandleMouseRelease(e)

	case core.MouseWheelEvent:
		// Stamp the screen position once; translations preserve it.
		e.ScreenX, e.ScreenY = e.X, e.Y
		// An active gesture stays latched to its claimant.
		if core.DeliverLatchedWheel(e) {
			return true
		}
		// Open menus scroll before anything beneath them.
		if d.menuBar != nil && d.menuBar.ActiveMenu() != nil {
			if d.menuBar.HandleMouseWheel(e) {
				return true
			}
		}
		return wm.HandleMouseWheel(e)
	}
	return false
}

// handleShortcut checks if a key event matches a global shortcut.
// This is called BEFORE the focus manager, so these shortcuts work even
// when an EditBox or other input widget has focus.
//
// All shortcuts are now handled through menu items - this method delegates
// to handleMenuShortcut which checks both the system menu and app menus.
func (d *Desktop) handleShortcut(event core.KeyPressEvent) bool {
	// Check global menu shortcuts (system menu + active app's menus)
	return d.handleMenuShortcut(event)
}

// handleMenuShortcut checks if a key event matches any menu item shortcut.
// This allows menu shortcuts to work globally even when menus are closed.
// Checks both the system menu and the active application's menus.
func (d *Desktop) handleMenuShortcut(event core.KeyPressEvent) bool {
	// Check system menu first (contains Exit Desktop, etc.)
	if d.systemMenu != nil {
		if d.checkMenuItemShortcuts(d.systemMenu, event) {
			return true
		}
	}

	d.mu.RLock()
	activeApp := d.activeApp
	d.mu.RUnlock()

	if activeApp == nil {
		return false
	}

	// Check all menus from the active app (includes standard items like Quit, Hide, etc.)
	for _, menu := range activeApp.MenuBarContent() {
		if d.checkMenuItemShortcuts(menu, event) {
			return true
		}
	}

	// Also check the merged menu (which includes standard app items added by Desktop)
	// This is needed because MenuBarContent() returns the app's original menus,
	// but appendStandardAppItems adds Quit, Hide, etc. to a merged copy
	if d.menuBar != nil {
		for _, menu := range d.menuBar.Menus() {
			// Skip system menu (already checked above)
			if menu == d.systemMenu {
				continue
			}
			if d.checkMenuItemShortcuts(menu, event) {
				return true
			}
		}
	}

	return false
}

// checkMenuItemShortcuts recursively checks menu items for matching shortcuts.
func (d *Desktop) checkMenuItemShortcuts(menu *Menu, event core.KeyPressEvent) bool {
	if menu == nil {
		return false
	}

	for _, item := range menu.Items() {
		if item == nil || item.Separator || !item.Enabled {
			continue
		}

		// Check if this item's shortcut matches. Trigger routes
		// through the command registry (dispatch by stable ID), and
		// keeps checkable-toggle semantics consistent with clicking.
		if item.Shortcut != "" && item.Shortcut.Matches(event) {
			item.Trigger()
			return true
		}

		// Recursively check submenus
		if item.SubMenu != nil {
			if d.checkMenuItemShortcuts(item.SubMenu, event) {
				return true
			}
		}
	}
	return false
}

// ProcessTimers checks and fires due timers.
// This is called by the Application's event loop to process desktop timers.
func (d *Desktop) ProcessTimers() {
	d.timerMutex.Lock()
	now := time.Now()
	var toFire []*DesktopTimer
	var remaining []*DesktopTimer

	for _, timer := range d.timers {
		if timer.stopped {
			continue
		}

		if now.After(timer.nextFire) || now.Equal(timer.nextFire) {
			toFire = append(toFire, timer)
			if timer.Repeat {
				timer.nextFire = now.Add(timer.Interval)
				remaining = append(remaining, timer)
			}
		} else {
			remaining = append(remaining, timer)
		}
	}

	d.timers = remaining
	d.timerMutex.Unlock()

	// Fire timers outside lock
	for _, timer := range toFire {
		if timer.Callback != nil {
			timer.Callback()
		}
	}
}

// Post schedules fn on the platform (UI) thread - the D21
// cross-thread door, exposed for embedders like the display-service
// transport whose socket readers must hand work to the UI. Before
// the desktop runs (no platform yet) fn runs inline; use only from
// wiring code in that window.
func (d *Desktop) Post(fn func()) {
	d.mu.RLock()
	p := d.platform
	d.mu.RUnlock()
	if p != nil {
		p.Post(fn)
		return
	}
	fn()
}

// RequestUpdate requests a screen update (damage-driven: invalidates
// the surface; the platform schedules the frame).
func (d *Desktop) RequestUpdate() {
	d.mu.RLock()
	s := d.surface
	d.mu.RUnlock()
	if s != nil {
		s.Invalidate(core.UnitRect{})
	}
	select {
	case d.updateChan <- struct{}{}:
	default:
		// Channel full, update already pending
	}
}

// Quit requests the desktop to quit.
func (d *Desktop) Quit() {
	d.QuitWithCode(0)
}

// QuitWithCode requests the desktop to quit with an exit code.
func (d *Desktop) QuitWithCode(code int) {
	d.mu.Lock()
	d.exitCode = code
	p := d.platform
	d.mu.Unlock()
	d.running.Store(false)
	if p != nil {
		p.Quit(code)
	}
	select {
	case <-d.quitChan:
		// Already closed
	default:
		close(d.quitChan)
	}
}

// IsRunning returns whether the desktop is running.
func (d *Desktop) IsRunning() bool {
	return d.running.Load()
}

// StartTimer starts a single-shot timer.
func (d *Desktop) StartTimer(interval time.Duration, callback func()) *DesktopTimer {
	return d.startTimerInternal(interval, false, callback)
}

// StartRepeatingTimer starts a repeating timer.
func (d *Desktop) StartRepeatingTimer(interval time.Duration, callback func()) *DesktopTimer {
	return d.startTimerInternal(interval, true, callback)
}

func (d *Desktop) startTimerInternal(interval time.Duration, repeat bool, callback func()) *DesktopTimer {
	d.timerMutex.Lock()
	defer d.timerMutex.Unlock()

	timer := &DesktopTimer{
		ID:       len(d.timers) + 1,
		Interval: interval,
		Repeat:   repeat,
		Callback: callback,
		nextFire: time.Now().Add(interval),
	}
	d.timers = append(d.timers, timer)
	return timer
}

// StopTimer stops a timer.
func (d *Desktop) StopTimer(timer *DesktopTimer) {
	if timer != nil {
		timer.stopped = true
	}
}

// ScreenSize returns the current screen size in units.
func (d *Desktop) ScreenSize() core.UnitSize {
	d.mu.RLock()
	backend := d.backend
	d.mu.RUnlock()

	if backend != nil {
		return backend.Size()
	}
	return core.UnitSize{}
}

// Clipboard returns the clipboard contents.
func (d *Desktop) Clipboard() string {
	d.mu.RLock()
	backend := d.backend
	d.mu.RUnlock()

	if backend != nil {
		return backend.GetClipboard()
	}
	return ""
}

// SetClipboard sets the clipboard contents.
func (d *Desktop) SetClipboard(text string) {
	d.mu.RLock()
	backend := d.backend
	d.mu.RUnlock()

	if backend != nil {
		backend.SetClipboard(text)
	}
}

// Beep produces an audible alert.
func (d *Desktop) Beep() {
	d.mu.RLock()
	backend := d.backend
	d.mu.RUnlock()

	if backend != nil {
		backend.Beep()
	}
}

// Children returns all child widgets.
func (d *Desktop) Children() []core.Widget {
	var children []core.Widget
	if d.menuBar != nil {
		children = append(children, d.menuBar)
	}
	if d.content != nil {
		children = append(children, d.content)
	}
	if d.dockRow != nil && !d.dockRow.IsEmpty() {
		children = append(children, d.dockRow)
	}
	if d.statusBar != nil {
		children = append(children, d.statusBar)
	}
	return children
}

// AddChild adds a child widget (sets it as content).
func (d *Desktop) AddChild(child core.Widget) {
	d.SetContent(child)
}

// RemoveChild removes a child widget.
func (d *Desktop) RemoveChild(child core.Widget) {
	if d.content == child {
		d.content = nil
	}
}

// ChildAt returns the child at the given position.
func (d *Desktop) ChildAt(pos core.UnitPoint) core.Widget {
	metrics := d.EffectiveCellMetrics()
	bounds := d.Bounds()

	// Check menu bar
	if d.menuBar != nil && pos.Y < metrics.CellHeight {
		return d.menuBar
	}

	// Check status bar
	if d.statusBar != nil && pos.Y >= bounds.Height-metrics.CellHeight {
		return d.statusBar
	}

	// Check content
	if d.content != nil {
		clientArea := d.ClientArea()
		if pos.Y >= clientArea.Y && pos.Y < clientArea.Y+clientArea.Height {
			return d.content
		}
	}

	return nil
}

// Layout arranges children within the desktop.
func (d *Desktop) Layout() {
	d.layoutChildren()
}

// LayoutManager returns nil (desktop uses custom layout).
func (d *Desktop) LayoutManager() core.LayoutManager {
	return nil
}

// SetLayoutManager does nothing (desktop uses custom layout).
func (d *Desktop) SetLayoutManager(lm core.LayoutManager) {
	// Desktop uses custom layout, ignores layout manager
}

// SetMenuBar sets the menu bar (displayed at the top of the screen).
// The system menu (ψ) is automatically prepended to the menu bar.
func (d *Desktop) SetMenuBar(menuBar *MenuBar) {
	d.menuBar = menuBar
	if menuBar != nil {
		menuBar.SetParent(d)
		// Prepend system menu if we have one
		if d.systemMenu != nil {
			menuBar.InsertMenu(0, d.systemMenu)
		}
	}
	d.Update()
}

// MenuBar returns the menu bar.
func (d *Desktop) MenuBar() *MenuBar {
	return d.menuBar
}

// CloseActiveMenu closes any active dropdown menu.
func (d *Desktop) CloseActiveMenu() {
	if d.menuBar != nil && d.menuBar.ActiveMenu() != nil {
		d.menuBar.CloseMenu()
	}
}

// ActiveMenuBounds returns the bounds of the active dropdown menu.
// Returns an empty rect if no menu is open.
func (d *Desktop) ActiveMenuBounds() core.UnitRect {
	if d.menuBar == nil {
		return core.UnitRect{}
	}
	return d.menuBar.ActiveMenuBounds()
}

// IsMenuBarActive returns true if the menu bar should capture keyboard events.
// This is true when a menu is open, or when the menu bar is focused AND
// actively showing accelerators (not just technically holding focus).
func (d *Desktop) IsMenuBarActive() bool {
	if d.menuBar == nil {
		return false
	}
	// Menu open always captures
	if d.menuBar.ActiveMenu() != nil {
		return true
	}
	// Menu bar focused with accelerators active (F10 pressed, awaiting key)
	return d.menuBar.HasFocus() && d.menuBar.AcceleratorsActive()
}

// DeactivateMenuBar closes any open menu and unfocuses the menu bar.
// Uses CloseMenuWithoutRestore because when a window becomes active,
// we want that window to stay in front, not restore the previous window.
func (d *Desktop) DeactivateMenuBar() {
	if d.menuBar != nil {
		d.menuBar.CloseMenuWithoutRestore()
	}
}

// SetStatusBar sets the status bar (displayed at the bottom).
func (d *Desktop) SetStatusBar(statusBar *StatusBar) {
	d.statusBar = statusBar
	if statusBar != nil {
		statusBar.SetParent(d)
	}
	d.Update()
}

// StatusBar returns the status bar.
func (d *Desktop) StatusBar() *StatusBar {
	return d.statusBar
}

// SetBackgroundChar sets the background pattern character.
func (d *Desktop) SetBackgroundChar(ch rune) {
	d.bgChar = ch
	d.Update()
}

// BackgroundChar returns the background pattern character.
func (d *Desktop) BackgroundChar() rune {
	return d.bgChar
}

// KeyboardBlurChildren returns whether child windows include a virtual "blur"
// focus item that allows keyboard users to exit the window.
func (d *Desktop) KeyboardBlurChildren() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.keyboardBlurChildren
}

// SetKeyboardBlurChildren sets whether child windows include a virtual "blur"
// focus item that allows keyboard users to exit the window and focus the menu bar.
func (d *Desktop) SetKeyboardBlurChildren(enabled bool) {
	d.mu.Lock()
	d.keyboardBlurChildren = enabled
	d.mu.Unlock()
}

// PerformKeyboardBlur implements core.KeyboardBlurChildrenProvider.
// It performs the F10 action (focus the menu bar), same as pressing F10.
func (d *Desktop) PerformKeyboardBlur() {
	// Deactivate the active window first (so it visually shows as inactive)
	if d.windowManager != nil {
		d.windowManager.DeactivateActiveWindow()
	}
	// Then activate the menu bar
	if d.menuBar != nil {
		d.menuBar.HandleKeyPress(core.KeyPressEvent{Key: "F10"})
	}
}

// IsWindowPassive implements core.PassiveWindowProvider.
// A window is passive when:
// 1. It's the remembered previous window while the menu bar has focus, OR
// 2. It's active but contains an MDIPane with an active descendant window
func (d *Desktop) IsWindowPassive(win core.Widget) bool {
	if d.windowManager == nil {
		return false
	}

	activeWin := d.windowManager.ActiveWindow()
	previousWin := d.windowManager.PreviousActiveWindow()

	// Case 1: Menu bar has focus, this is the remembered previous window
	if activeWin == nil && previousWin != nil && previousWin == win {
		return true
	}

	// Case 2: Window is active but contains an MDIPane with an active window
	if activeWin == win {
		return hasActiveDescendantWindow(win)
	}

	return false
}

// SetContent sets the content widget (shown behind windows).
func (d *Desktop) SetContent(content core.Widget) {
	d.content = content
	if content != nil {
		content.SetParent(d)
	}
	d.Update()
}

// Content returns the content widget.
func (d *Desktop) Content() core.Widget {
	return d.content
}

// SetBounds sets the desktop bounds (typically the full screen).
func (d *Desktop) SetBounds(bounds core.UnitRect) {
	d.WidgetBase.SetBounds(bounds)
	d.layoutChildren()
}

// layoutChildren updates the bounds of menu bar, status bar, dock row, and content.
func (d *Desktop) layoutChildren() {
	bounds := d.Bounds()
	metrics := d.EffectiveCellMetrics()

	// Menu bar at top
	if d.menuBar != nil {
		d.menuBar.SetBounds(core.UnitRect{
			X:      0,
			Y:      0,
			Width:  bounds.Width,
			Height: metrics.CellHeight,
		})
	}

	// Calculate dock row position and size (above status bar)
	dockHeight := core.Unit(0)
	if d.dockRow != nil && !d.dockRow.IsEmpty() {
		// First set width so RowCount works correctly
		d.dockRow.SetBounds(core.UnitRect{
			X:     0,
			Y:     0,
			Width: bounds.Width,
		})
		dockHeight = d.dockRow.RequiredHeight()
	}

	// Status bar at bottom
	if d.statusBar != nil {
		d.statusBar.SetBounds(core.UnitRect{
			X:      0,
			Y:      bounds.Height - metrics.CellHeight,
			Width:  bounds.Width,
			Height: metrics.CellHeight,
		})
	}

	// Dock row above status bar
	if d.dockRow != nil && !d.dockRow.IsEmpty() {
		dockY := bounds.Height - metrics.CellHeight - dockHeight
		if d.statusBar == nil {
			dockY = bounds.Height - dockHeight
		}
		d.dockRow.SetBounds(core.UnitRect{
			X:      0,
			Y:      dockY,
			Width:  bounds.Width,
			Height: dockHeight,
		})
	}

	// Content in the middle
	if d.content != nil {
		clientArea := d.ClientArea()
		d.content.SetBounds(core.UnitRect{
			X:      0,
			Y:      0,
			Width:  clientArea.Width,
			Height: clientArea.Height,
		})
	}
}

// ClientArea returns the area available for windows (excluding menu/status/dock bars).
func (d *Desktop) ClientArea() core.UnitRect {
	bounds := d.Bounds()
	metrics := d.EffectiveCellMetrics()

	top := core.Unit(0)
	bottom := bounds.Height

	if d.menuBar != nil {
		top = metrics.CellHeight
	}
	if d.statusBar != nil {
		bottom -= metrics.CellHeight
	}
	// Account for dock row height (when not empty)
	if d.dockRow != nil && !d.dockRow.IsEmpty() {
		// Need to calculate height based on current width
		d.dockRow.SetBounds(core.UnitRect{Width: bounds.Width})
		bottom -= d.dockRow.RequiredHeight()
	}

	return core.UnitRect{
		X:      0,
		Y:      top,
		Width:  bounds.Width,
		Height: bottom - top,
	}
}

// MenuBarHeight returns the height of the menu bar area (0 if no menu bar).
func (d *Desktop) MenuBarHeight() core.Unit {
	if d.menuBar == nil {
		return 0
	}
	return d.EffectiveCellMetrics().CellHeight
}

// StatusBarHeight returns the height of the status bar area (0 if no status bar).
func (d *Desktop) StatusBarHeight() core.Unit {
	if d.statusBar == nil {
		return 0
	}
	return d.EffectiveCellMetrics().CellHeight
}

// StatusBarBounds returns the bounds of the status bar area (empty rect if no status bar).
func (d *Desktop) StatusBarBounds() core.UnitRect {
	if d.statusBar == nil {
		return core.UnitRect{}
	}
	bounds := d.Bounds()
	metrics := d.EffectiveCellMetrics()
	return core.UnitRect{
		X:      0,
		Y:      bounds.Height - metrics.CellHeight,
		Width:  bounds.Width,
		Height: metrics.CellHeight,
	}
}

// DockRow returns the dock row widget.
func (d *Desktop) DockRow() *DockRow {
	return d.dockRow
}

// DockRowHeight returns the height of the dock row area (0 if empty).
func (d *Desktop) DockRowHeight() core.Unit {
	if d.dockRow == nil || d.dockRow.IsEmpty() {
		return 0
	}
	return d.dockRow.RequiredHeight()
}

// DockBounds returns the bounds of the dock row area (empty rect if no dock or empty).
func (d *Desktop) DockBounds() core.UnitRect {
	if d.dockRow == nil || d.dockRow.IsEmpty() {
		return core.UnitRect{}
	}
	bounds := d.Bounds()
	metrics := d.EffectiveCellMetrics()
	dockHeight := d.dockRow.RequiredHeight()
	dockY := bounds.Height - dockHeight
	if d.statusBar != nil {
		dockY = bounds.Height - metrics.CellHeight - dockHeight
	}
	return core.UnitRect{
		X:      0,
		Y:      dockY,
		Width:  bounds.Width,
		Height: dockHeight,
	}
}

// DockEntryCount returns the number of entries in the dock.
func (d *Desktop) DockEntryCount() int {
	if d.dockRow == nil {
		return 0
	}
	return d.dockRow.EntryCount()
}

// IsDockFocused returns true if the dock currently has focus.
func (d *Desktop) IsDockFocused() bool {
	if d.dockRow == nil {
		return false
	}
	return d.dockRow.HasFocus()
}

// FocusDock sets focus to the dock.
func (d *Desktop) FocusDock() {
	if d.dockRow != nil && !d.dockRow.IsEmpty() {
		d.dockRow.SetFocus()
	}
}

// UnfocusDock removes focus from the dock.
func (d *Desktop) UnfocusDock() {
	if d.dockRow != nil {
		d.dockRow.ClearFocus()
	}
}

// SizeHint returns the preferred size.
func (d *Desktop) SizeHint() core.UnitSize {
	// Desktop fills available space
	return d.Bounds().Size()
}

// Paint renders the desktop.
func (d *Desktop) Paint(p *core.Painter) {
	bounds := d.Bounds()
	scheme := d.GetScheme()
	metrics := d.EffectiveCellMetrics()

	// Draw background pattern. Graphical targets tile the classic
	// 8x8 two-color bitmap wallpaper (chunked to WallpaperChunkPx);
	// cell targets keep the rune fill.
	bgStyle := scheme.GetDesktopFill()
	if !p.FillPattern(core.UnitRect{Width: bounds.Width, Height: bounds.Height},
		d.wallpaperPattern, d.wallpaperChunkPx, bgStyle) {
		for y := core.Unit(0); y < bounds.Height; y += metrics.CellHeight {
			for x := core.Unit(0); x < bounds.Width; x += metrics.CellWidth {
				p.DrawCell(x, y, d.bgChar, bgStyle)
			}
		}
	}

	// Draw content if any
	if d.content != nil {
		clientArea := d.ClientArea()
		contentPainter := p.WithOffset(clientArea.X, clientArea.Y).
			WithClip(core.UnitRect{Width: clientArea.Width, Height: clientArea.Height})
		d.content.Paint(contentPainter)
	}

	// Draw menu bar at top
	if d.menuBar != nil {
		// Set menu bar bounds
		d.menuBar.SetBounds(core.UnitRect{
			X:      0,
			Y:      0,
			Width:  bounds.Width,
			Height: metrics.CellHeight,
		})
		d.menuBar.Paint(p)
	}

	// Draw dock row above status bar (if not empty)
	if d.dockRow != nil && !d.dockRow.IsEmpty() {
		dockHeight := d.dockRow.RequiredHeight()
		dockY := bounds.Height - metrics.CellHeight - dockHeight
		if d.statusBar == nil {
			dockY = bounds.Height - dockHeight
		}
		d.dockRow.SetBounds(core.UnitRect{
			X:      0,
			Y:      dockY,
			Width:  bounds.Width,
			Height: dockHeight,
		})
		dockPainter := p.WithOffset(0, dockY)
		d.dockRow.Paint(dockPainter)
	}

	// Draw status bar at bottom
	if d.statusBar != nil {
		y := bounds.Height - metrics.CellHeight
		d.statusBar.SetBounds(core.UnitRect{
			X:      0,
			Y:      y,
			Width:  bounds.Width,
			Height: metrics.CellHeight,
		})
		statusPainter := p.WithOffset(0, y)
		d.statusBar.Paint(statusPainter)
	}
}

// HandleKeyPress handles keyboard input.
func (d *Desktop) HandleKeyPress(event core.KeyPressEvent) bool {
	// Check if menu bar wants to handle keys
	if d.menuBar != nil {
		// F10 toggles menu bar focus
		if event.Key == "F10" {
			// Deactivate the active window when invoking menu bar
			if d.windowManager != nil && !d.menuBar.HasFocus() {
				d.windowManager.DeactivateActiveWindow()
			}
			d.menuBar.HandleKeyPress(event)
			return true
		}
		// Alt+letter (M-<letter>) for menu shortcuts
		if strings.HasPrefix(event.Key, "M-") && len(event.Key) == 3 {
			if d.menuBar.HandleKeyPress(event) {
				return true
			}
		}
		// If menu bar is active (menu open, or focused with accelerators showing), forward keys
		if d.menuBar.ActiveMenu() != nil || (d.menuBar.HasFocus() && d.menuBar.AcceleratorsActive()) {
			handled := d.menuBar.HandleKeyPress(event)
			// If menu bar didn't handle Escape and has focus (no menu open),
			// unfocus the menu bar
			if !handled && event.Key == "Escape" && d.menuBar.HasFocus() {
				d.menuBar.CloseMenuAndUnfocus()
				return true
			}
			return handled
		}
	}

	// If dock has focus, forward keys to it
	if d.dockRow != nil && d.dockRow.HasFocus() {
		return d.dockRow.HandleKeyPress(event)
	}

	// Forward to content
	if d.content != nil {
		return d.content.HandleKeyPress(event)
	}

	return false
}

// PaintMenuDropdown paints the active menu dropdown (call after windows for z-order).
func (d *Desktop) PaintMenuDropdown(p *core.Painter) {
	if d.menuBar != nil {
		d.menuBar.PaintDropdown(p)
	}
}

// HandleMousePress handles mouse clicks.
func (d *Desktop) HandleMousePress(event core.MousePressEvent) bool {
	metrics := d.EffectiveCellMetrics()
	bounds := d.Bounds()

	// Helper to cancel drag state on a widget
	cancelDrag := func(w core.Widget) {
		if w == nil {
			return
		}
		if handler, ok := w.(interface {
			HandleMouseRelease(core.MouseReleaseEvent) bool
		}); ok {
			handler.HandleMouseRelease(core.MouseReleaseEvent{Button: event.Button})
		}
	}

	// Check menu bar first - either in menu bar area or when menu is open
	if d.menuBar != nil {
		if event.Y < metrics.CellHeight || d.menuBar.ActiveMenu() != nil {
			// Cancel drags on other children
			cancelDrag(d.statusBar)
			cancelDrag(d.dockRow)
			cancelDrag(d.content)
			return d.menuBar.HandleMousePress(event)
		}
	}

	// Check status bar
	if d.statusBar != nil {
		statusY := bounds.Height - metrics.CellHeight
		if event.Y >= statusY {
			// Cancel drags on other children
			cancelDrag(d.menuBar)
			cancelDrag(d.dockRow)
			cancelDrag(d.content)
			localEvent := event
			localEvent.Y -= statusY
			return d.statusBar.HandleMousePress(localEvent)
		}
	}

	// Check dock row (above status bar)
	if d.dockRow != nil && !d.dockRow.IsEmpty() {
		dockHeight := d.dockRow.RequiredHeight()
		dockY := bounds.Height - metrics.CellHeight - dockHeight
		if d.statusBar == nil {
			dockY = bounds.Height - dockHeight
		}
		if event.Y >= dockY && event.Y < dockY+dockHeight {
			// Cancel drags on other children
			cancelDrag(d.menuBar)
			cancelDrag(d.statusBar)
			cancelDrag(d.content)
			localEvent := event
			localEvent.Y -= dockY
			return d.dockRow.HandleMousePress(localEvent)
		}
	}

	// Check content
	if d.content != nil {
		clientArea := d.ClientArea()
		if event.Y >= clientArea.Y && event.Y < clientArea.Y+clientArea.Height {
			// Cancel drags on other children
			cancelDrag(d.menuBar)
			cancelDrag(d.statusBar)
			cancelDrag(d.dockRow)
			localEvent := event
			localEvent.X -= clientArea.X
			localEvent.Y -= clientArea.Y
			return d.content.HandleMousePress(localEvent)
		}
	}

	// Click was on blank desktop area - cancel all drags
	cancelDrag(d.menuBar)
	cancelDrag(d.statusBar)
	cancelDrag(d.dockRow)
	cancelDrag(d.content)
	return false
}

// HandleMouseMove handles mouse movement.
func (d *Desktop) HandleMouseMove(event core.MouseMoveEvent) bool {
	// Forward to menu bar for drag navigation
	if d.menuBar != nil {
		if d.menuBar.HandleMouseMove(event) {
			return true
		}
	}
	return false
}

// HandleMouseRelease handles mouse release.
func (d *Desktop) HandleMouseRelease(event core.MouseReleaseEvent) bool {
	// Forward to menu bar for drag release
	if d.menuBar != nil {
		if d.menuBar.HandleMouseRelease(event) {
			return true
		}
	}
	return false
}

// StatusBar is a simple status bar widget.
type StatusBar struct {
	core.WidgetBase

	// Status sections
	sections []StatusSection
}

// StatusTextSpan represents a span of text with optional style override.
type StatusTextSpan struct {
	Text  string
	Style *style.CellStyle // nil = use default status bar style
}

// StatusSection represents a section of the status bar.
type StatusSection struct {
	Text      string           // Plain text (used if Spans is empty)
	Spans     []StatusTextSpan // Styled text spans (takes precedence over Text)
	Width     int              // 0 = auto, -1 = stretch
	Alignment int              // 0 = left, 1 = center, 2 = right
}

// NewStatusBar creates a new status bar.
func NewStatusBar() *StatusBar {
	s := &StatusBar{}
	s.WidgetBase = *core.NewWidgetBase()
	s.Init(s)
	s.SetFocusPolicy(core.NoFocus)
	return s
}

// SetText sets the main status text.
func (s *StatusBar) SetText(text string) {
	if len(s.sections) == 0 {
		s.sections = []StatusSection{{Text: text, Width: -1}}
	} else {
		s.sections[0].Text = text
		s.sections[0].Spans = nil // Clear any styled spans
	}
	s.Update()
}

// SetStyledText sets the main status text with styled spans.
func (s *StatusBar) SetStyledText(spans []StatusTextSpan) {
	if len(s.sections) == 0 {
		s.sections = []StatusSection{{Spans: spans, Width: -1}}
	} else {
		s.sections[0].Spans = spans
		s.sections[0].Text = "" // Clear plain text
	}
	s.Update()
}

// Text returns the main status text.
func (s *StatusBar) Text() string {
	if len(s.sections) == 0 {
		return ""
	}
	return s.sections[0].Text
}

// AddSection adds a section to the status bar.
func (s *StatusBar) AddSection(section StatusSection) {
	s.sections = append(s.sections, section)
	s.Update()
}

// SetSections sets all sections.
func (s *StatusBar) SetSections(sections []StatusSection) {
	s.sections = sections
	s.Update()
}

// Sections returns all sections.
func (s *StatusBar) Sections() []StatusSection {
	return s.sections
}

// SizeHint returns the preferred size.
func (s *StatusBar) SizeHint() core.UnitSize {
	metrics := s.EffectiveCellMetrics()
	return core.UnitSize{
		Width:  0, // Will stretch to fill
		Height: metrics.CellHeight,
	}
}

// Paint renders the status bar.
func (s *StatusBar) Paint(p *core.Painter) {
	bounds := s.Bounds()
	scheme := s.GetScheme()
	metrics := s.EffectiveCellMetrics()

	statusBarStyle := scheme.GetStatusBar()

	// Draw background
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', statusBarStyle)

	// Draw sections
	x := core.Unit(0)
	for _, section := range s.sections {
		// Calculate section width
		var sectionWidth core.Unit
		if section.Width == -1 {
			// Stretch to remaining space
			sectionWidth = bounds.Width - x
		} else if section.Width == 0 {
			// Auto width based on content
			textLen := len(section.Text)
			if len(section.Spans) > 0 {
				textLen = 0
				for _, span := range section.Spans {
					textLen += len(span.Text)
				}
			}
			sectionWidth = core.Unit(textLen+2) * metrics.CellWidth
		} else {
			sectionWidth = core.Unit(section.Width) * metrics.CellWidth
		}

		// Draw text - either from spans or plain text
		textX := x + metrics.CellWidth
		maxX := x + sectionWidth

		if len(section.Spans) > 0 {
			// Draw styled spans
			for _, span := range section.Spans {
				spanStyle := statusBarStyle
				if span.Style != nil {
					spanStyle = *span.Style
				}
				for _, ch := range span.Text {
					if textX >= maxX {
						break
					}
					p.DrawCell(textX, 0, ch, spanStyle)
					textX += metrics.CellWidth
				}
			}
		} else {
			// Draw plain text
			for _, ch := range section.Text {
				if textX >= maxX {
					break
				}
				p.DrawCell(textX, 0, ch, statusBarStyle)
				textX += metrics.CellWidth
			}
		}

		x += sectionWidth
	}
}

// HandleMousePress handles mouse clicks.
func (s *StatusBar) HandleMousePress(event core.MousePressEvent) bool {
	// Status bar clicks could be used for section-specific actions
	return true
}

// Verify Desktop implements KeyboardBlurChildrenProvider
var _ core.KeyboardBlurChildrenProvider = (*Desktop)(nil)
