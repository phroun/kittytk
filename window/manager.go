// Package window provides windowing support for the TUI toolkit.
package window

import (
	"sync"
	"time"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// Resize edge constants (can be combined for corners)
const (
	ResizeEdgeNone   = 0
	ResizeEdgeLeft   = 1 << 0
	ResizeEdgeRight  = 1 << 1
	ResizeEdgeTop    = 1 << 2
	ResizeEdgeBottom = 1 << 3
)

// WindowManager manages all windows in the application.
// It handles z-ordering, focus, modal windows, and window positioning.
type WindowManager struct {
	mu sync.RWMutex

	// All windows in z-order (back to front)
	windows []*Window

	// Active/focused window
	activeWindow *Window

	// Modal window stack
	modalStack []*Window

	// Desktop/root widget (what's behind all windows)
	desktop core.Widget

	// Screen bounds
	screenBounds core.UnitRect

	// Theme
	theme *style.Theme

	// Drag state
	dragging    *Window
	dragStartX  core.Unit
	dragStartY  core.Unit
	dragOffsetX core.Unit
	dragOffsetY core.Unit

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

	// Callbacks
	onWindowAdded     func(*Window)
	onWindowRemoved   func(*Window)
	onActiveChanged   func(*Window)
	onRepaintNeeded   func()
	onWindowMinimized func(*Window) // Called when a window is minimized
	onWindowRestored  func(*Window) // Called when a window is restored
}

// NewWindowManager creates a new window manager.
func NewWindowManager() *WindowManager {
	return &WindowManager{
		theme: style.DefaultTheme(),
	}
}

// abs returns the absolute value of an integer.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// detectResizeEdge determines which window edge(s) the mouse is near.
// Returns a combination of ResizeEdge constants.
// Note: Top edge is excluded since the titlebar is used for dragging.
func (m *WindowManager) detectResizeEdge(win *Window, x, y core.Unit) int {
	if win.Flags()&WindowFlagNoResize != 0 {
		return ResizeEdgeNone
	}

	bounds := win.Bounds()
	metrics := core.DefaultCellMetrics()

	// Edge detection threshold (one cell for edges)
	edgeThreshold := metrics.CellWidth
	// Corner detection threshold (2 cells for corners)
	cornerThreshold := metrics.CellWidth * 2

	edge := ResizeEdgeNone

	// Check if at bottom edge
	atBottom := y >= bounds.Y+bounds.Height-metrics.CellHeight && y < bounds.Y+bounds.Height

	// Check horizontal edges (left/right)
	if x >= bounds.X && x < bounds.X+edgeThreshold {
		edge |= ResizeEdgeLeft
	} else if x >= bounds.X+bounds.Width-edgeThreshold && x < bounds.X+bounds.Width {
		edge |= ResizeEdgeRight
	}

	// Check bottom edge
	if atBottom {
		edge |= ResizeEdgeBottom
		// For bottom corners, extend the left/right detection zone
		if x >= bounds.X && x < bounds.X+cornerThreshold {
			edge |= ResizeEdgeLeft
		} else if x >= bounds.X+bounds.Width-cornerThreshold && x < bounds.X+bounds.Width {
			edge |= ResizeEdgeRight
		}
	}

	// Only return resize edge if we're on a side or bottom edge
	// (top edge is the titlebar for dragging)
	return edge
}

// SetDesktop sets the desktop widget (background behind windows).
func (m *WindowManager) SetDesktop(desktop core.Widget) {
	m.mu.Lock()
	m.desktop = desktop
	bounds := m.screenBounds
	m.mu.Unlock()

	// Set the desktop bounds to the screen size
	if desktop != nil && !bounds.IsEmpty() {
		desktop.SetBounds(bounds)
	}
}

// Desktop returns the desktop widget.
func (m *WindowManager) Desktop() core.Widget {
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

// ScreenBounds returns the screen bounds.
func (m *WindowManager) ScreenBounds() core.UnitRect {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.screenBounds
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
	handler := m.onWindowAdded
	m.mu.Unlock()

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

// RemoveWindow removes a window from the manager.
func (m *WindowManager) RemoveWindow(win *Window) {
	m.mu.Lock()
	for i, w := range m.windows {
		if w == win {
			m.windows = append(m.windows[:i], m.windows[i+1:]...)
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
	if m.activeWindow == win {
		m.activeWindow = nil
		if len(m.windows) > 0 {
			m.activeWindow = m.windows[len(m.windows)-1]
		}
	}

	handler := m.onWindowRemoved
	activeHandler := m.onActiveChanged
	newActive := m.activeWindow
	m.mu.Unlock()

	if handler != nil {
		handler(win)
	}
	if activeHandler != nil && newActive != win {
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

	handler := m.onActiveChanged
	m.mu.Unlock()

	// Update focus states
	if oldActive != nil {
		oldActive.ClearFocus()
		if oldActive.onActivate != nil {
			oldActive.onActivate(false)
		}
	}
	if win != nil {
		win.SetFocus()
		if win.onActivate != nil {
			win.onActivate(true)
		}
	}

	if handler != nil {
		handler(win)
	}
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

// MaximizeWindow maximizes a window to fill the client area.
func (m *WindowManager) MaximizeWindow(win *Window) {
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
	offset := metrics.CellWidth * 2

	// Standard size for cascaded windows - align to cell boundaries
	width := metrics.RoundDownToCellX(clientArea.Width * 3 / 4)
	height := metrics.RoundDownToCellY(clientArea.Height * 3 / 4)

	for i, win := range windows {
		x := clientArea.X + core.Unit(i)*offset
		y := clientArea.Y + core.Unit(i)*offset

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
		win.Restore()
	}
}

// HandleMousePress processes mouse events for windows.
func (m *WindowManager) HandleMousePress(event core.MousePressEvent) bool {
	m.mu.RLock()
	windows := m.windows
	desktop := m.desktop
	m.mu.RUnlock()

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

	// Check windows from top to bottom
	for i := len(windows) - 1; i >= 0; i-- {
		win := windows[i]
		if !win.IsVisible() || win.IsMinimized() {
			continue
		}

		bounds := win.Bounds()
		if bounds.Contains(core.UnitPoint{X: event.X, Y: event.Y}) {
			// Close any active menu before processing window click
			if desktop != nil {
				if menuCloser, ok := desktop.(interface {
					CloseActiveMenu()
				}); ok {
					menuCloser.CloseActiveMenu()
				}
			}

			// Activate window
			m.ActivateWindow(win)

			// Check for resize edge first
			resizeEdge := m.detectResizeEdge(win, event.X, event.Y)
			if resizeEdge != ResizeEdgeNone {
				// Start resize
				m.mu.Lock()
				m.resizing = win
				m.resizeEdge = resizeEdge
				m.resizeStartX = event.X
				m.resizeStartY = event.Y
				m.resizeOriginal = bounds
				m.mu.Unlock()
				return true
			}

			// Check for title bar interaction
			metrics := core.DefaultCellMetrics()
			if event.Y < bounds.Y+metrics.CellHeight &&
				win.Flags()&WindowFlagNoTitle == 0 {

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
					m.mu.Unlock()
				}
				return true
			}

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
	m.mu.Unlock()

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

		// Align to cell boundaries
		newBounds = metrics.AlignRect(newBounds)

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

		resizing.SetBounds(newBounds)
		m.RequestRepaint()
		return true
	}

	// Handle drag
	if dragging != nil {
		// Move window
		newX := event.X - offsetX
		newY := event.Y - offsetY

		bounds := dragging.Bounds()
		bounds.X = newX
		bounds.Y = newY
		dragging.SetBounds(bounds)

		// Request repaint to show the window at its new position
		m.RequestRepaint()

		return true
	}

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

	// Forward to active window (for splitter/widget dragging)
	if active != nil {
		bounds := active.Bounds()
		localEvent := event
		localEvent.X -= bounds.X
		localEvent.Y -= bounds.Y
		if active.HandleMouseMove(localEvent) {
			// Request repaint since widget state may have changed
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
	m.dragging = nil
	m.resizing = nil
	m.resizeEdge = ResizeEdgeNone
	m.mu.Unlock()

	if dragging != nil || resizing != nil {
		return true
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

	// Forward to active window (for splitter/widget release)
	if active != nil {
		bounds := active.Bounds()
		localEvent := event
		localEvent.X -= bounds.X
		localEvent.Y -= bounds.Y
		if active.HandleMouseRelease(localEvent) {
			// Request repaint since widget state may have changed
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

	// Window switching shortcuts
	// Uses direct-key-handler naming: M- = Alt, C- = Ctrl, S- = Shift
	switch event.Key {
	case "M-Tab", "C-Tab":
		m.CycleWindows(true)
		return true
	case "M-S-Tab", "C-S-Tab":
		m.CycleWindows(false)
		return true
	}

	// Pass to active window first
	if active != nil {
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

// CycleWindows cycles through windows.
func (m *WindowManager) CycleWindows(forward bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.windows) <= 1 {
		return
	}

	// Find current window index
	currentIdx := -1
	for i, w := range m.windows {
		if w == m.activeWindow {
			currentIdx = i
			break
		}
	}

	// Calculate next index
	var nextIdx int
	if forward {
		nextIdx = (currentIdx + 1) % len(m.windows)
	} else {
		nextIdx = (currentIdx - 1 + len(m.windows)) % len(m.windows)
	}

	// Skip minimized windows
	startIdx := nextIdx
	for m.windows[nextIdx].IsMinimized() {
		if forward {
			nextIdx = (nextIdx + 1) % len(m.windows)
		} else {
			nextIdx = (nextIdx - 1 + len(m.windows)) % len(m.windows)
		}
		if nextIdx == startIdx {
			return // All windows minimized
		}
	}

	// Activate (will also bring to front)
	m.mu.Unlock()
	m.ActivateWindow(m.windows[nextIdx])
	m.mu.Lock()
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

	// Paint windows from bottom to top
	for _, win := range windows {
		if win.IsVisible() && !win.IsMinimized() {
			bounds := win.Bounds()
			windowPainter := p.WithOffset(bounds.X, bounds.Y).
				WithClip(core.UnitRect{Width: bounds.Width, Height: bounds.Height})
			win.Paint(windowPainter)
		}
	}

	// Paint menu dropdown on top of windows (if any is open)
	if desktop != nil {
		if dd, ok := desktop.(interface{ PaintMenuDropdown(*core.Painter) }); ok {
			dd.PaintMenuDropdown(p)
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
