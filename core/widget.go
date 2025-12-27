// Package core provides fundamental types for the TUI toolkit.
package core

import (
	"sync"

	"github.com/phroun/tuitk/style"
)

// Widget is the base interface for all UI elements.
// All coordinates and sizes are in abstract units (see units.go).
type Widget interface {
	// Identity and hierarchy

	// Name returns an optional identifier for the widget.
	Name() string
	SetName(name string)

	// Parent returns the parent container, or nil if this is a root widget.
	Parent() Container
	SetParent(parent Container)

	// Geometry (all in abstract units)

	// Bounds returns the widget's position and size within its parent.
	Bounds() UnitRect
	SetBounds(bounds UnitRect)

	// Pos returns just the position.
	Pos() UnitPoint
	SetPos(pos UnitPoint)

	// Size returns just the dimensions.
	Size() UnitSize
	SetSize(size UnitSize)

	// MinimumSize returns the smallest size the widget can be.
	MinimumSize() UnitSize
	SetMinimumSize(size UnitSize)

	// MaximumSize returns the largest size the widget can be.
	MaximumSize() UnitSize
	SetMaximumSize(size UnitSize)

	// SizeHint returns the preferred/natural size.
	SizeHint() UnitSize

	// SizePolicy returns how the widget should resize.
	SizePolicy() SizePolicyPair
	SetSizePolicy(policy SizePolicyPair)

	// Margins returns spacing around the widget.
	Margins() UnitMargins
	SetMargins(margins UnitMargins)

	// State

	// IsVisible returns whether the widget is visible.
	IsVisible() bool
	SetVisible(visible bool)
	Show()
	Hide()

	// IsEnabled returns whether the widget accepts input.
	IsEnabled() bool
	SetEnabled(enabled bool)

	// Focus

	// FocusPolicy returns how the widget can receive focus.
	FocusPolicy() FocusPolicy
	SetFocusPolicy(policy FocusPolicy)

	// HasFocus returns whether this widget currently has focus.
	HasFocus() bool

	// SetFocus attempts to give focus to this widget.
	SetFocus()

	// ClearFocus removes focus from this widget.
	ClearFocus()

	// Styling

	// Style returns the widget's style (may be nil to use parent/theme).
	Style() *style.CellStyle
	SetStyle(s *style.CellStyle)

	// Theme returns the theme to use for this widget.
	Theme() *style.Theme

	// Rendering

	// Paint renders the widget using the provided painter.
	// The painter is already configured with the widget's coordinate system.
	Paint(p *Painter)

	// Update marks the widget as needing repaint.
	Update()

	// NeedsRepaint returns true if the widget needs repainting.
	NeedsRepaint() bool

	// Events

	// HandleKeyPress processes a key press event.
	// Returns true if the event was consumed.
	HandleKeyPress(event KeyPressEvent) bool

	// HandleKeyRelease processes a key release event.
	// Note: Not all terminals support key release events.
	HandleKeyRelease(event KeyReleaseEvent) bool

	// HandleMousePress processes a mouse button press.
	HandleMousePress(event MousePressEvent) bool

	// HandleMouseRelease processes a mouse button release.
	HandleMouseRelease(event MouseReleaseEvent) bool

	// HandleMouseMove processes mouse movement.
	HandleMouseMove(event MouseMoveEvent) bool

	// HandleMouseWheel processes mouse wheel scrolling.
	HandleMouseWheel(event MouseWheelEvent) bool

	// HandleFocusIn is called when the widget gains focus.
	HandleFocusIn()

	// HandleFocusOut is called when the widget loses focus.
	HandleFocusOut()

	// HandleResize is called when the widget is resized.
	HandleResize(oldSize, newSize UnitSize)

	// Application returns the application this widget belongs to.
	Application() *Application
}

// Container is a widget that can contain child widgets.
type Container interface {
	Widget

	// Children returns all child widgets.
	Children() []Widget

	// AddChild adds a child widget.
	AddChild(child Widget)

	// RemoveChild removes a child widget.
	RemoveChild(child Widget)

	// ChildAt returns the child at the given position, or nil.
	ChildAt(pos UnitPoint) Widget

	// Layout arranges children within this container.
	Layout()

	// LayoutManager returns the layout manager, if any.
	LayoutManager() LayoutManager

	// SetLayoutManager sets the layout manager.
	SetLayoutManager(layout LayoutManager)
}

// LayoutManager arranges widgets within a container.
type LayoutManager interface {
	// Layout arranges children within the given bounds.
	Layout(container Container, bounds UnitRect)

	// SizeHint returns the preferred size for the container.
	SizeHint(container Container) UnitSize

	// MinimumSize returns the minimum size for the container.
	MinimumSize(container Container) UnitSize

	// Spacing returns the gap between widgets.
	Spacing() Unit

	// SetSpacing sets the gap between widgets.
	SetSpacing(spacing Unit)

	// Margins returns the margins around all content.
	ContentsMargins() UnitMargins

	// SetContentsMargins sets the margins around all content.
	SetContentsMargins(margins UnitMargins)
}

// FocusableWidget extends Widget with focus chain navigation.
type FocusableWidget interface {
	Widget

	// NextFocusWidget returns the next widget in the focus chain.
	NextFocusWidget() Widget
	SetNextFocusWidget(w Widget)

	// PrevFocusWidget returns the previous widget in the focus chain.
	PrevFocusWidget() Widget
	SetPrevFocusWidget(w Widget)
}

// InlineWidget is an optional interface for text-style/inline widgets.
// Widgets implementing this interface will receive horizontal margins
// when placed in a vertical box layout. Block-style widgets (like
// ListView, TreeView, TabWidget) should NOT implement this interface.
type InlineWidget interface {
	// IsInlineWidget returns true for text-style controls that should
	// receive horizontal margins in vertical layouts.
	IsInlineWidget() bool
}

// PopupRequest contains information about a popup to be shown.
type PopupRequest struct {
	// Unique identifier for the popup
	ID string
	// Bounds in screen coordinates
	Bounds UnitRect
	// Paint function to render the popup
	Paint func(p *Painter)
	// HandleMousePress function to handle clicks (returns true if handled)
	HandleMousePress func(event MousePressEvent) bool
}

// PopupController is an interface for managing popup overlays.
// Widgets that need to show popups (like ComboBox) can use this
// to have their popups rendered on top of all windows.
type PopupController interface {
	// RegisterPopup registers a popup to be rendered on top of windows.
	RegisterPopup(request *PopupRequest)
	// UnregisterPopup removes a popup by ID.
	UnregisterPopup(id string)
	// MapToScreen converts local widget coordinates to screen coordinates.
	MapToScreen(widget Widget, local UnitPoint) UnitPoint
}

// Application is a forward declaration (defined in app package).
// This interface allows widgets to access application-level services.
type Application struct {
	mu           sync.RWMutex
	backend      RenderBackend
	theme        *style.Theme
	focusWidget  Widget
	rootWidget   Widget
	running      bool
	needsRepaint bool
	quitChan     chan struct{}
}

// WidgetBase provides a default implementation of the Widget interface.
// Embed this in concrete widget types.
type WidgetBase struct {
	mu sync.RWMutex

	// self is a reference to the outer widget that embeds this WidgetBase.
	// This is needed because Go embedding doesn't support polymorphism -
	// when WidgetBase methods are called, 'w' refers to WidgetBase, not the
	// outer type. Widgets should call Init(self) after embedding.
	self Widget

	name   string
	parent Container
	app    *Application

	bounds      UnitRect
	minSize     UnitSize
	maxSize     UnitSize
	sizePolicy  SizePolicyPair
	margins     UnitMargins

	visible bool
	enabled bool
	focused bool

	focusPolicy     FocusPolicy
	style           *style.CellStyle
	backgroundColor *style.Color // nil = inherit from parent
	popupController PopupController

	needsRepaint bool
}

// NewWidgetBase creates a new widget base with default values.
func NewWidgetBase() *WidgetBase {
	return &WidgetBase{
		visible:     true,
		enabled:     true,
		focusPolicy: NoFocus,
		sizePolicy:  NewSizePolicy(SizePreferred, SizePreferred),
		maxSize:     UnitSize{Width: 1<<30 - 1, Height: 1<<30 - 1},
	}
}

// Init initializes the WidgetBase with a reference to the outer widget.
// This must be called by widgets after embedding WidgetBase to enable
// proper polymorphic behavior (focus management, key forwarding, etc.).
func (w *WidgetBase) Init(self Widget) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.self = self
}

// Self returns the outer widget reference, or w if not set.
func (w *WidgetBase) Self() Widget {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.self != nil {
		return w.self
	}
	return w
}

// Name returns the widget's name.
func (w *WidgetBase) Name() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.name
}

// SetName sets the widget's name.
func (w *WidgetBase) SetName(name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.name = name
}

// Parent returns the parent container.
func (w *WidgetBase) Parent() Container {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.parent
}

// SetParent sets the parent container.
func (w *WidgetBase) SetParent(parent Container) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.parent = parent
}

// Bounds returns the widget's bounds.
func (w *WidgetBase) Bounds() UnitRect {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.bounds
}

// SetBounds sets the widget's bounds.
func (w *WidgetBase) SetBounds(bounds UnitRect) {
	w.mu.Lock()
	oldSize := w.bounds.Size()
	w.bounds = bounds
	newSize := bounds.Size()
	w.needsRepaint = true
	app := w.app
	w.mu.Unlock()

	if oldSize != newSize {
		w.HandleResize(oldSize, newSize)
	}

	// Notify app to repaint
	if app != nil {
		app.requestRepaint()
	}
}

// Pos returns the widget's position.
func (w *WidgetBase) Pos() UnitPoint {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return UnitPoint{X: w.bounds.X, Y: w.bounds.Y}
}

// SetPos sets the widget's position.
func (w *WidgetBase) SetPos(pos UnitPoint) {
	w.mu.Lock()
	w.bounds.X = pos.X
	w.bounds.Y = pos.Y
	w.needsRepaint = true
	app := w.app
	w.mu.Unlock()

	// Notify app to repaint
	if app != nil {
		app.requestRepaint()
	}
}

// Size returns the widget's size.
func (w *WidgetBase) Size() UnitSize {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return UnitSize{Width: w.bounds.Width, Height: w.bounds.Height}
}

// SetSize sets the widget's size.
func (w *WidgetBase) SetSize(size UnitSize) {
	w.mu.Lock()
	oldSize := w.bounds.Size()
	w.bounds.Width = size.Width
	w.bounds.Height = size.Height
	w.needsRepaint = true
	w.mu.Unlock()

	if oldSize != size {
		w.HandleResize(oldSize, size)
	}
}

// MinimumSize returns the minimum size.
func (w *WidgetBase) MinimumSize() UnitSize {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.minSize
}

// SetMinimumSize sets the minimum size.
func (w *WidgetBase) SetMinimumSize(size UnitSize) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.minSize = size
}

// MaximumSize returns the maximum size.
func (w *WidgetBase) MaximumSize() UnitSize {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.maxSize
}

// SetMaximumSize sets the maximum size.
func (w *WidgetBase) SetMaximumSize(size UnitSize) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.maxSize = size
}

// SizeHint returns the preferred size (override in subclasses).
func (w *WidgetBase) SizeHint() UnitSize {
	w.mu.RLock()
	defer w.mu.RUnlock()
	// Default: use current size
	return w.bounds.Size()
}

// SizePolicy returns the size policy.
func (w *WidgetBase) SizePolicy() SizePolicyPair {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.sizePolicy
}

// SetSizePolicy sets the size policy.
func (w *WidgetBase) SetSizePolicy(policy SizePolicyPair) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.sizePolicy = policy
}

// Margins returns the margins.
func (w *WidgetBase) Margins() UnitMargins {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.margins
}

// SetMargins sets the margins.
func (w *WidgetBase) SetMargins(margins UnitMargins) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.margins = margins
	w.needsRepaint = true
}

// IsVisible returns whether the widget is visible.
func (w *WidgetBase) IsVisible() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.visible
}

// SetVisible sets visibility.
func (w *WidgetBase) SetVisible(visible bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.visible = visible
	w.needsRepaint = true
}

// Show makes the widget visible.
func (w *WidgetBase) Show() {
	w.SetVisible(true)
}

// Hide makes the widget invisible.
func (w *WidgetBase) Hide() {
	w.SetVisible(false)
}

// IsEnabled returns whether the widget is enabled.
func (w *WidgetBase) IsEnabled() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.enabled
}

// SetEnabled sets the enabled state.
func (w *WidgetBase) SetEnabled(enabled bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.enabled = enabled
	w.needsRepaint = true
}

// FocusPolicy returns the focus policy.
func (w *WidgetBase) FocusPolicy() FocusPolicy {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.focusPolicy
}

// SetFocusPolicy sets the focus policy.
func (w *WidgetBase) SetFocusPolicy(policy FocusPolicy) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.focusPolicy = policy
}

// HasFocus returns whether the widget has focus.
func (w *WidgetBase) HasFocus() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.focused
}

// SetFocus attempts to give focus to this widget.
func (w *WidgetBase) SetFocus() {
	w.mu.Lock()
	if w.focusPolicy == NoFocus {
		w.mu.Unlock()
		return
	}
	wasFocused := w.focused
	w.focused = true
	app := w.app
	parent := w.parent
	self := w.self // Get the outer widget reference
	w.mu.Unlock()

	// Use self (the outer widget) for focus management, falling back to w
	focusWidget := Widget(w)
	if self != nil {
		focusWidget = self
	}

	if !wasFocused {
		// Call HandleFocusIn on the actual widget (self) to get polymorphic behavior
		focusWidget.HandleFocusIn()
		w.Update()
	}

	// Find a parent with a FocusManager and notify it
	// This ensures the old focused widget gets ClearFocus() called
	w.notifyFocusManager(parent, focusWidget)

	// Notify application of focus change
	if app != nil {
		app.setFocusWidget(focusWidget)
	}
}

// notifyFocusManager walks up the parent chain to find a FocusManagerOwner
// and calls SetFocusedWidget to properly transfer focus.
func (w *WidgetBase) notifyFocusManager(parent Container, focusWidget Widget) {
	current := parent
	for current != nil {
		if owner, ok := current.(FocusManagerOwner); ok {
			if fm := owner.FocusManager(); fm != nil {
				// Use internal method to avoid calling SetFocus again
				// The FocusManager will clear focus from the old widget
				fm.setFocusedWidgetInternal(focusWidget)
				return
			}
		}
		// Walk up to next parent
		if widget, ok := current.(Widget); ok {
			current = widget.Parent()
		} else {
			break
		}
	}
}

// ClearFocus removes focus from this widget.
func (w *WidgetBase) ClearFocus() {
	w.mu.Lock()
	wasFocused := w.focused
	w.focused = false
	self := w.self
	w.mu.Unlock()

	if wasFocused {
		// Call HandleFocusOut on the actual widget (self) to get polymorphic behavior
		if self != nil {
			self.HandleFocusOut()
		} else {
			w.HandleFocusOut()
		}
		w.Update()
	}
}

// Style returns the custom style.
func (w *WidgetBase) Style() *style.CellStyle {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.style
}

// SetStyle sets a custom style.
func (w *WidgetBase) SetStyle(s *style.CellStyle) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.style = s
	w.needsRepaint = true
}

// BackgroundColor returns the explicitly set background color, or nil if inherited.
func (w *WidgetBase) BackgroundColor() *style.Color {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.backgroundColor
}

// SetBackgroundColor sets an explicit background color.
// Pass nil to inherit from parent.
func (w *WidgetBase) SetBackgroundColor(c *style.Color) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.backgroundColor = c
	w.needsRepaint = true
}

// EffectiveBackgroundColor returns the background color to use for this widget.
// If this widget has an explicit background color set, it returns that.
// Otherwise, it walks up the parent chain to find an inherited background color.
// If no parent has a background color set, it returns style.ColorDefault.
func (w *WidgetBase) EffectiveBackgroundColor() style.Color {
	w.mu.RLock()
	if w.backgroundColor != nil {
		c := *w.backgroundColor
		w.mu.RUnlock()
		return c
	}
	parent := w.parent
	w.mu.RUnlock()

	// Walk up the parent chain
	for parent != nil {
		// Check if parent has BackgroundColor method
		if bgProvider, ok := parent.(interface{ BackgroundColor() *style.Color }); ok {
			if bg := bgProvider.BackgroundColor(); bg != nil {
				return *bg
			}
		}
		// Move to next parent
		if widget, ok := parent.(Widget); ok {
			parent = widget.Parent()
		} else {
			break
		}
	}

	return style.ColorDefault
}

// Theme returns the theme for this widget.
func (w *WidgetBase) Theme() *style.Theme {
	w.mu.RLock()
	app := w.app
	w.mu.RUnlock()

	if app != nil {
		return app.Theme()
	}
	return style.DefaultTheme()
}

// Paint is a no-op (override in subclasses).
func (w *WidgetBase) Paint(p *Painter) {
	// Default: do nothing
}

// Update marks the widget as needing repaint.
func (w *WidgetBase) Update() {
	w.mu.Lock()
	w.needsRepaint = true
	app := w.app
	w.mu.Unlock()

	if app != nil {
		app.requestRepaint()
	}
}

// NeedsRepaint returns whether the widget needs repainting.
func (w *WidgetBase) NeedsRepaint() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.needsRepaint
}

// HandleKeyPress handles key events (override in subclasses).
func (w *WidgetBase) HandleKeyPress(event KeyPressEvent) bool {
	return false
}

// HandleKeyRelease handles key release (override in subclasses).
func (w *WidgetBase) HandleKeyRelease(event KeyReleaseEvent) bool {
	return false
}

// HandleMousePress handles mouse press (override in subclasses).
func (w *WidgetBase) HandleMousePress(event MousePressEvent) bool {
	return false
}

// HandleMouseRelease handles mouse release (override in subclasses).
func (w *WidgetBase) HandleMouseRelease(event MouseReleaseEvent) bool {
	return false
}

// HandleMouseMove handles mouse movement (override in subclasses).
func (w *WidgetBase) HandleMouseMove(event MouseMoveEvent) bool {
	return false
}

// HandleMouseWheel handles mouse wheel (override in subclasses).
func (w *WidgetBase) HandleMouseWheel(event MouseWheelEvent) bool {
	return false
}

// HandleFocusIn is called when focus is gained (override in subclasses).
func (w *WidgetBase) HandleFocusIn() {
}

// HandleFocusOut is called when focus is lost (override in subclasses).
func (w *WidgetBase) HandleFocusOut() {
}

// HandleResize is called when size changes (override in subclasses).
func (w *WidgetBase) HandleResize(oldSize, newSize UnitSize) {
}

// Application returns the application.
func (w *WidgetBase) Application() *Application {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.app
}

// SetApplication sets the application (called internally).
func (w *WidgetBase) SetApplication(app *Application) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.app = app
}

// PopupController returns the popup controller for this widget.
// Returns nil if no popup controller has been set.
func (w *WidgetBase) PopupController() PopupController {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.popupController
}

// SetPopupController sets the popup controller for this widget.
// This is typically called by the window manager when adding windows.
func (w *WidgetBase) SetPopupController(pc PopupController) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.popupController = pc
}

// setFocusWidget updates the focused widget, clearing focus from the previous one.
func (a *Application) setFocusWidget(w interface{ ClearFocus() }) {
	if a == nil {
		return
	}

	a.mu.Lock()
	oldFocus := a.focusWidget
	// Type assert to Widget to store it
	if widget, ok := w.(Widget); ok {
		a.focusWidget = widget
	}
	a.mu.Unlock()

	// Clear focus from the old widget (if different)
	if oldFocus != nil && oldFocus != w {
		oldFocus.ClearFocus()
	}
}

// requestRepaint is a forward reference for Application.
func (a *Application) requestRepaint() {
	// Will be implemented in the app package
}

// Theme returns the application theme.
func (a *Application) Theme() *style.Theme {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.theme != nil {
		return a.theme
	}
	return style.DefaultTheme()
}
