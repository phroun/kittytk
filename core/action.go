// Package core provides fundamental types for the TUI toolkit.
package core

import (
	"sync"
)

// Action represents a named operation that can be triggered by
// keyboard shortcuts, menu items, buttons, or programmatically.
// Actions support configurable keybindings.
type Action struct {
	mu sync.RWMutex

	// ID is a unique identifier for the action (e.g., "file.save", "edit.copy").
	ID string

	// Text is the human-readable name shown in menus/buttons.
	Text string

	// Description is a longer description for tooltips/accessibility.
	Description string

	// Icon is an optional icon identifier (for future GUI support).
	Icon string

	// Shortcut is the primary keyboard shortcut.
	Shortcut Shortcut

	// AlternateShortcuts are additional shortcuts for this action.
	AlternateShortcuts []Shortcut

	// Enabled controls whether the action can be triggered.
	Enabled bool

	// Visible controls whether the action appears in menus.
	Visible bool

	// Checkable indicates this is a toggle action.
	Checkable bool

	// Checked is the current state if Checkable.
	Checked bool

	// OnTriggered is called when the action is activated.
	OnTriggered func()

	// OnToggled is called when a checkable action changes state.
	OnToggled func(checked bool)
}

// NewAction creates a new action with the given ID and text.
func NewAction(id, text string) *Action {
	return &Action{
		ID:      id,
		Text:    text,
		Enabled: true,
		Visible: true,
	}
}

// WithShortcut sets the primary shortcut and returns the action.
func (a *Action) WithShortcut(s Shortcut) *Action {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Shortcut = s
	return a
}

// WithDescription sets the description and returns the action.
func (a *Action) WithDescription(desc string) *Action {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Description = desc
	return a
}

// WithHandler sets the triggered handler and returns the action.
func (a *Action) WithHandler(handler func()) *Action {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.OnTriggered = handler
	return a
}

// Trigger activates the action if enabled.
func (a *Action) Trigger() {
	a.mu.RLock()
	enabled := a.Enabled
	handler := a.OnTriggered
	checkable := a.Checkable
	a.mu.RUnlock()

	if !enabled {
		return
	}

	if checkable {
		a.Toggle()
	} else if handler != nil {
		handler()
	}
}

// Toggle changes the checked state of a checkable action.
func (a *Action) Toggle() {
	a.mu.Lock()
	if !a.Checkable {
		a.mu.Unlock()
		return
	}
	a.Checked = !a.Checked
	checked := a.Checked
	handler := a.OnToggled
	a.mu.Unlock()

	if handler != nil {
		handler(checked)
	}
}

// SetChecked sets the checked state.
func (a *Action) SetChecked(checked bool) {
	a.mu.Lock()
	if a.Checked == checked {
		a.mu.Unlock()
		return
	}
	a.Checked = checked
	handler := a.OnToggled
	a.mu.Unlock()

	if handler != nil {
		handler(checked)
	}
}

// MatchesKey returns true if the key event matches any shortcut for this action.
func (a *Action) MatchesKey(event KeyPressEvent) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.Shortcut.Matches(event) {
		return true
	}
	for _, s := range a.AlternateShortcuts {
		if s.Matches(event) {
			return true
		}
	}
	return false
}

// Shortcut represents a keyboard shortcut.
type Shortcut struct {
	Key       string       // Key name (e.g., "s", "F1", "Enter")
	Modifiers KeyModifiers // Required modifiers
}

// NoShortcut represents the absence of a shortcut.
var NoShortcut = Shortcut{}

// NewShortcut creates a shortcut from a key and modifiers.
func NewShortcut(key string, mods KeyModifiers) Shortcut {
	return Shortcut{Key: key, Modifiers: mods}
}

// Ctrl creates a Ctrl+key shortcut.
func Ctrl(key string) Shortcut {
	return Shortcut{Key: key, Modifiers: ControlModifier}
}

// Alt creates an Alt+key shortcut.
func Alt(key string) Shortcut {
	return Shortcut{Key: key, Modifiers: AltModifier}
}

// CtrlShift creates a Ctrl+Shift+key shortcut.
func CtrlShift(key string) Shortcut {
	return Shortcut{Key: key, Modifiers: ControlModifier | ShiftModifier}
}

// Matches returns true if the key event matches this shortcut.
func (s Shortcut) Matches(event KeyPressEvent) bool {
	if s.Key == "" {
		return false
	}
	// Normalize key for comparison
	mods, key := ParseKeyModifiers(event.Key)
	return key == s.Key && mods == s.Modifiers
}

// String returns a human-readable representation of the shortcut.
func (s Shortcut) String() string {
	if s.Key == "" {
		return ""
	}
	result := ""
	if s.Modifiers&ControlModifier != 0 {
		result += "Ctrl+"
	}
	if s.Modifiers&AltModifier != 0 {
		result += "Alt+"
	}
	if s.Modifiers&ShiftModifier != 0 {
		result += "Shift+"
	}
	if s.Modifiers&MetaModifier != 0 {
		result += "Super+"
	}
	result += s.Key
	return result
}

// ActionGroup manages a collection of related actions.
type ActionGroup struct {
	mu       sync.RWMutex
	actions  map[string]*Action
	order    []string // Maintains insertion order
}

// NewActionGroup creates a new action group.
func NewActionGroup() *ActionGroup {
	return &ActionGroup{
		actions: make(map[string]*Action),
	}
}

// Add adds an action to the group.
func (g *ActionGroup) Add(action *Action) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.actions[action.ID]; !exists {
		g.order = append(g.order, action.ID)
	}
	g.actions[action.ID] = action
}

// Get returns an action by ID.
func (g *ActionGroup) Get(id string) *Action {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actions[id]
}

// All returns all actions in insertion order.
func (g *ActionGroup) All() []*Action {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make([]*Action, 0, len(g.order))
	for _, id := range g.order {
		if a, ok := g.actions[id]; ok {
			result = append(result, a)
		}
	}
	return result
}

// HandleKey tries to match a key event against all actions.
// Returns the matched action if found and triggered, nil otherwise.
func (g *ActionGroup) HandleKey(event KeyPressEvent) *Action {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, action := range g.actions {
		if action.Enabled && action.MatchesKey(event) {
			// Trigger outside lock to avoid deadlock
			go action.Trigger()
			return action
		}
	}
	return nil
}

// ShortcutMap provides a way to customize keybindings.
// It maps action IDs to shortcuts.
type ShortcutMap struct {
	mu       sync.RWMutex
	bindings map[string][]Shortcut
}

// NewShortcutMap creates a new shortcut map.
func NewShortcutMap() *ShortcutMap {
	return &ShortcutMap{
		bindings: make(map[string][]Shortcut),
	}
}

// Set sets the shortcuts for an action.
func (m *ShortcutMap) Set(actionID string, shortcuts ...Shortcut) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bindings[actionID] = shortcuts
}

// Get returns the shortcuts for an action.
func (m *ShortcutMap) Get(actionID string) []Shortcut {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bindings[actionID]
}

// Apply applies this shortcut map to an action group.
func (m *ShortcutMap) Apply(group *ActionGroup) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for actionID, shortcuts := range m.bindings {
		action := group.Get(actionID)
		if action == nil {
			continue
		}
		action.mu.Lock()
		if len(shortcuts) > 0 {
			action.Shortcut = shortcuts[0]
			action.AlternateShortcuts = shortcuts[1:]
		} else {
			action.Shortcut = NoShortcut
			action.AlternateShortcuts = nil
		}
		action.mu.Unlock()
	}
}

// StandardActions provides common action IDs.
var StandardActions = struct {
	// File actions
	New    string
	Open   string
	Save   string
	SaveAs string
	Close  string
	Quit   string

	// Edit actions
	Undo      string
	Redo      string
	Cut       string
	Copy      string
	Paste     string
	Delete    string
	SelectAll string
	Find      string
	Replace   string

	// View actions
	ZoomIn    string
	ZoomOut   string
	ZoomReset string

	// Window actions
	Minimize  string
	Maximize  string
	Restore   string
	CloseWindow string

	// Navigation
	FocusNext string
	FocusPrev string
	Escape    string
	Confirm   string
}{
	New:    "file.new",
	Open:   "file.open",
	Save:   "file.save",
	SaveAs: "file.save_as",
	Close:  "file.close",
	Quit:   "app.quit",

	Undo:      "edit.undo",
	Redo:      "edit.redo",
	Cut:       "edit.cut",
	Copy:      "edit.copy",
	Paste:     "edit.paste",
	Delete:    "edit.delete",
	SelectAll: "edit.select_all",
	Find:      "edit.find",
	Replace:   "edit.replace",

	ZoomIn:    "view.zoom_in",
	ZoomOut:   "view.zoom_out",
	ZoomReset: "view.zoom_reset",

	Minimize:    "window.minimize",
	Maximize:    "window.maximize",
	Restore:     "window.restore",
	CloseWindow: "window.close",

	FocusNext: "focus.next",
	FocusPrev: "focus.prev",
	Escape:    "dialog.escape",
	Confirm:   "dialog.confirm",
}

// DefaultShortcuts returns the default keyboard shortcuts.
func DefaultShortcuts() *ShortcutMap {
	m := NewShortcutMap()

	// File
	m.Set(StandardActions.New, Ctrl("n"))
	m.Set(StandardActions.Open, Ctrl("o"))
	m.Set(StandardActions.Save, Ctrl("s"))
	m.Set(StandardActions.SaveAs, CtrlShift("s"))
	m.Set(StandardActions.Close, Ctrl("w"))
	m.Set(StandardActions.Quit, Ctrl("q"))

	// Edit
	m.Set(StandardActions.Undo, Ctrl("z"))
	m.Set(StandardActions.Redo, CtrlShift("z"), Ctrl("y"))
	m.Set(StandardActions.Cut, Ctrl("x"))
	m.Set(StandardActions.Copy, Ctrl("c"))
	m.Set(StandardActions.Paste, Ctrl("v"))
	m.Set(StandardActions.Delete, NewShortcut("Delete", 0))
	m.Set(StandardActions.SelectAll, Ctrl("a"))
	m.Set(StandardActions.Find, Ctrl("f"))
	m.Set(StandardActions.Replace, Ctrl("h"))

	// Navigation
	m.Set(StandardActions.FocusNext, NewShortcut("Tab", 0))
	m.Set(StandardActions.FocusPrev, NewShortcut("Tab", ShiftModifier))
	m.Set(StandardActions.Escape, NewShortcut("Escape", 0))
	m.Set(StandardActions.Confirm, NewShortcut("Enter", 0))

	return m
}
