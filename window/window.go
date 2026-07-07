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
	WindowFlagNone       WindowFlags = 0
	WindowFlagFrameless  WindowFlags = 1 << iota // No window frame
	WindowFlagNoTitle                            // No title bar
	WindowFlagNoResize                           // Cannot be resized
	WindowFlagNoMove                             // Cannot be moved
	WindowFlagNoClose                            // No close button
	WindowFlagNoMinimize                         // No minimize button
	WindowFlagNoMaximize                         // No maximize button
	WindowFlagModal                              // Blocks input to other windows
	WindowFlagStaysOnTop                         // Always on top
	WindowFlagToolWindow                         // Smaller title bar, no taskbar entry
)

// windowCornerRadius is the corner radius (in units) of the graphical
// window frame's single rounded-rect surface. Kept below the frame's
// one-cell inset (8 units) so titlebar buttons and content never
// overlap the curve; cell surfaces ignore it entirely.
const windowCornerRadius core.Unit = 6

// FrameCornerRadius reports the graphical frame's corner radius in
// units, for hosts that shape OS windows around torn-off frames.
func FrameCornerRadius() core.Unit { return windowCornerRadius }

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
	title string
	flags WindowFlags
	state WindowState

	// G4 dual mode: the app's request for a native OS window,
	// honored when the platform can create surfaces.
	nativeRequested bool

	// smoothPositioning is stamped by the hosting window manager
	// from the surface capability (core.SmoothPositioner): pixel
	// surfaces drag/resize at unit granularity, cell surfaces snap.
	// Nested hosts (MDI panes) inherit it via FindSmoothPositioning.
	smoothPositioning bool

	// Position before maximization (for restore)
	normalBounds core.UnitRect

	// Content
	content core.Widget
	layout  core.LayoutManager

	// Focus management
	focusManager *core.FocusManager

	// Child windows (MDI support)
	parent   *Window
	children []*Window

	// Window chrome
	borderStyle style.BorderStyle
	titleStyle  style.CellStyle
	frameStyle  style.CellStyle

	// Font (nil = inherit from desktop/MDI pane)
	font *core.Font

	// Sizing
	minWidth  core.Unit
	minHeight core.Unit
	maxWidth  core.Unit
	maxHeight core.Unit

	// Callbacks
	onClose       func() bool // Return false to prevent close
	onResize      func(width, height core.Unit)
	onMove        func(x, y core.Unit)
	onActivate    func(active bool)
	onStateChange func(state WindowState)

	// Request callbacks (for WindowManager integration)
	onMinimizeRequest     func()                   // Called when user clicks minimize button
	onMaximizeRequest     func()                   // Called when user clicks maximize button
	onBoundsRequest       func(core.UnitRect) bool // Takes title-focus keyboard geometry whole (torn-off hosts)
	onCloseComplete       func()                   // Called when window is closed, to remove from manager
	getConstrainingBounds func() core.UnitRect     // Returns the client area for movement constraints
	popupController       core.PopupController     // Popup controller for ComboBox etc.

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
		minWidth:    80, // 10 characters minimum
		minHeight:   48, // 3 lines minimum
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

// SetNativeRequested records the app's preference for a native OS
// window (G4 dual mode). It is a REQUEST, honored when the hosting
// platform can create surfaces (see SurfaceHost); single-surface
// platforms (the terminal) keep the window in-surface under the
// WindowManager. Matches the wire's `native` flag.
func (w *Window) SetNativeRequested(native bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.nativeRequested = native
}

// NativeRequested reports whether a native window was requested.
func (w *Window) NativeRequested() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.nativeRequested
}

// SetSmoothPositioning is stamped by the hosting manager from the
// surface capability.
func (w *Window) SetSmoothPositioning(smooth bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.smoothPositioning = smooth
}

// SmoothWindowPositioning implements core.SmoothPositioningProvider,
// letting widgets inside this window (e.g. MDI panes) inherit the
// surface's positioning granularity.
func (w *Window) SmoothWindowPositioning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.smoothPositioning
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
	title := w.title
	w.mu.Unlock()

	// Announce window activation for accessibility
	if active {
		if am := core.FindAccessibilityManager(w); am != nil {
			am.AnnouncePolite(title + ", window")
		}
	}

	if handler != nil {
		handler(active)
	}
	w.Update()
}

// Close attempts to close the window.
func (w *Window) Close() bool {
	w.mu.RLock()
	handler := w.onClose
	closeComplete := w.onCloseComplete
	title := w.title
	w.mu.RUnlock()

	if handler != nil && !handler() {
		return false
	}

	// Announce window closing for accessibility
	if am := core.FindAccessibilityManager(w); am != nil {
		am.AnnouncePolite(title + ", closed")
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

	// Notify manager to remove this window
	if closeComplete != nil {
		closeComplete()
	}

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

// SetOnCloseComplete sets the callback for when the window is fully closed.
// This is called by WindowManager to remove the window from its list.
func (w *Window) SetOnCloseComplete(handler func()) {
	w.mu.Lock()
	w.onCloseComplete = handler
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

// Font returns the window's font, or nil if inheriting from desktop.
func (w *Window) Font() *core.Font {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.font
}

// SetFont sets the window's font.
// Set to nil to inherit from the desktop/MDI pane.
func (w *Window) SetFont(f *core.Font) {
	w.mu.Lock()
	w.font = f
	w.mu.Unlock()
	w.Layout() // Recalculate layout since font affects widget sizes
	w.Update()
}

// EffectiveFont returns the font to use for this window and its contents.
func (w *Window) EffectiveFont() *core.Font {
	w.mu.RLock()
	if w.font != nil {
		f := w.font
		w.mu.RUnlock()
		return f
	}
	w.mu.RUnlock()

	// Check parent's effective font (walks up the chain through MDI pane, desktop, etc.)
	if parent := w.Parent(); parent != nil {
		if widget, ok := parent.(core.Widget); ok {
			return core.FindEffectiveFont(widget)
		}
	}

	return core.DefaultFont()
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
		return clampClientArea(core.UnitRect{
			X:      0,
			Y:      metrics.CellHeight, // One row for title
			Width:  bounds.Width,
			Height: bounds.Height - metrics.CellHeight,
		})
	}

	// Normal mode with full frame
	if flags&WindowFlagFrameless != 0 {
		return clampClientArea(core.UnitRect{Width: bounds.Width, Height: bounds.Height})
	}

	// Graphical frames: the border is a hairline on the window's own
	// surface, not a cell band - only the titlebar reserves a full
	// row, and content extends to the left, right, and bottom edges
	// (the frame re-strokes over it, and a rounded clip keeps it
	// inside the corners).
	if core.FindGraphicalFrames(w) {
		top := metrics.CellHeight
		if flags&WindowFlagNoTitle != 0 {
			top = 0
		}
		return clampClientArea(core.UnitRect{
			X:      0,
			Y:      top,
			Width:  bounds.Width,
			Height: bounds.Height - top,
		})
	}

	// Cell frames: the border occupies a full cell on every side.
	left := metrics.CellWidth
	top := metrics.CellHeight
	right := metrics.CellWidth
	bottom := metrics.CellHeight

	if flags&WindowFlagNoTitle != 0 {
		top = metrics.CellHeight // Just border, no extra title row
	}

	return clampClientArea(core.UnitRect{
		X:      left,
		Y:      top,
		Width:  bounds.Width - left - right,
		Height: bounds.Height - top - bottom,
	})
}

// clampClientArea guarantees the client area is never empty: a window
// squeezed below its chrome still exposes a 1-unit sliver so content
// paints (clipped) instead of spilling unclipped.
func clampClientArea(r core.UnitRect) core.UnitRect {
	if r.Width < 1 {
		r.Width = 1
	}
	if r.Height < 1 {
		r.Height = 1
	}
	return r
}

// ClientAreaOffset returns the offset from the window's top-left corner
// to the client (content) area. This accounts for title bar and frame.
func (w *Window) ClientAreaOffset() core.UnitPoint {
	cb := w.contentBounds()
	return core.UnitPoint{X: cb.X, Y: cb.Y}
}

// denominations returns the grid-metrics currency of the window's own
// coordinate space (outer: the parent's, in which bounds and chrome
// live) and of its content area (interior: honoring a per-window
// override). Equal unless an override is set on this window.
func (w *Window) denominations() (outer, interior core.CellMetrics) {
	interior = w.EffectiveCellMetrics()
	if w.CellMetricsOverride() == nil {
		return interior, interior
	}
	return core.ParentCellMetrics(w.Self()), interior
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
	// The content area is denominated in the window's interior currency:
	// the same physical area, re-expressed in interior units.
	outer, interior := w.denominations()
	localContentRect := core.UnitRect{
		X:      0,
		Y:      0,
		Width:  core.ExchangeX(contentRect.Width, outer, interior),
		Height: core.ExchangeY(contentRect.Height, outer, interior),
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
	outer, interior := w.denominations()
	localPos := core.UnitPoint{
		X: core.ExchangeX(pos.X-contentRect.X, outer, interior),
		Y: core.ExchangeY(pos.Y-contentRect.Y, outer, interior),
	}

	if content.Bounds().Contains(localPos) {
		return content
	}
	return nil
}

// Layout implements core.Container.
func (w *Window) Layout() {
	w.layoutContent()

	// Force content to re-layout with fresh SizeHints.
	// This is important when parent chain changes (e.g., window added to MDIPane)
	// since EffectiveFont may now return a different font.
	w.mu.RLock()
	content := w.content
	w.mu.RUnlock()

	if content != nil {
		if container, ok := content.(core.Container); ok {
			container.Layout()
		}
	}
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

	// Paint content (in the window's interior denomination)
	outer, interior := w.denominations()
	localBounds := core.UnitRect{Width: bounds.Width, Height: bounds.Height}
	graphicalFrame := state != WindowStateMaximized && flags&WindowFlagFrameless == 0 &&
		core.FindGraphicalFrames(w)
	if content != nil {
		contentBounds := w.contentBounds()
		contentBase := p
		if graphicalFrame {
			// Edge-to-edge content stays inside the frame's rounded
			// outline (bottom corners in particular).
			contentBase = p.WithRoundedClipRegion(localBounds, windowCornerRadius)
		}
		contentPainter := contentBase.WithOffset(contentBounds.X, contentBounds.Y).
			WithClip(core.UnitRect{Width: contentBounds.Width, Height: contentBounds.Height}).
			WithDenomination(outer, interior)
		content.Paint(contentPainter)
	}
	if graphicalFrame {
		// Content reaches the window edges, so the hairline border is
		// re-stroked over it - the frame stays visible on all sides.
		frameStyle := w.GetScheme().GetWindowBorder(focused || isPassive)
		p.StrokeRoundedRect(localBounds, windowCornerRadius, frameBorder, frameStyle)
	}

	// Paint child windows (within the content area, clipped)
	if len(w.ChildWindows()) > 0 {
		contentBounds := w.contentBounds()
		// Create a painter clipped to the content area
		contentPainter := p.WithOffset(contentBounds.X, contentBounds.Y).
			WithClip(core.UnitRect{Width: contentBounds.Width, Height: contentBounds.Height}).
			WithDenomination(outer, interior)

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

	font := w.EffectiveFont()

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
	// These are decorative buttons - use cell-based sizing (3 cells each)
	buttonWidth := metrics.CellWidth * 3
	controlX := core.Unit(0)
	if flags&WindowFlagNoClose == 0 {
		isFocused := titleFocus == TitleFocusClose
		isPressed := pressedButton == TitleButtonClose && buttonHovered
		btnStyle := scheme.GetTitleBarButton(focused, isFocused, isPressed)
		p.DrawCell(controlX, 0, '[', btnStyle)
		p.DrawCell(controlX+metrics.CellWidth, 0, 'x', btnStyle)
		p.DrawCell(controlX+metrics.CellWidth*2, 0, ']', btnStyle)
		controlX += buttonWidth
	}
	if flags&WindowFlagNoMinimize == 0 {
		isFocused := titleFocus == TitleFocusMinimize
		isPressed := pressedButton == TitleButtonMinimize && buttonHovered
		btnStyle := scheme.GetTitleBarButton(focused, isFocused, isPressed)
		p.DrawCell(controlX, 0, '[', btnStyle)
		p.DrawCell(controlX+metrics.CellWidth, 0, '.', btnStyle)
		p.DrawCell(controlX+metrics.CellWidth*2, 0, ']', btnStyle)
		controlX += buttonWidth
	}
	if flags&WindowFlagNoMaximize == 0 {
		isFocused := titleFocus == TitleFocusMaximize
		isPressed := pressedButton == TitleButtonMaximize && buttonHovered
		btnStyle := scheme.GetTitleBarButton(focused, isFocused, isPressed)
		var icon rune
		if state == WindowStateMaximized {
			icon = 'o' // Restore icon
		} else {
			icon = '^' // Maximize icon
		}
		p.DrawCell(controlX, 0, '[', btnStyle)
		p.DrawCell(controlX+metrics.CellWidth, 0, icon, btnStyle)
		p.DrawCell(controlX+metrics.CellWidth*2, 0, ']', btnStyle)
		controlX += buttonWidth
	}

	// Draw title text centered, with angle brackets and cyan bg if title has keyboard focus
	if titleFocus == TitleFocusTitle {
		// Title has focus - draw with decorative angle brackets
		titleDisplayStyle := scheme.GetTitleBarButton(focused, true, false)

		// Calculate total width: "< " (2 cells) + title (font) + " >" (2 cells)
		bracketWidth := metrics.CellWidth * 2 // Each side: bracket + space
		titleTextWidth := font.MeasureText(title)
		totalWidth := bracketWidth + titleTextWidth + bracketWidth

		// Center the total width in the title area
		startX := (titleRect.Width - totalWidth) / 2
		x := startX

		// Draw left bracket and space (decorative)
		p.DrawCell(x, 0, '<', titleDisplayStyle)
		p.DrawCell(x+metrics.CellWidth, 0, ' ', titleDisplayStyle)
		x += bracketWidth

		// Draw title text (font-based)
		p.DrawText(x, 0, title, titleDisplayStyle, font)
		x += titleTextWidth

		// Draw space and right bracket (decorative)
		p.DrawCell(x, 0, ' ', titleDisplayStyle)
		p.DrawCell(x+metrics.CellWidth, 0, '>', titleDisplayStyle)
	} else {
		rightLimit := bounds.Width
		if titleFocus == TitleFocusBlur {
			rightLimit = bounds.Width - buttonWidth
		}
		w.paintTitleText(p, title, titleStyle, font, metrics, controlX, rightLimit, bounds.Width)
	}

	// Draw blur button on far right when blur item is focused
	// This is a decorative button - use cell-based sizing (3 cells)
	if titleFocus == TitleFocusBlur {
		blurBtnStyle := scheme.GetTitleBarButton(focused, true, false) // Focused button style
		blurX := bounds.Width - buttonWidth                            // Position at far right
		p.DrawCell(blurX, 0, '[', blurBtnStyle)
		p.DrawCell(blurX+metrics.CellWidth, 0, '~', blurBtnStyle)
		p.DrawCell(blurX+metrics.CellWidth*2, 0, ']', blurBtnStyle)
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

	// Graphical path (D1): the window's entire surface is ONE rounded
	// rectangle - filled with the window background, stroked with the
	// border color (2 device px for double, 1 for single). Title,
	// buttons, and content then draw over it as usual. Cell surfaces
	// return false and take the box-drawing path below.
	roundedStyle := frameStyle.WithBg(w.Theme().WindowBackground.Bg)
	rounded := p.DrawRoundedRect(localBounds, windowCornerRadius, border, roundedStyle)
	if rounded {
		// Frame painted; fall through to title/buttons/content.
	} else if titleFocus == TitleFocusBlur {
		// When blur item is focused, draw dashed frame with inactive title
		// color but keep corners, horizontally adjacent chars, and buttons
		// in active color
		scheme := w.GetScheme()
		blurFrameStyle := scheme.GetWindowTitle(false)   // Inactive title color for dashed lines
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
	font := w.EffectiveFont()

	// Draw title if enabled
	if flags&WindowFlagNoTitle == 0 {
		// Draw window controls on the LEFT: [x][.][^] or [x][.][o]
		// These are decorative buttons - use cell-based sizing (3 cells each)
		buttonWidth := metrics.CellWidth * 3
		controlX := metrics.CellWidth // Start after left border
		if flags&WindowFlagNoClose == 0 {
			isFocused := titleFocus == TitleFocusClose
			isPressed := pressedButton == TitleButtonClose && buttonHovered
			btnStyle := scheme.GetTitleBarButton(buttonFocused, isFocused, isPressed)
			p.DrawCell(controlX, 0, '[', btnStyle)
			p.DrawCell(controlX+metrics.CellWidth, 0, 'x', btnStyle)
			p.DrawCell(controlX+metrics.CellWidth*2, 0, ']', btnStyle)
			controlX += buttonWidth
		}
		if flags&WindowFlagNoMinimize == 0 {
			isFocused := titleFocus == TitleFocusMinimize
			isPressed := pressedButton == TitleButtonMinimize && buttonHovered
			btnStyle := scheme.GetTitleBarButton(buttonFocused, isFocused, isPressed)
			p.DrawCell(controlX, 0, '[', btnStyle)
			p.DrawCell(controlX+metrics.CellWidth, 0, '.', btnStyle)
			p.DrawCell(controlX+metrics.CellWidth*2, 0, ']', btnStyle)
			controlX += buttonWidth
		}
		if flags&WindowFlagNoMaximize == 0 {
			isFocused := titleFocus == TitleFocusMaximize
			isPressed := pressedButton == TitleButtonMaximize && buttonHovered
			btnStyle := scheme.GetTitleBarButton(buttonFocused, isFocused, isPressed)
			var icon rune
			if state == WindowStateMaximized {
				icon = 'o' // Restore icon
			} else {
				icon = '^' // Maximize icon
			}
			p.DrawCell(controlX, 0, '[', btnStyle)
			p.DrawCell(controlX+metrics.CellWidth, 0, icon, btnStyle)
			p.DrawCell(controlX+metrics.CellWidth*2, 0, ']', btnStyle)
			controlX += buttonWidth
		}

		// Calculate title area (centered on top border)
		titleRect := core.UnitRect{
			X:      0,
			Y:      0,
			Width:  bounds.Width,
			Height: metrics.CellHeight,
		}

		// Draw title text centered, with angle brackets and cyan bg if title has keyboard focus
		if titleFocus == TitleFocusTitle {
			// Title has focus - draw with decorative angle brackets
			titleDisplayStyle := scheme.GetTitleBarButton(focused, true, false)

			// Calculate total width: "< " (2 cells) + title (font) + " >" (2 cells)
			bracketWidth := metrics.CellWidth * 2 // Each side: bracket + space
			titleTextWidth := font.MeasureText(title)
			totalWidth := bracketWidth + titleTextWidth + bracketWidth

			// Center the total width in the title area
			startX := (titleRect.Width - totalWidth) / 2
			x := startX

			// Draw left bracket and space (decorative)
			p.DrawCell(x, 0, '<', titleDisplayStyle)
			p.DrawCell(x+metrics.CellWidth, 0, ' ', titleDisplayStyle)
			x += bracketWidth

			// Draw title text (font-based)
			p.DrawText(x, 0, title, titleDisplayStyle, font)
			x += titleTextWidth

			// Draw space and right bracket (decorative)
			p.DrawCell(x, 0, ' ', titleDisplayStyle)
			p.DrawCell(x+metrics.CellWidth, 0, '>', titleDisplayStyle)
		} else {
			// Normal title or blur focused
			titleDisplayStyle := titleStyle
			if titleFocus == TitleFocusBlur {
				// Blur item focused - use inactive title style for the title text
				titleDisplayStyle = scheme.GetWindowTitle(false)
			}
			rightLimit := bounds.Width - metrics.CellWidth
			if titleFocus == TitleFocusBlur {
				rightLimit = localBounds.Width - metrics.CellWidth - buttonWidth
			}
			w.paintTitleText(p, title, titleDisplayStyle, font, metrics, controlX, rightLimit, bounds.Width)
		}

		// Draw blur button on far right when blur item is focused
		// This is a decorative button - use cell-based sizing (3 cells)
		if titleFocus == TitleFocusBlur {
			blurBtnStyle := scheme.GetTitleBarButton(true, true, false)  // Focused button style
			blurX := localBounds.Width - metrics.CellWidth - buttonWidth // Position before right border
			p.DrawCell(blurX, 0, '[', blurBtnStyle)
			p.DrawCell(blurX+metrics.CellWidth, 0, '~', blurBtnStyle)
			p.DrawCell(blurX+metrics.CellWidth*2, 0, ']', blurBtnStyle)
		}
	}

	// Fill content area with background. Skipped when the rounded
	// frame painted: the whole window surface (corners included) is
	// already filled, and a square fill here would put background
	// pixels back outside the bottom corner arcs.
	if !rounded {
		contentBounds := w.contentBounds()
		theme := w.Theme()
		p.FillRect(contentBounds, ' ', theme.WindowBackground)
	}
}

// ellipsizeToWidth trims s so that with a trailing ellipsis it fits
// within avail; empty when not even the ellipsis fits. The ellipsis
// is three periods, not the "\u2026" glyph, matching the tab strip -
// on cell surfaces it is three cells wide, and MeasureText adjusts
// the need-for-ellipsis math on both surfaces.
func ellipsizeToWidth(s string, avail core.Unit, font *core.Font) string {
	const ell = "..."
	if font.MeasureText(s) <= avail {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		if font.MeasureText(string(runes)+ell) <= avail {
			return string(runes) + ell
		}
	}
	return ""
}

// paintTitleText draws the (unfocused) titlebar title. Centered when
// a centered title fits between the left buttons and the right limit
// (the blur button when shown, else the right edge); otherwise its
// left edge sits just past the buttons and the text ellipsizes so
// the "..." butts against the right limit - the right side keeps no
// mirrored reserve. A span of zero or less clips the title entirely.
func (w *Window) paintTitleText(p *core.Painter, title string, ts style.CellStyle, font *core.Font, metrics core.CellMetrics, leftUsed, rightLimit, barWidth core.Unit) {
	leftEdge := leftUsed + metrics.CellWidth
	avail := rightLimit - leftEdge
	if avail <= 0 || title == "" {
		return
	}
	display := title
	titleW := font.MeasureText(display)
	if titleW > avail {
		display = ellipsizeToWidth(title, avail, font)
		if display == "" {
			return
		}
		titleW = font.MeasureText(display)
	}
	x := (barWidth - titleW) / 2
	if x < leftEdge {
		x = leftEdge
	}
	if x+titleW > rightLimit {
		x = rightLimit - titleW
	}
	p.DrawText(x, 0, display, ts, font)
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
	oldFocus := w.titleFocus
	w.titleFocus = focus
	if focus == TitleFocusNone {
		w.resizeEdges = ResizeEdgeNone // Clear resize state when leaving title bar
	}
	title := w.title
	w.mu.Unlock()

	// Announce titlebar element change for accessibility
	if focus != oldFocus && focus != TitleFocusNone {
		if am := core.FindAccessibilityManager(w); am != nil {
			var elementName string
			switch focus {
			case TitleFocusClose:
				elementName = "close button"
			case TitleFocusMinimize:
				elementName = "minimize button"
			case TitleFocusMaximize:
				if w.IsMaximized() {
					elementName = "restore button"
				} else {
					elementName = "maximize button"
				}
			case TitleFocusTitle:
				elementName = title + ", title bar"
			case TitleFocusBlur:
				elementName = "blur button"
			}
			if elementName != "" {
				am.AnnouncePolite(elementName)
			}
		}
	}

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
					w.requestKeyboardBounds(newBounds, false)
				} else if resizeEdges&ResizeEdgeRight != 0 {
					// Continue right resize: shrink right edge
					newBounds := bounds
					newBounds.Width -= horizStep
					if newBounds.Width >= w.minWidth {
						w.requestKeyboardBounds(newBounds, false)
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
					w.requestKeyboardBounds(newBounds, false)
				}
			} else {
				// Move window left
				newBounds := bounds
				newBounds.X -= horizStep
				w.requestKeyboardBounds(newBounds, true)
			}
			return true

		case "Right":
			if hasShift {
				// Start/continue resizing right edge
				if resizeEdges&ResizeEdgeRight != 0 {
					// Continue right resize: expand right
					newBounds := bounds
					newBounds.Width += horizStep
					w.requestKeyboardBounds(newBounds, false)
				} else if resizeEdges&ResizeEdgeLeft != 0 {
					// Continue left resize: shrink left edge
					newBounds := bounds
					newBounds.X += horizStep
					newBounds.Width -= horizStep
					if newBounds.Width >= w.minWidth {
						w.requestKeyboardBounds(newBounds, false)
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
					w.requestKeyboardBounds(newBounds, false)
				}
			} else {
				// Move window right
				newBounds := bounds
				newBounds.X += horizStep
				w.requestKeyboardBounds(newBounds, true)
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
					w.requestKeyboardBounds(newBounds, false)
				} else if resizeEdges&ResizeEdgeBottom != 0 {
					// Continue bottom resize: shrink bottom edge
					newBounds := bounds
					newBounds.Height -= vertStep
					if newBounds.Height >= w.minHeight {
						w.requestKeyboardBounds(newBounds, false)
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
					w.requestKeyboardBounds(newBounds, false)
				}
			} else {
				// Move window up
				newBounds := bounds
				newBounds.Y -= vertStep
				w.requestKeyboardBounds(newBounds, true)
			}
			return true

		case "Down":
			if hasShift {
				// Start/continue resizing bottom edge
				if resizeEdges&ResizeEdgeBottom != 0 {
					// Continue bottom resize: expand bottom
					newBounds := bounds
					newBounds.Height += vertStep
					w.requestKeyboardBounds(newBounds, false)
				} else if resizeEdges&ResizeEdgeTop != 0 {
					// Continue top resize: shrink top edge
					newBounds := bounds
					newBounds.Y += vertStep
					newBounds.Height -= vertStep
					if newBounds.Height >= w.minHeight {
						w.requestKeyboardBounds(newBounds, false)
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
					w.requestKeyboardBounds(newBounds, false)
				}
			} else {
				// Move window down
				newBounds := bounds
				newBounds.Y += vertStep
				w.requestKeyboardBounds(newBounds, true)
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
				w.requestKeyboardBounds(startBounds, false)
			} else {
				w.mu.Unlock()
			}
			return true
		}
	}

	return false
}

// SetOnBoundsRequest installs a delegate for title-focus keyboard
// geometry changes (arrow moves, Shift-arrow resizes, Escape
// reverts). A torn-off window's host maps the deltas onto its OS
// window; nil restores normal in-surface SetBounds handling.
func (w *Window) SetOnBoundsRequest(handler func(core.UnitRect) bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onBoundsRequest = handler
}

// requestKeyboardBounds applies a title-focus keyboard geometry
// change: the bounds delegate takes it whole when installed,
// otherwise it applies in-surface - constrained to the client area
// when the change is a pure move.
func (w *Window) requestKeyboardBounds(b core.UnitRect, isMove bool) {
	w.mu.RLock()
	delegate := w.onBoundsRequest
	w.mu.RUnlock()
	if delegate != nil && delegate(b) {
		return
	}
	if isMove {
		b = w.constrainBoundsForMovement(b)
	}
	w.SetBounds(b)
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

	// A click below the title bar moves keyboard focus into the
	// content: drop any title-bar keyboard focus (set by Tab/Shift+Tab)
	// so it stops intercepting keys and Tab resumes from the clicked
	// control rather than the title-bar element.
	if w.TitleFocus() != TitleFocusNone {
		w.SetTitleFocus(TitleFocusNone)
	}

	// Pass to content (converted into the interior denomination)
	if content != nil {
		contentBounds := w.contentBounds()
		outer, interior := w.denominations()
		localEvent := event
		localEvent.X = core.ExchangeX(event.X-contentBounds.X, outer, interior)
		localEvent.Y = core.ExchangeY(event.Y-contentBounds.Y, outer, interior)
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
			outer, interior := w.denominations()
			localEvent := event
			localEvent.X = core.ExchangeX(event.X-contentBounds.X, outer, interior)
			localEvent.Y = core.ExchangeY(event.Y-contentBounds.Y, outer, interior)
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
			outer, interior := w.denominations()
			localEvent := event
			localEvent.X = core.ExchangeX(event.X-contentBounds.X, outer, interior)
			localEvent.Y = core.ExchangeY(event.Y-contentBounds.Y, outer, interior)
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
		// Content hints are denominated in the interior currency;
		// convert to the window's own (outer) currency.
		outer, interior := w.denominations()
		hint := core.ExchangeSize(content.SizeHint(), interior, outer)
		width = hint.Width
		height = hint.Height
	}

	// Add frame
	if flags&WindowFlagFrameless == 0 {
		width += metrics.CellWidth * 2   // Left and right borders
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

// HandleMouseWheel forwards a wheel event to the content (in the
// window's interior denomination).
func (w *Window) HandleMouseWheel(event core.MouseWheelEvent) bool {
	w.mu.RLock()
	content := w.content
	w.mu.RUnlock()
	if content == nil {
		return false
	}
	handler, ok := content.(interface {
		HandleMouseWheel(core.MouseWheelEvent) bool
	})
	if !ok {
		return false
	}
	contentBounds := w.contentBounds()
	outer, interior := w.denominations()
	local := event
	local.X = core.ExchangeX(event.X-contentBounds.X, outer, interior)
	local.Y = core.ExchangeY(event.Y-contentBounds.Y, outer, interior)
	return handler.HandleMouseWheel(local)
}
