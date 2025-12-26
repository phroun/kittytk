// Package core provides fundamental types for the TUI toolkit.
package core

import (
	"strings"
	"sync"
)

// AccessibleRole describes the semantic role of a widget for assistive technology.
type AccessibleRole int

const (
	RoleNone AccessibleRole = iota
	RoleWindow
	RoleDialog
	RoleAlertDialog
	RoleMenuBar
	RoleMenu
	RoleMenuItem
	RoleMenuItemCheckbox
	RoleMenuItemRadio
	RoleButton
	RoleCheckbox
	RoleRadioButton
	RoleComboBox
	RoleTextInput
	RolePasswordInput
	RoleSpinBox
	RoleSlider
	RoleProgressBar
	RoleScrollBar
	RoleTab
	RoleTabList
	RoleTabPanel
	RoleToolBar
	RoleToolTip
	RoleTree
	RoleTreeItem
	RoleList
	RoleListItem
	RoleTable
	RoleTableRow
	RoleTableCell
	RoleTableHeader
	RoleLabel
	RoleStaticText
	RoleLink
	RoleImage
	RoleGroup
	RoleSeparator
	RoleStatusBar
	RoleSplitter
)

// String returns the role name.
func (r AccessibleRole) String() string {
	names := map[AccessibleRole]string{
		RoleNone:             "none",
		RoleWindow:           "window",
		RoleDialog:           "dialog",
		RoleAlertDialog:      "alert dialog",
		RoleMenuBar:          "menu bar",
		RoleMenu:             "menu",
		RoleMenuItem:         "menu item",
		RoleMenuItemCheckbox: "menu item checkbox",
		RoleMenuItemRadio:    "menu item radio",
		RoleButton:           "button",
		RoleCheckbox:         "checkbox",
		RoleRadioButton:      "radio button",
		RoleComboBox:         "combo box",
		RoleTextInput:        "text input",
		RolePasswordInput:    "password input",
		RoleSpinBox:          "spin box",
		RoleSlider:           "slider",
		RoleProgressBar:      "progress bar",
		RoleScrollBar:        "scroll bar",
		RoleTab:              "tab",
		RoleTabList:          "tab list",
		RoleTabPanel:         "tab panel",
		RoleToolBar:          "toolbar",
		RoleToolTip:          "tooltip",
		RoleTree:             "tree",
		RoleTreeItem:         "tree item",
		RoleList:             "list",
		RoleListItem:         "list item",
		RoleTable:            "table",
		RoleTableRow:         "row",
		RoleTableCell:        "cell",
		RoleTableHeader:      "header",
		RoleLabel:            "label",
		RoleStaticText:       "text",
		RoleLink:             "link",
		RoleImage:            "image",
		RoleGroup:            "group",
		RoleSeparator:        "separator",
		RoleStatusBar:        "status bar",
		RoleSplitter:         "splitter",
	}
	if name, ok := names[r]; ok {
		return name
	}
	return "unknown"
}

// AccessibleState represents the current state of a widget for accessibility.
type AccessibleState int

const (
	StateNone AccessibleState = 0
	StateChecked AccessibleState = 1 << iota
	StateSelected
	StateExpanded
	StateCollapsed
	StateDisabled
	StateFocused
	StatePressed
	StateReadOnly
	StateRequired
	StateMultiSelectable
	StateBusy
)

// AccessibleInfo contains accessibility information for a widget.
// Widgets should populate this to support assistive technology.
type AccessibleInfo struct {
	// Role describes what kind of element this is.
	Role AccessibleRole

	// Name is the accessible name (what screen readers announce).
	// This should be concise and descriptive.
	Name string

	// Description provides additional context.
	Description string

	// Value is the current value (for sliders, progress bars, etc.).
	Value string

	// ValueMin is the minimum value (for range controls).
	ValueMin string

	// ValueMax is the maximum value (for range controls).
	ValueMax string

	// State represents the current accessible state.
	State AccessibleState

	// KeyboardShortcut is the shortcut to activate this widget.
	KeyboardShortcut string

	// LiveRegion indicates how updates should be announced.
	// "off" = don't announce, "polite" = wait for idle, "assertive" = interrupt
	LiveRegion string

	// RelatedWidgets contains IDs of related widgets.
	// For example, a label's related widget would be the input it labels.
	RelatedWidgets []string

	// RowIndex and ColumnIndex for table/grid cells.
	RowIndex    int
	ColumnIndex int

	// RowCount and ColumnCount for tables/grids.
	RowCount    int
	ColumnCount int

	// Level for tree items and headings.
	Level int

	// PositionInSet and SetSize for list items.
	PositionInSet int
	SetSize       int
}

// Accessible is an interface for widgets that support accessibility.
type Accessible interface {
	// AccessibleInfo returns the accessibility information.
	AccessibleInfo() AccessibleInfo
}

// AccessibilityAnnouncement represents a message to be spoken by screen readers.
type AccessibilityAnnouncement struct {
	// Message is the text to announce.
	Message string

	// Priority determines if the message should interrupt ("assertive")
	// or wait for idle ("polite").
	Priority string

	// Source is the widget that generated the announcement.
	Source Widget
}

// AccessibilityManager handles accessibility features.
// It collects announcements and can be connected to platform accessibility APIs.
type AccessibilityManager struct {
	mu sync.Mutex

	// Enabled controls whether accessibility features are active.
	Enabled bool

	// Announcements is a channel of pending announcements.
	Announcements chan AccessibilityAnnouncement

	// OnAnnounce is called when an announcement is made.
	// This can be connected to platform TTS or other accessibility services.
	OnAnnounce func(announcement AccessibilityAnnouncement)

	// FocusedWidget tracks the currently focused widget for announcements.
	FocusedWidget Widget
}

// NewAccessibilityManager creates a new accessibility manager.
func NewAccessibilityManager() *AccessibilityManager {
	return &AccessibilityManager{
		Enabled:       true,
		Announcements: make(chan AccessibilityAnnouncement, 100),
	}
}

// Announce queues an announcement.
func (am *AccessibilityManager) Announce(message string, priority string) {
	am.mu.Lock()
	enabled := am.Enabled
	handler := am.OnAnnounce
	am.mu.Unlock()

	if !enabled {
		return
	}

	announcement := AccessibilityAnnouncement{
		Message:  message,
		Priority: priority,
	}

	// Try to send to channel (non-blocking)
	select {
	case am.Announcements <- announcement:
	default:
		// Channel full, drop the announcement
	}

	// Also call handler if set
	if handler != nil {
		handler(announcement)
	}
}

// AnnouncePolite queues a polite announcement (waits for idle).
func (am *AccessibilityManager) AnnouncePolite(message string) {
	am.Announce(message, "polite")
}

// AnnounceAssertive queues an assertive announcement (interrupts).
func (am *AccessibilityManager) AnnounceAssertive(message string) {
	am.Announce(message, "assertive")
}

// AnnounceFocus announces focus change on a widget.
func (am *AccessibilityManager) AnnounceFocus(widget Widget) {
	am.mu.Lock()
	am.FocusedWidget = widget
	enabled := am.Enabled
	am.mu.Unlock()

	if !enabled {
		return
	}

	// Get accessible info if available
	var info AccessibleInfo
	if accessible, ok := widget.(Accessible); ok {
		info = accessible.AccessibleInfo()
	} else {
		// Default to widget name
		info.Name = widget.Name()
		info.Role = RoleNone
	}

	// Build announcement
	var parts []string

	// Add name
	if info.Name != "" {
		parts = append(parts, info.Name)
	}

	// Add role
	if info.Role != RoleNone {
		parts = append(parts, info.Role.String())
	}

	// Add state information
	if info.State&StateDisabled != 0 {
		parts = append(parts, "disabled")
	}
	if info.State&StateChecked != 0 {
		parts = append(parts, "checked")
	}
	if info.State&StateSelected != 0 {
		parts = append(parts, "selected")
	}
	if info.State&StateExpanded != 0 {
		parts = append(parts, "expanded")
	}
	if info.State&StateCollapsed != 0 {
		parts = append(parts, "collapsed")
	}

	// Add value
	if info.Value != "" {
		parts = append(parts, info.Value)
	}

	// Add position in set
	if info.SetSize > 0 && info.PositionInSet > 0 {
		parts = append(parts, formatPosition(info.PositionInSet, info.SetSize))
	}

	// Add keyboard shortcut
	if info.KeyboardShortcut != "" {
		parts = append(parts, info.KeyboardShortcut)
	}

	if len(parts) > 0 {
		am.AnnouncePolite(strings.Join(parts, ", "))
	}
}

// AnnounceAction announces an action being taken.
func (am *AccessibilityManager) AnnounceAction(widget Widget, action string) {
	am.mu.Lock()
	enabled := am.Enabled
	am.mu.Unlock()

	if !enabled {
		return
	}

	var name string
	if accessible, ok := widget.(Accessible); ok {
		info := accessible.AccessibleInfo()
		name = info.Name
	} else {
		name = widget.Name()
	}

	if name != "" {
		am.AnnouncePolite(name + " " + action)
	} else {
		am.AnnouncePolite(action)
	}
}

// AnnounceError announces an error.
func (am *AccessibilityManager) AnnounceError(message string) {
	am.AnnounceAssertive("Error: " + message)
}

// AnnounceAlert announces an alert.
func (am *AccessibilityManager) AnnounceAlert(message string) {
	am.AnnounceAssertive(message)
}

// formatPosition formats "N of M" text.
func formatPosition(position, total int) string {
	return strings.Builder{}.String() // Placeholder
}

// AccessibleWidget provides a base implementation of accessibility features.
// Embed this in widgets to get default accessibility support.
type AccessibleWidget struct {
	accessibleName        string
	accessibleDescription string
	accessibleRole        AccessibleRole
}

// SetAccessibleName sets the accessible name.
func (aw *AccessibleWidget) SetAccessibleName(name string) {
	aw.accessibleName = name
}

// SetAccessibleDescription sets the accessible description.
func (aw *AccessibleWidget) SetAccessibleDescription(desc string) {
	aw.accessibleDescription = desc
}

// SetAccessibleRole sets the accessible role.
func (aw *AccessibleWidget) SetAccessibleRole(role AccessibleRole) {
	aw.accessibleRole = role
}

// AccessibleInfo returns the accessibility information.
func (aw *AccessibleWidget) AccessibleInfo() AccessibleInfo {
	return AccessibleInfo{
		Name:        aw.accessibleName,
		Description: aw.accessibleDescription,
		Role:        aw.accessibleRole,
	}
}

// ScreenReaderAdapter is an interface for connecting to platform screen readers.
// Implementations would connect to:
// - macOS: NSAccessibility
// - Windows: UI Automation / MSAA
// - Linux: AT-SPI
// - Web: ARIA live regions
type ScreenReaderAdapter interface {
	// Announce sends a message to the screen reader.
	Announce(message string, priority string)

	// SetFocus notifies the screen reader of focus change.
	SetFocus(info AccessibleInfo)

	// IsAvailable returns true if a screen reader is active.
	IsAvailable() bool
}

// NullScreenReader is a no-op screen reader adapter for when none is available.
type NullScreenReader struct{}

func (NullScreenReader) Announce(message string, priority string) {}
func (NullScreenReader) SetFocus(info AccessibleInfo)              {}
func (NullScreenReader) IsAvailable() bool                         { return false }

// TerminalScreenReader outputs accessibility messages to a terminal.
// This is useful for development and debugging.
type TerminalScreenReader struct {
	mu     sync.Mutex
	Output func(message string)
}

func (t *TerminalScreenReader) Announce(message string, priority string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Output != nil {
		t.Output("[" + priority + "] " + message)
	}
}

func (t *TerminalScreenReader) SetFocus(info AccessibleInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Output != nil {
		t.Output("[focus] " + info.Name + " (" + info.Role.String() + ")")
	}
}

func (t *TerminalScreenReader) IsAvailable() bool {
	return true
}
