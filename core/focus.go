// Package core provides fundamental types for the TUI toolkit.
package core

import (
	"sync"
)

// FocusManager handles focus navigation within a scope (window/dialog).
// Each window typically has its own focus manager.
type FocusManager struct {
	mu sync.RWMutex

	// The root widget/container for this focus scope
	root Widget

	// Currently focused widget
	focusedWidget Widget

	// Focus chain (ordered list of focusable widgets)
	focusChain []Widget

	// Focus policy determines how focus behaves
	wrapAround bool // Whether tab wraps from last to first

	// Callbacks
	onFocusChanged func(old, new Widget)

	// Accessibility manager for announcements
	accessibilityManager *AccessibilityManager
}

// NewFocusManager creates a new focus manager for a widget scope.
func NewFocusManager(root Widget) *FocusManager {
	return &FocusManager{
		root:       root,
		wrapAround: true,
	}
}

// SetRoot sets the root widget for this focus scope.
func (fm *FocusManager) SetRoot(root Widget) {
	fm.mu.Lock()
	fm.root = root
	fm.focusChain = nil // Clear cached chain
	fm.mu.Unlock()
}

// Root returns the root widget.
func (fm *FocusManager) Root() Widget {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.root
}

// SetAccessibilityManager sets the accessibility manager for focus announcements.
func (fm *FocusManager) SetAccessibilityManager(am *AccessibilityManager) {
	fm.mu.Lock()
	fm.accessibilityManager = am
	fm.mu.Unlock()
}

// FocusedWidget returns the currently focused widget.
func (fm *FocusManager) FocusedWidget() Widget {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.focusedWidget
}

// SetFocusedWidget sets the focused widget.
func (fm *FocusManager) SetFocusedWidget(widget Widget) bool {
	if widget != nil && !fm.canFocus(widget) {
		return false
	}

	fm.mu.Lock()
	if fm.focusedWidget == widget {
		fm.mu.Unlock()
		return true
	}

	oldFocus := fm.focusedWidget
	fm.focusedWidget = widget
	handler := fm.onFocusChanged
	am := fm.accessibilityManager
	fm.mu.Unlock()

	// Notify old widget of focus loss
	if oldFocus != nil {
		oldFocus.HandleFocusOut()
	}

	// Notify new widget of focus gain
	if widget != nil {
		widget.HandleFocusIn()
	}

	// Announce focus change for accessibility
	if am != nil && widget != nil {
		am.AnnounceFocus(widget)
	}

	// Call callback
	if handler != nil {
		handler(oldFocus, widget)
	}

	return true
}

// ClearFocus removes focus from the current widget.
func (fm *FocusManager) ClearFocus() {
	fm.SetFocusedWidget(nil)
}

// canFocus checks if a widget can receive focus.
func (fm *FocusManager) canFocus(widget Widget) bool {
	if widget == nil {
		return false
	}

	// Check enabled
	if !widget.IsEnabled() {
		return false
	}

	// Check visible
	if !widget.IsVisible() {
		return false
	}

	// Check focus policy
	policy := widget.FocusPolicy()
	return policy == StrongFocus || policy == TabFocus || policy == ClickFocus
}

// FocusNext moves focus to the next widget in the focus chain.
func (fm *FocusManager) FocusNext() bool {
	fm.mu.RLock()
	root := fm.root
	current := fm.focusedWidget
	wrap := fm.wrapAround
	fm.mu.RUnlock()

	chain := fm.buildFocusChain(root)
	if len(chain) == 0 {
		return false
	}

	// Find current index
	currentIdx := -1
	for i, w := range chain {
		if w == current {
			currentIdx = i
			break
		}
	}

	// Find next focusable widget
	for i := 1; i <= len(chain); i++ {
		nextIdx := currentIdx + i
		if nextIdx >= len(chain) {
			if wrap {
				nextIdx = nextIdx % len(chain)
			} else {
				break
			}
		}

		if fm.canFocus(chain[nextIdx]) {
			return fm.SetFocusedWidget(chain[nextIdx])
		}
	}

	return false
}

// FocusPrevious moves focus to the previous widget in the focus chain.
func (fm *FocusManager) FocusPrevious() bool {
	fm.mu.RLock()
	root := fm.root
	current := fm.focusedWidget
	wrap := fm.wrapAround
	fm.mu.RUnlock()

	chain := fm.buildFocusChain(root)
	if len(chain) == 0 {
		return false
	}

	// Find current index
	currentIdx := len(chain)
	for i, w := range chain {
		if w == current {
			currentIdx = i
			break
		}
	}

	// Find previous focusable widget
	for i := 1; i <= len(chain); i++ {
		prevIdx := currentIdx - i
		if prevIdx < 0 {
			if wrap {
				prevIdx = len(chain) + prevIdx
			} else {
				break
			}
		}

		if fm.canFocus(chain[prevIdx]) {
			return fm.SetFocusedWidget(chain[prevIdx])
		}
	}

	return false
}

// FocusFirst moves focus to the first focusable widget.
func (fm *FocusManager) FocusFirst() bool {
	fm.mu.RLock()
	root := fm.root
	fm.mu.RUnlock()

	chain := fm.buildFocusChain(root)
	for _, w := range chain {
		if fm.canFocus(w) {
			return fm.SetFocusedWidget(w)
		}
	}
	return false
}

// FocusLast moves focus to the last focusable widget.
func (fm *FocusManager) FocusLast() bool {
	fm.mu.RLock()
	root := fm.root
	fm.mu.RUnlock()

	chain := fm.buildFocusChain(root)
	for i := len(chain) - 1; i >= 0; i-- {
		if fm.canFocus(chain[i]) {
			return fm.SetFocusedWidget(chain[i])
		}
	}
	return false
}

// buildFocusChain builds the ordered list of focusable widgets.
func (fm *FocusManager) buildFocusChain(root Widget) []Widget {
	if root == nil {
		return nil
	}

	var chain []Widget
	fm.collectFocusable(root, &chain)
	return chain
}

// collectFocusable recursively collects focusable widgets.
func (fm *FocusManager) collectFocusable(widget Widget, chain *[]Widget) {
	if widget == nil {
		return
	}

	// Check if this widget can be in focus chain
	policy := widget.FocusPolicy()
	if policy == StrongFocus || policy == TabFocus {
		*chain = append(*chain, widget)
	}

	// Recurse into children if container
	if container, ok := widget.(Container); ok {
		for _, child := range container.Children() {
			fm.collectFocusable(child, chain)
		}
	}
}

// SetWrapAround sets whether focus wraps around at chain ends.
func (fm *FocusManager) SetWrapAround(wrap bool) {
	fm.mu.Lock()
	fm.wrapAround = wrap
	fm.mu.Unlock()
}

// WrapAround returns whether focus wraps around.
func (fm *FocusManager) WrapAround() bool {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.wrapAround
}

// SetOnFocusChanged sets the focus changed callback.
func (fm *FocusManager) SetOnFocusChanged(handler func(old, new Widget)) {
	fm.mu.Lock()
	fm.onFocusChanged = handler
	fm.mu.Unlock()
}

// HandleKeyPress handles focus-related keyboard events.
// Returns true if the event was handled.
func (fm *FocusManager) HandleKeyPress(event KeyPressEvent) bool {
	switch event.Key {
	case "Tab":
		if event.Modifiers&ShiftModifier != 0 {
			return fm.FocusPrevious()
		}
		return fm.FocusNext()

	case "S-Tab", "Shift-Tab":
		return fm.FocusPrevious()
	}

	return false
}

// FocusScope represents a focus containment boundary.
// Widgets can have their own focus scope (like dialogs or tool windows).
type FocusScope struct {
	mu sync.RWMutex

	// The widget that owns this scope
	owner Widget

	// Focus manager for this scope
	manager *FocusManager

	// Parent scope (for focus restoration)
	parent *FocusScope

	// Active child scope (if focus is in a child)
	activeChild *FocusScope
}

// NewFocusScope creates a new focus scope for a widget.
func NewFocusScope(owner Widget) *FocusScope {
	return &FocusScope{
		owner:   owner,
		manager: NewFocusManager(owner),
	}
}

// Manager returns the focus manager for this scope.
func (fs *FocusScope) Manager() *FocusManager {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return fs.manager
}

// Owner returns the widget that owns this scope.
func (fs *FocusScope) Owner() Widget {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return fs.owner
}

// SetParent sets the parent focus scope.
func (fs *FocusScope) SetParent(parent *FocusScope) {
	fs.mu.Lock()
	fs.parent = parent
	fs.mu.Unlock()
}

// Parent returns the parent focus scope.
func (fs *FocusScope) Parent() *FocusScope {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return fs.parent
}

// Activate activates this focus scope.
func (fs *FocusScope) Activate() {
	fs.mu.Lock()
	parent := fs.parent
	fs.mu.Unlock()

	if parent != nil {
		parent.mu.Lock()
		parent.activeChild = fs
		parent.mu.Unlock()
	}

	// Restore focus within this scope
	fs.manager.FocusFirst()
}

// Deactivate deactivates this focus scope and restores parent focus.
func (fs *FocusScope) Deactivate() {
	fs.mu.Lock()
	parent := fs.parent
	fs.mu.Unlock()

	fs.manager.ClearFocus()

	if parent != nil {
		parent.mu.Lock()
		if parent.activeChild == fs {
			parent.activeChild = nil
		}
		parent.mu.Unlock()
	}
}

// IsActive returns whether this scope is active.
func (fs *FocusScope) IsActive() bool {
	fs.mu.RLock()
	parent := fs.parent
	fs.mu.RUnlock()

	if parent == nil {
		return true // Root scope is always active
	}

	parent.mu.RLock()
	active := parent.activeChild == fs
	parent.mu.RUnlock()
	return active
}

// GlobalFocusManager coordinates focus across all focus scopes.
type GlobalFocusManager struct {
	mu sync.RWMutex

	// Root focus scope (usually the main window)
	rootScope *FocusScope

	// Active focus scope
	activeScope *FocusScope

	// All registered scopes
	scopes []*FocusScope

	// Accessibility manager
	accessibilityManager *AccessibilityManager

	// Callback when active scope changes
	onActiveScopeChanged func(*FocusScope)
}

// NewGlobalFocusManager creates a new global focus manager.
func NewGlobalFocusManager() *GlobalFocusManager {
	return &GlobalFocusManager{}
}

// SetRootScope sets the root focus scope.
func (gfm *GlobalFocusManager) SetRootScope(scope *FocusScope) {
	gfm.mu.Lock()
	gfm.rootScope = scope
	if gfm.activeScope == nil {
		gfm.activeScope = scope
	}
	gfm.mu.Unlock()
}

// RootScope returns the root focus scope.
func (gfm *GlobalFocusManager) RootScope() *FocusScope {
	gfm.mu.RLock()
	defer gfm.mu.RUnlock()
	return gfm.rootScope
}

// ActiveScope returns the currently active focus scope.
func (gfm *GlobalFocusManager) ActiveScope() *FocusScope {
	gfm.mu.RLock()
	defer gfm.mu.RUnlock()
	return gfm.activeScope
}

// SetActiveScope sets the active focus scope.
func (gfm *GlobalFocusManager) SetActiveScope(scope *FocusScope) {
	gfm.mu.Lock()
	if gfm.activeScope == scope {
		gfm.mu.Unlock()
		return
	}

	oldScope := gfm.activeScope
	gfm.activeScope = scope
	handler := gfm.onActiveScopeChanged
	gfm.mu.Unlock()

	if oldScope != nil {
		oldScope.Deactivate()
	}
	if scope != nil {
		scope.Activate()
	}

	if handler != nil {
		handler(scope)
	}
}

// RegisterScope registers a focus scope.
func (gfm *GlobalFocusManager) RegisterScope(scope *FocusScope) {
	gfm.mu.Lock()
	gfm.scopes = append(gfm.scopes, scope)
	if gfm.accessibilityManager != nil {
		scope.Manager().SetAccessibilityManager(gfm.accessibilityManager)
	}
	gfm.mu.Unlock()
}

// UnregisterScope unregisters a focus scope.
func (gfm *GlobalFocusManager) UnregisterScope(scope *FocusScope) {
	gfm.mu.Lock()
	for i, s := range gfm.scopes {
		if s == scope {
			gfm.scopes = append(gfm.scopes[:i], gfm.scopes[i+1:]...)
			break
		}
	}

	// If this was the active scope, switch to root
	if gfm.activeScope == scope {
		gfm.activeScope = gfm.rootScope
	}
	gfm.mu.Unlock()
}

// SetAccessibilityManager sets the accessibility manager for all scopes.
func (gfm *GlobalFocusManager) SetAccessibilityManager(am *AccessibilityManager) {
	gfm.mu.Lock()
	gfm.accessibilityManager = am
	for _, scope := range gfm.scopes {
		scope.Manager().SetAccessibilityManager(am)
	}
	gfm.mu.Unlock()
}

// FocusedWidget returns the currently focused widget across all scopes.
func (gfm *GlobalFocusManager) FocusedWidget() Widget {
	gfm.mu.RLock()
	activeScope := gfm.activeScope
	gfm.mu.RUnlock()

	if activeScope != nil {
		return activeScope.Manager().FocusedWidget()
	}
	return nil
}

// SetOnActiveScopeChanged sets the callback for scope changes.
func (gfm *GlobalFocusManager) SetOnActiveScopeChanged(handler func(*FocusScope)) {
	gfm.mu.Lock()
	gfm.onActiveScopeChanged = handler
	gfm.mu.Unlock()
}

// HandleKeyPress handles focus-related keyboard events.
func (gfm *GlobalFocusManager) HandleKeyPress(event KeyPressEvent) bool {
	gfm.mu.RLock()
	activeScope := gfm.activeScope
	gfm.mu.RUnlock()

	if activeScope != nil {
		return activeScope.Manager().HandleKeyPress(event)
	}
	return false
}
