// Package window provides windowing support for the TUI toolkit.
package window

import (
	"strings"
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

// TitleButton identifies a titlebar button.
type TitleButton int

const (
	TitleButtonNone     TitleButton = iota
	TitleButtonClose                // [x] button
	TitleButtonMinimize             // [.] button
	TitleButtonMaximize             // [^] or [o] button
)

// TitleFocus identifies which title bar element has keyboard focus.
type TitleFocus int

const (
	TitleFocusNone     TitleFocus = iota // No title bar element focused
	TitleFocusTitle                      // Title text focused (for moving)
	TitleFocusClose                      // Close button focused
	TitleFocusMinimize                   // Minimize button focused
	TitleFocusMaximize                   // Maximize button focused
	TitleFocusBlur                       // Blur item focused (exit window)
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
	onMinimizeRequest     func()               // Called when user clicks minimize button
	onMaximizeRequest     func()               // Called when user clicks maximize button
	getConstrainingBounds func() core.UnitRect // Returns the client area for movement constraints
	popupController       core.PopupController // Popup controller for ComboBox etc.

	// Button press tracking
	pressedButton TitleButton // Currently pressed titlebar button
	buttonHovered bool        // Whether mouse is still over the pressed button

	// Title bar keyboard focus
	titleFocus        TitleFocus    // Which title bar element has keyboard focus
	resizeEdges       int           // Which edges are being keyboard-resized (ResizeEdge* constants)
	resizeStartBounds core.UnitRect // Bounds when resize operation started (for Escape to revert)

	// Active state (set by WindowManager/MDIPane, separate from focus)
	isActive bool
}

// NewWindow creates a new window with the given title.
func NewWindow(title string) *Window {
	w := &Window{
		title:       title,
		state:       WindowStateNormal,
		borderStyle: style.BorderDouble,
		titleStyle:  style.DefaultStyle().WithFg(style.ColorWhite).WithBg(style.ColorBlue).Bold(),
		frameStyle:  style.DefaultStyle().WithFg(style.ColorBrightCyan).WithBg(style.ColorBlue),
		minWidth:    80,  // 10 characters minimum
		minHeight:   48,  // 3 lines minimum
		maxWidth:    1<<30 - 1,
		maxHeight:   1<<30 - 1,
	}
	w.WidgetBase = *core.NewWidgetBase()
	w.Init(w)
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

	// Update focus manager root and focus first non-furtive widget
	if fm != nil {
		fm.SetRoot(widget)
		fm.FocusFirstNonFurtive()
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
	w.pressedButton = TitleButtonNone // Reset pressed button state
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

// IsActive returns true if this window is the active window in its container
// (WindowManager or MDIPane). This is separate from focus - a window is active
// when it's selected, even if a child widget has keyboard focus.
func (w *Window) IsActive() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.isActive
}

// SetActive sets the window's active state. This is called by WindowManager
// or MDIPane when the window becomes the active (selected) window.
func (w *Window) SetActive(active bool) {
	w.mu.Lock()
	if w.isActive == active {
		w.mu.Unlock()
		return
	}
	w.isActive = active
	handler := w.onActivate
	w.mu.Unlock()

	if handler != nil {
		handler(active)
	}
	w.Update()
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

// SetGetConstrainingBounds sets the callback to get the client area for movement constraints.
// This is called during keyboard window movement to constrain the window position.
func (w *Window) SetGetConstrainingBounds(handler func() core.UnitRect) {
	w.mu.Lock()
	w.getConstrainingBounds = handler
	w.mu.Unlock()
}

// SetPopupController sets the popup controller for this window.
// This is called by WindowManager when the window is added.
func (w *Window) SetPopupController(pc core.PopupController) {
	w.mu.Lock()
	w.popupController = pc
	w.mu.Unlock()
}

// PopupController returns the popup controller for this window.
// This implements the interface needed by widgets like ComboBox.
func (w *Window) PopupController() core.PopupController {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.popupController
}

// RegisterPopup implements core.PopupController by delegating to the stored controller.
func (w *Window) RegisterPopup(request *core.PopupRequest) {
	w.mu.RLock()
	pc := w.popupController
	w.mu.RUnlock()
	if pc != nil {
		pc.RegisterPopup(request)
	}
}

// UnregisterPopup implements core.PopupController by delegating to the stored controller.
func (w *Window) UnregisterPopup(id string) {
	w.mu.RLock()
	pc := w.popupController
	w.mu.RUnlock()
	if pc != nil {
		pc.UnregisterPopup(id)
	}
}

// MapToScreen implements core.PopupController by delegating to the stored controller.
func (w *Window) MapToScreen(widget core.Widget, local core.UnitPoint) core.UnitPoint {
	w.mu.RLock()
	pc := w.popupController
	w.mu.RUnlock()
	if pc != nil {
		return pc.MapToScreen(widget, local)
	}
	return local
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

// BackgroundColor returns the window's explicit background color, if set.
func (w *Window) BackgroundColor() *style.Color {
	return w.WidgetBase.BackgroundColor()
}

// SchemeBackgroundColor returns the window's scheme-derived background color.
// This is the color the window paints its content area with, based on its scheme.
func (w *Window) SchemeBackgroundColor() *style.Color {
	scheme := w.GetScheme()
	focused := w.HasFocus()
	bgColor := scheme.GetWindowBG(focused)
	return &bgColor
}

// contentBounds returns the bounds for the content area.
func (w *Window) contentBounds() core.UnitRect {
	bounds := w.Bounds()
	metrics := core.DefaultCellMetrics()

	w.mu.RLock()
	state := w.state
	flags := w.flags
	w.mu.RUnlock()

	// In maximized mode, no side borders (full width)
	if state == WindowStateMaximized && flags&WindowFlagNoTitle == 0 {
		// Only top title bar, no side borders
		return core.UnitRect{
			X:      0,
			Y:      metrics.CellHeight, // One row for title
			Width:  bounds.Width,
			Height: bounds.Height - metrics.CellHeight,
		}
	}

	// Normal mode with full frame
	if flags&WindowFlagFrameless != 0 {
		return core.UnitRect{Width: bounds.Width, Height: bounds.Height}
	}

	// Account for frame
	left := metrics.CellWidth
	top := metrics.CellHeight
	right := metrics.CellWidth
	bottom := metrics.CellHeight

	if flags&WindowFlagNoTitle != 0 {
		top = metrics.CellHeight // Just border, no extra title row
	}

	return core.UnitRect{
		X:      left,
		Y:      top,
		Width:  bounds.Width - left - right,
		Height: bounds.Height - top - bottom,
	}
}

// ClientAreaOffset returns the offset from the window's top-left corner
// to the client (content) area. This accounts for title bar and frame.
func (w *Window) ClientAreaOffset() core.UnitPoint {
	cb := w.contentBounds()
	return core.UnitPoint{X: cb.X, Y: cb.Y}
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

	// Content bounds should be relative to the content area (0,0), not the window.
	// The window's Paint method handles the offset translation.
	localContentRect := core.UnitRect{
		X:      0,
		Y:      0,
		Width:  contentRect.Width,
		Height: contentRect.Height,
	}

	if layout != nil {
		layout.Layout(w, localContentRect)
	} else {
		content.SetBounds(localContentRect)
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
	content := w.content
	isActive := w.isActive
	w.mu.RUnlock()

	bounds := w.Bounds()
	metrics := p.Metrics()
	scheme := w.GetScheme()

	// Window appears focused if it's the active window in its container.
	// For MDI children (parent is MDIPane with StrongFocus): also require parent to have focus,
	// so MDI windows only appear focused when their tab is active.
	// For top-level windows (parent is Desktop with NoFocus): don't check parent focus.
	focused := isActive
	if focused {
		if parent := w.Parent(); parent != nil {
			policy := parent.FocusPolicy()
			if policy == core.StrongFocus || policy == core.TabFocus {
				// MDI-style container: check if parent has focus OR this window has internal focus.
				// When clicking on a widget inside the window, focus goes to that widget (not parent).
				if !parent.HasFocus() {
					windowHasInternalFocus := false
					if fm := w.FocusManager(); fm != nil {
						if focusedWidget := fm.FocusedWidget(); focusedWidget != nil {
							windowHasInternalFocus = focusedWidget.HasFocus()
						}
					}
					focused = windowHasInternalFocus
				}
			}
		}
	}

	// Check for passive state: window is remembered by menu bar while no window is active
	isPassive := false
	if parent := w.Parent(); parent != nil {
		if provider, ok := parent.(core.PassiveWindowProvider); ok {
			isPassive = provider.IsWindowPassive(w)
		}
	}

	// Get styles from scheme based on focus state
	// Passive windows use active colors (same as focused)
	titleStyle := scheme.GetWindowTitle(focused || isPassive)
	frameStyle := scheme.GetWindowBorder(focused || isPassive)

	// Passive windows use heavy (thick single-line) border instead of double
	frameBorder := border
	if isPassive {
		frameBorder = style.BorderHeavy
	}

	// Draw frame based on state
	if state == WindowStateMaximized && flags&WindowFlagNoTitle == 0 {
		// Maximized: only title bar, no side borders
		w.paintMaximizedFrame(p, bounds, metrics, title, titleStyle, frameStyle, frameBorder)
	} else if flags&WindowFlagFrameless == 0 {
		// Normal frame
		w.paintNormalFrame(p, bounds, metrics, title, titleStyle, frameStyle, frameBorder, flags)
	}

	// Paint content
	if content != nil {
		contentBounds := w.contentBounds()
		contentPainter := p.WithOffset(contentBounds.X, contentBounds.Y).
			WithClip(core.UnitRect{Width: contentBounds.Width, Height: contentBounds.Height})
		content.Paint(contentPainter)
	}

	// Paint child windows (within the content area, clipped)
	if len(w.ChildWindows()) > 0 {
		contentBounds := w.contentBounds()
		// Create a painter clipped to the content area
		contentPainter := p.WithOffset(contentBounds.X, contentBounds.Y).
			WithClip(core.UnitRect{Width: contentBounds.Width, Height: contentBounds.Height})

		for _, child := range w.ChildWindows() {
			if child.IsVisible() && !child.IsMinimized() {
				childBounds := child.Bounds()
				childPainter := contentPainter.WithOffset(childBounds.X, childBounds.Y)
				child.Paint(childPainter)
			}
		}
	}
}

// paintMaximizedFrame draws the title bar only (no side borders).
func (w *Window) paintMaximizedFrame(p *core.Painter, bounds core.UnitRect, metrics core.CellMetrics,
	title string, titleStyle, frameStyle style.CellStyle, border style.BorderStyle) {

	w.mu.RLock()
	flags := w.flags
	state := w.state
	pressedButton := w.pressedButton
	buttonHovered := w.buttonHovered
	titleFocus := w.titleFocus
	w.mu.RUnlock()

	// Fill title bar background
	titleRect := core.UnitRect{
		X:      0,
		Y:      0,
		Width:  bounds.Width,
		Height: metrics.CellHeight,
	}
	p.FillRect(titleRect, ' ', titleStyle)

	scheme := w.GetScheme()
	// Derive visual focus: active AND (parent has focus OR window has internal focus)
	focused := w.IsActive()
	if focused {
		if parent := w.Parent(); parent != nil {
			policy := parent.FocusPolicy()
			if policy == core.StrongFocus || policy == core.TabFocus {
				if !parent.HasFocus() {
					windowHasInternalFocus := false
					if fm := w.FocusManager(); fm != nil {
						if focusedWidget := fm.FocusedWidget(); focusedWidget != nil {
							windowHasInternalFocus = focusedWidget.HasFocus()
						}
					}
					focused = windowHasInternalFocus
				}
			}
		}
	}

	// Draw window controls on the LEFT: [x][.][^] or [x][.][o]
	controlX := core.Unit(0)
	if flags&WindowFlagNoClose == 0 {
		isFocused := titleFocus == TitleFocusClose
		isPressed := pressedButton == TitleButtonClose && buttonHovered
		btnStyle := scheme.GetTitleBarButton(focused, isFocused, isPressed)
		p.DrawText(controlX, 0, "[x]", btnStyle)
		controlX += metrics.TextWidth(3)
	}
	if flags&WindowFlagNoMinimize == 0 {
		isFocused := titleFocus == TitleFocusMinimize
		isPressed := pressedButton == TitleButtonMinimize && buttonHovered
		btnStyle := scheme.GetTitleBarButton(focused, isFocused, isPressed)
		p.DrawText(controlX, 0, "[.]", btnStyle)
		controlX += metrics.TextWidth(3)
	}
	if flags&WindowFlagNoMaximize == 0 {
		isFocused := titleFocus == TitleFocusMaximize
		isPressed := pressedButton == TitleButtonMaximize && buttonHovered
		btnStyle := scheme.GetTitleBarButton(focused, isFocused, isPressed)
		if state == WindowStateMaximized {
			p.DrawText(controlX, 0, "[o]", btnStyle) // Restore icon
		} else {
			p.DrawText(controlX, 0, "[^]", btnStyle) // Maximize icon
		}
		controlX += metrics.TextWidth(3)
	}

	// Draw title text centered, with angle brackets and cyan bg if title has keyboard focus
	displayTitle := title
	titleDisplayStyle := titleStyle
	if titleFocus == TitleFocusTitle {
		displayTitle = "< " + title + " >"
		titleDisplayStyle = scheme.GetTitleBarButton(focused, true, false)
	}
	p.DrawTextAligned(titleRect, displayTitle, core.AlignCenter, core.AlignMiddle, titleDisplayStyle)

	// Draw blur button on far right when blur item is focused
	if titleFocus == TitleFocusBlur {
		blurBtnStyle := scheme.GetTitleBarButton(focused, true, false) // Focused button style
		blurX := bounds.Width - metrics.TextWidth(3)                   // Position at far right
		p.DrawText(blurX, 0, "[~]", blurBtnStyle)
	}

	// Fill content area with background (same as normal frame)
	contentBounds := w.contentBounds()
	theme := w.Theme()
	p.FillRect(contentBounds, ' ', theme.WindowBackground)
}

// paintNormalFrame draws the full window frame with borders.
func (w *Window) paintNormalFrame(p *core.Painter, bounds core.UnitRect, metrics core.CellMetrics,
	title string, titleStyle, frameStyle style.CellStyle, border style.BorderStyle, flags WindowFlags) {

	w.mu.RLock()
	state := w.state
	pressedButton := w.pressedButton
	buttonHovered := w.buttonHovered
	titleFocus := w.titleFocus
	w.mu.RUnlock()

	// Draw border at local (0,0) - painter is already offset to window position
	localBounds := core.UnitRect{Width: bounds.Width, Height: bounds.Height}

	// When blur item is focused, draw dashed frame with inactive title color
	// but keep corners, horizontally adjacent chars, and buttons in active color
	if titleFocus == TitleFocusBlur {
		scheme := w.GetScheme()
		blurFrameStyle := scheme.GetWindowTitle(false)  // Inactive title color for dashed lines
		activeFrameStyle := scheme.GetWindowBorder(true) // Active color for corners

		// Dashed line characters
		horizDash := '┄' // U+2504 BOX DRAWINGS LIGHT TRIPLE DASH HORIZONTAL
		vertDash := '┆'  // U+2506 BOX DRAWINGS LIGHT TRIPLE DASH VERTICAL

		// Double corners (in active color)
		topLeft := '╔'
		topRight := '╗'
		bottomLeft := '╚'
		bottomRight := '╝'

		// Get border character for horizontally adjacent positions
		horizLine := border.Horizontal

		// Draw corners in active color
		p.DrawCell(0, 0, topLeft, activeFrameStyle)
		p.DrawCell(localBounds.Width-metrics.CellWidth, 0, topRight, activeFrameStyle)
		p.DrawCell(0, localBounds.Height-metrics.CellHeight, bottomLeft, activeFrameStyle)
		p.DrawCell(localBounds.Width-metrics.CellWidth, localBounds.Height-metrics.CellHeight, bottomRight, activeFrameStyle)

		// Draw top edge - first and last chars adjacent to corners in active color, rest dashed
		for x := metrics.CellWidth; x < localBounds.Width-metrics.CellWidth; x += metrics.CellWidth {
			if x == metrics.CellWidth || x == localBounds.Width-2*metrics.CellWidth {
				// Adjacent to corner - use active style with normal horizontal line
				p.DrawCell(x, 0, horizLine, activeFrameStyle)
			} else {
				p.DrawCell(x, 0, horizDash, blurFrameStyle)
			}
		}

		// Draw bottom edge - first and last chars adjacent to corners in active color, rest dashed
		for x := metrics.CellWidth; x < localBounds.Width-metrics.CellWidth; x += metrics.CellWidth {
			if x == metrics.CellWidth || x == localBounds.Width-2*metrics.CellWidth {
				// Adjacent to corner - use active style with normal horizontal line
				p.DrawCell(x, localBounds.Height-metrics.CellHeight, horizLine, activeFrameStyle)
			} else {
				p.DrawCell(x, localBounds.Height-metrics.CellHeight, horizDash, blurFrameStyle)
			}
		}

		// Draw left edge - all dashed
		for y := metrics.CellHeight; y < localBounds.Height-metrics.CellHeight; y += metrics.CellHeight {
			p.DrawCell(0, y, vertDash, blurFrameStyle)
		}

		// Draw right edge - all dashed
		for y := metrics.CellHeight; y < localBounds.Height-metrics.CellHeight; y += metrics.CellHeight {
			p.DrawCell(localBounds.Width-metrics.CellWidth, y, vertDash, blurFrameStyle)
		}
	} else {
		p.DrawRect(localBounds, border, frameStyle)
	}

	scheme := w.GetScheme()
	// Derive visual focus: active AND (parent has focus OR window has internal focus)
	// When blur item is focused, buttons stay in active color but title bar text uses inactive
	focused := w.IsActive()
	if focused {
		if parent := w.Parent(); parent != nil {
			policy := parent.FocusPolicy()
			if policy == core.StrongFocus || policy == core.TabFocus {
				if !parent.HasFocus() {
					windowHasInternalFocus := false
					if fm := w.FocusManager(); fm != nil {
						if focusedWidget := fm.FocusedWidget(); focusedWidget != nil {
							windowHasInternalFocus = focusedWidget.HasFocus()
						}
					}
					focused = windowHasInternalFocus
				}
			}
		}
	}

	// For button styling, use active appearance even when blur is focused
	buttonFocused := focused || titleFocus == TitleFocusBlur

	// Draw title if enabled
	if flags&WindowFlagNoTitle == 0 {
		// Draw window controls on the LEFT: [x][.][^] or [x][.][o]
		controlX := metrics.CellWidth // Start after left border
		if flags&WindowFlagNoClose == 0 {
			isFocused := titleFocus == TitleFocusClose
			isPressed := pressedButton == TitleButtonClose && buttonHovered
			btnStyle := scheme.GetTitleBarButton(buttonFocused, isFocused, isPressed)
			p.DrawText(controlX, 0, "[x]", btnStyle)
			controlX += metrics.TextWidth(3)
		}
		if flags&WindowFlagNoMinimize == 0 {
			isFocused := titleFocus == TitleFocusMinimize
			isPressed := pressedButton == TitleButtonMinimize && buttonHovered
			btnStyle := scheme.GetTitleBarButton(buttonFocused, isFocused, isPressed)
			p.DrawText(controlX, 0, "[.]", btnStyle)
			controlX += metrics.TextWidth(3)
		}
		if flags&WindowFlagNoMaximize == 0 {
			isFocused := titleFocus == TitleFocusMaximize
			isPressed := pressedButton == TitleButtonMaximize && buttonHovered
			btnStyle := scheme.GetTitleBarButton(buttonFocused, isFocused, isPressed)
			if state == WindowStateMaximized {
				p.DrawText(controlX, 0, "[o]", btnStyle) // Restore icon
			} else {
				p.DrawText(controlX, 0, "[^]", btnStyle) // Maximize icon
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

		// Draw title text centered, with angle brackets and cyan bg if title has keyboard focus
		displayTitle := title
		titleDisplayStyle := titleStyle
		if titleFocus == TitleFocusTitle {
			displayTitle = "< " + title + " >"
			titleDisplayStyle = scheme.GetTitleBarButton(focused, true, false)
		} else if titleFocus == TitleFocusBlur {
			// Blur item focused - use inactive title style for the title text
			titleDisplayStyle = scheme.GetWindowTitle(false)
		}
		maxTitleWidth := metrics.CharsForWidth(bounds.Width) - 12 // Leave room for controls on both sides
		if len(displayTitle) > maxTitleWidth && maxTitleWidth > 0 {
			displayTitle = displayTitle[:maxTitleWidth-1] + "…"
		}
		p.DrawTextAligned(titleRect, displayTitle, core.AlignCenter, core.AlignMiddle, titleDisplayStyle)

		// Draw blur button on far right when blur item is focused
		if titleFocus == TitleFocusBlur {
			blurBtnStyle := scheme.GetTitleBarButton(true, true, false) // Focused button style
			blurX := localBounds.Width - metrics.CellWidth - metrics.TextWidth(3) // Position before right border
			p.DrawText(blurX, 0, "[~]", blurBtnStyle)
		}
	}

	// Fill content area with background
	contentBounds := w.contentBounds()
	theme := w.Theme()
	p.FillRect(contentBounds, ' ', theme.WindowBackground)
}

// buttonAtPosition returns which titlebar button is at the given local coordinates.
// Returns TitleButtonNone if not on a button.
func (w *Window) buttonAtPosition(x, y core.Unit) TitleButton {
	w.mu.RLock()
	flags := w.flags
	state := w.state
	w.mu.RUnlock()

	metrics := core.DefaultCellMetrics()

	// Must be in titlebar
	if flags&WindowFlagNoTitle != 0 || y >= metrics.CellHeight {
		return TitleButtonNone
	}

	// Control buttons are on the left
	controlX := metrics.CellWidth // Start after left border (for normal frame)
	if state == WindowStateMaximized {
		controlX = 0 // No border in maximized state
	}

	buttonWidth := metrics.TextWidth(3)

	// Check close button [x]
	if flags&WindowFlagNoClose == 0 {
		if x >= controlX && x < controlX+buttonWidth {
			return TitleButtonClose
		}
		controlX += buttonWidth
	}

	// Check minimize button [.]
	if flags&WindowFlagNoMinimize == 0 {
		if x >= controlX && x < controlX+buttonWidth {
			return TitleButtonMinimize
		}
		controlX += buttonWidth
	}

	// Check maximize/restore button [^] or [o]
	if flags&WindowFlagNoMaximize == 0 {
		if x >= controlX && x < controlX+buttonWidth {
			return TitleButtonMaximize
		}
	}

	return TitleButtonNone
}

// TitleFocus returns the current title bar focus.
func (w *Window) TitleFocus() TitleFocus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.titleFocus
}

// SetTitleFocus sets which title bar element has keyboard focus.
func (w *Window) SetTitleFocus(focus TitleFocus) {
	w.mu.Lock()
	w.titleFocus = focus
	if focus == TitleFocusNone {
		w.resizeEdges = ResizeEdgeNone // Clear resize state when leaving title bar
	}
	w.mu.Unlock()
	w.Update()
}

// HasTitleFocus returns true if any title bar element has keyboard focus.
func (w *Window) HasTitleFocus() bool {
	return w.TitleFocus() != TitleFocusNone
}

// hasKeyboardBlurEnabled returns true if the parent container has keyboard blur enabled.
func (w *Window) hasKeyboardBlurEnabled() bool {
	parent := w.Parent()
	if parent == nil {
		return false
	}
	if provider, ok := parent.(core.KeyboardBlurChildrenProvider); ok {
		return provider.KeyboardBlurChildren()
	}
	return false
}

// performKeyboardBlur calls the parent's PerformKeyboardBlur if available.
func (w *Window) performKeyboardBlur() {
	parent := w.Parent()
	if parent == nil {
		return
	}
	if provider, ok := parent.(core.KeyboardBlurChildrenProvider); ok {
		provider.PerformKeyboardBlur()
	}
}

// handleTitleBarKey handles keyboard input when title bar has focus.
func (w *Window) handleTitleBarKey(event core.KeyPressEvent) bool {
	w.mu.RLock()
	titleFocus := w.titleFocus
	resizeEdges := w.resizeEdges
	flags := w.flags
	w.mu.RUnlock()

	metrics := core.DefaultCellMetrics()

	// Handle navigation between title bar elements
	switch event.Key {
	case "Tab":
		// Check if Shift is held - use same logic as S-Tab case
		if event.Modifiers&core.ShiftModifier != 0 {
			prev := w.prevTitleFocus(titleFocus)
			if prev == titleFocus {
				// At first title element, loop to content's last widget
				w.SetTitleFocus(TitleFocusNone)
				if fm := w.FocusManager(); fm != nil {
					fm.FocusLast()
				}
			} else {
				w.SetTitleFocus(prev)
			}
			return true
		}
		// Move to next title element or exit to content
		next := w.nextTitleFocus(titleFocus)
		if next == TitleFocusNone {
			// Exit title bar, focus first widget in content
			w.SetTitleFocus(TitleFocusNone)
			if fm := w.FocusManager(); fm != nil {
				fm.FocusFirst()
			}
		} else {
			w.SetTitleFocus(next)
		}
		return true

	case "S-Tab", "Shift-Tab":
		// Move to previous title element, or loop to content's last widget
		prev := w.prevTitleFocus(titleFocus)
		if prev == titleFocus {
			// At first title element, loop to content's last widget
			w.SetTitleFocus(TitleFocusNone)
			if fm := w.FocusManager(); fm != nil {
				fm.FocusLast()
			}
		} else {
			w.SetTitleFocus(prev)
		}
		return true

	case "Escape":
		// Exit title bar focus, return to content
		w.SetTitleFocus(TitleFocusNone)
		w.mu.Lock()
		w.resizeEdges = ResizeEdgeNone
		w.mu.Unlock()
		if fm := w.FocusManager(); fm != nil {
			fm.FocusFirst()
		}
		return true

	case "Enter", " ", "Space":
		// Activate focused button or confirm resize
		switch titleFocus {
		case TitleFocusClose:
			if flags&WindowFlagNoClose == 0 {
				w.Close()
			}
		case TitleFocusMinimize:
			if flags&WindowFlagNoMinimize == 0 {
				w.mu.RLock()
				handler := w.onMinimizeRequest
				w.mu.RUnlock()
				if handler != nil {
					handler()
				}
			}
		case TitleFocusMaximize:
			if flags&WindowFlagNoMaximize == 0 {
				w.mu.RLock()
				handler := w.onMaximizeRequest
				w.mu.RUnlock()
				if handler != nil {
					handler()
				}
			}
		case TitleFocusTitle:
			// Confirm resize - clear edges so next Shift+arrow starts fresh
			w.mu.Lock()
			if w.resizeEdges != ResizeEdgeNone {
				w.resizeEdges = ResizeEdgeNone
				w.resizeStartBounds = w.Bounds()
			}
			w.mu.Unlock()
		case TitleFocusBlur:
			// Blur the window - return focus to parent container
			w.SetTitleFocus(TitleFocusNone)
			w.performKeyboardBlur()
		}
		return true
	}

	// Handle window movement and resizing when title has focus
	if titleFocus == TitleFocusTitle {
		bounds := w.Bounds()
		hasShift := event.Modifiers&core.ShiftModifier != 0
		hasCtrl := event.Modifiers&core.ControlModifier != 0
		hasMeta := event.Modifiers&core.MetaModifier != 0
		hasAlt := event.Modifiers&core.AltModifier != 0

		// Determine movement multiplier based on modifiers
		// Alt/Meta/Ctrl increases horizontal by 10 chars, vertical by 4 lines
		horizStep := metrics.CellWidth
		vertStep := metrics.CellHeight
		if hasMeta || hasAlt || hasCtrl {
			horizStep = metrics.CellWidth * 10
			vertStep = metrics.CellHeight * 4
		}

		// Normalize key names - handle both "Left" and "S-Left" etc.
		key := event.Key
		if strings.HasPrefix(key, "S-") {
			hasShift = true
			key = key[2:]
		}
		if strings.HasPrefix(key, "M-") || strings.HasPrefix(key, "A-") || strings.HasPrefix(key, "C-") {
			hasMeta = true
			hasAlt = true
			hasCtrl = true
			key = key[2:]
			horizStep = metrics.CellWidth * 10
			vertStep = metrics.CellHeight * 4
		}

		switch key {
		case "Left":
			if hasShift {
				// Start/continue resizing left edge
				if resizeEdges&ResizeEdgeLeft != 0 {
					// Continue left resize: expand left
					newBounds := bounds
					newBounds.X -= horizStep
					newBounds.Width += horizStep
					w.SetBounds(newBounds)
				} else if resizeEdges&ResizeEdgeRight != 0 {
					// Continue right resize: shrink right edge
					newBounds := bounds
					newBounds.Width -= horizStep
					if newBounds.Width >= w.minWidth {
						w.SetBounds(newBounds)
					}
				} else {
					// Start: expand left edge
					w.mu.Lock()
					if w.resizeEdges == ResizeEdgeNone {
						w.resizeStartBounds = bounds // Save for Escape to revert
					}
					w.resizeEdges = ResizeEdgeLeft
					w.mu.Unlock()
					newBounds := bounds
					newBounds.X -= horizStep
					newBounds.Width += horizStep
					w.SetBounds(newBounds)
				}
			} else {
				// Move window left
				newBounds := bounds
				newBounds.X -= horizStep
				w.SetBounds(w.constrainBoundsForMovement(newBounds))
			}
			return true

		case "Right":
			if hasShift {
				// Start/continue resizing right edge
				if resizeEdges&ResizeEdgeRight != 0 {
					// Continue right resize: expand right
					newBounds := bounds
					newBounds.Width += horizStep
					w.SetBounds(newBounds)
				} else if resizeEdges&ResizeEdgeLeft != 0 {
					// Continue left resize: shrink left edge
					newBounds := bounds
					newBounds.X += horizStep
					newBounds.Width -= horizStep
					if newBounds.Width >= w.minWidth {
						w.SetBounds(newBounds)
					}
				} else {
					// Start: expand right edge
					w.mu.Lock()
					if w.resizeEdges == ResizeEdgeNone {
						w.resizeStartBounds = bounds // Save for Escape to revert
					}
					w.resizeEdges = ResizeEdgeRight
					w.mu.Unlock()
					newBounds := bounds
					newBounds.Width += horizStep
					w.SetBounds(newBounds)
				}
			} else {
				// Move window right
				newBounds := bounds
				newBounds.X += horizStep
				w.SetBounds(w.constrainBoundsForMovement(newBounds))
			}
			return true

		case "Up":
			if hasShift {
				// Start/continue resizing top edge
				if resizeEdges&ResizeEdgeTop != 0 {
					// Continue top resize: expand top
					newBounds := bounds
					newBounds.Y -= vertStep
					newBounds.Height += vertStep
					w.SetBounds(newBounds)
				} else if resizeEdges&ResizeEdgeBottom != 0 {
					// Continue bottom resize: shrink bottom edge
					newBounds := bounds
					newBounds.Height -= vertStep
					if newBounds.Height >= w.minHeight {
						w.SetBounds(newBounds)
					}
				} else {
					// Start: expand top edge
					w.mu.Lock()
					if w.resizeEdges == ResizeEdgeNone {
						w.resizeStartBounds = bounds // Save for Escape to revert
					}
					w.resizeEdges |= ResizeEdgeTop
					w.mu.Unlock()
					newBounds := bounds
					newBounds.Y -= vertStep
					newBounds.Height += vertStep
					w.SetBounds(newBounds)
				}
			} else {
				// Move window up
				newBounds := bounds
				newBounds.Y -= vertStep
				w.SetBounds(w.constrainBoundsForMovement(newBounds))
			}
			return true

		case "Down":
			if hasShift {
				// Start/continue resizing bottom edge
				if resizeEdges&ResizeEdgeBottom != 0 {
					// Continue bottom resize: expand bottom
					newBounds := bounds
					newBounds.Height += vertStep
					w.SetBounds(newBounds)
				} else if resizeEdges&ResizeEdgeTop != 0 {
					// Continue top resize: shrink top edge
					newBounds := bounds
					newBounds.Y += vertStep
					newBounds.Height -= vertStep
					if newBounds.Height >= w.minHeight {
						w.SetBounds(newBounds)
					}
				} else {
					// Start: expand bottom edge
					w.mu.Lock()
					if w.resizeEdges == ResizeEdgeNone {
						w.resizeStartBounds = bounds // Save for Escape to revert
					}
					w.resizeEdges |= ResizeEdgeBottom
					w.mu.Unlock()
					newBounds := bounds
					newBounds.Height += vertStep
					w.SetBounds(newBounds)
				}
			} else {
				// Move window down
				newBounds := bounds
				newBounds.Y += vertStep
				w.SetBounds(w.constrainBoundsForMovement(newBounds))
			}
			return true

		case "Enter", "Return", "KPEnter":
			// Confirm resize - clear edges so next Shift+arrow starts fresh
			// Also update resizeStartBounds to current bounds
			w.mu.Lock()
			if w.resizeEdges != ResizeEdgeNone {
				w.resizeEdges = ResizeEdgeNone
				w.resizeStartBounds = w.Bounds()
			}
			w.mu.Unlock()
			return true

		case "Escape", "Esc":
			// Cancel resize - revert to bounds from when resize started
			w.mu.Lock()
			if w.resizeEdges != ResizeEdgeNone {
				startBounds := w.resizeStartBounds
				w.resizeEdges = ResizeEdgeNone
				w.mu.Unlock()
				w.SetBounds(startBounds)
			} else {
				w.mu.Unlock()
			}
			return true
		}
	}

	return false
}

// constrainBoundsForMovement adjusts bounds to keep titlebar visible within client area.
// Horizontally: allows window to go almost off-screen (just 1 unit border visible)
// Vertically: titlebar must stay within client area
func (w *Window) constrainBoundsForMovement(newBounds core.UnitRect) core.UnitRect {
	w.mu.RLock()
	getBounds := w.getConstrainingBounds
	w.mu.RUnlock()

	if getBounds == nil {
		return newBounds
	}

	clientArea := getBounds()
	metrics := core.DefaultCellMetrics()

	// Keep titlebar visible vertically
	// Don't allow titlebar above client area
	if newBounds.Y < clientArea.Y {
		newBounds.Y = clientArea.Y
	}
	// Don't allow titlebar below client area
	maxY := clientArea.Y + clientArea.Height - metrics.CellHeight
	if newBounds.Y > maxY {
		newBounds.Y = maxY
	}

	// Allow window to go almost completely off-screen horizontally
	// Just keep 1 unit (border) visible for retrieval
	minVisibleX := core.Unit(1)
	minVisibleFromLeft := core.Unit(1)

	// Left constraint: window can go so far left that only right border is visible
	minX := clientArea.X - newBounds.Width + minVisibleFromLeft
	if newBounds.X < minX {
		newBounds.X = minX
	}
	// Right constraint: window can go so far right that only left border is visible
	maxX := clientArea.X + clientArea.Width - minVisibleX
	if newBounds.X > maxX {
		newBounds.X = maxX
	}

	// Limit height to client area height (windows can be wider but not taller)
	if newBounds.Height > clientArea.Height {
		newBounds.Height = clientArea.Height
	}

	return newBounds
}

// nextTitleFocus returns the next title bar element after the given one.
func (w *Window) nextTitleFocus(current TitleFocus) TitleFocus {
	w.mu.RLock()
	flags := w.flags
	w.mu.RUnlock()

	// Order: Close -> Minimize -> Maximize -> Title -> Blur (if enabled) -> (exit to content)
	switch current {
	case TitleFocusClose:
		if flags&WindowFlagNoMinimize == 0 {
			return TitleFocusMinimize
		}
		fallthrough
	case TitleFocusMinimize:
		if flags&WindowFlagNoMaximize == 0 {
			return TitleFocusMaximize
		}
		fallthrough
	case TitleFocusMaximize:
		return TitleFocusTitle
	case TitleFocusTitle:
		// If keyboard blur is enabled, go to blur item next
		if w.hasKeyboardBlurEnabled() {
			return TitleFocusBlur
		}
		return TitleFocusNone // Exit to content
	case TitleFocusBlur:
		return TitleFocusNone // Exit to content
	}
	return TitleFocusNone
}

// prevTitleFocus returns the previous title bar element before the given one.
func (w *Window) prevTitleFocus(current TitleFocus) TitleFocus {
	w.mu.RLock()
	flags := w.flags
	w.mu.RUnlock()

	// Reverse order: Blur -> Title -> Maximize -> Minimize -> Close
	switch current {
	case TitleFocusBlur:
		return TitleFocusTitle
	case TitleFocusTitle:
		if flags&WindowFlagNoMaximize == 0 {
			return TitleFocusMaximize
		}
		fallthrough
	case TitleFocusMaximize:
		if flags&WindowFlagNoMinimize == 0 {
			return TitleFocusMinimize
		}
		fallthrough
	case TitleFocusMinimize:
		if flags&WindowFlagNoClose == 0 {
			return TitleFocusClose
		}
		fallthrough
	case TitleFocusClose:
		return TitleFocusClose // Stay at close, can't go back further
	}
	return TitleFocusClose
}

// HandleKeyPress handles keyboard input.
func (w *Window) HandleKeyPress(event core.KeyPressEvent) bool {
	w.mu.RLock()
	fm := w.focusManager
	titleFocus := w.titleFocus
	w.mu.RUnlock()

	// If title bar has focus, handle title bar keys
	if titleFocus != TitleFocusNone {
		if w.handleTitleBarKey(event) {
			return true
		}
	}

	// Check if this is a Tab or Shift+Tab event
	isShiftTab := event.Key == "S-Tab" || event.Key == "Shift-Tab" ||
		(event.Key == "Tab" && event.Modifiers&core.ShiftModifier != 0)
	isTab := event.Key == "Tab" && event.Modifiers&core.ShiftModifier == 0

	// For Tab/Shift+Tab, first give the focused widget a chance to handle it.
	// This is critical for containers like MDIPane that manage their own Tab navigation.
	// If the focused widget handles it, we're done.
	if (isTab || isShiftTab) && fm != nil {
		focused := fm.FocusedWidget()
		if focused != nil && focused.HandleKeyPress(event) {
			return true
		}

		// Focused widget didn't handle it.
		// For Shift+Tab at first widget, enter title bar (blur item if enabled, otherwise title).
		if isShiftTab {
			chain := fm.FocusChain()
			for _, widget := range chain {
				if widget.IsVisible() && widget.IsEnabled() {
					if widget == focused {
						// At first widget, enter blur item if enabled, otherwise title bar
						if w.hasKeyboardBlurEnabled() {
							w.SetTitleFocus(TitleFocusBlur)
						} else {
							w.SetTitleFocus(TitleFocusTitle)
						}
						fm.ClearFocus()
						return true
					}
					break // Not at first widget
				}
			}
			// Not at first widget, move to previous
			return fm.FocusPrevious()
		}

		// Regular Tab - check if at last widget
		if isTab {
			chain := fm.FocusChain()
			// Find the last visible/enabled widget
			var lastWidget core.Widget
			for _, widget := range chain {
				if widget.IsVisible() && widget.IsEnabled() {
					lastWidget = widget
				}
			}
			if focused == lastWidget && w.hasKeyboardBlurEnabled() {
				// At last widget with blur enabled, go to blur item
				w.SetTitleFocus(TitleFocusBlur)
				fm.ClearFocus()
				return true
			}
			// Not at last widget, or blur not enabled - move to next
			return fm.FocusNext()
		}
	}

	// For non-Tab keys, use focus manager
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
	w.mu.RUnlock()

	metrics := core.DefaultCellMetrics()

	// Check for title bar clicks
	if flags&WindowFlagNoTitle == 0 && event.Y < metrics.CellHeight {
		// Check if clicking on a button
		button := w.buttonAtPosition(event.X, event.Y)
		if button != TitleButtonNone {
			// Start tracking button press - don't trigger yet
			w.mu.Lock()
			w.pressedButton = button
			w.buttonHovered = true
			w.mu.Unlock()
			w.Update()
			return true
		}

		// Title bar click outside buttons - return false to let WindowManager handle drag
		return false
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
	pressedButton := w.pressedButton
	w.mu.RUnlock()

	// If tracking a button press, update hover state
	if pressedButton != TitleButtonNone {
		currentButton := w.buttonAtPosition(event.X, event.Y)
		newHovered := currentButton == pressedButton

		w.mu.Lock()
		if w.buttonHovered != newHovered {
			w.buttonHovered = newHovered
			w.mu.Unlock()
			w.Update()
		} else {
			w.mu.Unlock()
		}
		return true // Capture mouse while button is pressed
	}

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
	pressedButton := w.pressedButton
	buttonHovered := w.buttonHovered
	minHandler := w.onMinimizeRequest
	maxHandler := w.onMaximizeRequest
	w.mu.RUnlock()

	// If tracking a button press, handle release
	if pressedButton != TitleButtonNone {
		// Clear pressed state
		w.mu.Lock()
		w.pressedButton = TitleButtonNone
		w.buttonHovered = false
		w.mu.Unlock()
		w.Update()

		// Only trigger action if mouse is still on the button
		if buttonHovered {
			switch pressedButton {
			case TitleButtonClose:
				w.Close()
			case TitleButtonMinimize:
				if minHandler != nil {
					minHandler()
				} else {
					w.Minimize()
				}
			case TitleButtonMaximize:
				if maxHandler != nil {
					maxHandler()
				} else if w.IsMaximized() {
					w.Restore()
				} else {
					w.Maximize()
				}
			}
		}
		return true
	}

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

// SetBounds sets the window bounds and triggers layout.
func (w *Window) SetBounds(bounds core.UnitRect) {
	oldSize := w.Bounds().Size()
	w.WidgetBase.SetBounds(bounds)
	newSize := bounds.Size()
	// Manually call our HandleResize since embedded SetBounds won't do it
	if oldSize != newSize {
		w.HandleResize(oldSize, newSize)
	}
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
