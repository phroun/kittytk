// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/phroun/tuitk/core"
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
}

// Desktop represents the application desktop (background behind windows).
// It can optionally display a menu bar at the top (Mac-style) and a
// status bar at the bottom. Desktop can serve as the top-level object
// managing multiple applications.
type Desktop struct {
	core.WidgetBase

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

	// Theme
	theme *style.Theme

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

// NewDesktop creates a new desktop widget.
func NewDesktop() *Desktop {
	d := &Desktop{
		bgChar:     '▓', // Default pattern (three-quarter shade block)
		dockRow:    NewDockRow(),
		quitChan:   make(chan struct{}),
		updateChan: make(chan struct{}, 100),
		theme:      style.DefaultTheme(),
	}
	d.WidgetBase = *core.NewWidgetBase()
	d.Init(d)
	d.SetFocusPolicy(core.NoFocus)
	d.dockRow.SetParent(d)

	// Create system menu
	d.systemMenu = d.createSystemMenu()

	return d
}

// createSystemMenu creates the always-present system menu (ψ).
func (d *Desktop) createSystemMenu() *Menu {
	menu := NewMenu("ψ")
	menu.AddItem(NewMenuItem("&About Desktop").SetOnTriggered(func() {
		// Show about dialog
		// TODO: Implement about dialog
	}))
	menu.AddItem(NewSeparator())
	menu.AddItem(NewMenuItem("Desktop &Accessories").SetEnabled(false)) // Placeholder
	menu.AddItem(NewSeparator())
	menu.AddItem(NewMenuItem("&Quit").SetShortcut(core.Shortcut("^Q")).SetOnTriggered(func() {
		d.Quit()
	}))
	return menu
}

// SetBackend sets the render backend and initializes related components.
func (d *Desktop) SetBackend(backend core.RenderBackend) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.backend = backend
	d.windowManager = window.NewWindowManager()
	d.windowManager.SetOnRepaintNeeded(func() {
		d.RequestUpdate()
	})
	d.focusManager = core.NewGlobalFocusManager()
	d.windowManager.SetDesktop(d)

	// Wire up dock row integration
	if d.dockRow != nil {
		d.windowManager.SetOnWindowMinimized(func(win *window.Window) {
			entry := &DockEntry{
				Title: win.Title(),
				OnClick: func() {
					d.windowManager.RestoreWindow(win)
				},
			}
			d.dockRow.AddEntry(entry)
		})

		d.windowManager.SetOnWindowRestored(func(win *window.Window) {
			d.dockRow.RemoveEntryByTitle(win.Title())
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

// AddApplication registers an application with the desktop.
func (d *Desktop) AddApplication(app ApplicationProvider) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.applications = append(d.applications, app)

	// If this is the first app, make it active
	if d.activeApp == nil {
		d.activeApp = app
		app.OnActivate()
		d.updateMenuBarContent()
		d.updateStatusBarContent()
	}
}

// RemoveApplication unregisters an application from the desktop.
func (d *Desktop) RemoveApplication(app ApplicationProvider) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for i, a := range d.applications {
		if a == app {
			d.applications = append(d.applications[:i], d.applications[i+1:]...)
			break
		}
	}

	// If this was the active app, switch to another or none
	if d.activeApp == app {
		app.OnDeactivate()
		if len(d.applications) > 0 {
			d.activeApp = d.applications[0]
			d.activeApp.OnActivate()
		} else {
			d.activeApp = nil
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

	// Add active app's menus
	if activeApp != nil {
		for _, menu := range activeApp.MenuBarContent() {
			d.menuBar.AddMenu(menu)
		}
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

// Run starts the desktop event loop.
// Returns the exit code when the desktop quits.
// This is an alternative to using app.Application.Run() - only use one.
func (d *Desktop) Run() int {
	d.mu.Lock()
	backend := d.backend
	onStartup := d.onStartup
	d.mu.Unlock()

	if backend == nil {
		return 1
	}

	// Initialize backend
	if err := backend.Init(); err != nil {
		return 1
	}
	defer backend.Shutdown()

	// Update screen bounds
	d.mu.Lock()
	wm := d.windowManager
	d.mu.Unlock()

	size := backend.Size()
	wm.SetScreenBounds(core.UnitRect{Width: size.Width, Height: size.Height})

	// Mark as running
	d.running.Store(true)
	defer d.running.Store(false)

	// Call startup handler
	if onStartup != nil {
		onStartup()
	}

	// Run event loop
	d.eventLoop()

	// Call shutdown handler
	d.mu.RLock()
	onShutdown := d.onShutdown
	exitCode := d.exitCode
	d.mu.RUnlock()

	if onShutdown != nil {
		onShutdown()
	}

	return exitCode
}

// eventLoop is the main event processing loop.
func (d *Desktop) eventLoop() {
	for d.running.Load() {
		d.processTimers()
		d.processEvents()
		d.render()
	}
}

// processEvents handles pending events.
func (d *Desktop) processEvents() {
	d.mu.RLock()
	backend := d.backend
	wm := d.windowManager
	fm := d.focusManager
	d.mu.RUnlock()

	// Process all pending events
	for {
		event := backend.PollEvent()
		if event == nil {
			// No more events - wait for next event or update request
			select {
			case <-d.quitChan:
				return
			case <-d.updateChan:
				return
			default:
				// Wait briefly for events
				event = d.waitEventWithTimeout(50 * time.Millisecond)
				if event == nil {
					return
				}
			}
		}

		// Handle event based on type
		switch e := event.(type) {
		case core.ResizeEvent:
			wm.SetScreenBounds(core.UnitRect{Width: e.Width, Height: e.Height})

		case core.QuitEvent:
			d.running.Store(false)
			return

		case core.KeyPressEvent:
			// Check global shortcuts first
			if d.handleShortcut(e) {
				continue
			}
			// Try focus manager
			if fm != nil && fm.HandleKeyPress(e) {
				continue
			}
			// Pass to window manager
			wm.HandleKeyPress(e)

		case core.MousePressEvent:
			wm.HandleMousePress(e)

		case core.MouseMoveEvent:
			wm.HandleMouseMove(e)

		case core.MouseReleaseEvent:
			wm.HandleMouseRelease(e)
		}
	}
}

// waitEventWithTimeout waits for an event with a timeout.
func (d *Desktop) waitEventWithTimeout(timeout time.Duration) core.Event {
	d.mu.RLock()
	backend := d.backend
	d.mu.RUnlock()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		event := backend.PollEvent()
		if event != nil {
			return event
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

// handleShortcut checks if a key event matches a global shortcut.
func (d *Desktop) handleShortcut(event core.KeyPressEvent) bool {
	switch event.Key {
	case "^Q": // Ctrl+Q - Quit
		d.Quit()
		return true
	case "^W": // Ctrl+W - Close window
		d.mu.RLock()
		wm := d.windowManager
		d.mu.RUnlock()
		if wm != nil {
			if active := wm.ActiveWindow(); active != nil {
				active.Close()
				return true
			}
		}
	}
	return false
}

// render redraws the screen.
func (d *Desktop) render() {
	d.mu.RLock()
	backend := d.backend
	wm := d.windowManager
	theme := d.theme
	d.mu.RUnlock()

	backend.BeginFrame()

	// Clear with theme background
	backend.Clear(theme.Normal)

	// Create painter
	painter := core.NewPainter(backend)

	// Paint window manager (includes desktop and windows)
	wm.Paint(painter)

	backend.EndFrame()
}

// processTimers checks and fires due timers.
func (d *Desktop) processTimers() {
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

// RequestUpdate requests a screen update.
func (d *Desktop) RequestUpdate() {
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
	d.mu.Unlock()
	d.running.Store(false)
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
	metrics := core.DefaultCellMetrics()
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
	metrics := core.DefaultCellMetrics()

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
	metrics := core.DefaultCellMetrics()

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
	return core.DefaultCellMetrics().CellHeight
}

// StatusBarHeight returns the height of the status bar area (0 if no status bar).
func (d *Desktop) StatusBarHeight() core.Unit {
	if d.statusBar == nil {
		return 0
	}
	return core.DefaultCellMetrics().CellHeight
}

// StatusBarBounds returns the bounds of the status bar area (empty rect if no status bar).
func (d *Desktop) StatusBarBounds() core.UnitRect {
	if d.statusBar == nil {
		return core.UnitRect{}
	}
	bounds := d.Bounds()
	metrics := core.DefaultCellMetrics()
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
	metrics := core.DefaultCellMetrics()
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
	metrics := p.Metrics()

	// Draw background pattern
	bgStyle := scheme.GetDesktopFill()
	for y := core.Unit(0); y < bounds.Height; y += metrics.CellHeight {
		for x := core.Unit(0); x < bounds.Width; x += metrics.CellWidth {
			p.DrawCell(x, y, d.bgChar, bgStyle)
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
	metrics := core.DefaultCellMetrics()
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
	Text      string            // Plain text (used if Spans is empty)
	Spans     []StatusTextSpan  // Styled text spans (takes precedence over Text)
	Width     int               // 0 = auto, -1 = stretch
	Alignment int               // 0 = left, 1 = center, 2 = right
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
	metrics := core.DefaultCellMetrics()
	return core.UnitSize{
		Width:  0, // Will stretch to fill
		Height: metrics.CellHeight,
	}
}

// Paint renders the status bar.
func (s *StatusBar) Paint(p *core.Painter) {
	bounds := s.Bounds()
	scheme := s.GetScheme()
	metrics := p.Metrics()

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
