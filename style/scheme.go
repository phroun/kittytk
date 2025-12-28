// Package style provides theming and visual styling for the TUI toolkit.
package style

import "sync"

// Scheme defines a color scheme that can be applied to widgets.
// A Theme can contain multiple Schemes (e.g., default, modal dialogs, toolbars).
// Each color field is a pointer; nil means use the fallback defined in scheme-plan.txt.
type Scheme struct {
	// =========================================================================
	// Environment Related Colors
	// =========================================================================

	DesktopFill       *CellStyle
	StatusBar         *CellStyle
	StatusBarShortcut *CellStyle
	Dock              *CellStyle
	DockItem          *CellStyle
	FocusedDockItem   *CellStyle

	// =========================================================================
	// Window Frame Related Colors
	// =========================================================================

	ActiveWindowBorder     *CellStyle
	InactiveWindowBorder   *CellStyle
	ActiveWindowTitle      *CellStyle // currently bright and bold white on blue
	InactiveWindowTitle    *CellStyle
	ActiveTitleBarButton   *CellStyle
	InactiveTitleBarButton *CellStyle
	FocusedTitleBarButton  *CellStyle
	PressedTitleBarButton  *CellStyle
	InactiveWindowFG       *CellStyle // FG only
	ActiveWindowFG         *CellStyle // FG only
	InactiveWindowBG       *CellStyle // BG only
	ActiveWindowBG         *CellStyle // BG only

	// =========================================================================
	// State Related Colors (used as defaults for many other scheme values)
	// =========================================================================

	FocusBG        *CellStyle // normal cyan (BG only, used as default for many)
	FocusFG        *CellStyle // normal black (FG only, used as default for many)
	FocusTextFG    *CellStyle // bright cyan (FG only, used as default for many)
	DisabledTextFG *CellStyle // FG only
	Selection      *CellStyle // bright white on blue
	Normal         *CellStyle // nil = WindowFG on WindowBG

	// =========================================================================
	// Menu Related Colors
	// =========================================================================

	MenuBar                *CellStyle
	MenuBarMeta            *CellStyle // accelerator keys on menu bar
	MenuBarInfo            *CellStyle // clock, etc.
	ActiveMenuBarItem      *CellStyle
	ActiveMenuBarMeta      *CellStyle // accelerator keys on active menu
	MenuGutter             *CellStyle
	MenuCheckIcon          *CellStyle
	MenuRadioIcon          *CellStyle
	MenuItemText           *CellStyle
	MenuAccelerator        *CellStyle
	MenuShortcut           *CellStyle
	MenuSeparator          *CellStyle
	MenuSeparatorGutter    *CellStyle
	MenuBarButton          *CellStyle // for [<][>]
	DisabledMenuBarButton  *CellStyle
	DisabledMenuGutter     *CellStyle
	DisabledMenuItem       *CellStyle // applies to Text, Shortcut and Accelerator
	DisabledMenuIcon       *CellStyle // applies to Check and Radio
	FocusedMenuCheckIcon   *CellStyle
	FocusedMenuRadioIcon   *CellStyle
	FocusedMenuItemText    *CellStyle
	FocusedMenuAccelerator *CellStyle
	FocusedMenuShortcut    *CellStyle

	// =========================================================================
	// Widget Group Colors (used as defaults for other things)
	// =========================================================================

	WidgetGroupFG   *CellStyle // bright white (FG only)
	WidgetGroupBG   *CellStyle // black (BG only)
	WidgetContentFG *CellStyle // regular white (FG only)
	WidgetContentBG *CellStyle // black (BG only)

	// =========================================================================
	// Basic Widget Related Colors
	// =========================================================================

	// Label
	LabelFG         *CellStyle // nil = WindowFG
	LabelBG         *CellStyle // nil = inherit
	DisabledLabelFG *CellStyle // nil = DisabledTextFG
	DisabledLabelBG *CellStyle // nil = inherit

	// Button
	Button                    *CellStyle // black on regular white
	DisabledButtonFG          *CellStyle // nil = DisabledTextFG
	DisabledButtonBG          *CellStyle // nil = inherit
	FocusedButton             *CellStyle // nil = FocusBG + FocusFG
	PressedButton             *CellStyle // nil = WindowBG + WindowFG
	DefaultPaneButtonShadowFG *CellStyle // dim blue, used when inherited BG is ansi 49 (FG only)
	DarkPaneButtonShadowFG    *CellStyle // dark gray, used when inherited BG is black (FG only)
	ButtonShadowFG            *CellStyle // black, used in all other cases (FG only)

	// EditBox / TextInput
	EditBox                     *CellStyle // regular white on black
	DefaultPaneEditBox          *CellStyle // regular white on dark blue, when inherited BG is ansi 49
	DarkPaneEditBox             *CellStyle // regular white on dark blue, when inherited BG is black
	EditBoxPlaceholder          *CellStyle
	DefaultPaneEditBoxPlaceholder *CellStyle // nil = EditBoxPlaceholder
	DarkPaneEditBoxPlaceholder  *CellStyle // nil = EditBoxPlaceholder
	FocusedEditBoxText          *CellStyle // black on dark cyan
	FocusedEditBoxCursor        *CellStyle // black on white
	FocusedEditBoxFill          *CellStyle // white on cyan

	// ComboBox
	ComboBox                 *CellStyle // nil = same as EditBox
	ComboBoxArrow            *CellStyle // nil = same as EditBox
	DisabledComboBoxFG       *CellStyle // nil = DisabledButtonFG
	DisabledComboBoxArrow    *CellStyle // nil = DisabledComboBoxFG + DisabledComboBoxBG
	DisabledComboBoxBG       *CellStyle // nil = DisabledButtonBG
	FocusedComboBox          *CellStyle // nil = FocusedButton
	FocusedComboBoxArrow     *CellStyle // nil = FocusedComboBox
	PressedComboBox          *CellStyle // nil = PressedButton
	PressedComboBoxArrow     *CellStyle // nil = PressedComboBox
	DropdownItemText         *CellStyle // nil = MenuItemText
	FocusedDropdownItemText  *CellStyle // nil = FocusedMenuItemText
	DropdownSeparator        *CellStyle // nil = MenuSeparator
	DisabledDropdownItem     *CellStyle // nil = DisabledMenuItem

	// CheckBox
	CheckBoxFG            *CellStyle // nil = same as LabelFG
	CheckBoxBG            *CellStyle // nil = inherit
	CheckBoxLabelFG       *CellStyle // nil = same as LabelFG
	FocusedCheckBoxFG     *CellStyle // nil = FocusTextFG
	FocusedCheckBoxBG     *CellStyle // nil = inherit
	FocusedCheckBoxLabelFG *CellStyle // nil = FocusTextFG

	// RadioButton
	RadioButtonFG            *CellStyle // nil = same as LabelFG
	RadioButtonBG            *CellStyle // nil = inherit
	RadioButtonLabelFG       *CellStyle // nil = same as LabelFG
	FocusedRadioButtonFG     *CellStyle // nil = FocusTextFG
	FocusedRadioButtonBG     *CellStyle // nil = inherit
	FocusedRadioButtonLabelFG *CellStyle // nil = FocusTextFG

	// =========================================================================
	// Splitter Related Colors
	// =========================================================================

	Splitter              *CellStyle // dark gray on black
	SplitterHandle        *CellStyle // nil = Splitter
	SplitterTitle         *CellStyle // nil = Splitter
	FocusedSplitter       *CellStyle // nil = FocusBG + FocusFG
	FocusedSplitterHandle *CellStyle // nil = FocusedSplitter
	FocusedSplitterTitle  *CellStyle // nil = FocusedSplitter

	// =========================================================================
	// Tab Widget (Page Control) Related Colors
	// =========================================================================

	TabsFG             *CellStyle // nil = WindowFG, currently bright white
	TabsBG             *CellStyle // nil = inherit
	TabsButton         *CellStyle // nil = TabsFG + TabsBG
	PressedTabsButton  *CellStyle // nil = black on white
	DisabledTabsButton *CellStyle // nil = DisabledTextFG on TabsBG
	ActiveTabFG        *CellStyle // currently bright and bold yellow, nil = WidgetGroupFG
	ActiveTabBG        *CellStyle // nil = WidgetGroupBG, currently ansi 49
	FocusedTab         *CellStyle // nil = FocusBG + FocusFG
	TabCloseButton     *CellStyle // close button on tabs
	FocusedTabCloseButton *CellStyle
	PressedTabCloseButton *CellStyle

	// =========================================================================
	// List Related Colors (TreeView, ListView)
	// =========================================================================

	ListBG           *CellStyle // nil = WidgetContentBG
	ListFG           *CellStyle // nil = WidgetContentFG
	FocusedListBG    *CellStyle // nil = ListBG
	FocusedListFG    *CellStyle // nil = ListFG
	SelectedListItem *CellStyle // nil = Selection
	FocusedListItem  *CellStyle // nil = FocusBG + FocusFG

	// =========================================================================
	// Scrollbar Colors (ScrollArea, ListView, TreeView)
	// =========================================================================

	Scrollbar      *CellStyle // dark gray on black
	ScrollbarThumb *CellStyle // regular white on black

	// =========================================================================
	// ProgressBar Colors
	// =========================================================================

	ProgressFull      *CellStyle // bright green on dim green
	ProgressEmpty     *CellStyle // dim green on black
	ProgressFullText  *CellStyle // black on dim green
	ProgressEmptyText *CellStyle // bright yellow on black

	// =========================================================================
	// GroupBox / FramedPane Related Colors
	// =========================================================================

	GroupBoxBorder        *CellStyle // nil = WindowFG on inherit
	GroupBoxTitle         *CellStyle // nil = WindowFG on inherit
	FocusedGroupBoxBorder *CellStyle // nil = FocusTextFG on inherit
	FocusedGroupBoxTitle  *CellStyle // nil = FocusTextFG on inherit
}

// SchemeID represents a scheme identifier.
type SchemeID int

const (
	// SchemeInherit means the widget inherits its scheme from its container.
	SchemeInherit SchemeID = -1
	// SchemeDefault is the default scheme (index 0).
	SchemeDefault SchemeID = 0
)

// SchemeRegistry manages the collection of available schemes.
type SchemeRegistry struct {
	mu      sync.RWMutex
	schemes map[SchemeID]*Scheme
}

// globalRegistry is the default scheme registry.
var globalRegistry = &SchemeRegistry{
	schemes: make(map[SchemeID]*Scheme),
}

// init initializes the default scheme (scheme 0).
func init() {
	globalRegistry.Register(SchemeDefault, DefaultScheme())
}

// GlobalSchemeRegistry returns the global scheme registry.
func GlobalSchemeRegistry() *SchemeRegistry {
	return globalRegistry
}

// Register adds or updates a scheme in the registry.
func (r *SchemeRegistry) Register(id SchemeID, scheme *Scheme) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.schemes[id] = scheme
}

// Get retrieves a scheme by ID. Returns the default scheme if not found.
func (r *SchemeRegistry) Get(id SchemeID) *Scheme {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if scheme, ok := r.schemes[id]; ok {
		return scheme
	}
	// Fall back to default scheme for invalid IDs
	if scheme, ok := r.schemes[SchemeDefault]; ok {
		return scheme
	}
	// Last resort: return an empty scheme
	return &Scheme{}
}

// Has checks if a scheme exists in the registry.
func (r *SchemeRegistry) Has(id SchemeID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.schemes[id]
	return ok
}

// ptr is a helper to create a pointer to a CellStyle.
func ptr(s CellStyle) *CellStyle {
	return &s
}

// DefaultScheme creates the default scheme (scheme 0) with current hard-coded values.
func DefaultScheme() *Scheme {
	return &Scheme{
		// Environment Related Colors
		DesktopFill:       ptr(DefaultStyle().WithFg(ColorBlue).WithBg(ColorBlue)),
		StatusBar:         ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorWhite)),
		StatusBarShortcut: ptr(DefaultStyle().WithFg(ColorRed).WithBg(ColorWhite)),
		Dock:              ptr(DefaultStyle().WithFg(ColorWhite).WithBg(ColorBlue)),
		DockItem:          ptr(DefaultStyle().WithFg(ColorWhite).WithBg(ColorBlue)),
		FocusedDockItem:   ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorCyan)),

		// Window Frame Related Colors
		ActiveWindowBorder:     ptr(DefaultStyle().WithFg(ColorWhite).WithBg(ColorBlue)),
		InactiveWindowBorder:   ptr(DefaultStyle().WithFg(ColorWhite).WithBg(ColorBlue)),
		ActiveWindowTitle:      ptr(DefaultStyle().WithFg(ColorBrightWhite).WithBg(ColorBlue).Bold()),
		InactiveWindowTitle:    ptr(DefaultStyle().WithFg(ColorWhite).WithBg(ColorBlue)),
		ActiveTitleBarButton:   ptr(DefaultStyle().WithFg(ColorWhite).WithBg(ColorBlue)),
		InactiveTitleBarButton: ptr(DefaultStyle().WithFg(ColorWhite).WithBg(ColorBlue)),
		FocusedTitleBarButton:  ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorCyan)),
		PressedTitleBarButton:  ptr(DefaultStyle().WithFg(ColorBlue).WithBg(ColorWhite)),
		InactiveWindowFG:       ptr(DefaultStyle().WithFg(ColorWhite)),
		ActiveWindowFG:         ptr(DefaultStyle().WithFg(ColorWhite)),
		InactiveWindowBG:       ptr(DefaultStyle().WithBg(ColorBlue)),
		ActiveWindowBG:         ptr(DefaultStyle().WithBg(ColorBlue)),

		// State Related Colors
		FocusBG:        ptr(DefaultStyle().WithBg(ColorCyan)),
		FocusFG:        ptr(DefaultStyle().WithFg(ColorBlack)),
		FocusTextFG:    ptr(DefaultStyle().WithFg(ColorBrightCyan)),
		DisabledTextFG: ptr(DefaultStyle().WithFg(ColorBrightBlack)),
		Selection:      ptr(DefaultStyle().WithFg(ColorBrightWhite).WithBg(ColorBlue)),
		Normal:         nil, // nil = WindowFG on WindowBG

		// Menu Related Colors
		MenuBar:                ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorWhite)),
		MenuBarMeta:            ptr(DefaultStyle().WithFg(ColorRed).WithBg(ColorWhite)),
		MenuBarInfo:            ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorWhite)),
		ActiveMenuBarItem:      ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorCyan)),
		ActiveMenuBarMeta:      ptr(DefaultStyle().WithFg(ColorRed).WithBg(ColorCyan)),
		MenuGutter:             ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorWhite)),
		MenuCheckIcon:          ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorWhite)),
		MenuRadioIcon:          ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorWhite)),
		MenuItemText:           ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorWhite)),
		MenuAccelerator:        ptr(DefaultStyle().WithFg(ColorRed).WithBg(ColorWhite)),
		MenuShortcut:           ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorWhite)),
		MenuSeparator:          ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorWhite)),
		MenuSeparatorGutter:    ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorWhite)),
		MenuBarButton:          ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorWhite)),
		DisabledMenuBarButton:  ptr(DefaultStyle().WithFg(ColorBrightBlack).WithBg(ColorWhite)),
		DisabledMenuGutter:     ptr(DefaultStyle().WithFg(ColorBrightBlack).WithBg(ColorWhite)),
		DisabledMenuItem:       ptr(DefaultStyle().WithFg(ColorBrightBlack).WithBg(ColorWhite)),
		DisabledMenuIcon:       ptr(DefaultStyle().WithFg(ColorBrightBlack).WithBg(ColorWhite)),
		FocusedMenuCheckIcon:   ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorCyan)),
		FocusedMenuRadioIcon:   ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorCyan)),
		FocusedMenuItemText:    ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorCyan)),
		FocusedMenuAccelerator: ptr(DefaultStyle().WithFg(ColorRed).WithBg(ColorCyan)),
		FocusedMenuShortcut:    ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorCyan)),

		// Widget Group Colors
		WidgetGroupFG:   ptr(DefaultStyle().WithFg(ColorBrightWhite)),
		WidgetGroupBG:   ptr(DefaultStyle().WithBg(ColorBlack)),
		WidgetContentFG: ptr(DefaultStyle().WithFg(ColorWhite)),
		WidgetContentBG: ptr(DefaultStyle().WithBg(ColorBlack)),

		// Label
		LabelFG:         nil, // WindowFG
		LabelBG:         nil, // inherit
		DisabledLabelFG: nil, // DisabledTextFG
		DisabledLabelBG: nil, // inherit

		// Button
		Button:                    ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorWhite)),
		DisabledButtonFG:          nil, // DisabledTextFG
		DisabledButtonBG:          nil, // inherit
		FocusedButton:             ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorCyan)),
		PressedButton:             nil, // WindowBG + WindowFG
		DefaultPaneButtonShadowFG: ptr(DefaultStyle().WithFg(ColorBlue).WithAttrs(StyleDim)),
		DarkPaneButtonShadowFG:    ptr(DefaultStyle().WithFg(ColorBrightBlack)),
		ButtonShadowFG:            ptr(DefaultStyle().WithFg(ColorBlack)),

		// EditBox / TextInput
		EditBox:                       ptr(DefaultStyle().WithFg(ColorWhite).WithBg(ColorBlack)),
		DefaultPaneEditBox:            ptr(DefaultStyle().WithFg(ColorWhite).WithBg(ColorBlue)),
		DarkPaneEditBox:               ptr(DefaultStyle().WithFg(ColorWhite).WithBg(ColorBlue)),
		EditBoxPlaceholder:            ptr(DefaultStyle().WithFg(ColorBrightBlack).WithBg(ColorBlack)),
		DefaultPaneEditBoxPlaceholder: nil, // EditBoxPlaceholder
		DarkPaneEditBoxPlaceholder:    nil, // EditBoxPlaceholder
		FocusedEditBoxText:            ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorCyan)),
		FocusedEditBoxCursor:          ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorWhite)),
		FocusedEditBoxFill:            ptr(DefaultStyle().WithFg(ColorWhite).WithBg(ColorCyan)),

		// ComboBox
		ComboBox:                nil, // EditBox
		ComboBoxArrow:           nil, // EditBox
		DisabledComboBoxFG:      nil, // DisabledButtonFG
		DisabledComboBoxArrow:   nil, // DisabledComboBoxFG + DisabledComboBoxBG
		DisabledComboBoxBG:      nil, // DisabledButtonBG
		FocusedComboBox:         nil, // FocusedButton
		FocusedComboBoxArrow:    nil, // FocusedComboBox
		PressedComboBox:         nil, // PressedButton
		PressedComboBoxArrow:    nil, // PressedComboBox
		DropdownItemText:        nil, // MenuItemText
		FocusedDropdownItemText: nil, // FocusedMenuItemText
		DropdownSeparator:       nil, // MenuSeparator
		DisabledDropdownItem:    nil, // DisabledMenuItem

		// CheckBox
		CheckBoxFG:             nil, // LabelFG
		CheckBoxBG:             nil, // inherit
		CheckBoxLabelFG:        nil, // LabelFG
		FocusedCheckBoxFG:      ptr(DefaultStyle().WithFg(ColorBrightCyan)),
		FocusedCheckBoxBG:      nil, // inherit
		FocusedCheckBoxLabelFG: ptr(DefaultStyle().WithFg(ColorBrightCyan)),

		// RadioButton
		RadioButtonFG:             nil, // LabelFG
		RadioButtonBG:             nil, // inherit
		RadioButtonLabelFG:        nil, // LabelFG
		FocusedRadioButtonFG:      ptr(DefaultStyle().WithFg(ColorBrightCyan)),
		FocusedRadioButtonBG:      nil, // inherit
		FocusedRadioButtonLabelFG: ptr(DefaultStyle().WithFg(ColorBrightCyan)),

		// Splitter
		Splitter:              ptr(DefaultStyle().WithFg(ColorBrightBlack).WithBg(ColorBlack)),
		SplitterHandle:        nil, // Splitter
		SplitterTitle:         nil, // Splitter
		FocusedSplitter:       ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorCyan)),
		FocusedSplitterHandle: nil, // FocusedSplitter
		FocusedSplitterTitle:  nil, // FocusedSplitter

		// Tab Widget
		TabsFG:                ptr(DefaultStyle().WithFg(ColorBrightWhite)),
		TabsBG:                nil, // inherit
		TabsButton:            nil, // TabsFG + TabsBG
		PressedTabsButton:     ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorWhite)),
		DisabledTabsButton:    nil, // DisabledTextFG on TabsBG
		ActiveTabFG:           ptr(DefaultStyle().WithFg(ColorBrightYellow).Bold()),
		ActiveTabBG:           ptr(DefaultStyle().WithBg(ColorDefault)), // ansi 49
		FocusedTab:            ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorCyan)),
		TabCloseButton:        nil, // TabsFG + TabsBG
		FocusedTabCloseButton: nil, // FocusedTab
		PressedTabCloseButton: nil, // PressedTabsButton

		// List
		ListBG:           ptr(DefaultStyle().WithBg(ColorBlack)),
		ListFG:           ptr(DefaultStyle().WithFg(ColorWhite)),
		FocusedListBG:    nil, // ListBG
		FocusedListFG:    nil, // ListFG
		SelectedListItem: ptr(DefaultStyle().WithFg(ColorBrightWhite).WithBg(ColorBlue)),
		FocusedListItem:  ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorCyan)),

		// Scrollbar
		Scrollbar:      ptr(DefaultStyle().WithFg(ColorBrightBlack).WithBg(ColorBlack)),
		ScrollbarThumb: ptr(DefaultStyle().WithFg(ColorWhite).WithBg(ColorBlack)),

		// ProgressBar
		ProgressFull:      ptr(DefaultStyle().WithFg(ColorBrightGreen).WithBg(ColorGreen)),
		ProgressEmpty:     ptr(DefaultStyle().WithFg(ColorGreen).WithBg(ColorBlack)),
		ProgressFullText:  ptr(DefaultStyle().WithFg(ColorBlack).WithBg(ColorGreen)),
		ProgressEmptyText: ptr(DefaultStyle().WithFg(ColorBrightYellow).WithBg(ColorBlack)),

		// GroupBox / FramedPane
		GroupBoxBorder:        nil, // WindowFG on inherit
		GroupBoxTitle:         nil, // WindowFG on inherit
		FocusedGroupBoxBorder: nil, // FocusTextFG on inherit
		FocusedGroupBoxTitle:  nil, // FocusTextFG on inherit
	}
}
