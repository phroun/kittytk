// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// TextInput is a single-line text entry widget.
type TextInput struct {
	core.WidgetBase
	core.AccessibleWidget

	text        []rune
	placeholder string
	maxLength   int
	echoMode    EchoMode
	readOnly    bool

	// Cursor and selection
	cursorPos    int
	selStart     int
	selEnd       int
	scrollOffset int

	// Callbacks
	onTextChanged   func(text string)
	onReturnPressed func()

	// Graphical caret blink: the bar toggles while focused and
	// restarts visible on every keystroke. Without a running timer
	// (cell surfaces, no desktop) the caret is steady.
	caretTimer *DesktopTimer
	caretOn    bool

	// Drag selection in progress (armed by a left press, extended by
	// motion while the button is held).
	selecting bool

	// Context menu hover row (-1 = none).
	menuHover int
}

// EchoMode controls how text is displayed.
type EchoMode int

const (
	EchoNormal         EchoMode = iota // Show text normally
	EchoPassword                       // Show bullets/asterisks
	EchoPasswordOnEdit                 // Show char briefly, then bullet
	EchoNoEcho                         // Show nothing
)

// NewTextInput creates a new text input.
func NewTextInput() *TextInput {
	t := &TextInput{
		echoMode:  EchoNormal,
		maxLength: -1, // No limit
	}
	t.WidgetBase = *core.NewWidgetBase()
	t.Init(t) // Enable polymorphic focus handling
	t.SetFocusPolicy(core.StrongFocus)
	t.SetAccessibleRole(core.RoleTextInput)
	return t
}

// CursorShape implements core.CursorProvider: an editable text field
// shows the text I-beam while hovered.
func (t *TextInput) CursorShape() core.CursorShape {
	return core.CursorText
}

// Text returns the current text.
func (t *TextInput) Text() string {
	return string(t.text)
}

// SetText sets the text content.
func (t *TextInput) SetText(text string) {
	t.text = []rune(text)
	t.cursorPos = len(t.text)
	t.selStart = 0
	t.selEnd = 0
	t.scrollOffset = 0
	t.Update()

	if t.onTextChanged != nil {
		t.onTextChanged(text)
	}
}

// Placeholder returns the placeholder text.
func (t *TextInput) Placeholder() string {
	return t.placeholder
}

// SetPlaceholder sets the placeholder text.
func (t *TextInput) SetPlaceholder(text string) {
	t.placeholder = text
	t.Update()
}

// MaxLength returns the maximum text length.
func (t *TextInput) MaxLength() int {
	return t.maxLength
}

// SetMaxLength sets the maximum text length (-1 for no limit).
func (t *TextInput) SetMaxLength(length int) {
	t.maxLength = length
}

// EchoMode returns the echo mode.
func (t *TextInput) EchoMode() EchoMode {
	return t.echoMode
}

// SetEchoMode sets the echo mode.
func (t *TextInput) SetEchoMode(mode EchoMode) {
	t.echoMode = mode
	if mode == EchoPassword {
		t.SetAccessibleRole(core.RolePasswordInput)
	} else {
		t.SetAccessibleRole(core.RoleTextInput)
	}
	t.Update()
}

// IsReadOnly returns whether the input is read-only.
func (t *TextInput) IsReadOnly() bool {
	return t.readOnly
}

// SetReadOnly sets the read-only state.
func (t *TextInput) SetReadOnly(readOnly bool) {
	t.readOnly = readOnly
	t.Update()
}

// CursorPosition returns the cursor position.
func (t *TextInput) CursorPosition() int {
	return t.cursorPos
}

// SetCursorPosition sets the cursor position.
func (t *TextInput) SetCursorPosition(pos int) {
	if pos < 0 {
		pos = 0
	}
	if pos > len(t.text) {
		pos = len(t.text)
	}
	t.cursorPos = pos
	t.selStart = pos
	t.selEnd = pos
	t.ensureCursorVisible()
	// Moving the caret restarts the blink visible, so its new position
	// shows immediately.
	t.resetCaretBlink()
	t.Update()
}

// HasSelection returns whether there is a text selection.
func (t *TextInput) HasSelection() bool {
	return t.selStart != t.selEnd
}

// SelectedText returns the selected text.
func (t *TextInput) SelectedText() string {
	if t.selStart == t.selEnd {
		return ""
	}
	start, end := t.selStart, t.selEnd
	if start > end {
		start, end = end, start
	}
	return string(t.text[start:end])
}

// SelectAll selects all text.
func (t *TextInput) SelectAll() {
	t.selStart = 0
	t.selEnd = len(t.text)
	t.cursorPos = t.selEnd
	t.Update()
}

// ClearSelection clears the selection.
func (t *TextInput) ClearSelection() {
	t.selStart = t.cursorPos
	t.selEnd = t.cursorPos
	t.Update()
}

// SetOnTextChanged sets the text changed callback.
func (t *TextInput) SetOnTextChanged(handler func(text string)) {
	t.onTextChanged = handler
}

// SetOnReturnPressed sets the return pressed callback.
func (t *TextInput) SetOnReturnPressed(handler func()) {
	t.onReturnPressed = handler
}

// insert inserts text at the cursor position.
func (t *TextInput) insert(text string) {
	if t.readOnly {
		return
	}

	// Delete selection first
	t.deleteSelection()

	// Check max length
	runes := []rune(text)
	if t.maxLength >= 0 && len(t.text)+len(runes) > t.maxLength {
		remaining := t.maxLength - len(t.text)
		if remaining <= 0 {
			return
		}
		runes = runes[:remaining]
	}

	// Insert
	newText := make([]rune, len(t.text)+len(runes))
	copy(newText[:t.cursorPos], t.text[:t.cursorPos])
	copy(newText[t.cursorPos:], runes)
	copy(newText[t.cursorPos+len(runes):], t.text[t.cursorPos:])
	t.text = newText
	t.cursorPos += len(runes)
	t.selStart = t.cursorPos
	t.selEnd = t.cursorPos

	t.textChanged()
}

// deleteSelection deletes the selected text.
func (t *TextInput) deleteSelection() {
	if t.selStart == t.selEnd {
		return
	}

	start, end := t.selStart, t.selEnd
	if start > end {
		start, end = end, start
	}

	newText := make([]rune, len(t.text)-(end-start))
	copy(newText[:start], t.text[:start])
	copy(newText[start:], t.text[end:])
	t.text = newText
	t.cursorPos = start
	t.selStart = start
	t.selEnd = start
}

// backspace deletes the character before the cursor.
func (t *TextInput) backspace() {
	if t.readOnly {
		return
	}

	if t.HasSelection() {
		t.deleteSelection()
		t.textChanged()
		return
	}

	if t.cursorPos > 0 {
		newText := make([]rune, len(t.text)-1)
		copy(newText[:t.cursorPos-1], t.text[:t.cursorPos-1])
		copy(newText[t.cursorPos-1:], t.text[t.cursorPos:])
		t.text = newText
		t.cursorPos--
		t.selStart = t.cursorPos
		t.selEnd = t.cursorPos
		t.textChanged()
	}
}

// delete deletes the character after the cursor.
func (t *TextInput) delete() {
	if t.readOnly {
		return
	}

	if t.HasSelection() {
		t.deleteSelection()
		t.textChanged()
		return
	}

	if t.cursorPos < len(t.text) {
		newText := make([]rune, len(t.text)-1)
		copy(newText[:t.cursorPos], t.text[:t.cursorPos])
		copy(newText[t.cursorPos:], t.text[t.cursorPos+1:])
		t.text = newText
		t.textChanged()
	}
}

// textChanged triggers the text changed callback.
func (t *TextInput) textChanged() {
	t.ensureCursorVisible()
	t.Update()
	if t.onTextChanged != nil {
		t.onTextChanged(string(t.text))
	}
}

// ensureCursorVisible scrolls to make the cursor visible.
func (t *TextInput) ensureCursorVisible() {
	bounds := t.Bounds()
	font := t.EffectiveFont()
	metrics := t.EffectiveCellMetrics()

	if bounds.Width <= 0 {
		return
	}

	// Scroll left if cursor is before visible area
	if t.cursorPos < t.scrollOffset {
		t.scrollOffset = t.cursorPos
	}

	// Scroll right if cursor is after visible area
	// Calculate width of text from scrollOffset to cursor using font metrics
	displayText := t.getDisplayText()

	// Calculate cursor character width (space if at end, otherwise char under cursor)
	var cursorWidth core.Unit
	if t.cursorPos < len(displayText) {
		cursorWidth = font.MeasureText(string(displayText[t.cursorPos]))
	} else {
		cursorWidth = metrics.CellWidth // Space for cursor at end of text
	}

	for t.cursorPos > t.scrollOffset {
		// Calculate width from scrollOffset to cursorPos
		start := t.scrollOffset
		end := t.cursorPos
		if end > len(displayText) {
			end = len(displayText)
		}
		if start >= len(displayText) {
			break
		}
		visibleText := string(displayText[start:end])
		textWidth := font.MeasureText(visibleText)

		// Need room for text before cursor PLUS the cursor character itself
		if textWidth+cursorWidth <= bounds.Width {
			break
		}
		// Scroll right by one character
		t.scrollOffset++
	}
}

// SizeHint returns the preferred size.
func (t *TextInput) SizeHint() core.UnitSize {
	metrics := t.EffectiveCellMetrics()
	// TextInput has a fixed size in units (160 wide x 16 tall) - does not scale with font
	return core.UnitSize{
		Width:  metrics.TextWidth(20),
		Height: metrics.TextHeight(1),
	}
}

// IsInlineWidget returns true to indicate this is a text-style widget
// that should receive horizontal margins when in a vertical box layout.
func (t *TextInput) IsInlineWidget() bool {
	return true
}

// Paint renders the text input.
func (t *TextInput) Paint(p *core.Painter) {
	bounds := t.Bounds()
	scheme := t.GetScheme()
	focused := t.HasFocus()
	font := t.EffectiveFont()

	// Get inherited background color to determine pane type
	inheritedBg := t.EffectiveBackgroundColor()
	paneType := style.GetPaneType(inheritedBg)

	// Determine style
	var s style.CellStyle
	var fillChar rune = ' '
	if !t.IsEnabled() {
		s = style.DefaultStyle().WithFg(scheme.GetDisabledTextFG()).WithBg(scheme.GetEditBox(paneType).Bg)
	} else if focused {
		s = scheme.GetFocusedEditBoxText()
		// Use speckled fill character for focused state
		fillChar = '░'
	} else {
		// Unfocused editbox style depends on pane type
		s = scheme.GetEditBox(paneType)
	}

	// Draw background - use fill style with speckled pattern for focused state
	fillStyle := s
	if focused && t.IsEnabled() {
		// Focused fill uses the fill style from scheme
		fillStyle = scheme.GetFocusedEditBoxFill()
		// Text uses the text style from scheme
		s = scheme.GetFocusedEditBoxText()
	}
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, fillChar, fillStyle)

	// Get display text
	var displayText []rune
	isPlaceholder := false
	if len(t.text) == 0 && !focused && t.placeholder != "" {
		displayText = []rune(t.placeholder)
		s = s.WithAttrs(style.StyleDim)
		isPlaceholder = true
	} else {
		displayText = t.getDisplayText()
	}

	// Apply scroll offset
	if t.scrollOffset > 0 && t.scrollOffset < len(displayText) {
		displayText = displayText[t.scrollOffset:]
	} else if t.scrollOffset >= len(displayText) {
		displayText = nil
	}

	// Truncate to visible width using font metrics
	visibleText := t.truncateToWidth(displayText, bounds.Width, font)
	displayText = []rune(visibleText)

	// Draw text using font-aware rendering
	// For text without selection, use DrawText for proper font rendering
	if !t.HasSelection() || isPlaceholder {
		p.DrawText(0, 0, string(displayText), s, font)
	} else {
		// With selection, draw character by character to apply selection style
		x := core.Unit(0)
		start, end := t.selStart, t.selEnd
		if start > end {
			start, end = end, start
		}

		selStyle := scheme.GetEditBoxSelection(focused && t.IsEnabled(), paneType)
		for i, r := range displayText {
			charStyle := s
			textPos := t.scrollOffset + i
			if textPos >= start && textPos < end {
				charStyle = selStyle
			}

			// Draw single character using DrawText for font rendering
			p.DrawText(x, 0, string(r), charStyle, font)
			x += font.MeasureText(string(r))
		}

		// The selection can extend past the last visible glyph while
		// more selected text is scrolled off the right (proportional
		// fonts leave a sliver of the box unfilled where a cell grid
		// would not). Color that trailing gap as selection so the
		// highlight reaches the edge. No-op on cell surfaces, where
		// the text fills the box exactly.
		lastVisiblePos := t.scrollOffset + len(displayText)
		if x < bounds.Width && start <= lastVisiblePos && end > lastVisiblePos {
			p.FillRect(core.UnitRect{X: x, Width: bounds.Width - x, Height: bounds.Height}, ' ', selStyle)
		}
	}

	// Draw cursor - cursor position is still cell-based for consistency.
	// Only in the active window chain: a widget keeps local focus while
	// its window is in the background, but showing the caret there
	// would put two carets on screen.
	if focused && !t.readOnly && core.FocusChainActive(t.Self()) {
		// Calculate cursor X position based on font metrics of text before cursor
		cursorTextPos := t.cursorPos - t.scrollOffset
		if cursorTextPos < 0 {
			cursorTextPos = 0
		}
		var cursorX core.Unit
		if cursorTextPos > 0 && cursorTextPos <= len(displayText) {
			cursorX = font.MeasureText(string(displayText[:cursorTextPos]))
		}

		if cursorX >= 0 && cursorX < bounds.Width {
			// The graphical bar caret uses a brighter white than the cell
			// block cursor, for contrast; the block fallback keeps the
			// regular (silver) white.
			cursorStyle := scheme.GetFocusedEditBoxCursor()
			barStyle := scheme.GetFocusedEditBoxBarCursor()
			// The graphical bar caret blinks (keystrokes restart the
			// phase); the cell-surface block stays steady.
			if p.Graphical() {
				t.ensureCaretTimer()
			}
			if !p.Graphical() || t.caretVisible() {
				// Pixel surfaces draw a vertical bar at the left edge
				// of the glyph box; cell surfaces fall back to the block.
				if !p.DrawCaret(cursorX, 0, font.LineHeight(), barStyle) {
					var cursorChar rune = ' '
					if t.cursorPos < len(t.getDisplayText()) {
						cursorChar = t.getDisplayText()[t.cursorPos]
					}
					// Draw cursor character using DrawText for consistency
					p.DrawText(cursorX, 0, string(cursorChar), cursorStyle, font)
				}
			}
		}
	}
}

// caretVisible reports the blink state: visible whenever no blink
// timer is running (cell surfaces, detached widgets).
func (t *TextInput) caretVisible() bool {
	return t.caretTimer == nil || t.caretOn
}

// ensureCaretTimer starts the ~2Hz blink cycle when the widget can
// reach a desktop timer source.
func (t *TextInput) ensureCaretTimer() {
	if t.caretTimer != nil {
		return
	}
	d := findDesktopFor(t)
	if d == nil {
		return
	}
	t.caretOn = true
	t.caretTimer = d.StartRepeatingTimer(500*time.Millisecond, func() {
		t.caretOn = !t.caretOn
		t.Update()
	})
}

func (t *TextInput) stopCaretTimer() {
	if t.caretTimer != nil {
		t.caretTimer.Stop()
		t.caretTimer = nil
	}
	t.caretOn = true
}

// resetCaretBlink restarts the blink phase with the caret visible -
// typing never happens behind an invisible caret.
func (t *TextInput) resetCaretBlink() {
	if t.caretTimer == nil {
		return
	}
	t.stopCaretTimer()
	t.ensureCaretTimer()
}

// getDisplayText returns the text with echo mode applied.
func (t *TextInput) getDisplayText() []rune {
	switch t.echoMode {
	case EchoPassword:
		result := make([]rune, len(t.text))
		for i := range result {
			result[i] = '•'
		}
		return result
	case EchoNoEcho:
		return nil
	default:
		return t.text
	}
}

// truncateToWidth truncates text to fit within the given width using font metrics.
func (t *TextInput) truncateToWidth(text []rune, maxWidth core.Unit, font *core.Font) string {
	if len(text) == 0 {
		return ""
	}

	// Find how many characters fit within maxWidth
	result := make([]rune, 0, len(text))
	var totalWidth core.Unit
	for _, r := range text {
		charWidth := font.MeasureText(string(r))
		if totalWidth+charWidth > maxWidth {
			break
		}
		result = append(result, r)
		totalWidth += charWidth
	}
	return string(result)
}

// findCharAtX finds the character index at the given X position using font metrics.
func (t *TextInput) findCharAtX(x core.Unit, font *core.Font) int {
	displayText := t.getDisplayText()
	if t.scrollOffset > 0 && t.scrollOffset < len(displayText) {
		displayText = displayText[t.scrollOffset:]
	} else if t.scrollOffset >= len(displayText) {
		return t.scrollOffset
	}

	var accumulatedWidth core.Unit
	for i, r := range displayText {
		charWidth := font.MeasureText(string(r))
		// Check if x is within this character's bounds
		if x < accumulatedWidth+charWidth/2 {
			return t.scrollOffset + i
		}
		accumulatedWidth += charWidth
	}
	// x is past all characters
	return t.scrollOffset + len(displayText)
}

// HandleKeyPress handles keyboard input.
func (t *TextInput) HandleKeyPress(event core.KeyPressEvent) bool {
	// Any keystroke makes the caret immediately visible.
	t.resetCaretBlink()

	// Both backends deliver navigation keys with their "S-" prefix
	// intact (e.g. "S-Left") alongside the parsed modifier. Fold the
	// prefix into the bare name so shift-extends the selection; the
	// caret-anchor logic in each case reads Modifiers. Control/Meta
	// spellings ("^A", "C-S-a") stay literal - they are matched whole.
	key := event.Key
	switch key {
	case "S-Left", "S-Right", "S-Home", "S-End", "S-Up", "S-Down":
		event.Modifiers |= core.ShiftModifier
		key = key[2:]
	}

	// Handle special keys
	switch key {
	case "Left":
		if t.cursorPos > 0 {
			t.cursorPos--
			if event.Modifiers&core.ShiftModifier == 0 {
				t.selStart = t.cursorPos
				t.selEnd = t.cursorPos
			} else {
				t.selEnd = t.cursorPos
			}
			t.ensureCursorVisible()
			t.Update()
		}
		return true

	case "Right":
		if t.cursorPos < len(t.text) {
			t.cursorPos++
			if event.Modifiers&core.ShiftModifier == 0 {
				t.selStart = t.cursorPos
				t.selEnd = t.cursorPos
			} else {
				t.selEnd = t.cursorPos
			}
			t.ensureCursorVisible()
			t.Update()
		}
		return true

	case "Home":
		t.cursorPos = 0
		if event.Modifiers&core.ShiftModifier == 0 {
			t.selStart = 0
			t.selEnd = 0
		} else {
			t.selEnd = 0
		}
		t.ensureCursorVisible()
		t.Update()
		return true

	case "End":
		t.cursorPos = len(t.text)
		if event.Modifiers&core.ShiftModifier == 0 {
			t.selStart = t.cursorPos
			t.selEnd = t.cursorPos
		} else {
			t.selEnd = t.cursorPos
		}
		t.ensureCursorVisible()
		t.Update()
		return true

	case "Backspace":
		t.backspace()
		return true

	case "Delete":
		t.delete()
		return true

	case "Enter":
		if t.onReturnPressed != nil {
			t.onReturnPressed()
		}
		return true

	case "^U":
		// Clear line
		t.text = nil
		t.cursorPos = 0
		t.selStart = 0
		t.selEnd = 0
		t.scrollOffset = 0
		t.textChanged()
		return true

	case "M-a":
		// Select all (Meta+A)
		t.SelectAll()
		return true

	case "^A":
		// Go to beginning (Emacs binding)
		t.cursorPos = 0
		if event.Modifiers&core.ShiftModifier == 0 {
			t.selStart = 0
			t.selEnd = 0
		} else {
			t.selEnd = 0
		}
		t.ensureCursorVisible()
		t.Update()
		return true

	case "^E":
		// Go to end (Emacs binding)
		t.cursorPos = len(t.text)
		if event.Modifiers&core.ShiftModifier == 0 {
			t.selStart = t.cursorPos
			t.selEnd = t.cursorPos
		} else {
			t.selEnd = t.cursorPos
		}
		t.ensureCursorVisible()
		t.Update()
		return true

	case "C-S-a", "C-S-A":
		// Shift+Ctrl+A: extend the selection to the beginning (the
		// anchor is wherever the caret was when the selection began).
		t.cursorPos = 0
		t.selEnd = 0
		t.ensureCursorVisible()
		t.Update()
		return true

	case "C-S-e", "C-S-E":
		// Shift+Ctrl+E: extend the selection to the end.
		t.cursorPos = len(t.text)
		t.selEnd = t.cursorPos
		t.ensureCursorVisible()
		t.Update()
		return true
	}

	// Handle printable characters
	if event.Text != "" && utf8.RuneCountInString(event.Text) == 1 {
		t.insert(event.Text)
		return true
	}

	return false
}

// HandleMousePress handles mouse clicks.
func (t *TextInput) HandleMousePress(event core.MousePressEvent) bool {
	if event.Button == core.LeftButton {
		font := t.EffectiveFont()
		pos := t.findCharAtX(event.X, font)
		if pos > len(t.text) {
			pos = len(t.text)
		}
		if event.Modifiers&core.ShiftModifier != 0 {
			// Shift+click extends: the previous caret position is
			// (already) the anchor; only the moving end follows.
			t.cursorPos = pos
			t.selEnd = pos
		} else {
			t.cursorPos = pos
			t.selStart = pos
			t.selEnd = pos
		}
		t.selecting = true
		t.SetFocus()
		// A click that repositions the caret shows it immediately.
		t.resetCaretBlink()
		t.Update()
		return true
	}
	if event.Button == core.RightButton {
		t.SetFocus()
		t.showContextMenu(event)
		return true
	}
	return false
}

// HandleMouseMove extends the selection while the button is held.
func (t *TextInput) HandleMouseMove(event core.MouseMoveEvent) bool {
	if !t.selecting || event.Buttons&core.LeftButton == 0 {
		return false
	}
	font := t.EffectiveFont()
	pos := t.findCharAtX(event.X, font)
	if pos > len(t.text) {
		pos = len(t.text)
	}
	if pos != t.cursorPos {
		t.cursorPos = pos
		t.selEnd = pos
		t.ensureCursorVisible()
		// Keep the caret visible as it tracks the drag.
		t.resetCaretBlink()
		t.Update()
	}
	return true
}

// HandleMouseRelease ends a drag selection.
func (t *TextInput) HandleMouseRelease(event core.MouseReleaseEvent) bool {
	if t.selecting {
		t.selecting = false
		return true
	}
	return false
}

// HandleFocusIn is called when focus is gained.
func (t *TextInput) HandleFocusIn() {
	t.Update()
}

// HandleFocusOut is called when focus is lost.
func (t *TextInput) HandleFocusOut() {
	t.stopCaretTimer()
	t.selecting = false
	// The selection survives - it shows in the resting selection
	// colors until the box is edited again.
	t.Update()
}

// AccessibleInfo returns accessibility information.
func (t *TextInput) AccessibleInfo() core.AccessibleInfo {
	info := t.AccessibleWidget.AccessibleInfo()
	if t.echoMode == EchoPassword {
		info.Role = core.RolePasswordInput
	} else {
		info.Role = core.RoleTextInput
	}
	info.Value = string(t.text)
	if t.readOnly {
		info.State |= core.StateReadOnly
	}
	if !t.IsEnabled() {
		info.State |= core.StateDisabled
	}
	return info
}

// ---------------------------------------------------------------
// Clipboard actions + context menu
// ---------------------------------------------------------------

// clipboardAccess finds the clipboard for this widget: the desktop
// when the widget lives in one, otherwise the popup controller (a
// torn-off window's host bridges the platform clipboard).
func (t *TextInput) clipboardAccess() (get func() string, set func(string)) {
	if d := findDesktopFor(t); d != nil {
		return d.Clipboard, d.SetClipboard
	}
	type clipper interface {
		Clipboard() string
		SetClipboard(string)
	}
	if c, ok := t.PopupController().(clipper); ok {
		return c.Clipboard, c.SetClipboard
	}
	return nil, nil
}

// Copy puts the selected text on the clipboard.
func (t *TextInput) Copy() {
	sel := t.SelectedText()
	if sel == "" {
		return
	}
	if _, set := t.clipboardAccess(); set != nil {
		set(sel)
	}
}

// Cut copies the selected text to the clipboard and removes it.
func (t *TextInput) Cut() {
	if t.readOnly || !t.HasSelection() {
		return
	}
	t.Copy()
	t.deleteSelection()
	t.textChanged()
}

// Paste inserts the clipboard at the caret, replacing any selection.
// A single-line input flattens newlines to spaces.
func (t *TextInput) Paste() {
	if t.readOnly {
		return
	}
	get, _ := t.clipboardAccess()
	if get == nil {
		return
	}
	s := get()
	if s == "" {
		return
	}
	flat := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '\n' || r == '\r' {
			r = ' '
		}
		flat = append(flat, r)
	}
	t.insert(string(flat))
}

// contextMenuID names this input's popup uniquely.
func (t *TextInput) contextMenuID() string {
	return fmt.Sprintf("textinput-menu-%d", t.ObjectID())
}

// contextMenuItems builds the right-click menu, each item equivalent
// to the matching Edit-menu action.
func (t *TextInput) contextMenuItems() []termMenuItem {
	return []termMenuItem{
		{label: "Cut", action: t.Cut},
		{label: "Copy", action: t.Copy},
		{label: "Paste", action: t.Paste},
		{separator: true},
		{label: "Select All", action: t.SelectAll},
	}
}

// showContextMenu opens the right-click menu as a popup overlay,
// using the same presentation as PurfecTerm's terminal menu.
func (t *TextInput) showContextMenu(event core.MousePressEvent) {
	pc := t.PopupController()
	if pc == nil {
		return
	}
	items := t.contextMenuItems()
	height := core.Unit(0)
	for _, it := range items {
		if it.separator {
			height += 4
		} else {
			height += gfxMenuItemHeight
		}
	}
	height += 4 // padding
	at := pc.MapToScreen(t.Self(), core.UnitPoint{X: event.X, Y: event.Y})
	screen := pc.ScreenBounds()
	if at.X+gfxMenuWidth > screen.X+screen.Width {
		at.X = screen.X + screen.Width - gfxMenuWidth
	}
	if at.Y+height > screen.Y+screen.Height {
		at.Y = screen.Y + screen.Height - height
	}
	menuBounds := core.UnitRect{X: at.X, Y: at.Y, Width: gfxMenuWidth, Height: height}
	t.menuHover = -1

	itemAt := func(y core.Unit) int {
		pos := core.Unit(2)
		for i, it := range items {
			h := gfxMenuItemHeight
			if it.separator {
				h = 4
			}
			if y >= pos && y < pos+h {
				if it.separator {
					return -1
				}
				return i
			}
			pos += h
		}
		return -1
	}

	pc.RegisterPopup(&core.PopupRequest{
		ID:     t.contextMenuID(),
		Bounds: menuBounds,
		Paint: func(p *core.Painter) {
			bg := style.DefaultStyle().WithFg(style.RGB(32, 32, 32)).WithBg(style.RGB(238, 238, 238))
			hover := style.DefaultStyle().WithFg(style.RGB(255, 255, 255)).WithBg(style.RGB(56, 120, 220))
			p.FillRect(core.UnitRect{X: menuBounds.X, Y: menuBounds.Y, Width: menuBounds.Width, Height: menuBounds.Height}, ' ', bg)
			pos := menuBounds.Y + 2
			for i, it := range items {
				if it.separator {
					p.FillRect(core.UnitRect{X: menuBounds.X + 4, Y: pos + 2, Width: menuBounds.Width - 8, Height: 1}, ' ',
						style.DefaultStyle().WithBg(style.RGB(200, 200, 200)))
					pos += 4
					continue
				}
				st := bg
				if i == t.menuHover {
					st = hover
					p.FillRect(core.UnitRect{X: menuBounds.X, Y: pos, Width: menuBounds.Width, Height: gfxMenuItemHeight}, ' ', st)
				}
				p.DrawText(menuBounds.X+8, pos, it.label, st.WithBg(style.ColorTransparent), nil)
				pos += gfxMenuItemHeight
			}
		},
		HandleMouseMove: func(event core.MouseMoveEvent) bool {
			if !menuBounds.Contains(core.UnitPoint{X: event.X, Y: event.Y}) {
				return false
			}
			idx := itemAt(event.Y - menuBounds.Y)
			if idx != t.menuHover {
				t.menuHover = idx
				t.Update()
			}
			return true
		},
		HandleMousePress: func(event core.MousePressEvent) bool {
			idx := itemAt(event.Y - menuBounds.Y)
			pc.UnregisterPopup(t.contextMenuID())
			if idx >= 0 && items[idx].action != nil {
				items[idx].action()
			}
			t.Update()
			return true
		},
	})
	t.Update()
}
