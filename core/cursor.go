package core

// CursorShape identifies a system mouse cursor. The platform maps these
// to its native cursors; targets without cursor support keep the arrow.
type CursorShape int

const (
	// CursorDefault is the ordinary arrow pointer.
	CursorDefault CursorShape = iota
	// CursorText is the text I-beam (hovering an editable text control).
	CursorText
	// CursorResizeH is the horizontal (west-east) resize cursor.
	CursorResizeH
	// CursorResizeV is the vertical (north-south) resize cursor.
	CursorResizeV
	// CursorResizeNWSE is the top-left/bottom-right diagonal resize cursor.
	CursorResizeNWSE
	// CursorResizeNESW is the top-right/bottom-left diagonal resize cursor.
	CursorResizeNESW
)

// CursorProvider is an optional trinket capability: a trinket that wants a
// particular mouse cursor while the pointer is over it (a text field
// returns CursorText, for example). The desktop resolves it on mouse
// move, after resize-edge cursors take precedence.
type CursorProvider interface {
	CursorShape() CursorShape
}
