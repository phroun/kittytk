// Package window provides windowing support for KittyTK.
package window

import (
	"sync"
	"time"

	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/style"
)

// Resize edge constants (can be combined for corners)
const (
	ResizeEdgeNone   = 0
	ResizeEdgeLeft   = 1 << 0
	ResizeEdgeRight  = 1 << 1
	ResizeEdgeTop    = 1 << 2
	ResizeEdgeBottom = 1 << 3
)

// DockProvider is implemented by desktops that have a dock for minimized windows.
type DockProvider interface {
	// DockEntryCount returns the number of entries in the dock.
	DockEntryCount() int
	// IsDockFocused returns true if the dock currently has focus.
	IsDockFocused() bool
	// FocusDock sets focus to the dock.
	FocusDock()
	// UnfocusDock removes focus from the dock.
	UnfocusDock()
}

// WindowManager manages all windows in the application.
// It handles z-ordering, focus, modal windows, and window positioning.
//
// Scope (G4): the WindowManager composites windows WITHIN ONE
// surface - its "screen" bounds are that surface's bounds, set by
// the desktop from Surface.Size. Windows granted native mode live
// outside its jurisdiction entirely (one window per surface, hosted
// by SurfaceHost with OS-provided chrome); which mode a window gets
// is host policy consulting Window.NativeRequested.
type WindowManager struct {
	mu sync.RWMutex

	// All windows in z-order (back to front)
	windows []*Window

	// Active/focused window
	activeWindow *Window

	// Previously active window (remembered when menu bar activates)
	previousActiveWindow *Window

	// Modal window stack
	modalStack []*Window

	// Desktop/root trinket (what's behind all windows)
	desktop core.Trinket

	// Screen bounds
	screenBounds core.UnitRect

	// Theme
	theme *style.Theme

	// Tear-off policy (G4 granting): when a drag crosses the surface
	// edge, the host may lift the window out into its own OS surface.
	// Returning true means the window left this manager.
	tearOff func(win *Window, event core.MouseMoveEvent, offsetX, offsetY core.Unit) bool

	// Drag state
	dragging    *Window
	dragStartX  core.Unit
	dragStartY  core.Unit
	dragOffsetX core.Unit
	dragOffsetY core.Unit
	// dragNeedsButton marks a drag armed programmatically (BeginDrag,
	// re-dock): its press happened in another surface, so its release
	// can be lost there too. Such a drag ends the moment motion
	// arrives without the button held. Press-armed drags keep the
	// historical behavior (terminal backends don't always report
	// button state on motion).
	dragNeedsButton bool
	// dragIsTearHandle marks a drag begun on the %/# tear handle: only
	// such a drag may tear the window off (or re-dock it); a plain
	// title drag just moves it in-surface. dragMoved tracks whether the
	// pointer left the press point, so a handle press released in place
	// is a click (toggles detach) rather than a drag.
	dragIsTearHandle bool
	dragMoved        bool

	// Resize state
	resizing       *Window
	resizeEdge     int
	resizeStartX   core.Unit
	resizeStartY   core.Unit
	resizeOriginal core.UnitRect

	// Double-click detection
	lastClickTime   time.Time
	lastClickX      core.Unit
	lastClickY      core.Unit
	lastClickWindow *Window

	// Focus-without-raise: track pressed window for conditional raise on release
	pressedWindow *Window

	// Callbacks
	onWindowAdded     func(*Window)
	onWindowRemoved   func(*Window)
	onActiveChanged   func(*Window)
	onRepaintNeeded   func()
	onWindowMinimized func(*Window) // Called when a window is minimized
	onWindowRestored  func(*Window) // Called when a window is restored

	// Popup overlays (painted on top of everything)
	popups []*PopupOverlay

	// Cycle order for M-Tab: tracks activation order of windows and dock.
	// Items are *Window or nil (nil represents the dock).
	cycleOrder []interface{}

	// Smooth positioning: when the surface's backend supports sub-cell
	// placement (pixel surfaces), drag and resize track the pointer at
	// unit granularity instead of snapping to cell boundaries.
	smoothPositioning bool

	// resizeGrip narrows the resize-handle zones on graphical frames
	// to the outer sliver of each edge (units; 0 = classic cell-wide
	// zones), so trinkets at a window's edge remain clickable.
	resizeGrip core.Unit
}

// PopupOverlay represents a popup that should be painted on top of all windows.
type PopupOverlay struct {
	// Unique identifier for the popup
	ID string
	// Bounds in screen coordinates
	Bounds core.UnitRect
	// Paint function to render the popup
	Paint func(p *core.Painter)
	// HandleMousePress function to handle clicks (returns true if handled)
	HandleMousePress func(event core.MousePressEvent) bool
	// HandleMouseMove function to handle mouse movement (returns true if handled)
	HandleMouseMove func(event core.MouseMoveEvent) bool
	// HandleMouseRelease function to handle mouse release (returns true if handled)
	HandleMouseRelease func(event core.MouseReleaseEvent) bool
	// HandleMouseWheel function to handle wheel scrolling (returns true if handled)
	HandleMouseWheel func(event core.MouseWheelEvent) bool
}

// NewWindowManager creates a new window manager.
func NewWindowManager() *WindowManager {
	return &WindowManager{
		theme: style.DefaultTheme(),
	}
}

// SetSmoothPositioning controls whether drag and resize snap to cell
// boundaries. Pixel-capable surfaces enable smooth positioning; cell-grid
// surfaces leave it off so windows always land on whole cells.
func (m *WindowManager) SetSmoothPositioning(smooth bool) {
	m.mu.Lock()
	m.smoothPositioning = smooth
	windows := make([]*Window, len(m.windows))
	copy(windows, m.windows)
	m.mu.Unlock()
	for _, win := range windows {
		win.SetSmoothPositioning(smooth)
	}
}

// SetResizeGrip sets the resize-handle thickness in units for
// graphical frames. Zero restores the cell-frame behavior (the whole
// border row/column is the grip - it IS the frame there).
func (m *WindowManager) SetResizeGrip(grip core.Unit) {
	m.mu.Lock()
	m.resizeGrip = grip
	m.mu.Unlock()
}

// SmoothPositioning reports whether drag and resize track the pointer at
// unit granularity rather than snapping to cell boundaries.
// SetTearOffHandler installs the host's tear-off policy: called
// during a title drag when the pointer leaves the surface. A nil
// handler (the default) keeps every drag in-surface.
func (m *WindowManager) SetTearOffHandler(h func(win *Window, event core.MouseMoveEvent, offsetX, offsetY core.Unit) bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tearOff = h
}

// BeginDrag arms the title-bar drag state programmatically, as if the
// user had pressed on the titlebar with the given grab offset. The
// re-dock choreography uses it so a window dropped back onto the
// desktop keeps following the held pointer.
func (m *WindowManager) BeginDrag(win *Window, offsetX, offsetY core.Unit) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dragging = win
	m.dragOffsetX = offsetX
	m.dragOffsetY = offsetY
	m.dragNeedsButton = true
}

func (m *WindowManager) SmoothPositioning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.smoothPositioning
}

// abs returns the absolute value of an integer.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ResizeEdgeAt returns the resize edge bits for point (x, y) against a
// window occupying `bounds` (all in the same coordinate space). `grip` is
// the effective grab-zone thickness in units - the graphical grip sliver
// plus any frame border; when it is 0 the cell-frame defaults from
// `metrics` apply (a full cell on the sides and bottom) and the top edge
// is NOT grabbable (the top row is the titlebar, used for dragging). The
// bottom and (when grabbable) top edges widen at the corners so diagonal
// resize is easy to hit.
//
// This is the single source of resize-edge geometry: the desktop
// WindowManager and the embedded MDIPane both call it, so desktop and MDI
// windows detect identical edges and corners.
func ResizeEdgeAt(bounds core.UnitRect, x, y core.Unit, metrics core.CellMetrics, grip core.Unit) int {
	edgeThreshold := metrics.CellWidth
	cornerThreshold := metrics.CellWidth * 2
	bottomBand := metrics.CellHeight
	if grip > 0 {
		edgeThreshold = grip
		cornerThreshold = grip * 2
		bottomBand = grip
	}

	edge := ResizeEdgeNone

	atBottom := y >= bounds.Y+bounds.Height-bottomBand && y < bounds.Y+bounds.Height
	atTop := grip > 0 && y >= bounds.Y && y < bounds.Y+grip

	if x >= bounds.X && x < bounds.X+edgeThreshold {
		edge |= ResizeEdgeLeft
	} else if x >= bounds.X+bounds.Width-edgeThreshold && x < bounds.X+bounds.Width {
		edge |= ResizeEdgeRight
	}

	if atBottom {
		edge |= ResizeEdgeBottom
		if x >= bounds.X && x < bounds.X+cornerThreshold {
			edge |= ResizeEdgeLeft
		} else if x >= bounds.X+bounds.Width-cornerThreshold && x < bounds.X+bounds.Width {
			edge |= ResizeEdgeRight
		}
	}

	if atTop {
		edge |= ResizeEdgeTop
		if x >= bounds.X && x < bounds.X+cornerThreshold {
			edge |= ResizeEdgeLeft
		} else if x >= bounds.X+bounds.Width-cornerThreshold && x < bounds.X+bounds.Width {
			edge |= ResizeEdgeRight
		}
	}

	return edge
}

// EffectiveResizeGrip is the grab-zone thickness in units for `win`: the
// base grip sliver plus the frame border (a thicker border makes a
// proportionally thicker grip that also overlaps a little into content).
// Shared so WindowManager and MDIPane compute the same grip.
func EffectiveResizeGrip(win *Window, baseGrip core.Unit) core.Unit {
	return baseGrip + core.FindFrameBorderUnits(win)
}

// detectResizeEdge determines which window edge(s) the mouse is near.
// Returns a combination of ResizeEdge constants.
func (m *WindowManager) detectResizeEdge(win *Window, x, y core.Unit) int {
	if win.Flags()&WindowFlagNoResize != 0 || win.IsMaximized() {
		return ResizeEdgeNone
	}
	m.mu.RLock()
	baseGrip := m.resizeGrip
	m.mu.RUnlock()
	return ResizeEdgeAt(win.Bounds(), x, y, core.DefaultCellMetrics(),
		EffectiveResizeGrip(win, baseGrip))
}

// resizeEdgeRects returns the window-local rectangles (one per set edge
// bit, two for a corner) covering the size-sensitive band(s) for the
// given resize edge, matching detectResizeEdge's thresholds. Used to
// highlight the edge under the pointer.
func (m *WindowManager) resizeEdgeRects(win *Window, edge int) []core.UnitRect {
	b := win.Bounds()
	metrics := core.DefaultCellMetrics()
	edgeThreshold := metrics.CellWidth
	bottomBand := metrics.CellHeight
	m.mu.RLock()
	baseGrip := m.resizeGrip
	m.mu.RUnlock()
	// Match detectResizeEdge: border width plus the grip sliver, so the
	// highlight covers the whole outer border and the small overlap into
	// the content.
	grip := EffectiveResizeGrip(win, baseGrip)
	if grip > 0 {
		edgeThreshold = grip
		bottomBand = grip
	}

	var rects []core.UnitRect
	if edge&ResizeEdgeLeft != 0 {
		rects = append(rects, core.UnitRect{Width: edgeThreshold, Height: b.Height})
	}
	if edge&ResizeEdgeRight != 0 {
		rects = append(rects, core.UnitRect{X: b.Width - edgeThreshold, Width: edgeThreshold, Height: b.Height})
	}
	if edge&ResizeEdgeTop != 0 {
		// The top band is grip-thick (top resize is graphical-only, where
		// bottomBand == grip).
		rects = append(rects, core.UnitRect{Width: b.Width, Height: bottomBand})
	}
	if edge&ResizeEdgeBottom != 0 {
		rects = append(rects, core.UnitRect{Y: b.Height - bottomBand, Width: b.Width, Height: bottomBand})
	}
	return rects
}

// updateResizeHover highlights the size-sensitive edge(s) of the topmost
// window under the pointer, clearing the highlight on every other window.
// Called on mouse move when no drag or resize is in progress.
func (m *WindowManager) updateResizeHover(x, y core.Unit) {
	m.mu.RLock()
	windows := make([]*Window, len(m.windows))
	copy(windows, m.windows)
	m.mu.RUnlock()

	var target *Window
	edge := ResizeEdgeNone
	for i := len(windows) - 1; i >= 0; i-- {
		win := windows[i]
		if !win.IsVisible() || win.IsMinimized() {
			continue
		}
		if win.Bounds().Contains(core.UnitPoint{X: x, Y: y}) {
			target = win
			edge = m.detectResizeEdge(win, x, y)
			break
		}
	}

	changed := false
	for _, win := range windows {
		var rects []core.UnitRect
		if win == target && edge != ResizeEdgeNone {
			rects = m.resizeEdgeRects(win, edge)
		}
		if win.SetResizeHoverRects(rects) {
			changed = true
		}
	}
	if changed {
		m.RequestRepaint()
	}
}

// topWindowAt returns the topmost visible, non-minimized window whose
// bounds contain the point, or nil.
func (m *WindowManager) topWindowAt(x, y core.Unit) *Window {
	m.mu.RLock()
	windows := make([]*Window, len(m.windows))
	copy(windows, m.windows)
	m.mu.RUnlock()
	for i := len(windows) - 1; i >= 0; i-- {
		win := windows[i]
		if !win.IsVisible() || win.IsMinimized() {
			continue
		}
		if win.Bounds().Contains(core.UnitPoint{X: x, Y: y}) {
			return win
		}
	}
	return nil
}

// ResizeCursorForEdge maps a set of resize edges to the cursor shape that
// signals resizing them (H/V for a single edge, the two diagonals for
// corners, default for none). Shared by the desktop WindowManager and the
// embedded MDIPane so both show the same resize cursors.
func ResizeCursorForEdge(edge int) core.CursorShape {
	left := edge&ResizeEdgeLeft != 0
	right := edge&ResizeEdgeRight != 0
	top := edge&ResizeEdgeTop != 0
	bottom := edge&ResizeEdgeBottom != 0
	switch {
	case (left && top) || (right && bottom):
		return core.CursorResizeNWSE // top-left / bottom-right diagonal
	case (right && top) || (left && bottom):
		return core.CursorResizeNESW // top-right / bottom-left diagonal
	case left || right:
		return core.CursorResizeH
	case top || bottom:
		return core.CursorResizeV
	default:
		return core.CursorDefault
	}
}

// CursorAt resolves the mouse cursor for a desktop-coordinate point: a
// resize cursor when over a window's size-sensitive edge, otherwise the
// cursor requested by the trinket under the pointer (e.g. a text I-beam),
// or the default arrow.
func (m *WindowManager) CursorAt(x, y core.Unit) core.CursorShape {
	win := m.topWindowAt(x, y)
	if win == nil {
		return core.CursorDefault
	}
	if s := ResizeCursorForEdge(m.detectResizeEdge(win, x, y)); s != core.CursorDefault {
		return s
	}
	b := win.Bounds()
	return win.CursorShapeAt(x-b.X, y-b.Y)
}

// ClearResizeHover removes the resize-edge highlight from every window.
// Called when the pointer leaves the surface, so no stale band lingers.
func (m *WindowManager) ClearResizeHover() {
	m.mu.RLock()
	windows := make([]*Window, len(m.windows))
	copy(windows, m.windows)
	m.mu.RUnlock()
	changed := false
	for _, win := range windows {
		if win.SetResizeHoverRects(nil) {
			changed = true
		}
	}
	if changed {
		m.RequestRepaint()
	}
}

// SetDesktop sets the desktop trinket (background behind windows).
func (m *WindowManager) SetDesktop(desktop core.Trinket) {
	m.mu.Lock()
	m.desktop = desktop
	bounds := m.screenBounds
	m.mu.Unlock()

	// Set the desktop bounds to the screen size
	if desktop != nil && !bounds.IsEmpty() {
		desktop.SetBounds(bounds)
	}
}

// Desktop returns the desktop trinket.
func (m *WindowManager) Desktop() core.Trinket {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.desktop
}

// SetScreenBounds sets the available screen area.
func (m *WindowManager) SetScreenBounds(bounds core.UnitRect) {
	m.mu.Lock()
	m.screenBounds = bounds
	desktop := m.desktop
	m.mu.Unlock()

	// Update desktop bounds
	if desktop != nil {
		desktop.SetBounds(bounds)
	}

	// Adjust maximized windows to client area
	clientArea := m.ClientArea()
	for _, win := range m.windows {
		if win.IsMaximized() {
			win.SetBounds(clientArea)
		}
	}
}

// ClientArea returns the area available for windows (excluding desktop chrome like menu bars).
// ClampToClientArea keeps a window within reach on re-dock or
// placement: title bar vertically inside the client area (below any
// menu bar, above any dock/status bar) and a couple of columns
// visible horizontally.
func (m *WindowManager) ClampToClientArea(bounds core.UnitRect) core.UnitRect {
	return clampWindowToClientArea(bounds, m.ClientArea(), m.ScreenCellMetrics())
}

// displayBounds returns where a window is drawn and hit-tested: its
// logical bounds corralled into the current client area. The corral is
// PROVISIONAL - never written back - so shrinking the desktop nudges an
// off-screen window into view, and growing it again lets the window
// re-spread to its original spot. A deliberate interaction commits the
// corral (see commitDisplayBounds). Maximized windows are exempt (they
// already track the client area).
func (m *WindowManager) displayBounds(win *Window) core.UnitRect {
	if win.IsMaximized() {
		return win.Bounds()
	}
	return clampWindowToClientArea(win.Bounds(), m.ClientArea(), m.ScreenCellMetrics())
}

func (m *WindowManager) ClientArea() core.UnitRect {
	m.mu.RLock()
	screen := m.screenBounds
	desktop := m.desktop
	m.mu.RUnlock()

	// If desktop has a ClientArea method, use it
	if da, ok := desktop.(interface{ ClientArea() core.UnitRect }); ok {
		return da.ClientArea()
	}

	return screen
}

// ScreenBounds returns the available screen area for popups.
// This returns the ClientArea (excluding desktop chrome like menu bars and dock)
// so popups are positioned within the visible window area.
func (m *WindowManager) ScreenBounds() core.UnitRect {
	return m.ClientArea()
}

// AddWindow adds a window to the manager.
func (m *WindowManager) AddWindow(win *Window) {
	m.mu.Lock()
	// Check if already added
	for _, w := range m.windows {
		if w == win {
			m.mu.Unlock()
			return
		}
	}
	m.windows = append(m.windows, win)
	// Add to cycle order (for M-Tab cycling)
	m.cycleOrder = append(m.cycleOrder, win)
	handler := m.onWindowAdded
	desktop := m.desktop
	smooth := m.smoothPositioning
	m.mu.Unlock()

	win.SetSmoothPositioning(smooth)

	// Set window's parent to desktop so trinkets can traverse up to find timer provider
	if desktop != nil {
		if container, ok := desktop.(core.Container); ok {
			win.SetParent(container)
			// Ancestry decides capability lookups (graphical frames,
			// smooth positioning, metrics): a window laid out before
			// joining the manager used cell-frame insets, so re-lay it
			// out under its real context.
			win.Layout()
		}
	}

	// Set up request callbacks so button clicks go through WindowManager
	win.SetOnMinimizeRequest(func() {
		m.MinimizeWindow(win)
	})
	win.SetOnMaximizeRequest(func() {
		if win.IsMaximized() {
			m.RestoreWindow(win)
		} else {
			m.MaximizeWindow(win)
		}
	})
	win.SetOnCloseComplete(func() {
		m.RemoveWindow(win)
	})
	win.SetGetConstrainingBounds(func() core.UnitRect {
		return m.ClientArea()
	})

	// Set popup controller on window and its content so trinkets can use overlays
	win.SetPopupController(m)
	if content := win.Content(); content != nil {
		m.setPopupControllerRecursive(content)
	}

	// Position if not explicitly set (X and Y both at default 0)
	bounds := win.Bounds()
	if bounds.X == 0 && bounds.Y == 0 {
		m.positionWindow(win)
	}

	// Activate
	m.ActivateWindow(win)

	if handler != nil {
		handler(win)
	}
}

// setPopupControllerRecursive sets this WindowManager as the popup controller
// for a trinket and all its descendants.
func (m *WindowManager) setPopupControllerRecursive(trinket core.Trinket) {
	stampPopupController(trinket, m)
}

// stampPopupController assigns the popup controller to a trinket and
// its whole subtree. The WindowManager stamps windows it manages; a
// TearOffHost stamps its torn window so popups (combobox dropdowns,
// context menus) open on the torn surface instead of the desktop's.
func stampPopupController(trinket core.Trinket, pc core.PopupController) {
	if setter, ok := trinket.(interface{ SetPopupController(core.PopupController) }); ok {
		setter.SetPopupController(pc)
	}
	// Prefer AllChildren over Children: a TabTrinket's Children() is only the
	// active tab, so a combobox on an inactive tab would keep a stale
	// controller (e.g. the desktop's, from when its tab was last active) and
	// open its popup on the wrong surface after the window is torn off.
	if ac, ok := trinket.(interface{ AllChildren() []core.Trinket }); ok {
		for _, child := range ac.AllChildren() {
			stampPopupController(child, pc)
		}
		return
	}
	if container, ok := trinket.(core.Container); ok {
		for _, child := range container.Children() {
			stampPopupController(child, pc)
		}
	}
}

// RemoveWindow removes a window from the manager.
func (m *WindowManager) RemoveWindow(win *Window) {
	m.mu.Lock()
	for i, w := range m.windows {
		if w == win {
			m.windows = append(m.windows[:i], m.windows[i+1:]...)
			break
		}
	}

	// Remove from cycle order
	for i, it := range m.cycleOrder {
		if it == win {
			m.cycleOrder = append(m.cycleOrder[:i], m.cycleOrder[i+1:]...)
			break
		}
	}

	// Remove from modal stack
	for i, w := range m.modalStack {
		if w == win {
			m.modalStack = append(m.modalStack[:i], m.modalStack[i+1:]...)
			break
		}
	}

	// Update active window
	wasActive := m.activeWindow == win
	var newActive *Window
	if wasActive {
		m.activeWindow = nil
		if len(m.windows) > 0 {
			newActive = m.windows[len(m.windows)-1]
			m.activeWindow = newActive
		}
	}

	handler := m.onWindowRemoved
	activeHandler := m.onActiveChanged
	m.mu.Unlock()

	// Deactivate the removed window
	if wasActive {
		win.SetActive(false)
	}

	// Activate the new active window
	if newActive != nil {
		newActive.SetActive(true)
		// Focus the window's first trinket if no trinket is focused
		if fm := newActive.FocusManager(); fm != nil {
			if fm.FocusedTrinket() == nil {
				fm.FocusFirst()
			}
		}
	}

	if handler != nil {
		handler(win)
	}
	if activeHandler != nil && wasActive {
		activeHandler(newActive)
	}
}

// Windows returns all windows in z-order.
func (m *WindowManager) Windows() []*Window {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Window, len(m.windows))
	copy(result, m.windows)
	return result
}

// ActiveWindow returns the currently active window.
func (m *WindowManager) ActiveWindow() *Window {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeWindow
}

// PreviousActiveWindow returns the window that was active before the menu bar was activated.
func (m *WindowManager) PreviousActiveWindow() *Window {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.previousActiveWindow
}

// ActivateWindow brings a window to the front and gives it focus.
func (m *WindowManager) ActivateWindow(win *Window) {
	m.mu.Lock()
	if win == m.activeWindow {
		m.mu.Unlock()
		return
	}

	// Check if blocked by modal
	if len(m.modalStack) > 0 {
		topModal := m.modalStack[len(m.modalStack)-1]
		if win != topModal && !m.isChildOf(win, topModal) {
			m.mu.Unlock()
			return
		}
	}

	oldActive := m.activeWindow
	m.activeWindow = win

	// Move to top of z-order
	m.bringToFront(win)

	// Move to front of cycle order (for M-Tab cycling)
	m.bringToCycleFront(win)

	handler := m.onActiveChanged
	desktop := m.desktop
	m.mu.Unlock()

	// Deactivate menu bar and dock when a window becomes active
	if desktop != nil {
		if deactivator, ok := desktop.(interface{ DeactivateMenuBar() }); ok {
			deactivator.DeactivateMenuBar()
		}
		if dockProvider, ok := desktop.(DockProvider); ok {
			dockProvider.UnfocusDock()
		}
	}

	// Update active states (SetActive handles the onActivate callback)
	if oldActive != nil {
		oldActive.SetActive(false)
	}
	if win != nil {
		win.SetActive(true)
		// Focus the window's first trinket if no trinket is focused
		if fm := win.FocusManager(); fm != nil {
			if fm.FocusedTrinket() == nil {
				fm.FocusFirst()
			}
		}
	}

	if handler != nil {
		handler(win)
	}
}

// DeactivateActiveWindow removes focus from the active window without closing it.
// This is used when the menu bar becomes active. The deactivated window is remembered
// so it can be restored when the menu bar is dismissed.
func (m *WindowManager) DeactivateActiveWindow() {
	m.mu.Lock()
	oldActive := m.activeWindow
	if oldActive == nil {
		m.mu.Unlock()
		return
	}

	m.previousActiveWindow = oldActive
	m.activeWindow = nil
	handler := m.onActiveChanged
	m.mu.Unlock()

	oldActive.SetActive(false)

	if handler != nil {
		handler(nil)
	}
}

// RestorePreviousActiveWindow activates the previously active window if one was remembered.
// This is used when the menu bar is dismissed via Escape.
func (m *WindowManager) RestorePreviousActiveWindow() {
	m.mu.Lock()
	prev := m.previousActiveWindow
	m.previousActiveWindow = nil
	m.mu.Unlock()

	if prev != nil {
		m.ActivateWindow(prev)
	}
}

// FocusWindow gives a window focus without raising it to the front.
// This is used for focus-follows-click behavior where the window only
// raises on mouse release within its bounds.
func (m *WindowManager) FocusWindow(win *Window) {
	m.mu.Lock()
	if win == m.activeWindow {
		m.mu.Unlock()
		return
	}

	// Check if blocked by modal
	if len(m.modalStack) > 0 {
		topModal := m.modalStack[len(m.modalStack)-1]
		if win != topModal && !m.isChildOf(win, topModal) {
			m.mu.Unlock()
			return
		}
	}

	oldActive := m.activeWindow
	m.activeWindow = win
	// Note: bringToFront is NOT called here - window stays in current z-order

	handler := m.onActiveChanged
	m.mu.Unlock()

	// Update active states (SetActive handles the onActivate callback)
	if oldActive != nil {
		oldActive.SetActive(false)
	}
	if win != nil {
		win.SetActive(true)
		// Focus the window's first trinket if no trinket is focused
		if fm := win.FocusManager(); fm != nil {
			if fm.FocusedTrinket() == nil {
				fm.FocusFirst()
			}
		}
	}

	if handler != nil {
		handler(win)
	}
}

// RaiseWindow brings a window to the front without changing focus.
func (m *WindowManager) RaiseWindow(win *Window) {
	m.mu.Lock()
	m.bringToFront(win)
	m.mu.Unlock()
	m.RequestRepaint()
}

// bringToFront moves a window to the top of the z-order.
func (m *WindowManager) bringToFront(win *Window) {
	for i, w := range m.windows {
		if w == win {
			// Remove from current position
			m.windows = append(m.windows[:i], m.windows[i+1:]...)
			// Add to end (top)
			m.windows = append(m.windows, win)
			return
		}
	}
}

// bringToCycleFront moves an item to the front (end) of the cycle order.
// item should be *Window or nil (nil represents the dock).
func (m *WindowManager) bringToCycleFront(item interface{}) {
	// Remove existing occurrence
	for i, it := range m.cycleOrder {
		if it == item {
			m.cycleOrder = append(m.cycleOrder[:i], m.cycleOrder[i+1:]...)
			break
		}
	}
	// Add to end (most recently activated)
	m.cycleOrder = append(m.cycleOrder, item)
}

// isChildOf checks if child is a descendant of parent.
func (m *WindowManager) isChildOf(child, parent *Window) bool {
	for p := child.ParentWindow(); p != nil; p = p.ParentWindow() {
		if p == parent {
			return true
		}
	}
	return false
}

// ShowModal shows a window as modal (blocks other windows).
func (m *WindowManager) ShowModal(win *Window) {
	m.mu.Lock()
	win.SetFlags(win.Flags() | WindowFlagModal)
	m.modalStack = append(m.modalStack, win)
	m.mu.Unlock()

	m.AddWindow(win)
}

// CloseModal closes the top modal window.
func (m *WindowManager) CloseModal() {
	m.mu.Lock()
	if len(m.modalStack) == 0 {
		m.mu.Unlock()
		return
	}
	win := m.modalStack[len(m.modalStack)-1]
	m.modalStack = m.modalStack[:len(m.modalStack)-1]
	m.mu.Unlock()

	win.Close()
}

// MaximizeWindow maximizes a window to fill the client area. Windows that
// can't be maximized (NoMaximize, or NoResize since maximizing is a
// resize) are left untouched, so callers - double-click, drag-to-top
// snap - don't silently resize a fixed-size dialog.
func (m *WindowManager) MaximizeWindow(win *Window) {
	if !canMaximize(win.Flags()) {
		return
	}
	clientArea := m.ClientArea()
	win.Maximize()
	win.SetBounds(clientArea)
}

// MinimizeWindow minimizes a window.
func (m *WindowManager) MinimizeWindow(win *Window) {
	win.Minimize()

	// Notify via callback (for dock row integration)
	m.mu.RLock()
	handler := m.onWindowMinimized
	m.mu.RUnlock()

	if handler != nil {
		handler(win)
	}

	m.RequestRepaint()
}

// RestoreWindow restores a minimized window.
func (m *WindowManager) RestoreWindow(win *Window) {
	win.Restore()
	m.ActivateWindow(win)

	// Notify via callback (for dock row integration)
	m.mu.RLock()
	handler := m.onWindowRestored
	m.mu.RUnlock()

	if handler != nil {
		handler(win)
	}

	m.RequestRepaint()
}

// SetOnWindowMinimized sets the callback for window minimization.
func (m *WindowManager) SetOnWindowMinimized(handler func(*Window)) {
	m.mu.Lock()
	m.onWindowMinimized = handler
	m.mu.Unlock()
}

// SetOnWindowRestored sets the callback for window restoration.
func (m *WindowManager) SetOnWindowRestored(handler func(*Window)) {
	m.mu.Lock()
	m.onWindowRestored = handler
	m.mu.Unlock()
}

// RegisterPopup implements core.PopupController.
// It registers a popup overlay to be painted on top of all windows.
func (m *WindowManager) RegisterPopup(request *core.PopupRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Remove any existing popup with the same ID
	for i, p := range m.popups {
		if p.ID == request.ID {
			m.popups = append(m.popups[:i], m.popups[i+1:]...)
			break
		}
	}
	// Convert core.PopupRequest to internal PopupOverlay
	overlay := &PopupOverlay{
		ID:                 request.ID,
		Bounds:             request.Bounds,
		Paint:              request.Paint,
		HandleMousePress:   request.HandleMousePress,
		HandleMouseMove:    request.HandleMouseMove,
		HandleMouseRelease: request.HandleMouseRelease,
		HandleMouseWheel:   request.HandleMouseWheel,
	}
	m.popups = append(m.popups, overlay)
}

// UnregisterPopup removes a popup overlay by ID.
func (m *WindowManager) UnregisterPopup(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, p := range m.popups {
		if p.ID == id {
			m.popups = append(m.popups[:i], m.popups[i+1:]...)
			return
		}
	}
}

// HasPopups returns true if there are any registered popups.
func (m *WindowManager) HasPopups() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.popups) > 0
}

// MapToScreen implements core.PopupController.
func (m *WindowManager) MapToScreen(trinket core.Trinket, local core.UnitPoint) core.UnitPoint {
	return MapTrinketToScreen(trinket, local)
}

// MapTrinketToScreen converts local trinket coordinates to surface
// coordinates by walking the ancestry, exchanging denominations at
// each re-denominating container boundary. Pure ancestry - both the
// WindowManager and a TearOffHost use it.
func MapTrinketToScreen(trinket core.Trinket, local core.UnitPoint) core.UnitPoint {
	// Traverse up the trinket hierarchy to accumulate offsets.
	// Each trinket's Bounds().X/Y is its position within its parent,
	// denominated in the parent's currency. The accumulated point is
	// kept in the currency of the space it currently describes.
	result := local

	current := trinket
	for current != nil {
		parent := current.Parent()

		// Leaving a container that re-denominates its interior: the
		// accumulated point is in its interior currency; re-express it
		// in the outer currency its bounds live in. (Windows exchange
		// in the parent branch below, where the client-area offset must
		// be added in the outer currency - skip them here.)
		//
		// The ROOT (a container with no parent - the desktop, or a torn
		// host) is skipped: its denomination IS the screen currency, so
		// its coordinates are final. Exchanging there would rescale every
		// mapped point by root/DefaultCellMetrics - which is exactly what
		// broke once the desktop's own denomination stopped being 8x16
		// (font_size), sending popups to the wrong place.
		if _, isWin := current.(*Window); !isWin && current.Parent() != nil {
			if mp, ok := current.(core.CellMetricsProvider); ok {
				if ov := mp.CellMetricsOverride(); ov != nil {
					outer := core.ParentCellMetrics(current)
					result.X = core.ExchangeX(result.X, *ov, outer)
					result.Y = core.ExchangeY(result.Y, *ov, outer)
				}
			}
		}

		bounds := current.Bounds()
		result.X += bounds.X
		result.Y += bounds.Y

		if parent == nil {
			break
		}

		// Check if parent is a scroll container and adjust for scroll
		// offset. Unit-denominated scrollers (smooth surfaces) report
		// units directly; classic scrollers report cells of their own
		// denomination.
		if su, ok := parent.(core.ScrollOffsetUnitsProvider); ok {
			ox, oy := su.ScrollOffsetUnits()
			result.X -= ox
			result.Y -= oy
		} else if scroller, ok := parent.(core.ScrollOffsetProvider); ok {
			pm := core.DefaultCellMetrics()
			if pw, ok := parent.(core.Trinket); ok {
				pm = core.FindEffectiveCellMetrics(pw)
			}
			scrollX, scrollY := scroller.ScrollOffset()
			result.X -= core.Unit(scrollX) * pm.CellWidth
			result.Y -= core.Unit(scrollY) * pm.CellHeight
		}

		// Crossing a window's content boundary: content coordinates are
		// interior currency; exchange to the window's outer currency,
		// then add the client-area offset (outer currency).
		if win, ok := parent.(*Window); ok {
			outer, interior := win.denominations()
			result.X = core.ExchangeX(result.X, interior, outer)
			result.Y = core.ExchangeY(result.Y, interior, outer)
			offset := win.ClientAreaOffset()
			result.X += offset.X
			result.Y += offset.Y
		}

		if pw, ok := parent.(core.Trinket); ok {
			current = pw
		} else {
			break
		}
	}

	return result
}

// ScreenCellMetrics returns the grid metrics of the screen/desktop
// surface - the denomination popup overlays are composited in.
func (m *WindowManager) ScreenCellMetrics() core.CellMetrics {
	m.mu.RLock()
	desktop := m.desktop
	m.mu.RUnlock()
	if dw, ok := desktop.(core.Trinket); ok && dw != nil {
		return core.FindEffectiveCellMetrics(dw)
	}
	return core.DefaultCellMetrics()
}

// trinketIsInWindow checks if a trinket is contained within a window.
func (m *WindowManager) trinketIsInWindow(trinket core.Trinket, win *Window) bool {
	current := trinket
	for current != nil {
		if current == win.Content() {
			return true
		}
		parent := current.Parent()
		if parent == nil {
			break
		}
		if pw, ok := parent.(core.Trinket); ok {
			current = pw
		} else {
			break
		}
	}
	return false
}

// positionWindow positions a new window using cascading.
func (m *WindowManager) positionWindow(win *Window) {
	m.mu.RLock()
	numWindows := len(m.windows)
	m.mu.RUnlock()

	clientArea := m.ClientArea()
	metrics := core.DefaultCellMetrics()

	// Use the window's current size if set, otherwise use SizeHint
	bounds := win.Bounds()
	width := bounds.Width
	height := bounds.Height
	if width <= 0 || height <= 0 {
		hint := win.SizeHint()
		width = hint.Width
		height = hint.Height
	}

	// Cascade offset (numWindows-1 because the window was already added to the list)
	cascadeIndex := numWindows - 1
	if cascadeIndex < 0 {
		cascadeIndex = 0
	}
	offset := core.Unit(cascadeIndex) * metrics.CellWidth * 2

	x := clientArea.X + offset
	y := clientArea.Y + offset

	// Wrap if off screen
	if x+width > clientArea.X+clientArea.Width {
		x = clientArea.X
	}
	if y+height > clientArea.Y+clientArea.Height {
		y = clientArea.Y
	}

	win.SetBounds(core.UnitRect{
		X:      x,
		Y:      y,
		Width:  width,
		Height: height,
	})
}

// TileWindows arranges windows in a tiled layout.
func (m *WindowManager) TileWindows() {
	m.mu.RLock()
	windows := make([]*Window, 0)
	for _, w := range m.windows {
		if w.IsVisible() && !w.IsMinimized() {
			windows = append(windows, w)
		}
	}
	m.mu.RUnlock()

	if len(windows) == 0 {
		return
	}

	clientArea := m.ClientArea()
	metrics := core.DefaultCellMetrics()

	// Calculate grid dimensions
	cols := 1
	rows := len(windows)
	for cols*cols < len(windows) {
		cols++
	}
	rows = (len(windows) + cols - 1) / cols

	// Align tile sizes to cell boundaries
	cellWidth := metrics.RoundDownToCellX(clientArea.Width / core.Unit(cols))
	cellHeight := metrics.RoundDownToCellY(clientArea.Height / core.Unit(rows))

	for i, win := range windows {
		col := i % cols
		row := i / cols

		win.SetBounds(core.UnitRect{
			X:      clientArea.X + core.Unit(col)*cellWidth,
			Y:      clientArea.Y + core.Unit(row)*cellHeight,
			Width:  cellWidth,
			Height: cellHeight,
		})
		win.Restore()
	}
}

// CascadeWindows arranges windows in a cascade.
func (m *WindowManager) CascadeWindows() {
	m.mu.RLock()
	windows := make([]*Window, 0)
	for _, w := range m.windows {
		if w.IsVisible() && !w.IsMinimized() {
			windows = append(windows, w)
		}
	}
	m.mu.RUnlock()

	if len(windows) == 0 {
		return
	}

	clientArea := m.ClientArea()
	metrics := core.DefaultCellMetrics()
	// The cascade step includes the frame border, so each window's whole
	// top chrome (border + titlebar) clears the one beneath it.
	border := core.FindFrameBorderUnits(windows[0])
	offset := metrics.CellWidth*2 + border

	// Standard size for cascaded windows - align to cell boundaries
	width := metrics.RoundDownToCellX(clientArea.Width * 3 / 4)
	height := metrics.RoundDownToCellY(clientArea.Height * 3 / 4)

	for i, win := range windows {
		// Leave any maximized/minimized state before positioning, so the
		// cascade bounds stick (Restore would otherwise overwrite them).
		win.Restore()

		x := clientArea.X + core.Unit(i)*offset
		y := clientArea.Y + core.Unit(i)*offset

		// A window that can't be resized is only repositioned, keeping its
		// own size; only resizable windows adopt the standard cascade size.
		w, h := width, height
		if win.Flags()&WindowFlagNoResize != 0 {
			b := win.Bounds()
			w, h = b.Width, b.Height
		}

		// Wrap if off screen
		if x+w > clientArea.X+clientArea.Width {
			x = clientArea.X
		}
		if y+h > clientArea.Y+clientArea.Height {
			y = clientArea.Y
		}

		win.SetBounds(core.UnitRect{
			X:      x,
			Y:      y,
			Width:  w,
			Height: h,
		})
	}
}

// HandleMousePress processes mouse events for windows.
func (m *WindowManager) HandleMousePress(event core.MousePressEvent) bool {
	m.mu.Lock()
	if m.dragging != nil && m.dragNeedsButton {
		// A press while an armed-without-press drag is live means its
		// release was lost: disarm and process the press normally.
		m.dragging = nil
		m.dragNeedsButton = false
	}
	m.mu.Unlock()

	m.mu.RLock()
	windows := m.windows
	desktop := m.desktop
	popups := m.popups
	m.mu.RUnlock()

	// Check popups first (highest z-order)
	for i := len(popups) - 1; i >= 0; i-- {
		popup := popups[i]
		if popup.Bounds.Contains(core.UnitPoint{X: event.X, Y: event.Y}) {
			if popup.HandleMousePress != nil {
				return popup.HandleMousePress(event)
			}
			return true // Consume click even if no handler
		}
	}

	// If there are popups but click was outside them, close all popups
	if len(popups) > 0 {
		m.mu.Lock()
		m.popups = nil
		m.mu.Unlock()
		m.RequestRepaint()
		// Don't consume the click - let it propagate to close the underlying popup source
	}

	// Check if click is within an active menu dropdown (rendered on top of windows)
	// Menu dropdowns have higher z-order than windows, so check them first
	if desktop != nil {
		if menuBoundsGetter, ok := desktop.(interface {
			ActiveMenuBounds() core.UnitRect
		}); ok {
			menuBounds := menuBoundsGetter.ActiveMenuBounds()
			if !menuBounds.IsEmpty() && menuBounds.Contains(core.UnitPoint{X: event.X, Y: event.Y}) {
				// Click is on the menu dropdown - pass to desktop for menu handling
				return desktop.HandleMousePress(event)
			}
		}
	}

	// Check if click is in the dock area (dock has higher z-order than windows)
	if desktop != nil {
		if dockBoundsGetter, ok := desktop.(interface {
			DockBounds() core.UnitRect
		}); ok {
			dockBounds := dockBoundsGetter.DockBounds()
			if !dockBounds.IsEmpty() && dockBounds.Contains(core.UnitPoint{X: event.X, Y: event.Y}) {
				// Click is on the dock - pass to desktop for dock handling
				return desktop.HandleMousePress(event)
			}
		}
	}

	// Check if click is in the status bar area (status bar has higher z-order than windows)
	if desktop != nil {
		if statusBarBoundsGetter, ok := desktop.(interface {
			StatusBarBounds() core.UnitRect
		}); ok {
			statusBarBounds := statusBarBoundsGetter.StatusBarBounds()
			if !statusBarBounds.IsEmpty() && statusBarBounds.Contains(core.UnitPoint{X: event.X, Y: event.Y}) {
				// Click is on the status bar - pass to desktop for status bar handling
				return desktop.HandleMousePress(event)
			}
		}
	}

	// Check windows from top to bottom
	for i := len(windows) - 1; i >= 0; i-- {
		win := windows[i]
		if !win.IsVisible() || win.IsMinimized() {
			continue
		}

		bounds := m.displayBounds(win)
		if bounds.Contains(core.UnitPoint{X: event.X, Y: event.Y}) {
			// Deliberate interaction: commit the provisional corral so
			// this window's displayed position becomes its real one and
			// all downstream geometry (resize edges, drag offsets) agrees.
			win.SetBounds(bounds)

			// Close any active menu before processing window click
			if desktop != nil {
				if menuCloser, ok := desktop.(interface {
					CloseActiveMenu()
				}); ok {
					menuCloser.CloseActiveMenu()
				}
			}

			// Check for resize edge first - resize operations raise immediately
			resizeEdge := m.detectResizeEdge(win, event.X, event.Y)
			if resizeEdge != ResizeEdgeNone {
				// Activate (focus + raise) for resize
				m.ActivateWindow(win)
				// Start resize
				m.mu.Lock()
				m.resizing = win
				m.resizeEdge = resizeEdge
				m.resizeStartX = event.X
				m.resizeStartY = event.Y
				m.resizeOriginal = bounds
				m.pressedWindow = nil // Clear pressed window for resize
				m.mu.Unlock()
				return true
			}

			// Check for title bar interaction - titlebar operations raise
			// immediately. The titlebar sits below the top frame border, so
			// the drag region covers the border AND the titlebar row.
			metrics := core.DefaultCellMetrics()
			titleTop := core.FindFrameBorderUnits(win)
			if event.Y < bounds.Y+titleTop+metrics.CellHeight &&
				win.Flags()&WindowFlagNoTitle == 0 {

				// Activate (focus + raise) for titlebar interaction
				m.ActivateWindow(win)

				// The tear handle is draggable AND clickable: grab it to
				// begin a tear-capable drag; a release in place is a click
				// that toggles detach/dock.
				if win.Flags()&WindowFlagTearable != 0 &&
					win.buttonAtPosition(event.X-bounds.X, event.Y-bounds.Y) == TitleButtonTear {
					m.mu.Lock()
					m.dragging = win
					m.dragStartX = event.X
					m.dragStartY = event.Y
					m.dragOffsetX = event.X - bounds.X
					m.dragOffsetY = event.Y - bounds.Y
					m.dragIsTearHandle = true
					m.dragMoved = false
					m.dragNeedsButton = false
					m.pressedWindow = nil
					m.mu.Unlock()
					win.SetTearHighlight(true) // Show the tear-off halo while grabbed.
					return true
				}

				// First, let the window handle button clicks (close, minimize, maximize)
				// Pass the event to the window - if it handles a button click, don't drag
				localEvent := event
				localEvent.X -= bounds.X
				localEvent.Y -= bounds.Y
				if win.HandleMousePress(localEvent) {
					// Window handled it (button click) - update click tracking but don't drag
					m.mu.Lock()
					m.lastClickTime = time.Now()
					m.lastClickX = event.X
					m.lastClickY = event.Y
					m.lastClickWindow = win
					m.pressedWindow = nil
					m.mu.Unlock()
					return true
				}

				// Check for double-click on titlebar (for maximize/restore)
				now := time.Now()
				m.mu.Lock()
				isDoubleClick := m.lastClickWindow == win &&
					now.Sub(m.lastClickTime) < 400*time.Millisecond &&
					abs(int(event.X-m.lastClickX)) < int(metrics.CellWidth) &&
					abs(int(event.Y-m.lastClickY)) < int(metrics.CellHeight)

				// Update last click info
				m.lastClickTime = now
				m.lastClickX = event.X
				m.lastClickY = event.Y
				m.lastClickWindow = win
				m.mu.Unlock()

				if isDoubleClick && win.Flags()&WindowFlagNoMaximize == 0 {
					if win.IsMaximized() {
						win.Restore()
					} else {
						m.MaximizeWindow(win)
					}
					// Clear double-click state so next click starts fresh
					m.mu.Lock()
					m.lastClickWindow = nil
					m.pressedWindow = nil
					m.mu.Unlock()
					return true
				}

				// Start drag (if movable)
				if win.Flags()&WindowFlagNoMove == 0 {
					m.mu.Lock()
					m.dragging = win
					m.dragStartX = event.X
					m.dragStartY = event.Y
					m.dragOffsetX = event.X - bounds.X
					m.dragOffsetY = event.Y - bounds.Y
					m.dragNeedsButton = false
					m.dragIsTearHandle = false
					m.dragMoved = false
					m.pressedWindow = nil // Clear pressed window for drag
					m.mu.Unlock()
				}
				return true
			}

			// Content area click: focus without raise (raise on release within bounds)
			m.FocusWindow(win)
			m.mu.Lock()
			m.pressedWindow = win
			m.mu.Unlock()

			// Pass to window
			localEvent := event
			localEvent.X -= bounds.X
			localEvent.Y -= bounds.Y
			return win.HandleMousePress(localEvent)
		}
	}

	// Check desktop (already read above, but re-read in case it changed)
	if desktop != nil {
		return desktop.HandleMousePress(event)
	}

	return false
}

// HandleMouseMove processes mouse movement for dragging.
func (m *WindowManager) HandleMouseMove(event core.MouseMoveEvent) bool {
	m.mu.Lock()
	dragging := m.dragging
	offsetX := m.dragOffsetX
	offsetY := m.dragOffsetY
	resizing := m.resizing
	resizeEdge := m.resizeEdge
	resizeStartX := m.resizeStartX
	resizeStartY := m.resizeStartY
	resizeOriginal := m.resizeOriginal
	popups := m.popups
	m.mu.Unlock()

	// Check popups first (highest z-order) - only when not window dragging/resizing
	if dragging == nil && resizing == nil {
		for i := len(popups) - 1; i >= 0; i-- {
			popup := popups[i]
			if popup.HandleMouseMove != nil {
				if popup.HandleMouseMove(event) {
					return true
				}
			}
		}
	}

	// Handle resize
	if resizing != nil {
		metrics := core.DefaultCellMetrics()

		deltaX := event.X - resizeStartX
		deltaY := event.Y - resizeStartY

		newBounds := resizeOriginal

		// Apply resize based on which edges are being dragged
		if resizeEdge&ResizeEdgeLeft != 0 {
			newBounds.X = resizeOriginal.X + deltaX
			newBounds.Width = resizeOriginal.Width - deltaX
		}
		if resizeEdge&ResizeEdgeRight != 0 {
			newBounds.Width = resizeOriginal.Width + deltaX
		}
		if resizeEdge&ResizeEdgeTop != 0 {
			newBounds.Y = resizeOriginal.Y + deltaY
			newBounds.Height = resizeOriginal.Height - deltaY
		}
		if resizeEdge&ResizeEdgeBottom != 0 {
			newBounds.Height = resizeOriginal.Height + deltaY
		}

		// Align to cell boundaries (skipped on pixel surfaces, where
		// windows resize smoothly at unit granularity)
		if !m.SmoothPositioning() {
			newBounds = metrics.AlignRect(newBounds)
		}

		// Enforce minimum window size (at least 3 cells wide, 2 cells tall)
		minWidth := metrics.CellWidth * 3
		minHeight := metrics.CellHeight * 2
		if newBounds.Width < minWidth {
			if resizeEdge&ResizeEdgeLeft != 0 {
				newBounds.X = resizeOriginal.X + resizeOriginal.Width - minWidth
			}
			newBounds.Width = minWidth
		}
		if newBounds.Height < minHeight {
			if resizeEdge&ResizeEdgeTop != 0 {
				newBounds.Y = resizeOriginal.Y + resizeOriginal.Height - minHeight
			}
			newBounds.Height = minHeight
		}

		// Keep window on screen (don't allow left edge or top to go off screen)
		clientArea := m.ClientArea()
		if newBounds.X < clientArea.X {
			if resizeEdge&ResizeEdgeLeft != 0 {
				// Resizing from left - adjust width instead
				newBounds.Width = resizeOriginal.X + resizeOriginal.Width - clientArea.X
			}
			newBounds.X = clientArea.X
		}
		if newBounds.Y < clientArea.Y {
			if resizeEdge&ResizeEdgeTop != 0 {
				// Resizing from top - adjust height instead
				newBounds.Height = resizeOriginal.Y + resizeOriginal.Height - clientArea.Y
			}
			newBounds.Y = clientArea.Y
		}

		// Limit height to client area height (windows can be wider but not taller)
		if newBounds.Height > clientArea.Height {
			newBounds.Height = clientArea.Height
		}

		resizing.SetBounds(newBounds)
		// Keep the edge highlight on the edge being dragged, tracking the
		// window's new size instead of leaving it stale at the start bounds.
		resizing.SetResizeHoverRects(m.resizeEdgeRects(resizing, resizeEdge))
		m.RequestRepaint()
		return true
	}

	// Handle drag
	if dragging != nil {
		m.mu.RLock()
		needsButton := m.dragNeedsButton
		m.mu.RUnlock()
		if needsButton && event.Buttons&core.LeftButton == 0 {
			// The release was lost in another surface (re-dock
			// hand-off): the gesture is over, stop following hovers.
			m.mu.Lock()
			if m.dragging == dragging {
				m.dragging = nil
				m.dragNeedsButton = false
			}
			m.mu.Unlock()
			dragging.SetTearHighlight(false)
			return true
		}

		// Any motion during a drag marks it moved (a handle press that
		// never moves is a click, not a drag).
		m.mu.Lock()
		if m.dragging == dragging {
			m.dragMoved = true
		}
		isTearHandle := m.dragIsTearHandle
		m.mu.Unlock()

		// Tear-off: past the surface edge, the host may lift the window
		// out into its own OS surface (G4 granting) - but ONLY when the
		// drag was begun on the tear handle. A plain title drag just
		// moves the window in-surface.
		m.mu.RLock()
		tear := m.tearOff
		screen := m.screenBounds
		m.mu.RUnlock()
		if tear != nil && isTearHandle && !dragging.IsMaximized() &&
			(event.X < screen.X || event.Y < screen.Y ||
				event.X >= screen.X+screen.Width || event.Y >= screen.Y+screen.Height) {
			if tear(dragging, event, offsetX, offsetY) {
				m.mu.Lock()
				if m.dragging == dragging {
					m.dragging = nil
				}
				m.mu.Unlock()
				dragging.SetTearHighlight(false)
				return true
			}
		}

		// Track if we just restored from maximized (to avoid immediate re-maximize)
		justRestored := false

		// Constrain to client area (below menu bar, above status bar)
		clientArea := m.ClientArea()
		metrics := core.DefaultCellMetrics()

		// If window is maximized, only restore if dragging DOWN (below menu bar)
		// Dragging left/right while in menu bar area keeps window maximized
		if dragging.IsMaximized() {
			// Calculate where the window would be positioned
			newY := event.Y - offsetY

			// Only restore if dragging below the menu bar
			if newY >= clientArea.Y {
				// Get the normalized bounds before restore
				oldBounds := dragging.Bounds()

				// Restore the window
				dragging.Restore()
				justRestored = true
				newBounds := dragging.Bounds()

				// Force layout recalculation for the restored window state
				// This ensures content bounds are recalculated for normal mode (with borders)
				dragging.Layout()

				// Recalculate offset so the cursor stays proportionally positioned
				// on the titlebar (e.g., if you grabbed the middle, keep it middle)
				proportion := float64(offsetX) / float64(oldBounds.Width)
				offsetX = core.Unit(proportion * float64(newBounds.Width))

				// Update stored offset
				m.mu.Lock()
				m.dragOffsetX = offsetX
				m.mu.Unlock()
			} else {
				// Still in menu bar area - keep maximized, don't process further
				return true
			}
		}

		// Move window
		newX := event.X - offsetX
		newY := event.Y - offsetY

		bounds := dragging.Bounds()
		bounds.X = newX
		bounds.Y = newY

		// Dragging into menu bar area = maximize gesture. Skipped for a
		// tear-handle drag: that gesture tears the window off, so it
		// must not snap-maximize on the way up. Also skipped for windows
		// that can't be maximized (fixed-size dialogs), which then fall
		// through to the normal clamped move.
		if !isTearHandle && bounds.Y < clientArea.Y && canMaximize(dragging.Flags()) && !justRestored {
			if !dragging.IsMaximized() {
				m.MaximizeWindow(dragging)
				m.RequestRepaint()
			}
			return true
		}

		// Keep the window retrievable: title bar vertically within the
		// client area, at least a couple of columns visible horizontally
		// on each side (dragging it down to a few pixels made it
		// impossible to grab back).
		bounds = clampWindowToClientArea(bounds, clientArea, metrics)

		// Limit height to client area height (windows can be wider but not taller)
		if bounds.Height > clientArea.Height {
			bounds.Height = clientArea.Height
		}

		// Align position to cell boundaries (important after restore from
		// maximized); pixel surfaces drag smoothly at unit granularity
		if !m.SmoothPositioning() {
			bounds = metrics.AlignRect(bounds)
		}

		dragging.SetBounds(bounds)

		// Request repaint to show the window at its new position
		m.RequestRepaint()

		return true
	}

	// Not dragging or resizing: highlight the resize edge under the pointer.
	m.updateResizeHover(event.X, event.Y)

	// Forward to desktop first (for menu bar drag navigation)
	m.mu.RLock()
	desktop := m.desktop
	active := m.activeWindow
	m.mu.RUnlock()

	if desktop != nil {
		if handler, ok := desktop.(interface {
			HandleMouseMove(core.MouseMoveEvent) bool
		}); ok {
			if handler.HandleMouseMove(event) {
				return true
			}
		}
	}

	// Forward to active window (for splitter/trinket dragging, but not if minimized)
	if active != nil && !active.IsMinimized() {
		bounds := m.displayBounds(active)
		localEvent := event
		localEvent.X -= bounds.X
		localEvent.Y -= bounds.Y
		if active.HandleMouseMove(localEvent) {
			// Request repaint since trinket state may have changed
			m.RequestRepaint()
			return true
		}
	}

	return false
}

// HandleMouseRelease processes mouse button release.
func (m *WindowManager) HandleMouseRelease(event core.MouseReleaseEvent) bool {
	m.mu.Lock()
	dragging := m.dragging
	resizing := m.resizing
	pressedWin := m.pressedWindow
	popups := m.popups
	tearHandleClick := m.dragIsTearHandle && !m.dragMoved
	m.dragging = nil
	m.resizing = nil
	m.resizeEdge = ResizeEdgeNone
	m.pressedWindow = nil
	m.dragIsTearHandle = false
	m.dragMoved = false
	m.mu.Unlock()

	if dragging != nil || resizing != nil {
		// The tear-off halo only shows while the handle is grabbed.
		if dragging != nil {
			dragging.SetTearHighlight(false)
		}
		// A tear-handle press released in place is a click: toggle the
		// window between docked and detached (retaining position/size).
		if dragging != nil && tearHandleClick {
			dragging.requestTear()
		}
		return true
	}

	// Check popups first (highest z-order)
	for i := len(popups) - 1; i >= 0; i-- {
		popup := popups[i]
		if popup.HandleMouseRelease != nil {
			if popup.HandleMouseRelease(event) {
				return true
			}
		}
	}

	// Check if we should raise the pressed window (focus-without-raise behavior)
	// Only raise if release is over a non-occluded part of the window
	if pressedWin != nil && !pressedWin.IsMinimized() {
		bounds := m.displayBounds(pressedWin)
		releasePoint := core.UnitPoint{X: event.X, Y: event.Y}
		if bounds.Contains(releasePoint) {
			// Check that no other window is on top at this position
			m.mu.RLock()
			windows := m.windows
			m.mu.RUnlock()

			topmostAtPoint := (*Window)(nil)
			for i := len(windows) - 1; i >= 0; i-- {
				win := windows[i]
				if win.IsVisible() && !win.IsMinimized() && m.displayBounds(win).Contains(releasePoint) {
					topmostAtPoint = win
					break
				}
			}

			// Only raise if the pressed window is the topmost at the release point
			if topmostAtPoint == pressedWin {
				m.RaiseWindow(pressedWin)
			}
		}
	}

	// Forward to desktop first (for menu bar drag release)
	m.mu.RLock()
	desktop := m.desktop
	active := m.activeWindow
	m.mu.RUnlock()

	if desktop != nil {
		if handler, ok := desktop.(interface {
			HandleMouseRelease(core.MouseReleaseEvent) bool
		}); ok {
			if handler.HandleMouseRelease(event) {
				return true
			}
		}
	}

	// Forward to active window (for splitter/trinket release, but not if minimized)
	if active != nil && !active.IsMinimized() {
		bounds := m.displayBounds(active)
		localEvent := event
		localEvent.X -= bounds.X
		localEvent.Y -= bounds.Y
		if active.HandleMouseRelease(localEvent) {
			// Request repaint since trinket state may have changed
			m.RequestRepaint()
			return true
		}
	}

	return false
}

// HandleKeyPress processes keyboard events.
func (m *WindowManager) HandleKeyPress(event core.KeyPressEvent) bool {
	m.mu.RLock()
	active := m.activeWindow
	desktop := m.desktop
	m.mu.RUnlock()

	// Global shortcuts
	// Uses direct-key-handler naming: M- = Alt, C- = Ctrl, S- = Shift
	switch event.Key {
	case "M-Tab", "C-Tab":
		m.CycleWindows(true)
		return true
	case "M-S-Tab", "C-S-Tab":
		m.CycleWindows(false)
		return true
	case "F10":
		// F10 always goes to desktop for menu bar toggle
		if desktop != nil {
			return desktop.HandleKeyPress(event)
		}
	}

	// Alt+letter (M-<letter>) always goes to desktop first for menu accelerators
	if len(event.Key) == 3 && event.Key[0] == 'M' && event.Key[1] == '-' {
		letter := event.Key[2]
		if letter >= 'a' && letter <= 'z' {
			if desktop != nil {
				if desktop.HandleKeyPress(event) {
					return true
				}
			}
		}
	}

	// Check if desktop's menu bar is active (has focus or has open menu)
	// If so, send keys to desktop first to prevent window from intercepting
	if desktop != nil {
		if menuActive, ok := desktop.(interface{ IsMenuBarActive() bool }); ok && menuActive.IsMenuBarActive() {
			if desktop.HandleKeyPress(event) {
				return true
			}
		}
	}

	// Pass to active window first (but not if minimized)
	if active != nil && !active.IsMinimized() {
		if active.HandleKeyPress(event) {
			return true
		}
	}

	// Forward unhandled keys to desktop for menu bar and other desktop shortcuts
	if desktop != nil {
		return desktop.HandleKeyPress(event)
	}

	return false
}

// CycleWindows cycles through windows and the dock (if it has entries).
// Uses activation order: most recently activated item is at the end.
// The dock participates in this order like a window (nil in cycleOrder).
func (m *WindowManager) CycleWindows(forward bool) {
	m.mu.Lock()
	desktop := m.desktop
	cycleOrder := make([]interface{}, len(m.cycleOrder))
	copy(cycleOrder, m.cycleOrder)
	activeWindow := m.activeWindow
	m.mu.Unlock()

	// Check if dock is available and has entries
	var dockProvider DockProvider
	hasDock := false
	isDockFocused := false
	if desktop != nil {
		if dp, ok := desktop.(DockProvider); ok {
			dockProvider = dp
			hasDock = dp.DockEntryCount() > 0
			isDockFocused = dp.IsDockFocused()
		}
	}

	// Build effective cycle list: non-minimized windows + dock (if has entries)
	// Filter cycleOrder to only include valid items
	var effectiveCycle []interface{}
	for _, item := range cycleOrder {
		if item == nil {
			// Dock - include only if it has entries
			if hasDock {
				effectiveCycle = append(effectiveCycle, nil)
			}
		} else if win, ok := item.(*Window); ok {
			// Window - include only if not minimized
			if !win.IsMinimized() {
				effectiveCycle = append(effectiveCycle, win)
			}
		}
	}

	// Add dock to cycle if it has entries but isn't in the order yet
	if hasDock {
		hasDockInCycle := false
		for _, item := range effectiveCycle {
			if item == nil {
				hasDockInCycle = true
				break
			}
		}
		if !hasDockInCycle {
			effectiveCycle = append(effectiveCycle, nil)
		}
	}

	// Nothing to cycle to
	if len(effectiveCycle) == 0 {
		return
	}

	// Find current position in cycle
	currentIdx := -1
	if isDockFocused {
		for i, item := range effectiveCycle {
			if item == nil {
				currentIdx = i
				break
			}
		}
	} else {
		for i, item := range effectiveCycle {
			if item == activeWindow {
				currentIdx = i
				break
			}
		}
	}

	// Default to end if not found
	if currentIdx < 0 {
		currentIdx = len(effectiveCycle) - 1
	}

	// Calculate next index with wrapping
	var nextIdx int
	if forward {
		nextIdx = (currentIdx + 1) % len(effectiveCycle)
	} else {
		nextIdx = (currentIdx - 1 + len(effectiveCycle)) % len(effectiveCycle)
	}

	// Activate the target
	nextItem := effectiveCycle[nextIdx]
	if nextItem == nil {
		// Moving to dock - deactivate current window first
		if activeWindow != nil {
			activeWindow.SetActive(false)
		}
		m.mu.Lock()
		m.activeWindow = nil
		m.bringToCycleFront(nil)
		m.mu.Unlock()
		if dockProvider != nil {
			dockProvider.FocusDock()
		}
		m.RequestRepaint()
	} else if win, ok := nextItem.(*Window); ok {
		// Moving to a window
		if isDockFocused && dockProvider != nil {
			dockProvider.UnfocusDock()
		}
		m.ActivateWindow(win)
	}
}

// Paint renders all windows.
func (m *WindowManager) Paint(p *core.Painter) {
	m.mu.RLock()
	desktop := m.desktop
	windows := m.windows
	m.mu.RUnlock()

	// Paint desktop
	if desktop != nil {
		desktop.Paint(p)
	}

	// Get client area to clip windows properly (avoid covering status bar)
	clientArea := m.ClientArea()

	// Paint windows from bottom to top, clipped to client area
	for _, win := range windows {
		if win.IsVisible() && !win.IsMinimized() {
			// Draw at the provisional (corralled) position so windows
			// left off-screen by a desktop shrink are nudged into view.
			bounds := m.displayBounds(win)

			// Calculate visible portion within client area
			visibleBounds := bounds.Intersection(clientArea)
			if visibleBounds.IsEmpty() {
				continue
			}

			// Tear-off affordance: a black halo just larger than the
			// window, drawn in desktop space (not clipped to the client
			// area) so a maximized window bleeds it over the menu and
			// status bars. Painted before the window so only the ring
			// beyond the frame shows.
			if win.TearIndicatorActive() {
				win.PaintTearHalo(p, bounds)
			}

			// Offset into window's local coordinates
			localClipX := visibleBounds.X - bounds.X
			localClipY := visibleBounds.Y - bounds.Y
			localClip := core.UnitRect{
				X:      localClipX,
				Y:      localClipY,
				Width:  visibleBounds.Width,
				Height: visibleBounds.Height,
			}

			windowPainter := p.WithOffset(bounds.X, bounds.Y).
				WithClip(localClip)
			win.Paint(windowPainter)
		}
	}

	// Paint menu dropdown on top of windows (if any is open)
	if desktop != nil {
		if dd, ok := desktop.(interface{ PaintMenuDropdown(*core.Painter) }); ok {
			dd.PaintMenuDropdown(p)
		}
	}

	// Paint registered popups on top of everything
	m.mu.RLock()
	popups := m.popups
	m.mu.RUnlock()
	for _, popup := range popups {
		if popup.Paint != nil {
			popup.Paint(p)
		}
	}
}

// SetOnWindowAdded sets the window added callback.
func (m *WindowManager) SetOnWindowAdded(handler func(*Window)) {
	m.mu.Lock()
	m.onWindowAdded = handler
	m.mu.Unlock()
}

// SetOnWindowRemoved sets the window removed callback.
func (m *WindowManager) SetOnWindowRemoved(handler func(*Window)) {
	m.mu.Lock()
	m.onWindowRemoved = handler
	m.mu.Unlock()
}

// SetOnActiveChanged sets the active window changed callback.
func (m *WindowManager) SetOnActiveChanged(handler func(*Window)) {
	m.mu.Lock()
	m.onActiveChanged = handler
	m.mu.Unlock()
}

// SetOnRepaintNeeded sets the repaint needed callback.
func (m *WindowManager) SetOnRepaintNeeded(handler func()) {
	m.mu.Lock()
	m.onRepaintNeeded = handler
	m.mu.Unlock()
}

// RequestRepaint requests a repaint from the application.
func (m *WindowManager) RequestRepaint() {
	m.mu.RLock()
	handler := m.onRepaintNeeded
	m.mu.RUnlock()
	if handler != nil {
		handler()
	}
}

// HandleMouseWheel routes a wheel event to the topmost visible
// window under the pointer (position routing; gesture latching
// happens above, in the desktop).
func (m *WindowManager) HandleMouseWheel(event core.MouseWheelEvent) bool {
	m.mu.RLock()
	windows := make([]*Window, len(m.windows))
	copy(windows, m.windows)
	m.mu.RUnlock()

	pos := core.UnitPoint{X: event.X, Y: event.Y}

	// Popup overlays float above everything.
	m.mu.RLock()
	popups := make([]*PopupOverlay, len(m.popups))
	copy(popups, m.popups)
	m.mu.RUnlock()
	for i := len(popups) - 1; i >= 0; i-- {
		popup := popups[i]
		if popup.HandleMouseWheel != nil && popup.Bounds.Contains(pos) {
			if popup.HandleMouseWheel(event) {
				m.RequestRepaint()
				return true
			}
		}
	}

	for i := len(windows) - 1; i >= 0; i-- {
		win := windows[i]
		b := m.displayBounds(win)
		if !win.IsVisible() || win.IsMinimized() || !b.Contains(pos) {
			continue
		}
		local := event
		local.X -= b.X
		local.Y -= b.Y
		if win.HandleMouseWheel(local) {
			m.RequestRepaint()
			return true
		}
		return false // topmost window under the pointer owns the point
	}
	return false
}
