// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
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
	cursorPos   int
	selStart    int
	selEnd      int
	scrollOffset int

	// Callbacks
	onTextChanged func(text string)
	onReturnPressed func()
}

// EchoMode controls how text is displayed.
type EchoMode int

const (
	EchoNormal       EchoMode = iota // Show text normally
	EchoPassword                     // Show bullets/asterisks
	EchoPasswordOnEdit               // Show char briefly, then bullet
	EchoNoEcho                       // Show nothing
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

		for i, r := range displayText {
			charStyle := s
			textPos := t.scrollOffset + i
			if textPos >= start && textPos < end {
				charStyle = scheme.GetSelection()
			}

			// Draw single character using DrawText for font rendering
			p.DrawText(x, 0, string(r), charStyle, font)
			x += font.MeasureText(string(r))
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
			cursorStyle := scheme.GetFocusedEditBoxCursor()
			// Pixel surfaces draw a vertical bar at the left edge of
			// the glyph box; cell surfaces fall back to the block.
			if !p.DrawCaret(cursorX, 0, font.LineHeight(), cursorStyle) {
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
	// Handle special keys
	switch event.Key {
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
		t.cursorPos = t.findCharAtX(event.X, font)
		if t.cursorPos > len(t.text) {
			t.cursorPos = len(t.text)
		}
		t.selStart = t.cursorPos
		t.selEnd = t.cursorPos
		t.SetFocus()
		t.Update()
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
	t.ClearSelection()
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
