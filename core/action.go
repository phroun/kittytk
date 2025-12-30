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

// Shortcut represents a keyboard shortcut using the key handler format.
// Examples: "^Q" (Ctrl+Q), "M-x" (Alt+x), "S-Tab" (Shift+Tab), "F1" (plain key)
type Shortcut string

// NoShortcut represents the absence of a shortcut.
const NoShortcut Shortcut = ""

// NewShortcut creates a shortcut from a key handler format string.
// This is the primary way to create shortcuts.
func NewShortcut(key string) Shortcut {
	return Shortcut(key)
}

// Matches returns true if the key event matches this shortcut.
func (s Shortcut) Matches(event KeyPressEvent) bool {
	if s == "" {
		return false
	}
	// Direct comparison - both use key handler format
	return event.Key == string(s)
}

// String returns the shortcut in key handler format.
func (s Shortcut) String() string {
	return string(s)
}

// DisplayString returns a human-readable representation of the shortcut
// for display in menus and tooltips.
// Uses compact notation: ^ for Ctrl, M- for Alt, S- for Shift.
func (s Shortcut) DisplayString() string {
	if s == "" {
		return ""
	}
	// Return the key handler format directly - it's already compact and readable
	return string(s)
}

// AccessibilityString returns a fully spelled-out representation of the shortcut
// for screen reader announcements.
// Translates: M- → Meta, A- → Alt, C- → Control, ^ → Control, S- → Shift, s- → Super, H- → Hyper
// Uppercase final letter implies Shift for hyphenated modifiers (e.g., M-O → Meta+Shift+O)
// but NOT for ^ notation (^X is just Control+X, case is irrelevant with ^)
func (s Shortcut) AccessibilityString() string {
	if s == "" {
		return ""
	}

	str := string(s)
	var modifiers []string
	hasExplicitShift := false
	usedCaretNotation := false

	// Parse modifier prefixes
	for len(str) > 0 {
		if len(str) >= 2 {
			prefix := str[:2]
			switch prefix {
			case "M-":
				modifiers = append(modifiers, "Meta")
				str = str[2:]
				continue
			case "A-":
				modifiers = append(modifiers, "Alt")
				str = str[2:]
				continue
			case "C-":
				modifiers = append(modifiers, "Control")
				str = str[2:]
				continue
			case "S-":
				modifiers = append(modifiers, "Shift")
				hasExplicitShift = true
				str = str[2:]
				continue
			case "s-":
				modifiers = append(modifiers, "Super")
				str = str[2:]
				continue
			case "H-":
				modifiers = append(modifiers, "Hyper")
				str = str[2:]
				continue
			}
		}
		// Check for ^ prefix (Control) - case of following letter doesn't imply shift
		if len(str) >= 1 && str[0] == '^' {
			modifiers = append(modifiers, "Control")
			usedCaretNotation = true
			str = str[1:]
			continue
		}
		break
	}

	// The remaining string is the key
	key := str

	// Check if single letter key is uppercase (implies Shift)
	// Only applies to hyphenated modifiers, NOT to ^ notation
	if len(key) == 1 && key[0] >= 'A' && key[0] <= 'Z' && !hasExplicitShift && !usedCaretNotation {
		modifiers = append(modifiers, "Shift")
	}

	// Build the result
	if len(modifiers) == 0 {
		return key
	}

	result := ""
	for i, mod := range modifiers {
		if i > 0 {
			result += "+"
		}
		result += mod
	}
	result += "+" + key

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

// Set sets the shortcuts for an action using key handler format strings.
func (m *ShortcutMap) Set(actionID string, keys ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	shortcuts := make([]Shortcut, len(keys))
	for i, key := range keys {
		shortcuts[i] = Shortcut(key)
	}
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

// FindAction returns the action ID that matches the given shortcut.
// Returns empty string if no match is found.
func (m *ShortcutMap) FindAction(shortcut Shortcut) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for actionID, shortcuts := range m.bindings {
		for _, s := range shortcuts {
			if s == shortcut {
				return actionID
			}
		}
	}
	return ""
}

// FindActionByKey returns the action ID that matches the given key string.
// This is a convenience method that takes a key handler format string directly.
func (m *ShortcutMap) FindActionByKey(key string) string {
	return m.FindAction(Shortcut(key))
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
// All shortcuts use the key handler format directly.
func DefaultShortcuts() *ShortcutMap {
	m := NewShortcutMap()

	// File - using key handler format: ^x = Ctrl+x
	m.Set(StandardActions.New, "^N")
	m.Set(StandardActions.Open, "^O")
	m.Set(StandardActions.Save, "^S")
	m.Set(StandardActions.SaveAs, "^S-S") // Ctrl+Shift+S
	m.Set(StandardActions.Close, "^W")
	m.Set(StandardActions.Quit, "^Q")

	// Edit
	m.Set(StandardActions.Undo, "^Z")
	m.Set(StandardActions.Redo, "^S-Z", "^Y") // Ctrl+Shift+Z or Ctrl+Y
	m.Set(StandardActions.Cut, "^X")
	m.Set(StandardActions.Copy, "^C")
	m.Set(StandardActions.Paste, "^V")
	m.Set(StandardActions.Delete, "Delete")
	m.Set(StandardActions.SelectAll, "M-a")
	m.Set(StandardActions.Find, "^F")
	m.Set(StandardActions.Replace, "^H")

	// Note: Tab, S-Tab, Escape, Enter are handled by Window's FocusManager
	// and dialog widgets directly, not through the global shortcut system.

	return m
}
