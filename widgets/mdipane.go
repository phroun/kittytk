// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"sync"
	"time"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/window"
)

// MDIPane is a container widget that manages multiple floating windows (MDI children).
// Unlike Desktop's WindowManager which is screen-level, MDIPane is a regular widget
// that can be embedded in any container (Window, TabWidget, Splitter, Panel, etc.).
//
// MDIPane provides:
// - Background content area (for widgets behind the floating windows)
// - Z-ordered floating window management
// - Window dragging, resizing, minimize/maximize/restore
// - Keyboard and mouse focus management
// - Tile and cascade window arrangement
type MDIPane struct {
	core.WidgetBase
	mu sync.RWMutex

	// Background content widget (shown behind windows)
	content core.Widget

	// Layout manager for content
	layout core.LayoutManager

	// Background pattern character
	bgChar rune

	// Whether to draw the pattern background (true) or solid background like ScrollArea (false)
	// Defaults to true when no content, automatically set to false when content is added
	drawPattern    bool
	drawPatternSet bool // tracks if drawPattern was explicitly set

	// Floating windows in z-order (back to front)
	windows []*window.Window

	// Active/focused window
	activeWindow *window.Window

	// Modal window stack
	modalStack []*window.Window

	// Drag state
	dragging    *window.Window
	dragStartX  core.Unit
	dragStartY  core.Unit
	dragOffsetX core.Unit
	dragOffsetY core.Unit

	// Resize state
	resizing       *window.Window
	resizeEdge     int
	resizeStartX   core.Unit
	resizeStartY   core.Unit
	resizeOriginal core.UnitRect

	// Double-click detection
	lastClickTime   time.Time
	lastClickX      core.Unit
	lastClickY      core.Unit
	lastClickWindow *window.Window

	// Focus-without-raise: track pressed window for conditional raise on release
	pressedWindow *window.Window

	// Callbacks
	onWindowAdded         func(*window.Window)
	onWindowRemoved       func(*window.Window)
	onActiveWindowChanged func(*window.Window)
	onWindowMinimized     func(*window.Window)
	onWindowRestored      func(*window.Window)
}

// NewMDIPane creates a new MDI pane widget.
func NewMDIPane() *MDIPane {
	m := &MDIPane{
		bgChar:      '░',  // Light shade for MDI background
		drawPattern: true, // Default to pattern when no content
	}
	m.WidgetBase = *core.NewWidgetBase()
	m.Init(m)
	m.SetFocusPolicy(core.StrongFocus)
	return m
}

// DrawPattern returns whether the MDI pane draws a pattern background.
func (m *MDIPane) DrawPattern() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.drawPattern
}

// SetDrawPattern sets whether the MDI pane draws a pattern background.
// When true, draws a pattern character with DesktopFill colors.
// When false, draws a solid background like ScrollArea.
func (m *MDIPane) SetDrawPattern(drawPattern bool) {
	m.mu.Lock()
	m.drawPattern = drawPattern
	m.drawPatternSet = true
	m.mu.Unlock()
	m.Update()
}

// SetContent sets the background content widget.
// This widget is displayed behind all floating windows.
// When content is added and drawPattern hasn't been explicitly set,
// drawPattern is automatically set to false for a solid background.
func (m *MDIPane) SetContent(content core.Widget) {
	m.mu.Lock()
	hadContent := m.content != nil
	m.content = content
	if content != nil {
		content.SetParent(m)
		// Auto-switch to solid background when first content is added
		// (unless drawPattern was explicitly set)
		if !hadContent && !m.drawPatternSet {
			m.drawPattern = false
		}
	}
	m.mu.Unlock()
	m.layoutContent()
	m.Update()
}

// Content returns the background content widget.
func (m *MDIPane) Content() core.Widget {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.content
}

// SetLayout sets the layout manager for the content area.
func (m *MDIPane) SetLayout(layout core.LayoutManager) {
	m.mu.Lock()
	m.layout = layout
	m.mu.Unlock()
	m.layoutContent()
}

// SetBackgroundChar sets the background pattern character.
func (m *MDIPane) SetBackgroundChar(ch rune) {
	m.mu.Lock()
	m.bgChar = ch
	m.mu.Unlock()
	m.Update()
}

// BackgroundChar returns the background pattern character.
func (m *MDIPane) BackgroundChar() rune {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bgChar
}

// AddWindow adds a floating window to the MDI pane.
func (m *MDIPane) AddWindow(win *window.Window) {
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

	// Set window's parent to this pane
	win.SetParent(m)

	// Set up request callbacks
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
	win.SetGetConstrainingBounds(func() core.UnitRect {
		return m.ClientArea()
	})

	// When the window is closed, remove it from the MDI pane
	win.SetOnClose(func() bool {
		m.RemoveWindow(win)
		return true // Allow the close
	})

	// Position if not explicitly set
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

// RemoveWindow removes a window from the MDI pane.
func (m *MDIPane) RemoveWindow(win *window.Window) {
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
	wasActive := m.activeWindow == win
	var newActive *window.Window
	if wasActive {
		m.activeWindow = nil
		if len(m.windows) > 0 {
			newActive = m.windows[len(m.windows)-1]
			m.activeWindow = newActive
		}
	}

	removedHandler := m.onWindowRemoved
	activeHandler := m.onActiveWindowChanged
	m.mu.Unlock()

	// Update active states and focus
	if wasActive {
		win.SetActive(false)
		if newActive != nil {
			newActive.SetActive(true)
			// Focus the new active window's first widget
			if fm := newActive.FocusManager(); fm != nil {
				if fm.FocusedWidget() == nil {
					fm.FocusFirst()
				}
			}
		}
		// MDIPane keeps focus so keyboard events come here
		m.SetFocus()
	}

	if removedHandler != nil {
		removedHandler(win)
	}
	if activeHandler != nil && wasActive {
		activeHandler(newActive)
	}

	m.Update()
}

// Windows returns all windows in z-order.
func (m *MDIPane) Windows() []*window.Window {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*window.Window, len(m.windows))
	copy(result, m.windows)
	return result
}

// ActiveWindow returns the currently active window.
func (m *MDIPane) ActiveWindow() *window.Window {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeWindow
}

// SetActiveWindow activates a specific window.
func (m *MDIPane) SetActiveWindow(win *window.Window) {
	m.ActivateWindow(win)
}

// ActivateWindow brings a window to the front and makes it the active window.
// When a window becomes active, MDIPane takes focus so that all keyboard
// events (including Tab) are routed through MDIPane to the active window.
func (m *MDIPane) ActivateWindow(win *window.Window) {
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

	handler := m.onActiveWindowChanged
	m.mu.Unlock()

	// Update active states
	if oldActive != nil {
		oldActive.SetActive(false)
	}
	if win != nil {
		win.SetActive(true)

		// Only take focus and initialize child window focus if MDIPane
		// already has focus. This prevents stealing focus during initial
		// setup when windows are added but the MDI tab isn't active yet.
		if m.HasFocus() {
			// Focus the window's first widget if no widget is focused
			if fm := win.FocusManager(); fm != nil {
				if fm.FocusedWidget() == nil {
					fm.FocusFirst()
				}
			}
		}
	}

	if handler != nil {
		handler(win)
	}

	m.Update()
}

// FocusWindow gives a window focus without raising it to the front.
// This is used for focus-follows-click behavior where the window only
// raises on mouse release within its bounds.
func (m *MDIPane) FocusWindow(win *window.Window) {
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

	handler := m.onActiveWindowChanged
	m.mu.Unlock()

	// Update active states
	if oldActive != nil {
		oldActive.SetActive(false)
	}
	if win != nil {
		win.SetActive(true)
		// Focus the window's first widget if no widget is focused
		if fm := win.FocusManager(); fm != nil {
			if fm.FocusedWidget() == nil {
				fm.FocusFirst()
			}
		}
	}

	if handler != nil {
		handler(win)
	}

	m.Update()
}

// RaiseWindow brings a window to the front without changing focus.
func (m *MDIPane) RaiseWindow(win *window.Window) {
	m.mu.Lock()
	m.bringToFront(win)
	m.mu.Unlock()
	m.Update()
}

// NextWindow activates the next window in z-order.
func (m *MDIPane) NextWindow() {
	m.mu.RLock()
	windows := m.windows
	active := m.activeWindow
	m.mu.RUnlock()

	if len(windows) <= 1 {
		return
	}

	// Find current index
	currentIdx := -1
	for i, w := range windows {
		if w == active {
			currentIdx = i
			break
		}
	}

	// Calculate next index (wrap around)
	nextIdx := (currentIdx + 1) % len(windows)
	m.ActivateWindow(windows[nextIdx])
}

// PrevWindow activates the previous window in z-order.
func (m *MDIPane) PrevWindow() {
	m.mu.RLock()
	windows := m.windows
	active := m.activeWindow
	m.mu.RUnlock()

	if len(windows) <= 1 {
		return
	}

	// Find current index
	currentIdx := 0
	for i, w := range windows {
		if w == active {
			currentIdx = i
			break
		}
	}

	// Calculate previous index (wrap around)
	prevIdx := currentIdx - 1
	if prevIdx < 0 {
		prevIdx = len(windows) - 1
	}
	m.ActivateWindow(windows[prevIdx])
}

// MaximizeWindow maximizes a window to fill the MDI pane.
func (m *MDIPane) MaximizeWindow(win *window.Window) {
	clientArea := m.ClientArea()
	win.Maximize()
	win.SetBounds(clientArea)
	m.Update()
}

// MinimizeWindow minimizes a window.
func (m *MDIPane) MinimizeWindow(win *window.Window) {
	win.Minimize()

	// Notify via callback
	m.mu.RLock()
	handler := m.onWindowMinimized
	m.mu.RUnlock()

	if handler != nil {
		handler(win)
	}

	m.Update()
}

// RestoreWindow restores a minimized or maximized window.
func (m *MDIPane) RestoreWindow(win *window.Window) {
	win.Restore()
	m.ActivateWindow(win)

	// Notify via callback
	m.mu.RLock()
	handler := m.onWindowRestored
	m.mu.RUnlock()

	if handler != nil {
		handler(win)
	}

	m.Update()
}

// TileWindows arranges windows in a tiled layout.
func (m *MDIPane) TileWindows() {
	m.mu.RLock()
	var visibleWindows []*window.Window
	for _, w := range m.windows {
		if w.IsVisible() && !w.IsMinimized() {
			visibleWindows = append(visibleWindows, w)
		}
	}
	m.mu.RUnlock()

	if len(visibleWindows) == 0 {
		return
	}

	clientArea := m.ClientArea()
	metrics := core.DefaultCellMetrics()

	// Calculate grid dimensions
	cols := 1
	rows := len(visibleWindows)
	for cols*cols < len(visibleWindows) {
		cols++
	}
	rows = (len(visibleWindows) + cols - 1) / cols

	// Align tile sizes to cell boundaries
	cellWidth := metrics.RoundDownToCellX(clientArea.Width / core.Unit(cols))
	cellHeight := metrics.RoundDownToCellY(clientArea.Height / core.Unit(rows))

	for i, win := range visibleWindows {
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

	m.Update()
}

// CascadeWindows arranges windows in a cascade.
func (m *MDIPane) CascadeWindows() {
	m.mu.RLock()
	var visibleWindows []*window.Window
	for _, w := range m.windows {
		if w.IsVisible() && !w.IsMinimized() {
			visibleWindows = append(visibleWindows, w)
		}
	}
	m.mu.RUnlock()

	if len(visibleWindows) == 0 {
		return
	}

	clientArea := m.ClientArea()
	metrics := core.DefaultCellMetrics()
	offset := metrics.CellWidth * 2

	// Standard size for cascaded windows
	width := metrics.RoundDownToCellX(clientArea.Width * 3 / 4)
	height := metrics.RoundDownToCellY(clientArea.Height * 3 / 4)

	for i, win := range visibleWindows {
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

	m.Update()
}

// ShowModal shows a window as modal.
func (m *MDIPane) ShowModal(win *window.Window) {
	m.mu.Lock()
	win.SetFlags(win.Flags() | window.WindowFlagModal)
	m.modalStack = append(m.modalStack, win)
	m.mu.Unlock()
	m.AddWindow(win)
}

// CloseModal closes the top modal window.
func (m *MDIPane) CloseModal() {
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

// SetOnWindowAdded sets the callback for when a window is added.
func (m *MDIPane) SetOnWindowAdded(handler func(*window.Window)) {
	m.mu.Lock()
	m.onWindowAdded = handler
	m.mu.Unlock()
}

// SetOnWindowRemoved sets the callback for when a window is removed.
func (m *MDIPane) SetOnWindowRemoved(handler func(*window.Window)) {
	m.mu.Lock()
	m.onWindowRemoved = handler
	m.mu.Unlock()
}

// SetOnActiveWindowChanged sets the callback for when the active window changes.
func (m *MDIPane) SetOnActiveWindowChanged(handler func(*window.Window)) {
	m.mu.Lock()
	m.onActiveWindowChanged = handler
	m.mu.Unlock()
}

// SetOnWindowMinimized sets the callback for when a window is minimized.
func (m *MDIPane) SetOnWindowMinimized(handler func(*window.Window)) {
	m.mu.Lock()
	m.onWindowMinimized = handler
	m.mu.Unlock()
}

// SetOnWindowRestored sets the callback for when a window is restored.
func (m *MDIPane) SetOnWindowRestored(handler func(*window.Window)) {
	m.mu.Lock()
	m.onWindowRestored = handler
	m.mu.Unlock()
}

// DeactivateActiveWindow deactivates the current active window (if any).
// This is called when the user clicks on the MDIPane's content area,
// which transfers focus away from the active MDI child.
func (m *MDIPane) DeactivateActiveWindow() {
	m.mu.Lock()
	oldActive := m.activeWindow
	if oldActive == nil {
		m.mu.Unlock()
		return
	}
	m.activeWindow = nil
	handler := m.onActiveWindowChanged
	m.mu.Unlock()

	oldActive.SetActive(false)

	if handler != nil {
		handler(nil)
	}

	m.Update()
}

// ClientArea returns the area available for windows.
func (m *MDIPane) ClientArea() core.UnitRect {
	bounds := m.Bounds()
	return core.UnitRect{
		X:      0,
		Y:      0,
		Width:  bounds.Width,
		Height: bounds.Height,
	}
}

// bringToFront moves a window to the top of the z-order.
func (m *MDIPane) bringToFront(win *window.Window) {
	for i, w := range m.windows {
		if w == win {
			m.windows = append(m.windows[:i], m.windows[i+1:]...)
			m.windows = append(m.windows, win)
			return
		}
	}
}

// isChildOf checks if child is a descendant of parent.
func (m *MDIPane) isChildOf(child, parent *window.Window) bool {
	for p := child.ParentWindow(); p != nil; p = p.ParentWindow() {
		if p == parent {
			return true
		}
	}
	return false
}

// positionWindow positions a new window using cascading.
func (m *MDIPane) positionWindow(win *window.Window) {
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

	// Cascade offset
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

// layoutContent updates the content widget bounds.
func (m *MDIPane) layoutContent() {
	m.mu.RLock()
	content := m.content
	layout := m.layout
	m.mu.RUnlock()

	if content == nil {
		return
	}

	clientArea := m.ClientArea()

	if layout != nil {
		layout.Layout(m, clientArea)
	} else {
		content.SetBounds(clientArea)
	}
}

// detectResizeEdge determines which window edge(s) the mouse is near.
func (m *MDIPane) detectResizeEdge(win *window.Window, localX, localY core.Unit) int {
	if win.Flags()&window.WindowFlagNoResize != 0 || win.IsMaximized() {
		return window.ResizeEdgeNone
	}

	bounds := win.Bounds()
	metrics := core.DefaultCellMetrics()

	// Convert local coordinates to global (within MDI pane)
	x := localX
	y := localY

	edgeThreshold := metrics.CellWidth
	cornerThreshold := metrics.CellWidth * 2

	edge := window.ResizeEdgeNone

	// Check if at bottom edge
	atBottom := y >= bounds.Y+bounds.Height-metrics.CellHeight && y < bounds.Y+bounds.Height

	// Check horizontal edges
	if x >= bounds.X && x < bounds.X+edgeThreshold {
		edge |= window.ResizeEdgeLeft
	} else if x >= bounds.X+bounds.Width-edgeThreshold && x < bounds.X+bounds.Width {
		edge |= window.ResizeEdgeRight
	}

	// Check bottom edge
	if atBottom {
		edge |= window.ResizeEdgeBottom
		if x >= bounds.X && x < bounds.X+cornerThreshold {
			edge |= window.ResizeEdgeLeft
		} else if x >= bounds.X+bounds.Width-cornerThreshold && x < bounds.X+bounds.Width {
			edge |= window.ResizeEdgeRight
		}
	}

	return edge
}

// SetBounds sets the MDI pane bounds and triggers layout.
func (m *MDIPane) SetBounds(bounds core.UnitRect) {
	m.WidgetBase.SetBounds(bounds)
	m.layoutContent()

	// Adjust maximized windows
	clientArea := m.ClientArea()
	for _, win := range m.windows {
		if win.IsMaximized() {
			win.SetBounds(clientArea)
		}
	}
}

// Children returns all child widgets.
func (m *MDIPane) Children() []core.Widget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var children []core.Widget
	if m.content != nil {
		children = append(children, m.content)
	}
	for _, win := range m.windows {
		children = append(children, win)
	}
	return children
}

// AddChild adds a child widget.
func (m *MDIPane) AddChild(child core.Widget) {
	if win, ok := child.(*window.Window); ok {
		m.AddWindow(win)
	} else {
		m.SetContent(child)
	}
}

// RemoveChild removes a child widget.
func (m *MDIPane) RemoveChild(child core.Widget) {
	if win, ok := child.(*window.Window); ok {
		m.RemoveWindow(win)
	} else if m.content == child {
		m.content = nil
	}
}

// ChildAt returns the child at the given position.
func (m *MDIPane) ChildAt(pos core.UnitPoint) core.Widget {
	m.mu.RLock()
	windows := m.windows
	content := m.content
	m.mu.RUnlock()

	// Check windows from top to bottom
	for i := len(windows) - 1; i >= 0; i-- {
		win := windows[i]
		if win.IsVisible() && !win.IsMinimized() && win.Bounds().Contains(pos) {
			return win
		}
	}

	// Check content
	if content != nil && content.Bounds().Contains(pos) {
		return content
	}

	return nil
}

// Layout arranges children within the MDI pane.
func (m *MDIPane) Layout() {
	m.layoutContent()
}

// LayoutManager returns the layout manager.
func (m *MDIPane) LayoutManager() core.LayoutManager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.layout
}

// SetLayoutManager sets the layout manager.
func (m *MDIPane) SetLayoutManager(lm core.LayoutManager) {
	m.SetLayout(lm)
}

// SizeHint returns the preferred size.
func (m *MDIPane) SizeHint() core.UnitSize {
	return m.Bounds().Size()
}

// HandleFocusIn is called when MDIPane gains focus.
// This ensures the active window's focus is properly initialized,
// which is important when the MDIPane first becomes visible.
func (m *MDIPane) HandleFocusIn() {
	m.mu.RLock()
	active := m.activeWindow
	m.mu.RUnlock()

	// Ensure active window has a focused widget
	if active != nil && !active.IsMinimized() {
		if fm := active.FocusManager(); fm != nil {
			if fm.FocusedWidget() == nil {
				fm.FocusFirst()
			}
		}
	}

	m.Update()
}

// CollectFocusChain implements core.FocusChainProvider.
// When an MDI child window is active, MDIPane acts as a focus boundary -
// Tab navigation stays within the MDIPane and is forwarded to the active window.
// When no MDI child is active, focus can move through the content widgets.
func (m *MDIPane) CollectFocusChain(collector func(core.Widget)) {
	m.mu.RLock()
	activeWindow := m.activeWindow
	content := m.content
	m.mu.RUnlock()

	// When an MDI child is active, MDIPane is the only focusable item
	// in the chain. This makes MDIPane a focus trap - Tab events come to
	// MDIPane which forwards them to the active window.
	if activeWindow != nil && !activeWindow.IsMinimized() {
		collector(m)
		return
	}

	// No active MDI child - include MDIPane and its content in the chain
	collector(m)
	if content != nil {
		collector(content)
	}
}

// Paint renders the MDI pane.
func (m *MDIPane) Paint(p *core.Painter) {
	bounds := m.Bounds()
	scheme := m.GetScheme()
	metrics := p.Metrics()

	m.mu.RLock()
	bgChar := m.bgChar
	drawPattern := m.drawPattern
	content := m.content
	windows := m.windows
	m.mu.RUnlock()

	// Draw background
	if drawPattern {
		// Draw pattern background (like Desktop)
		bgStyle := scheme.GetDesktopFill()
		for y := core.Unit(0); y < bounds.Height; y += metrics.CellHeight {
			for x := core.Unit(0); x < bounds.Width; x += metrics.CellWidth {
				p.DrawCell(x, y, bgChar, bgStyle)
			}
		}
	} else {
		// Draw solid background (like ScrollArea)
		inheritedBg := m.EffectiveBackgroundColor()
		bgStyle := scheme.GetNormal(true).WithBg(inheritedBg)
		p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', bgStyle)
	}

	// Draw content if any
	if content != nil {
		content.Paint(p)
	}

	// Paint windows from bottom to top
	clientArea := m.ClientArea()
	for _, win := range windows {
		if win.IsVisible() && !win.IsMinimized() {
			winBounds := win.Bounds()

			// Calculate visible portion within client area
			visibleBounds := winBounds.Intersection(clientArea)
			if visibleBounds.IsEmpty() {
				continue
			}

			// Offset into window's local coordinates
			localClipX := visibleBounds.X - winBounds.X
			localClipY := visibleBounds.Y - winBounds.Y
			localClip := core.UnitRect{
				X:      localClipX,
				Y:      localClipY,
				Width:  visibleBounds.Width,
				Height: visibleBounds.Height,
			}

			windowPainter := p.WithOffset(winBounds.X, winBounds.Y).
				WithClip(localClip)
			win.Paint(windowPainter)
		}
	}
}

// HandleKeyPress handles keyboard input.
// When an MDI child window is active, MDIPane forwards ALL keyboard events
// to that window, including Tab and Shift+Tab. This ensures focus stays
// within the active window until the user clicks elsewhere or closes it.
func (m *MDIPane) HandleKeyPress(event core.KeyPressEvent) bool {
	m.mu.RLock()
	active := m.activeWindow
	m.mu.RUnlock()

	// Check for MDI-specific shortcuts (window switching)
	switch event.Key {
	case "M-Tab", "C-Tab":
		m.NextWindow()
		return true
	case "M-S-Tab", "C-S-Tab":
		m.PrevWindow()
		return true
	}

	// Forward ALL key events to the active window.
	// This includes Tab and Shift+Tab which the window's FocusManager handles
	// for internal focus navigation.
	if active != nil && !active.IsMinimized() {
		// Ensure the window has a focused widget before processing key events.
		// This is critical for proper Tab/Shift+Tab behavior - if no widget is
		// focused, the first key press should establish focus.
		// BUT: don't do this if the title bar has focus (e.g., during window move/resize).
		if fm := active.FocusManager(); fm != nil {
			if fm.FocusedWidget() == nil && !active.HasTitleFocus() {
				fm.FocusFirst()
			}
		}

		if active.HandleKeyPress(event) {
			return true
		}
	}

	return false
}

// HandleMousePress handles mouse clicks.
func (m *MDIPane) HandleMousePress(event core.MousePressEvent) bool {
	// Any click inside the MDIPane should give it focus, so keyboard events
	// (including Tab) route through MDIPane to the active child window.
	m.SetFocus()

	m.mu.RLock()
	windows := m.windows
	content := m.content
	m.mu.RUnlock()

	metrics := core.DefaultCellMetrics()

	// Check windows from top to bottom
	for i := len(windows) - 1; i >= 0; i-- {
		win := windows[i]
		if !win.IsVisible() || win.IsMinimized() {
			continue
		}

		bounds := win.Bounds()
		if bounds.Contains(core.UnitPoint{X: event.X, Y: event.Y}) {
			// Check for resize edge first - resize operations raise immediately
			resizeEdge := m.detectResizeEdge(win, event.X, event.Y)
			if resizeEdge != window.ResizeEdgeNone {
				m.ActivateWindow(win)
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

			// Check for title bar interaction - titlebar operations raise immediately
			if event.Y < bounds.Y+metrics.CellHeight &&
				win.Flags()&window.WindowFlagNoTitle == 0 {

				m.ActivateWindow(win)

				// Let window handle button clicks
				localEvent := event
				localEvent.X -= bounds.X
				localEvent.Y -= bounds.Y
				if win.HandleMousePress(localEvent) {
					// Update click tracking
					m.mu.Lock()
					m.lastClickTime = time.Now()
					m.lastClickX = event.X
					m.lastClickY = event.Y
					m.lastClickWindow = win
					m.pressedWindow = nil
					m.mu.Unlock()
					return true
				}

				// Check for double-click
				now := time.Now()
				m.mu.Lock()
				isDoubleClick := m.lastClickWindow == win &&
					now.Sub(m.lastClickTime) < 400*time.Millisecond &&
					abs(int(event.X-m.lastClickX)) < int(metrics.CellWidth) &&
					abs(int(event.Y-m.lastClickY)) < int(metrics.CellHeight)
				m.lastClickTime = now
				m.lastClickX = event.X
				m.lastClickY = event.Y
				m.lastClickWindow = win
				m.mu.Unlock()

				if isDoubleClick && win.Flags()&window.WindowFlagNoMaximize == 0 {
					if win.IsMaximized() {
						win.Restore()
					} else {
						m.MaximizeWindow(win)
					}
					m.mu.Lock()
					m.lastClickWindow = nil
					m.pressedWindow = nil
					m.mu.Unlock()
					return true
				}

				// Start drag
				if win.Flags()&window.WindowFlagNoMove == 0 {
					m.mu.Lock()
					m.dragging = win
					m.dragStartX = event.X
					m.dragStartY = event.Y
					m.dragOffsetX = event.X - bounds.X
					m.dragOffsetY = event.Y - bounds.Y
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

	// Clicking on the background or content deactivates the active MDI child
	m.DeactivateActiveWindow()

	// Forward to content
	if content != nil {
		return content.HandleMousePress(event)
	}

	return false
}

// HandleMouseMove handles mouse movement.
func (m *MDIPane) HandleMouseMove(event core.MouseMoveEvent) bool {
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

		if resizeEdge&window.ResizeEdgeLeft != 0 {
			newBounds.X = resizeOriginal.X + deltaX
			newBounds.Width = resizeOriginal.Width - deltaX
		}
		if resizeEdge&window.ResizeEdgeRight != 0 {
			newBounds.Width = resizeOriginal.Width + deltaX
		}
		if resizeEdge&window.ResizeEdgeTop != 0 {
			newBounds.Y = resizeOriginal.Y + deltaY
			newBounds.Height = resizeOriginal.Height - deltaY
		}
		if resizeEdge&window.ResizeEdgeBottom != 0 {
			newBounds.Height = resizeOriginal.Height + deltaY
		}

		newBounds = metrics.AlignRect(newBounds)

		// Enforce minimum size
		minWidth := metrics.CellWidth * 3
		minHeight := metrics.CellHeight * 2
		if newBounds.Width < minWidth {
			if resizeEdge&window.ResizeEdgeLeft != 0 {
				newBounds.X = resizeOriginal.X + resizeOriginal.Width - minWidth
			}
			newBounds.Width = minWidth
		}
		if newBounds.Height < minHeight {
			if resizeEdge&window.ResizeEdgeTop != 0 {
				newBounds.Y = resizeOriginal.Y + resizeOriginal.Height - minHeight
			}
			newBounds.Height = minHeight
		}

		// Keep on screen
		clientArea := m.ClientArea()
		if newBounds.X < clientArea.X {
			if resizeEdge&window.ResizeEdgeLeft != 0 {
				newBounds.Width = resizeOriginal.X + resizeOriginal.Width - clientArea.X
			}
			newBounds.X = clientArea.X
		}
		if newBounds.Y < clientArea.Y {
			if resizeEdge&window.ResizeEdgeTop != 0 {
				newBounds.Height = resizeOriginal.Y + resizeOriginal.Height - clientArea.Y
			}
			newBounds.Y = clientArea.Y
		}

		resizing.SetBounds(newBounds)
		m.Update()
		return true
	}

	// Handle drag
	if dragging != nil {
		justRestored := false
		clientArea := m.ClientArea()
		metrics := core.DefaultCellMetrics()

		// Handle restore from maximized
		if dragging.IsMaximized() {
			newY := event.Y - offsetY
			if newY >= clientArea.Y {
				oldBounds := dragging.Bounds()
				dragging.Restore()
				justRestored = true
				newBounds := dragging.Bounds()
				dragging.Layout()

				proportion := float64(offsetX) / float64(oldBounds.Width)
				offsetX = core.Unit(proportion * float64(newBounds.Width))

				m.mu.Lock()
				m.dragOffsetX = offsetX
				m.mu.Unlock()
			} else {
				return true
			}
		}

		newX := event.X - offsetX
		newY := event.Y - offsetY

		bounds := dragging.Bounds()
		bounds.X = newX
		bounds.Y = newY

		// Maximize gesture
		if bounds.Y < clientArea.Y && dragging.Flags()&window.WindowFlagNoMaximize == 0 && !justRestored {
			if !dragging.IsMaximized() {
				m.MaximizeWindow(dragging)
			}
			return true
		}

		// Keep titlebar visible
		if bounds.Y < clientArea.Y {
			bounds.Y = clientArea.Y
		}
		maxY := clientArea.Y + clientArea.Height - metrics.CellHeight
		if bounds.Y > maxY {
			bounds.Y = maxY
		}

		// Allow going off-screen horizontally
		minVisibleX := core.Unit(1)
		minX := clientArea.X - bounds.Width + minVisibleX
		if bounds.X < minX {
			bounds.X = minX
		}
		maxX := clientArea.X + clientArea.Width - minVisibleX
		if bounds.X > maxX {
			bounds.X = maxX
		}

		bounds = metrics.AlignRect(bounds)
		dragging.SetBounds(bounds)
		m.Update()
		return true
	}

	// Forward to active window
	m.mu.RLock()
	active := m.activeWindow
	content := m.content
	m.mu.RUnlock()

	if active != nil && !active.IsMinimized() {
		bounds := active.Bounds()
		localEvent := event
		localEvent.X -= bounds.X
		localEvent.Y -= bounds.Y
		if active.HandleMouseMove(localEvent) {
			m.Update()
			return true
		}
	}

	// Forward to content (for hover states on buttons, etc.)
	if content != nil {
		if handler, ok := content.(interface {
			HandleMouseMove(core.MouseMoveEvent) bool
		}); ok {
			if handler.HandleMouseMove(event) {
				m.Update()
				return true
			}
		}
	}

	return false
}

// HandleMouseRelease handles mouse release.
func (m *MDIPane) HandleMouseRelease(event core.MouseReleaseEvent) bool {
	m.mu.Lock()
	dragging := m.dragging
	resizing := m.resizing
	pressedWin := m.pressedWindow
	m.dragging = nil
	m.resizing = nil
	m.resizeEdge = window.ResizeEdgeNone
	m.pressedWindow = nil
	m.mu.Unlock()

	if dragging != nil || resizing != nil {
		return true
	}

	// Check if we should raise the pressed window (focus-without-raise behavior)
	// Only raise if release is over a non-occluded part of the window
	if pressedWin != nil && !pressedWin.IsMinimized() {
		bounds := pressedWin.Bounds()
		releasePoint := core.UnitPoint{X: event.X, Y: event.Y}
		if bounds.Contains(releasePoint) {
			// Check that no other window is on top at this position
			m.mu.RLock()
			windows := m.windows
			m.mu.RUnlock()

			topmostAtPoint := (*window.Window)(nil)
			for i := len(windows) - 1; i >= 0; i-- {
				win := windows[i]
				if win.IsVisible() && !win.IsMinimized() && win.Bounds().Contains(releasePoint) {
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

	// Forward to active window
	m.mu.RLock()
	active := m.activeWindow
	content := m.content
	m.mu.RUnlock()

	if active != nil && !active.IsMinimized() {
		bounds := active.Bounds()
		localEvent := event
		localEvent.X -= bounds.X
		localEvent.Y -= bounds.Y
		if active.HandleMouseRelease(localEvent) {
			m.Update()
			return true
		}
	}

	// Forward to content (for buttons and other widgets in the MDI background)
	if content != nil {
		if handler, ok := content.(interface {
			HandleMouseRelease(core.MouseReleaseEvent) bool
		}); ok {
			if handler.HandleMouseRelease(event) {
				m.Update()
				return true
			}
		}
	}

	return false
}

// abs returns the absolute value of an integer.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Verify MDIPane implements Container
var _ core.Container = (*MDIPane)(nil)
