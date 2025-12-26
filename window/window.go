// Package window provides windowing support for the TUI toolkit.
package window

import (
	"sync"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// WindowState represents the current state of a window.
type WindowState int

const (
	WindowStateNormal WindowState = iota
	WindowStateMaximized
	WindowStateMinimized
)

// WindowFlags control window behavior and appearance.
type WindowFlags int

const (
	WindowFlagNone        WindowFlags = 0
	WindowFlagFrameless   WindowFlags = 1 << iota // No window frame
	WindowFlagNoTitle                             // No title bar
	WindowFlagNoResize                            // Cannot be resized
	WindowFlagNoMove                              // Cannot be moved
	WindowFlagNoClose                             // No close button
	WindowFlagNoMinimize                          // No minimize button
	WindowFlagNoMaximize                          // No maximize button
	WindowFlagModal                               // Blocks input to other windows
	WindowFlagStaysOnTop                          // Always on top
	WindowFlagToolWindow                          // Smaller title bar, no taskbar entry
)

// Window represents a floating window with frame, title bar, and content area.
// Windows support maximization, minimization, MDI-style child windows,
// and optional Mac-like menu integration.
type Window struct {
	core.WidgetBase
	mu sync.RWMutex

	// Window properties
	title      string
	flags      WindowFlags
	state      WindowState

	// Position before maximization (for restore)
	normalBounds core.UnitRect

	// Content
	content      core.Widget
	layout       core.LayoutManager

	// Focus management
	focusManager *core.FocusManager

	// Child windows (MDI support)
	parent       *Window
	children     []*Window

	// Window chrome
	borderStyle  style.BorderStyle
	titleStyle   style.CellStyle
	frameStyle   style.CellStyle

	// Sizing
	minWidth     core.Unit
	minHeight    core.Unit
	maxWidth     core.Unit
	maxHeight    core.Unit

	// Callbacks
	onClose       func() bool // Return false to prevent close
	onResize      func(width, height core.Unit)
	onMove        func(x, y core.Unit)
	onActivate    func(active bool)
	onStateChange func(state WindowState)

	// Request callbacks (for WindowManager integration)
	onMinimizeRequest func() // Called when user clicks minimize button
	onMaximizeRequest func() // Called when user clicks maximize button
}

// NewWindow creates a new window with the given title.
func NewWindow(title string) *Window {
	w := &Window{
		title:       title,
		state:       WindowStateNormal,
		borderStyle: style.BorderDouble,
		titleStyle:  style.DefaultStyle().WithFg(style.ColorWhite).WithBg(style.ColorBlue).Bold(),
		frameStyle:  style.DefaultStyle().WithFg(style.ColorWhite).WithBg(style.ColorBlue),
		minWidth:    80,  // 10 characters minimum
		minHeight:   48,  // 3 lines minimum
		maxWidth:    1<<30 - 1,
		maxHeight:   1<<30 - 1,
	}
	w.WidgetBase = *core.NewWidgetBase()
	w.SetFocusPolicy(core.StrongFocus)
	w.focusManager = core.NewFocusManager(nil)
	return w
}

// FocusManager returns the window's focus manager.
func (w *Window) FocusManager() *core.FocusManager {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.focusManager
}

// Title returns the window title.
func (w *Window) Title() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.title
}

// SetTitle sets the window title.
func (w *Window) SetTitle(title string) {
	w.mu.Lock()
	w.title = title
	w.mu.Unlock()
	w.Update()
}

// Flags returns the window flags.
func (w *Window) Flags() WindowFlags {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.flags
}

// SetFlags sets the window flags.
func (w *Window) SetFlags(flags WindowFlags) {
	w.mu.Lock()
	w.flags = flags
	w.mu.Unlock()
	w.Update()
}

// State returns the current window state.
func (w *Window) State() WindowState {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.state
}

// SetContent sets the window's content widget.
func (w *Window) SetContent(widget core.Widget) {
	w.mu.Lock()
	w.content = widget
	fm := w.focusManager
	if widget != nil {
		widget.SetParent(w)
	}
	w.mu.Unlock()

	// Update focus manager root and focus first widget
	if fm != nil {
		fm.SetRoot(widget)
		fm.FocusFirst()
	}

	w.layoutContent()
	w.Update()
}

// Content returns the window's content widget.
func (w *Window) Content() core.Widget {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.content
}

// SetLayout sets the layout manager for the content area.
func (w *Window) SetLayout(layout core.LayoutManager) {
	w.mu.Lock()
	w.layout = layout
	w.mu.Unlock()
	w.layoutContent()
}

// Layout returns the layout manager.
func (w *Window) LayoutManager() core.LayoutManager {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.layout
}

// SetLayoutManager implements core.Container.
func (w *Window) SetLayoutManager(layout core.LayoutManager) {
	w.SetLayout(layout)
}

// Parent returns the parent window (for MDI).
func (w *Window) ParentWindow() *Window {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.parent
}

// SetParentWindow sets the parent window (for MDI).
func (w *Window) SetParentWindow(parent *Window) {
	w.mu.Lock()
	oldParent := w.parent
	w.parent = parent
	w.mu.Unlock()

	// Remove from old parent
	if oldParent != nil {
		oldParent.removeChildWindow(w)
	}

	// Add to new parent
	if parent != nil {
		parent.addChildWindow(w)
	}
}

// ChildWindows returns all child windows.
func (w *Window) ChildWindows() []*Window {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]*Window, len(w.children))
	copy(result, w.children)
	return result
}

func (w *Window) addChildWindow(child *Window) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, c := range w.children {
		if c == child {
			return
		}
	}
	w.children = append(w.children, child)
}

func (w *Window) removeChildWindow(child *Window) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for i, c := range w.children {
		if c == child {
			w.children = append(w.children[:i], w.children[i+1:]...)
			return
		}
	}
}

// Maximize maximizes the window.
func (w *Window) Maximize() {
	w.mu.Lock()
	if w.state == WindowStateMaximized {
		w.mu.Unlock()
		return
	}
	if w.flags&WindowFlagNoMaximize != 0 {
		w.mu.Unlock()
		return
	}

	// Store current bounds for restore
	w.normalBounds = w.Bounds()
	w.state = WindowStateMaximized
	handler := w.onStateChange
	w.mu.Unlock()

	// Request the window manager to maximize us
	// (actual resize happens through SetBounds from manager)
	w.Update()

	if handler != nil {
		handler(WindowStateMaximized)
	}
}

// Minimize minimizes the window.
func (w *Window) Minimize() {
	w.mu.Lock()
	if w.state == WindowStateMinimized {
		w.mu.Unlock()
		return
	}
	if w.flags&WindowFlagNoMinimize != 0 {
		w.mu.Unlock()
		return
	}

	w.normalBounds = w.Bounds()
	w.state = WindowStateMinimized
	handler := w.onStateChange
	w.mu.Unlock()

	w.Update()

	if handler != nil {
		handler(WindowStateMinimized)
	}
}

// Restore restores the window from maximized or minimized state.
func (w *Window) Restore() {
	w.mu.Lock()
	if w.state == WindowStateNormal {
		w.mu.Unlock()
		return
	}

	bounds := w.normalBounds
	w.state = WindowStateNormal
	handler := w.onStateChange
	w.mu.Unlock()

	w.SetBounds(bounds)

	if handler != nil {
		handler(WindowStateNormal)
	}
}

// IsMaximized returns true if the window is maximized.
func (w *Window) IsMaximized() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.state == WindowStateMaximized
}

// IsMinimized returns true if the window is minimized.
func (w *Window) IsMinimized() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.state == WindowStateMinimized
}

// Close attempts to close the window.
func (w *Window) Close() bool {
	w.mu.RLock()
	handler := w.onClose
	w.mu.RUnlock()

	if handler != nil && !handler() {
		return false
	}

	// Close child windows first
	for _, child := range w.ChildWindows() {
		child.Close()
	}

	// Remove from parent
	if parent := w.ParentWindow(); parent != nil {
		parent.removeChildWindow(w)
	}

	w.Hide()
	return true
}

// SetOnClose sets the close handler.
func (w *Window) SetOnClose(handler func() bool) {
	w.mu.Lock()
	w.onClose = handler
	w.mu.Unlock()
}

// SetOnResize sets the resize handler.
func (w *Window) SetOnResize(handler func(width, height core.Unit)) {
	w.mu.Lock()
	w.onResize = handler
	w.mu.Unlock()
}

// SetOnMove sets the move handler.
func (w *Window) SetOnMove(handler func(x, y core.Unit)) {
	w.mu.Lock()
	w.onMove = handler
	w.mu.Unlock()
}

// SetOnActivate sets the activation handler.
func (w *Window) SetOnActivate(handler func(active bool)) {
	w.mu.Lock()
	w.onActivate = handler
	w.mu.Unlock()
}

// SetOnMinimizeRequest sets the minimize request handler.
// Called when the user clicks the minimize button. The handler should
// call WindowManager.MinimizeWindow() to properly minimize the window.
func (w *Window) SetOnMinimizeRequest(handler func()) {
	w.mu.Lock()
	w.onMinimizeRequest = handler
	w.mu.Unlock()
}

// SetOnMaximizeRequest sets the maximize/restore request handler.
// Called when the user clicks the maximize button or double-clicks titlebar.
// The handler should call WindowManager.MaximizeWindow() or RestoreWindow().
func (w *Window) SetOnMaximizeRequest(handler func()) {
	w.mu.Lock()
	w.onMaximizeRequest = handler
	w.mu.Unlock()
}

// SetBorderStyle sets the border style.
func (w *Window) SetBorderStyle(border style.BorderStyle) {
	w.mu.Lock()
	w.borderStyle = border
	w.mu.Unlock()
	w.Update()
}

// SetTitleStyle sets the title bar style.
func (w *Window) SetTitleStyle(s style.CellStyle) {
	w.mu.Lock()
	w.titleStyle = s
	w.mu.Unlock()
	w.Update()
}

// SetFrameStyle sets the frame style.
func (w *Window) SetFrameStyle(s style.CellStyle) {
	w.mu.Lock()
	w.frameStyle = s
	w.mu.Unlock()
	w.Update()
}

// contentBounds returns the bounds for the content area.
func (w *Window) contentBounds() core.UnitRect {
	bounds := w.Bounds()
	metrics := core.DefaultCellMetrics()

	// In maximized mode, no side borders (full width)
	if w.state == WindowStateMaximized && w.flags&WindowFlagNoTitle == 0 {
		// Only top title bar, no side borders
		return core.UnitRect{
			X:      0,
			Y:      metrics.CellHeight, // One row for title
			Width:  bounds.Width,
			Height: bounds.Height - metrics.CellHeight,
		}
	}

	// Normal mode with full frame
	if w.flags&WindowFlagFrameless != 0 {
		return core.UnitRect{Width: bounds.Width, Height: bounds.Height}
	}

	// Account for frame
	left := metrics.CellWidth
	top := metrics.CellHeight
	right := metrics.CellWidth
	bottom := metrics.CellHeight

	if w.flags&WindowFlagNoTitle != 0 {
		top = metrics.CellHeight // Just border, no extra title row
	}

	return core.UnitRect{
		X:      left,
		Y:      top,
		Width:  bounds.Width - left - right,
		Height: bounds.Height - top - bottom,
	}
}

// layoutContent lays out the content widget.
func (w *Window) layoutContent() {
	w.mu.RLock()
	content := w.content
	layout := w.layout
	w.mu.RUnlock()

	if content == nil {
		return
	}

	contentRect := w.contentBounds()

	if layout != nil {
		layout.Layout(w, contentRect)
	} else {
		content.SetBounds(contentRect)
	}
}

// Children implements core.Container.
func (w *Window) Children() []core.Widget {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.content == nil {
		return nil
	}
	return []core.Widget{w.content}
}

// AddChild implements core.Container.
func (w *Window) AddChild(child core.Widget) {
	w.SetContent(child)
}

// RemoveChild implements core.Container.
func (w *Window) RemoveChild(child core.Widget) {
	w.mu.Lock()
	if w.content == child {
		w.content = nil
	}
	w.mu.Unlock()
}

// ChildAt implements core.Container.
func (w *Window) ChildAt(pos core.UnitPoint) core.Widget {
	w.mu.RLock()
	content := w.content
	w.mu.RUnlock()

	if content == nil {
		return nil
	}

	contentRect := w.contentBounds()
	localPos := core.UnitPoint{
		X: pos.X - contentRect.X,
		Y: pos.Y - contentRect.Y,
	}

	if content.Bounds().Contains(localPos) {
		return content
	}
	return nil
}

// Layout implements core.Container.
func (w *Window) Layout() {
	w.layoutContent()
}

// Paint renders the window.
func (w *Window) Paint(p *core.Painter) {
	w.mu.RLock()
	flags := w.flags
	state := w.state
	title := w.title
	border := w.borderStyle
	titleStyle := w.titleStyle
	frameStyle := w.frameStyle
	content := w.content
	focused := w.HasFocus()
	w.mu.RUnlock()

	bounds := w.Bounds()
	metrics := p.Metrics()

	// Adjust styles based on focus
	if !focused {
		// Dim the title when not focused
		titleStyle = titleStyle.WithAttrs(style.StyleDim)
	}

	// Draw frame based on state
	if state == WindowStateMaximized && flags&WindowFlagNoTitle == 0 {
		// Maximized: only title bar, no side borders
		w.paintMaximizedFrame(p, bounds, metrics, title, titleStyle, frameStyle, border)
	} else if flags&WindowFlagFrameless == 0 {
		// Normal frame
		w.paintNormalFrame(p, bounds, metrics, title, titleStyle, frameStyle, border, flags)
	}

	// Paint content
	if content != nil {
		contentBounds := w.contentBounds()
		contentPainter := p.WithOffset(contentBounds.X, contentBounds.Y).
			WithClip(core.UnitRect{Width: contentBounds.Width, Height: contentBounds.Height})
		content.Paint(contentPainter)
	}

	// Paint child windows
	for _, child := range w.ChildWindows() {
		if child.IsVisible() && !child.IsMinimized() {
			childBounds := child.Bounds()
			childPainter := p.WithOffset(childBounds.X, childBounds.Y)
			child.Paint(childPainter)
		}
	}
}

// paintMaximizedFrame draws the title bar only (no side borders).
func (w *Window) paintMaximizedFrame(p *core.Painter, bounds core.UnitRect, metrics core.CellMetrics,
	title string, titleStyle, frameStyle style.CellStyle, border style.BorderStyle) {

	w.mu.RLock()
	flags := w.flags
	state := w.state
	w.mu.RUnlock()

	// Fill title bar background
	titleRect := core.UnitRect{
		X:      0,
		Y:      0,
		Width:  bounds.Width,
		Height: metrics.CellHeight,
	}
	p.FillRect(titleRect, ' ', titleStyle)

	// Draw window controls on the LEFT: [x][.][^] or [x][.][o]
	controlX := core.Unit(0)
	if flags&WindowFlagNoClose == 0 {
		p.DrawText(controlX, 0, "[x]", frameStyle)
		controlX += metrics.TextWidth(3)
	}
	if flags&WindowFlagNoMinimize == 0 {
		p.DrawText(controlX, 0, "[.]", frameStyle)
		controlX += metrics.TextWidth(3)
	}
	if flags&WindowFlagNoMaximize == 0 {
		if state == WindowStateMaximized {
			p.DrawText(controlX, 0, "[o]", frameStyle) // Restore icon
		} else {
			p.DrawText(controlX, 0, "[^]", frameStyle) // Maximize icon
		}
		controlX += metrics.TextWidth(3)
	}

	// Draw title text centered
	p.DrawTextAligned(titleRect, title, core.AlignCenter, core.AlignMiddle, titleStyle)
}

// paintNormalFrame draws the full window frame with borders.
func (w *Window) paintNormalFrame(p *core.Painter, bounds core.UnitRect, metrics core.CellMetrics,
	title string, titleStyle, frameStyle style.CellStyle, border style.BorderStyle, flags WindowFlags) {

	w.mu.RLock()
	state := w.state
	w.mu.RUnlock()

	// Draw border at local (0,0) - painter is already offset to window position
	localBounds := core.UnitRect{Width: bounds.Width, Height: bounds.Height}
	p.DrawRect(localBounds, border, frameStyle)

	// Draw title if enabled
	if flags&WindowFlagNoTitle == 0 {
		// Draw window controls on the LEFT: [x][.][^] or [x][.][o]
		controlX := metrics.CellWidth // Start after left border
		if flags&WindowFlagNoClose == 0 {
			p.DrawText(controlX, 0, "[x]", frameStyle)
			controlX += metrics.TextWidth(3)
		}
		if flags&WindowFlagNoMinimize == 0 {
			p.DrawText(controlX, 0, "[.]", frameStyle)
			controlX += metrics.TextWidth(3)
		}
		if flags&WindowFlagNoMaximize == 0 {
			if state == WindowStateMaximized {
				p.DrawText(controlX, 0, "[o]", frameStyle) // Restore icon
			} else {
				p.DrawText(controlX, 0, "[^]", frameStyle) // Maximize icon
			}
			controlX += metrics.TextWidth(3)
		}

		// Calculate title area (centered on top border)
		titleRect := core.UnitRect{
			X:      0,
			Y:      0,
			Width:  bounds.Width,
			Height: metrics.CellHeight,
		}

		// Draw title text centered
		displayTitle := title
		maxTitleWidth := metrics.CharsForWidth(bounds.Width) - 12 // Leave room for controls on both sides
		if len(displayTitle) > maxTitleWidth && maxTitleWidth > 0 {
			displayTitle = displayTitle[:maxTitleWidth-1] + "…"
		}
		p.DrawTextAligned(titleRect, displayTitle, core.AlignCenter, core.AlignMiddle, titleStyle)
	}

	// Fill content area with background
	contentBounds := w.contentBounds()
	theme := w.Theme()
	p.FillRect(contentBounds, ' ', theme.WindowBackground)
}

// HandleKeyPress handles keyboard input.
func (w *Window) HandleKeyPress(event core.KeyPressEvent) bool {
	w.mu.RLock()
	fm := w.focusManager
	w.mu.RUnlock()

	// Use focus manager to handle Tab navigation and forward to focused widget
	if fm != nil {
		if fm.HandleKeyPress(event) {
			return true
		}
	}

	// Handle window-specific keys
	switch event.Key {
	case "M-F4": // Alt+F4 - Close
		w.Close()
		return true
	case "M-F10": // Alt+F10 - Maximize/Restore
		if w.IsMaximized() {
			w.Restore()
		} else {
			w.Maximize()
		}
		return true
	}

	return false
}

// HandleMousePress handles mouse clicks.
func (w *Window) HandleMousePress(event core.MousePressEvent) bool {
	w.mu.RLock()
	content := w.content
	flags := w.flags
	state := w.state
	minHandler := w.onMinimizeRequest
	maxHandler := w.onMaximizeRequest
	w.mu.RUnlock()

	metrics := core.DefaultCellMetrics()

	// Check for title bar clicks
	if flags&WindowFlagNoTitle == 0 && event.Y < metrics.CellHeight {
		// Control buttons are on the left: [x][.][^]
		// Each button is 3 characters wide
		controlX := metrics.CellWidth // Start after left border (for normal frame)
		if state == WindowStateMaximized {
			controlX = 0 // No border in maximized state
		}

		// Check close button [x]
		if flags&WindowFlagNoClose == 0 {
			if event.X >= controlX && event.X < controlX+metrics.TextWidth(3) {
				w.Close()
				return true
			}
			controlX += metrics.TextWidth(3)
		}

		// Check minimize button [.]
		if flags&WindowFlagNoMinimize == 0 {
			if event.X >= controlX && event.X < controlX+metrics.TextWidth(3) {
				if minHandler != nil {
					minHandler()
				} else {
					w.Minimize()
				}
				return true
			}
			controlX += metrics.TextWidth(3)
		}

		// Check maximize/restore button [^] or [o]
		if flags&WindowFlagNoMaximize == 0 {
			if event.X >= controlX && event.X < controlX+metrics.TextWidth(3) {
				if maxHandler != nil {
					maxHandler()
				} else if w.IsMaximized() {
					w.Restore()
				} else {
					w.Maximize()
				}
				return true
			}
		}

		// Title bar click - could start drag (handled by window manager)
		return true
	}

	// Pass to content
	if content != nil {
		contentBounds := w.contentBounds()
		localEvent := event
		localEvent.X -= contentBounds.X
		localEvent.Y -= contentBounds.Y
		if content.HandleMousePress(localEvent) {
			return true
		}
	}

	return true // Consume click
}

// HandleMouseMove handles mouse movement.
func (w *Window) HandleMouseMove(event core.MouseMoveEvent) bool {
	w.mu.RLock()
	content := w.content
	w.mu.RUnlock()

	// Forward to content
	if content != nil {
		if handler, ok := content.(interface {
			HandleMouseMove(core.MouseMoveEvent) bool
		}); ok {
			contentBounds := w.contentBounds()
			localEvent := event
			localEvent.X -= contentBounds.X
			localEvent.Y -= contentBounds.Y
			if handler.HandleMouseMove(localEvent) {
				return true
			}
		}
	}

	return false
}

// HandleMouseRelease handles mouse button release.
func (w *Window) HandleMouseRelease(event core.MouseReleaseEvent) bool {
	w.mu.RLock()
	content := w.content
	w.mu.RUnlock()

	// Forward to content
	if content != nil {
		if handler, ok := content.(interface {
			HandleMouseRelease(core.MouseReleaseEvent) bool
		}); ok {
			contentBounds := w.contentBounds()
			localEvent := event
			localEvent.X -= contentBounds.X
			localEvent.Y -= contentBounds.Y
			if handler.HandleMouseRelease(localEvent) {
				return true
			}
		}
	}

	return false
}

// HandleResize is called when the window is resized.
func (w *Window) HandleResize(oldSize, newSize core.UnitSize) {
	w.layoutContent()

	w.mu.RLock()
	handler := w.onResize
	w.mu.RUnlock()

	if handler != nil {
		handler(newSize.Width, newSize.Height)
	}
}

// SizeHint returns the preferred size for the window.
func (w *Window) SizeHint() core.UnitSize {
	w.mu.RLock()
	content := w.content
	flags := w.flags
	w.mu.RUnlock()

	metrics := core.DefaultCellMetrics()

	var width, height core.Unit

	if content != nil {
		hint := content.SizeHint()
		width = hint.Width
		height = hint.Height
	}

	// Add frame
	if flags&WindowFlagFrameless == 0 {
		width += metrics.CellWidth * 2  // Left and right borders
		height += metrics.CellHeight * 2 // Top and bottom borders
	}

	// Ensure minimum size
	w.mu.RLock()
	if width < w.minWidth {
		width = w.minWidth
	}
	if height < w.minHeight {
		height = w.minHeight
	}
	w.mu.RUnlock()

	return core.UnitSize{Width: width, Height: height}
}

// verify Window implements Container
var _ core.Container = (*Window)(nil)
