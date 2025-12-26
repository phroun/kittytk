// Package window provides windowing support for the TUI toolkit.
package window

import (
	"fmt"
	"os"
	"sync"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
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
	dragging   *Window
	dragStartX core.Unit
	dragStartY core.Unit
	dragOffsetX core.Unit
	dragOffsetY core.Unit

	// Resize state
	resizing   *Window
	resizeEdge int

	// Callbacks
	onWindowAdded   func(*Window)
	onWindowRemoved func(*Window)
	onActiveChanged func(*Window)
	onRepaintNeeded func()
}

// NewWindowManager creates a new window manager.
func NewWindowManager() *WindowManager {
	return &WindowManager{
		theme: style.DefaultTheme(),
	}
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

	// Calculate grid dimensions
	cols := 1
	rows := len(windows)
	for cols*cols < len(windows) {
		cols++
	}
	rows = (len(windows) + cols - 1) / cols

	cellWidth := clientArea.Width / core.Unit(cols)
	cellHeight := clientArea.Height / core.Unit(rows)

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

	// Standard size for cascaded windows
	width := clientArea.Width * 3 / 4
	height := clientArea.Height * 3 / 4

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

			// Check for title bar drag
			metrics := core.DefaultCellMetrics()
			if event.Y < bounds.Y+metrics.CellHeight &&
				win.Flags()&WindowFlagNoMove == 0 &&
				win.Flags()&WindowFlagNoTitle == 0 {

				// Start drag
				m.mu.Lock()
				m.dragging = win
				m.dragStartX = event.X
				m.dragStartY = event.Y
				m.dragOffsetX = event.X - bounds.X
				m.dragOffsetY = event.Y - bounds.Y
				m.mu.Unlock()
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
	fmt.Fprintf(os.Stderr, "[WM] HandleMouseMove at (%d,%d)\n", event.X, event.Y)

	m.mu.Lock()
	dragging := m.dragging
	offsetX := m.dragOffsetX
	offsetY := m.dragOffsetY
	m.mu.Unlock()

	if dragging != nil {
		fmt.Fprintf(os.Stderr, "[WM] Window drag mode - moving window\n")
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
			fmt.Fprintf(os.Stderr, "[WM] Forwarding to desktop\n")
			if handler.HandleMouseMove(event) {
				fmt.Fprintf(os.Stderr, "[WM] Desktop handled the event\n")
				return true
			}
			fmt.Fprintf(os.Stderr, "[WM] Desktop did not handle the event\n")
		}
	}

	// Forward to active window (for splitter/widget dragging)
	if active != nil {
		bounds := active.Bounds()
		localEvent := event
		localEvent.X -= bounds.X
		localEvent.Y -= bounds.Y
		fmt.Fprintf(os.Stderr, "[WM] Forwarding to active window, local coords (%d,%d)\n", localEvent.X, localEvent.Y)
		if active.HandleMouseMove(localEvent) {
			fmt.Fprintf(os.Stderr, "[WM] Active window handled the event\n")
			// Request repaint since widget state may have changed
			m.RequestRepaint()
			return true
		}
		fmt.Fprintf(os.Stderr, "[WM] Active window did not handle the event\n")
	}

	return false
}

// HandleMouseRelease processes mouse button release.
func (m *WindowManager) HandleMouseRelease(event core.MouseReleaseEvent) bool {
	m.mu.Lock()
	dragging := m.dragging
	m.dragging = nil
	m.resizing = nil
	m.mu.Unlock()

	if dragging != nil {
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
